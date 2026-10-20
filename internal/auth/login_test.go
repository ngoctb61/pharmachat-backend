package auth

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ngoctb61/pharmachat-backend/internal/platform/db"
	"github.com/ngoctb61/pharmachat-backend/internal/platform/dbtest"
)

var env *dbtest.Env

func TestMain(m *testing.M) { os.Exit(dbtest.Run(m, &env)) }

const testPassword = "correct-horse-battery"

type fixture struct {
	tenantID, userID uuid.UUID
	email            string
}

// newUser creates a tenant and a user with the given roles, as the schema owner.
func newUser(t *testing.T, roles ...string) fixture {
	t.Helper()
	ctx := context.Background()
	slug := "t-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:20]
	f := fixture{email: slug + "@example.com"}
	require.NoError(t, env.Owner.QueryRow(ctx,
		"INSERT INTO tenants (slug, display_name) VALUES ($1, $1) RETURNING id", slug).Scan(&f.tenantID))
	hash, err := HashPassword(testPassword)
	require.NoError(t, err)
	require.NoError(t, env.Owner.QueryRow(ctx,
		"INSERT INTO users (tenant_id, display_name, email, password_hash) VALUES ($1, 'Nguoi dung', $2, $3) RETURNING id",
		f.tenantID, f.email, hash).Scan(&f.userID))
	for _, r := range roles {
		var roleID uuid.UUID
		require.NoError(t, env.Owner.QueryRow(ctx,
			"INSERT INTO roles (tenant_id, name) VALUES ($1, $2) RETURNING id", f.tenantID, r).Scan(&roleID))
		_, err := env.Owner.Exec(ctx,
			"INSERT INTO user_roles (tenant_id, user_id, role_id) VALUES ($1, $2, $3)", f.tenantID, f.userID, roleID)
		require.NoError(t, err)
	}
	return f
}

func newService(t *testing.T) *Service {
	t.Helper()
	ti, err := NewTokenIssuer(testKey, time.Now)
	require.NoError(t, err)
	svc, err := NewService(env.App, ti)
	require.NoError(t, err)
	return svc
}

type userState struct {
	failed      int
	lockoutEnd  *time.Time
	lastLoginAt *time.Time
}

func readState(t *testing.T, userID uuid.UUID) userState {
	t.Helper()
	var s userState
	require.NoError(t, env.Owner.QueryRow(context.Background(),
		"SELECT access_failed_count, lockout_end, last_login_at FROM users WHERE id = $1", userID).
		Scan(&s.failed, &s.lockoutEnd, &s.lastLoginAt))
	return s
}

func ownerExec(t *testing.T, sql string, args ...any) {
	t.Helper()
	_, err := env.Owner.Exec(context.Background(), sql, args...)
	require.NoError(t, err)
}

func TestLogin_Success(t *testing.T) {
	dbtest.Require(t, env)
	f := newUser(t, "Sale", "Admin")
	svc := newService(t)

	res, err := svc.Login(context.Background(), f.email, testPassword)
	require.NoError(t, err)

	assert.Equal(t, f.userID, res.User.ID)
	assert.Equal(t, f.tenantID, res.User.TenantID)
	assert.Equal(t, f.email, res.User.Email)
	assert.Equal(t, "Nguoi dung", res.User.DisplayName)
	assert.Equal(t, []string{"Admin", "Sale"}, res.User.Roles)

	claims, err := svc.tokens.Parse(res.Token)
	require.NoError(t, err)
	assert.Equal(t, f.userID, claims.UserID)
	assert.Equal(t, f.tenantID, claims.TenantID)
	assert.Equal(t, []string{"Admin", "Sale"}, claims.Roles)

	st := readState(t, f.userID)
	assert.NotNil(t, st.lastLoginAt)
	assert.Equal(t, 0, st.failed)
}

func TestLogin_UserWithoutRoles(t *testing.T) {
	dbtest.Require(t, env)
	f := newUser(t)
	res, err := newService(t).Login(context.Background(), f.email, testPassword)
	require.NoError(t, err)
	assert.Equal(t, []string{}, res.User.Roles)
}

