// Command pcctl is the operator CLI: schema migrations, seed data and admin accounts.
// It connects with DATABASE_ADMIN_URL (schema owner), never the service role.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/ngoctb61/pharmachat-backend/internal/platform/config"
	"github.com/ngoctb61/pharmachat-backend/internal/platform/crypto"
	"github.com/ngoctb61/pharmachat-backend/internal/platform/db"
	"github.com/ngoctb61/pharmachat-backend/internal/platform/logx"
	"github.com/ngoctb61/pharmachat-backend/internal/platform/migrate"
	"github.com/ngoctb61/pharmachat-backend/internal/seed"
)

const usage = `pcctl <command>

Commands:
  migrate up|down|status   apply, roll back one, or list migrations
  seed                     create the default tenant, roles and AI config (idempotent)
  create-admin             create an admin user
      -tenant  tenant slug (default huyen-song-khoe)
      -email   login email
      -name    display name
    Password comes from ADMIN_PASSWORD, or is prompted for on a terminal.

Environment: DATABASE_ADMIN_URL (required), ENCRYPTION_KEY, ANTHROPIC_API_KEY, OPENAI_API_KEY`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "pcctl:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Println(usage)
		return errors.New("missing command")
	}
	cfg, err := config.Load[config.Ctl]()
	if err != nil {
		return err
	}
	log := logx.New(os.Stderr, cfg.LogLevel)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.AdminDatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch args[0] {
	case "migrate":
		sub := "up"
		if len(args) > 1 {
			sub = args[1]
		}
		switch sub {
		case "up":
			return migrate.Up(ctx, pool, log)
		case "down":
			return migrate.Down(ctx, pool, log)
		case "status":
			return migrate.Status(ctx, pool, log)
		default:
			return fmt.Errorf("unknown migrate subcommand %q", sub)
		}

	case "seed":
		opts := seed.Options{
			TenantSlug: seed.DefaultTenantSlug, TenantName: seed.DefaultTenantName,
			AnthropicKey: cfg.AnthropicAPIKey, OpenAIKey: cfg.OpenAIAPIKey,
		}
		if cfg.EncryptionKey != "" {
			key, err := config.DecodeKey(cfg.EncryptionKey)
			if err != nil {
				return err
			}
			if opts.Sealer, err = crypto.NewSealer(key); err != nil {
				return err
			}
		}
		_, err := seed.Run(ctx, pool, opts, log)
		return err

	case "create-admin":
		fs := flag.NewFlagSet("create-admin", flag.ContinueOnError)
		tenant := fs.String("tenant", seed.DefaultTenantSlug, "tenant slug")
		email := fs.String("email", "", "login email")
		name := fs.String("name", "", "display name")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		password, err := readPassword()
		if err != nil {
			return err
		}
		id, err := seed.CreateAdmin(ctx, pool, seed.AdminInput{TenantSlug: *tenant, Email: *email, DisplayName: *name, Password: password})
		if err != nil {
			return err
		}
		log.InfoContext(ctx, "admin created", "user_id", id.String(), "tenant_slug", *tenant)
		return nil

	default:
		fmt.Println(usage)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func readPassword() (string, error) {
	if p := os.Getenv("ADMIN_PASSWORD"); p != "" {
		return p, nil
	}
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		fmt.Fprint(os.Stderr, "Password: ")
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("read password: %w", err)
		}
		return string(b), nil
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", errors.New("no password: set ADMIN_PASSWORD or pipe it on stdin")
	}
	return strings.TrimRight(line, "\r\n"), nil
}
