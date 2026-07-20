// Package config loads runtime configuration from the environment.
// Fail fast: if a required value is missing, return an error at startup.
package config

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	AppEnv     string `env:"APP_ENV"      envDefault:"development"`
	LogLevel   string `env:"LOG_LEVEL"    envDefault:"info"`
	APIPort    int    `env:"API_PORT"     envDefault:"8080"`
	APIBaseURL string `env:"API_BASE_URL" envDefault:"http://localhost:8080"`

	DatabaseURL   string `env:"DATABASE_URL,required"`
	RedisURL      string `env:"REDIS_URL,required"`
	AsynqRedisURL string `env:"ASYNQ_REDIS_URL,required"`

	// BackupDatabaseURL is the DSN the nightly ops:backup_database task runs
	// pg_dump against (SPEC-09 P0.2). It MUST connect to Postgres DIRECTLY
	// (postgres:5432), NOT through PgBouncer — a transaction pooler breaks the
	// session semantics pg_dump needs. Empty ⇒ the backup task self-reports a
	// failed run (visible on /ops/status) rather than crashing the worker.
	BackupDatabaseURL string `env:"BACKUP_DATABASE_URL"`

	// JWT signing keys: comma-separated kid:base64-secret pairs. The first
	// is the active signer; remaining keys remain valid for verification.
	// Example: "v2:NEW_BASE64,v1:OLD_BASE64"
	JWTKeys            string        `env:"JWT_SIGNING_KEYS,required"`
	JWTIssuer          string        `env:"JWT_ISSUER"     envDefault:"portal"`
	JWTAudience        string        `env:"JWT_AUDIENCE"   envDefault:"portal-api"`
	AccessTokenTTL     time.Duration `env:"ACCESS_TOKEN_TTL"   envDefault:"5m"`
	RefreshTTL         time.Duration `env:"REFRESH_TOKEN_TTL"  envDefault:"24h"` // 1 day (remember-me window)
	PermissionCacheTTL time.Duration `env:"PERMISSION_CACHE_TTL" envDefault:"5m"`

	S3Endpoint     string `env:"S3_ENDPOINT,required"`
	S3Region       string `env:"S3_REGION"        envDefault:"us-east-1"`
	S3Bucket       string `env:"S3_BUCKET,required"`
	S3AccessKey    string `env:"S3_ACCESS_KEY,required"`
	S3SecretKey    string `env:"S3_SECRET_KEY,required"`
	S3UsePathStyle bool   `env:"S3_USE_PATH_STYLE" envDefault:"true"`

	// Login is local password auth (ADR-06); no external IdP config.

	CookieDomain string `env:"COOKIE_DOMAIN"   envDefault:""`
	CookieSecure bool   `env:"COOKIE_SECURE"   envDefault:"true"`
	PostLoginURL string `env:"POST_LOGIN_URL"  envDefault:"/"`

	// Password reset (SPEC-04 P0.3). PasswordResetURL is the reset-link base the
	// email points at; the raw token is appended as ?token=.
	PasswordResetURL string        `env:"PASSWORD_RESET_URL" envDefault:"https://portal.localhost/reset-password"`
	PasswordResetTTL time.Duration `env:"PASSWORD_RESET_TTL" envDefault:"1h"`

	// Notification email channel (SPEC-04 P0.3). Dev = Mailpit (no auth). An
	// empty SMTPHost makes the worker fall back to a log-sink sender.
	SMTPHost             string `env:"SMTP_HOST" envDefault:""`
	SMTPPort             int    `env:"SMTP_PORT" envDefault:"1025"`
	SMTPFrom             string `env:"SMTP_FROM" envDefault:"Portal <no-reply@portal.localhost>"`
	NotifyEmailHourlyCap int    `env:"NOTIFY_EMAIL_HOURLY_CAP" envDefault:"200"` // global send ceiling (budget insurance); 0 = uncapped

	// Browser origins allowed to make credentialed (cookie-bearing) calls to the
	// API. The frontend login form POSTs cross-subdomain, so the exact origin
	// must be allowlisted — a wildcard is invalid with credentials.
	CORSAllowedOrigins []string `env:"CORS_ALLOWED_ORIGINS" envSeparator:"," envDefault:"https://portal.localhost"`

	// AppTimezone is the SINGLE display-timezone knob for the whole product (IANA
	// name, e.g. "UTC" or "Asia/Ho_Chi_Minh"). The API stores timestamps in UTC;
	// this only drives how the FRONTEND renders every date (served via
	// GET /api/v1/time → frontend lib/time.ts). Change it in one place to shift the
	// whole UI's date/time reckoning.
	AppTimezone string `env:"APP_TIMEZONE" envDefault:"UTC"`
}

// SigningKey is the parsed form of one entry in JWTKeys.
type SigningKey struct {
	ID     string
	Secret []byte
}

// ParsedKeys decodes JWTKeys into ordered SigningKey entries. The first key
// is the active signer; subsequent entries are kept for verifying tokens
// issued under previous keys (rotation grace period).
func (c *Config) ParsedKeys() ([]SigningKey, error) {
	raw := strings.TrimSpace(c.JWTKeys)
	if raw == "" {
		return nil, fmt.Errorf("config: JWT_SIGNING_KEYS is empty")
	}
	parts := strings.Split(raw, ",")
	out := make([]SigningKey, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		idx := strings.IndexByte(p, ':')
		if idx <= 0 {
			return nil, fmt.Errorf("config: malformed JWT key %q (expected kid:base64)", p)
		}
		kid := p[:idx]
		secB64 := p[idx+1:]
		secret, err := base64.StdEncoding.DecodeString(secB64)
		if err != nil {
			// Try URL-safe base64 as a convenience.
			secret, err = base64.RawURLEncoding.DecodeString(secB64)
			if err != nil {
				return nil, fmt.Errorf("config: JWT key %q: invalid base64", kid)
			}
		}
		if len(secret) < 32 {
			return nil, fmt.Errorf("config: JWT key %q: secret must be ≥32 bytes", kid)
		}
		out = append(out, SigningKey{ID: kid, Secret: secret})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("config: no usable JWT keys")
	}
	return out, nil
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}