func TestLogin_EmailNormalized(t *testing.T) {
	dbtest.Require(t, env)
	f := newUser(t)
	_, err := newService(t).Login(context.Background(), "  "+strings.ToUpper(f.email)+" ", testPassword)
	require.NoError(t, err)
}

func TestLogin_UnknownEmailAndWrongPasswordLookAlike(t *testing.T) {
	dbtest.Require(t, env)
	f := newUser(t)
	svc := newService(t)

	_, errUnknown := svc.Login(context.Background(), "nobody-"+f.email, testPassword)
	_, errWrong := svc.Login(context.Background(), f.email, "wrong-password")

	assert.ErrorIs(t, errUnknown, ErrInvalidCredentials)
	assert.ErrorIs(t, errWrong, ErrInvalidCredentials)
	assert.Equal(t, 401, ErrInvalidCredentials.Status)
}

func TestLogin_WrongPasswordIsPersisted(t *testing.T) {
	dbtest.Require(t, env)
	f := newUser(t)
	_, err := newService(t).Login(context.Background(), f.email, "wrong-password")
	require.ErrorIs(t, err, ErrInvalidCredentials)
	assert.Equal(t, 1, readState(t, f.userID).failed)
}

func TestLogin_LocksAfterFiveFailures(t *testing.T) {
	dbtest.Require(t, env)
	f := newUser(t)
	svc := newService(t)
	ctx := context.Background()

	for i := 1; i < MaxFailedAttempts; i++ {
		_, err := svc.Login(ctx, f.email, "wrong-password")
		require.ErrorIs(t, err, ErrInvalidCredentials, "attempt %d", i)
	}
	_, err := svc.Login(ctx, f.email, "wrong-password")
	require.ErrorIs(t, err, ErrAccountLocked)
	assert.Equal(t, 403, ErrAccountLocked.Status)

	st := readState(t, f.userID)
	require.NotNil(t, st.lockoutEnd)
	assert.WithinDuration(t, time.Now().Add(LockoutDuration), *st.lockoutEnd, time.Minute)
	assert.Equal(t, 0, st.failed)

	_, err = svc.Login(ctx, f.email, testPassword)
	require.ErrorIs(t, err, ErrAccountLocked, "correct password while locked")
	_, err = svc.Login(ctx, f.email, "wrong-password")
	require.ErrorIs(t, err, ErrAccountLocked)
	assert.Equal(t, 0, readState(t, f.userID).failed, "attempts while locked are not counted")
}

func TestLogin_LockExpires(t *testing.T) {
	dbtest.Require(t, env)
	f := newUser(t)
	ownerExec(t, "UPDATE users SET lockout_end = now() - interval '1 second' WHERE id = $1", f.userID)

	_, err := newService(t).Login(context.Background(), f.email, testPassword)
	require.NoError(t, err)
	assert.Nil(t, readState(t, f.userID).lockoutEnd)
}

func TestLogin_SuccessResetsCounter(t *testing.T) {
	dbtest.Require(t, env)
	f := newUser(t)
	svc := newService(t)
	for range 3 {
		_, err := svc.Login(context.Background(), f.email, "wrong-password")
		require.ErrorIs(t, err, ErrInvalidCredentials)
	}
	require.Equal(t, 3, readState(t, f.userID).failed)

	_, err := svc.Login(context.Background(), f.email, testPassword)
	require.NoError(t, err)
	assert.Equal(t, 0, readState(t, f.userID).failed)
}

