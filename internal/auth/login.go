package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ngoctb61/pharmachat-backend/internal/platform/db"
	"github.com/ngoctb61/pharmachat-backend/internal/platform/httpx"
	"github.com/ngoctb61/pharmachat-backend/internal/store"
)

// Lockout policy.
const (
	MaxFailedAttempts = 5
	LockoutDuration   = 15 * time.Minute
)

// Each argon2id verification takes 19 MiB and the api container has 256 MiB, so an
// unauthenticated flood of logins must queue instead of running all at once.
const (
	maxConcurrentHashes = 4
	hashSlotWait        = 2 * time.Second
)

// Login failures. A wrong password always yields ErrInvalidCredentials, so the
// other states are only revealed to someone who knows the password.
var (
	ErrInvalidCredentials = &httpx.Error{Status: http.StatusUnauthorized, Code: "INVALID_CREDENTIALS", Message: "Email hoac mat khau khong dung"}
	ErrAccountLocked      = &httpx.Error{Status: http.StatusForbidden, Code: "ACCOUNT_LOCKED", Message: "Tai khoan da bi khoa, vui long thu lai sau hoac lien he quan tri vien"}
	ErrAccountDisabled    = &httpx.Error{Status: http.StatusForbidden, Code: "ACCOUNT_DISABLED", Message: "Tai khoan da bi vo hieu hoa, vui long lien he quan tri vien"}
	ErrTenantInactive     = &httpx.Error{Status: http.StatusForbidden, Code: "TENANT_INACTIVE", Message: "Doanh nghiep da ngung hoat dong, vui long lien he quan tri vien"}
	ErrServiceBusy        = &httpx.Error{Status: http.StatusServiceUnavailable, Code: "SERVICE_BUSY", Message: "He thong dang ban, vui long thu lai sau"}
)

// User is the account returned by a successful login.
type User struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"tenantId"`
	DisplayName string    `json:"displayName"`
	Email       string    `json:"email"`
	Roles       []string  `json:"roles"`
}

// LoginResult is the body of a successful login.
type LoginResult struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// Service authenticates users.
type Service struct {
	pool      *pgxpool.Pool
	tokens    *TokenIssuer
	dummyHash string
	slots     chan struct{}
	slotWait  time.Duration
	// beforeRecord, when set by tests, runs between verification and the write.
	beforeRecord func(matched bool)
}

// NewService returns a Service on the RLS-bound app pool.
func NewService(pool *pgxpool.Pool, tokens *TokenIssuer) (*Service, error) {
	dummy, err := HashPassword("unknown-email-placeholder")
	if err != nil {
		return nil, fmt.Errorf("hash placeholder password: %w", err)
	}
	return &Service{
		pool:      pool,
		tokens:    tokens,
		dummyHash: dummy,
		slots:     make(chan struct{}, maxConcurrentHashes),
		slotWait:  hashSlotWait,
	}, nil
}

// Login checks the credentials and returns an access token.
//
// The user is read in one short transaction and the result written in another;
// argon2id runs between them so no pooled connection is held while hashing.
func (s *Service) Login(ctx context.Context, email, password string) (LoginResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	cand, err := store.New(s.pool).LoginCandidate(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return LoginResult{}, s.rejectUnknown(ctx, password)
	}
	if err != nil {
		return LoginResult{}, fmt.Errorf("look up login candidate: %w", err)
	}

	var u store.GetUserForLoginRow
	var roles []string
	err = db.WithTenant(ctx, s.pool, cand.TenantID, func(tx pgx.Tx) error {
		q := store.New(tx)
		var err error
		if u, err = q.GetUserForLogin(ctx, cand.UserID); err != nil {
			return fmt.Errorf("read user %s for login: %w", cand.UserID, err)
		}
		if roles, err = q.ListUserRoleNames(ctx, cand.UserID); err != nil {
			return fmt.Errorf("read roles of user %s: %w", cand.UserID, err)
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Deleted between the candidate lookup and the read.
		return LoginResult{}, s.rejectUnknown(ctx, password)
	}
	if err != nil {
		return LoginResult{}, err
	}
	if u.Locked {
		return LoginResult{}, ErrAccountLocked
	}

	matched, err := s.verify(ctx, password, u.PasswordHash)
	if err != nil {
		return LoginResult{}, fmt.Errorf("verify password of user %s: %w", u.ID, err)
	}
	if s.beforeRecord != nil {
		s.beforeRecord(matched)
	}
	if !matched {
		return LoginResult{}, s.recordFailure(ctx, u)
	}
	if !u.IsActive {
		return LoginResult{}, ErrAccountDisabled
	}
	if !u.TenantIsActive {
		return LoginResult{}, ErrTenantInactive
	}

	var updated int64
	err = db.WithTenant(ctx, s.pool, u.TenantID, func(tx pgx.Tx) error {
		var err error
		updated, err = store.New(tx).RecordLoginSuccess(ctx, u.ID)
		return err
	})
	if err != nil {
		return LoginResult{}, fmt.Errorf("record login of user %s: %w", u.ID, err)
	}
	if updated == 0 {
		return LoginResult{}, ErrAccountLocked
	}

	token, err := s.tokens.Issue(u.ID, u.TenantID, roles)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: token, User: User{
		ID: u.ID, TenantID: u.TenantID, DisplayName: u.DisplayName, Email: u.Email, Roles: roles,
	}}, nil
}

// rejectUnknown spends the same argon2id work as a real check, so response time
// does not reveal whether the email exists.
func (s *Service) rejectUnknown(ctx context.Context, password string) error {
	if _, err := s.verify(ctx, password, s.dummyHash); err != nil {
		return fmt.Errorf("verify placeholder password: %w", err)
	}
	return ErrInvalidCredentials
}

func (s *Service) recordFailure(ctx context.Context, u store.GetUserForLoginRow) error {
	var locked bool
	err := db.WithTenant(ctx, s.pool, u.TenantID, func(tx pgx.Tx) error {
		var err error
		locked, err = store.New(tx).RecordLoginFailure(ctx, store.RecordLoginFailureParams{
			ID: u.ID, MaxAttempts: MaxFailedAttempts, LockoutSeconds: int32(LockoutDuration / time.Second),
		})
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Another request locked the account while this one was hashing.
		return ErrAccountLocked
	}
	if err != nil {
		return fmt.Errorf("record failed login of user %s: %w", u.ID, err)
	}
	if locked {
		return ErrAccountLocked
	}
	return ErrInvalidCredentials
}

// verify runs argon2id once a hashing slot is free, or gives up with ErrServiceBusy.
func (s *Service) verify(ctx context.Context, password, hash string) (bool, error) {
	wait := time.NewTimer(s.slotWait)
	defer wait.Stop()
	select {
	case s.slots <- struct{}{}:
	case <-wait.C:
		return false, ErrServiceBusy
	case <-ctx.Done():
		return false, ctx.Err()
	}
	defer func() { <-s.slots }()
	return VerifyPassword(password, hash)
}
