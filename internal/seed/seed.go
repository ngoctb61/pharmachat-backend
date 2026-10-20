// Package seed creates the bootstrap data a fresh database needs: the first tenant,
// its system roles, default AI configuration, and admin users. Every step is
// idempotent so it can run on each deploy.
package seed

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ngoctb61/pharmachat-backend/internal/auth"
	"github.com/ngoctb61/pharmachat-backend/internal/platform/crypto"
	"github.com/ngoctb61/pharmachat-backend/internal/store"
)

// Default tenant (CONTEXT.md §1).
const (
	DefaultTenantSlug = "huyen-song-khoe"
	DefaultTenantName = "Huyền Sống Khỏe"
)

// Default AI configuration (decision B-3). Prices are USD per million tokens.
const (
	llmProvider       = "anthropic"
	llmModel          = "claude-sonnet-4-6"
	llmInputPrice     = "3"
	llmOutputPrice    = "15"
	embeddingProvider = "openai"
	embeddingModel    = "text-embedding-3-large"
)

// System role names.
const (
	RoleAdmin = "Admin"
	RoleSale  = "Sale"
)

var adminPermissions = []string{
	"inbox.read", "inbox.handoff", "kb.read", "kb.write", "kb.deploy", "agent.read", "agent.manage",
	"ai_config.read", "ai_config.write", "admin.system", "admin.audit",
}

var salePermissions = []string{"inbox.read", "inbox.handoff", "kb.read", "agent.read"}

// MinPasswordLength is enforced for every account created here.
const MinPasswordLength = 12

// Options configures Run. Empty API keys skip the matching AI config.
type Options struct {
	TenantSlug   string
	TenantName   string
	AnthropicKey string
	OpenAIKey    string
	Sealer       *crypto.Sealer
}

// Result reports what Run did.
type Result struct {
	TenantID        uuid.UUID
	LLMConfig       string // "created", "exists" or "skipped: <reason>"
	EmbeddingConfig string
}

// Run seeds the tenant, roles and AI configuration inside one transaction.
func Run(ctx context.Context, pool *pgxpool.Pool, opts Options, log *slog.Logger) (Result, error) {
	var res Result
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := store.New(tx)

		n, err := q.CountPermissions(ctx, adminPermissions)
		if err != nil {
			return fmt.Errorf("check permission catalogue: %w", err)
		}
		if int(n) != len(adminPermissions) {
			return fmt.Errorf("permission catalogue has %d of %d codes; run `pcctl migrate up` first", n, len(adminPermissions))
		}

		res.TenantID, err = q.UpsertTenant(ctx, store.UpsertTenantParams{Slug: opts.TenantSlug, DisplayName: opts.TenantName})
		if err != nil {
			return fmt.Errorf("upsert tenant %q: %w", opts.TenantSlug, err)
		}

		roles := []struct {
			name, desc string
			perms      []string
		}{
			{RoleAdmin, "Quan tri he thong", adminPermissions},
			{RoleSale, "Nhan vien ban hang", salePermissions},
		}
		for _, r := range roles {
			roleID, err := q.UpsertRole(ctx, store.UpsertRoleParams{TenantID: res.TenantID, Name: r.name, Description: pgtype.Text{String: r.desc, Valid: true}})
			if err != nil {
				return fmt.Errorf("upsert role %s: %w", r.name, err)
			}
			if _, err := q.GrantRolePermissions(ctx, store.GrantRolePermissionsParams{TenantID: res.TenantID, RoleID: roleID, Codes: r.perms}); err != nil {
				return fmt.Errorf("grant permissions to role %s: %w", r.name, err)
			}
		}

		if res.LLMConfig, err = seedLLM(ctx, q, res.TenantID, opts); err != nil {
			return err
		}
		res.EmbeddingConfig, err = seedEmbedding(ctx, q, res.TenantID, opts)
		return err
	})
	if err != nil {
		return Result{}, err
	}
	log.InfoContext(ctx, "seed complete", "tenant_id", res.TenantID.String(), "tenant_slug", opts.TenantSlug,
		"llm_config", res.LLMConfig, "embedding_config", res.EmbeddingConfig)
	return res, nil
}