// A request that read "not locked" and is still hashing when the fifth failure
// locks the account must not get a token, and must not clear the lock.
func TestLogin_ConcurrentCannotBypassLock(t *testing.T) {
	dbtest.Require(t, env)
	f := newUser(t)
	svc := newService(t)
	ctx := context.Background()

	reached, release := make(chan struct{}), make(chan struct{})
	svc.beforeRecord = func(matched bool) {
		if matched {
			close(reached)
			<-release
		}
	}

	var wg sync.WaitGroup
	var slowErr error
	wg.Go(func() { _, slowErr = svc.Login(ctx, f.email, testPassword) })
	<-reached

	for i := 1; i <= MaxFailedAttempts; i++ {
		_, err := svc.Login(ctx, f.email, "wrong-password")
		require.Error(t, err)
	}
	require.NotNil(t, readState(t, f.userID).lockoutEnd)

	close(release)
	wg.Wait()
	assert.ErrorIs(t, slowErr, ErrAccountLocked)
	assert.NotNil(t, readState(t, f.userID).lockoutEnd, "lock must survive the in-flight success")
}

func TestLogin_BusyReturns503(t *testing.T) {
	dbtest.Require(t, env)
	f := newUser(t)
	svc := newService(t)
	svc.slotWait = 50 * time.Millisecond
	for range cap(svc.slots) {
		svc.slots <- struct{}{}
	}

	_, err := svc.Login(context.Background(), f.email, testPassword)
	assert.ErrorIs(t, err, ErrServiceBusy)
	assert.Equal(t, 503, ErrServiceBusy.Status)
}

func TestLogin_InactiveStates(t *testing.T) {
	dbtest.Require(t, env)
	cases := map[string]struct {
		setup    string
		password string
		want     error
	}{
		"disabled user":                 {"UPDATE users SET is_active = false WHERE id = $1", testPassword, ErrAccountDisabled},
		"disabled user, wrong password": {"UPDATE users SET is_active = false WHERE id = $1", "wrong-password", ErrInvalidCredentials},
		"inactive tenant": {
			"UPDATE tenants SET is_active = false WHERE id = (SELECT tenant_id FROM users WHERE id = $1)",
			testPassword, ErrTenantInactive,
		},
		"inactive tenant, wrong password": {
			"UPDATE tenants SET is_active = false WHERE id = (SELECT tenant_id FROM users WHERE id = $1)",
			"wrong-password", ErrInvalidCredentials,
		},
		"soft-deleted user": {"UPDATE users SET deleted_at = now() WHERE id = $1", testPassword, ErrInvalidCredentials},
		"soft-deleted tenant": {
			"UPDATE tenants SET deleted_at = now() WHERE id = (SELECT tenant_id FROM users WHERE id = $1)",
			testPassword, ErrInvalidCredentials,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newUser(t)
			ownerExec(t, tc.setup, f.userID)
			_, err := newService(t).Login(context.Background(), f.email, tc.password)
			assert.ErrorIs(t, err, tc.want)
		})
	}
	assert.Equal(t, 403, ErrAccountDisabled.Status)
	assert.Equal(t, 403, ErrTenantInactive.Status)
}

func TestLogin_EmailUniqueAcrossTenants(t *testing.T) {
	dbtest.Require(t, env)
	f := newUser(t)
	other := newUser(t)
	_, err := env.Owner.Exec(context.Background(),
		"INSERT INTO users (tenant_id, display_name, email, password_hash) VALUES ($1, 'x', $2, 'x')",
		other.tenantID, strings.ToUpper(f.email))
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "want a unique violation, got %v", err)
	assert.Equal(t, "23505", pgErr.Code)
}

func TestLoginCandidate_AppRoleSeesOnlyIDs(t *testing.T) {
	dbtest.Require(t, env)
	f := newUser(t)
	ctx := context.Background()

	var userID, tenantID uuid.UUID
	require.NoError(t, env.App.QueryRow(ctx,
		"SELECT user_id, tenant_id FROM auth_login_candidate($1)", " "+strings.ToUpper(f.email)).Scan(&userID, &tenantID))
	assert.Equal(t, f.userID, userID)
	assert.Equal(t, f.tenantID, tenantID)

	var n int
	require.NoError(t, env.App.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&n))
	assert.Zero(t, n, "outside WithTenant the app role still sees no users")

	err := db.WithTenant(ctx, env.App, f.tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&n)
	})
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}
