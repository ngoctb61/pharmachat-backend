// Package config loads service configuration from environment variables only.
// Secrets never live in files committed to the repository.
package config

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// Common holds settings shared by every binary.
type Common struct {
	Env      string `env:"APP_ENV" envDefault:"development"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`
}

// API configures cmd/api.
type API struct {
	Common
	HTTPAddr        string        `env:"API_HTTP_ADDR" envDefault:":8080"`
	DatabaseURL     string        `env:"DATABASE_URL,notEmpty"`
	JWTSigningKey   string        `env:"JWT_SIGNING_KEY,notEmpty"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"15s"`
}

// Agent configures cmd/agent.
type Agent struct {
	Common
	GRPCAddr        string        `env:"AGENT_GRPC_LISTEN" envDefault:":9090"`
	HTTPAddr        string        `env:"AGENT_HTTP_ADDR" envDefault:":8081"`
	DatabaseURL     string        `env:"DATABASE_URL,notEmpty"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"15s"`
}

// Ctl configures cmd/pcctl. It connects as the schema owner, which bypasses RLS.
type Ctl struct {
	Common
	AdminDatabaseURL string `env:"DATABASE_ADMIN_URL,notEmpty"`
	EncryptionKey    string `env:"ENCRYPTION_KEY"`
	AnthropicAPIKey  string `env:"ANTHROPIC_API_KEY"`
	OpenAIAPIKey     string `env:"OPENAI_API_KEY"`
}

// Load parses environment variables into a config struct.
func Load[T any]() (T, error) {
	var cfg T
	if err := env.Parse(&cfg); err != nil {
		return cfg, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

// DecodeKey decodes a base64 AES-256 key and checks its length.
func DecodeKey(b64 string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("ENCRYPTION_KEY is not valid base64: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must decode to 32 bytes, got %d", len(key))
	}
	return key, nil
}