func seedLLM(ctx context.Context, q *store.Queries, tenantID uuid.UUID, opts Options) (string, error) {
	exists, err := q.HasActiveLLMConfig(ctx, tenantID)
	if err != nil {
		return "", fmt.Errorf("check llm config: %w", err)
	}
	switch {
	case exists:
		return "exists", nil
	case opts.AnthropicKey == "":
		return "skipped: ANTHROPIC_API_KEY not set", nil
	case opts.Sealer == nil:
		return "skipped: ENCRYPTION_KEY not set", nil
	}
	sealed, err := opts.Sealer.Seal(opts.AnthropicKey)
	if err != nil {
		return "", fmt.Errorf("encrypt anthropic key: %w", err)
	}
	var in, out pgtype.Numeric
	if err := in.Scan(llmInputPrice); err != nil {
		return "", fmt.Errorf("parse input price: %w", err)
	}
	if err := out.Scan(llmOutputPrice); err != nil {
		return "", fmt.Errorf("parse output price: %w", err)
	}
	if err := q.InsertLLMConfig(ctx, store.InsertLLMConfigParams{
		TenantID: tenantID, Provider: llmProvider, ModelID: llmModel, ApiKeyEncrypted: sealed,
		InputUsdPerMtok: in, OutputUsdPerMtok: out,
	}); err != nil {
		return "", fmt.Errorf("insert llm config: %w", err)
	}
	return "created", nil
}

func seedEmbedding(ctx context.Context, q *store.Queries, tenantID uuid.UUID, opts Options) (string, error) {
	exists, err := q.HasActiveEmbeddingConfig(ctx, tenantID)
	if err != nil {
		return "", fmt.Errorf("check embedding config: %w", err)
	}
	switch {
	case exists:
		return "exists", nil
	case opts.OpenAIKey == "":
		return "skipped: OPENAI_API_KEY not set", nil
	case opts.Sealer == nil:
		return "skipped: ENCRYPTION_KEY not set", nil
	}
	sealed, err := opts.Sealer.Seal(opts.OpenAIKey)
	if err != nil {
		return "", fmt.Errorf("encrypt openai key: %w", err)
	}
	if err := q.InsertEmbeddingConfig(ctx, store.InsertEmbeddingConfigParams{
		TenantID: tenantID, Provider: embeddingProvider, ModelID: embeddingModel, ApiKeyEncrypted: sealed,
	}); err != nil {
		return "", fmt.Errorf("insert embedding config: %w", err)
	}
	return "created", nil
}

// ErrUserExists means the email is already registered. Emails are unique across
// tenants, because login identifies the user by email alone.
var ErrUserExists = errors.New("a user with this email already exists")

// AdminInput describes the admin account to create.
type AdminInput struct {
	TenantSlug  string
	Email       string
	DisplayName string
	Password    string
}

// CreateAdmin creates a user with the Admin role. It replaces public registration,
// which the Go backend does not offer.
func CreateAdmin(ctx context.Context, pool *pgxpool.Pool, in AdminInput) (uuid.UUID, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if !strings.Contains(email, "@") {
		return uuid.Nil, errors.New("email is not valid")
	}
	if strings.TrimSpace(in.DisplayName) == "" {
		return uuid.Nil, errors.New("display name is required")
	}
	if len([]rune(in.Password)) < MinPasswordLength {
		return uuid.Nil, fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return uuid.Nil, err
	}

	var userID uuid.UUID
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := store.New(tx)
		tenantID, err := q.GetTenantIDBySlug(ctx, in.TenantSlug)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("tenant %q not found; run `pcctl seed` first", in.TenantSlug)
		}
		if err != nil {
			return fmt.Errorf("look up tenant %q: %w", in.TenantSlug, err)
		}
		roleID, err := q.GetRoleIDByName(ctx, store.GetRoleIDByNameParams{TenantID: tenantID, Name: RoleAdmin})
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("role %s missing in tenant %q; run `pcctl seed` first", RoleAdmin, in.TenantSlug)
		}
		if err != nil {
			return fmt.Errorf("look up role %s: %w", RoleAdmin, err)
		}
		userID, err = q.CreateUser(ctx, store.CreateUserParams{
			TenantID: tenantID, DisplayName: strings.TrimSpace(in.DisplayName), Email: email, PasswordHash: hash,
		})
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrUserExists
		}
		if err != nil {
			return fmt.Errorf("insert user: %w", err)
		}
		if err := q.AssignUserRole(ctx, store.AssignUserRoleParams{TenantID: tenantID, UserID: userID, RoleID: roleID}); err != nil {
			return fmt.Errorf("assign role %s: %w", RoleAdmin, err)
		}
		return nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}
