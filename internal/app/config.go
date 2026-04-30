package app

import (
	"fmt"
	"os"
	"strings"

	"kong-cert-lite/internal/usecase"
)

const defaultAddr = ":8080"
const defaultDBPath = "/data/app.db"
const defaultCertDir = "/data/certs"
const defaultAccountDir = "/data/accounts"
const defaultLetsEncryptEnv = "staging"
const defaultAutoRenewCron = "0 3 * * *"

type Config struct {
	Addr            string
	DBPath          string
	CertDir         string
	AccountDir      string
	CloudflareToken string
	LetsEncryptEnv  string
	AutoRenewCron   string
}

func LoadConfig() (Config, error) {
	cfg := Config{
		Addr:            envOrDefault("APP_ADDR", defaultAddr),
		DBPath:          envOrDefault("APP_DB_PATH", defaultDBPath),
		CertDir:         envOrDefault("APP_CERT_DIR", defaultCertDir),
		AccountDir:      envOrDefault("APP_ACCOUNT_DIR", defaultAccountDir),
		CloudflareToken: strings.TrimSpace(os.Getenv("CF_DNS_API_TOKEN")),
		LetsEncryptEnv:  strings.ToLower(strings.TrimSpace(envOrDefault("LETSENCRYPT_ENV", defaultLetsEncryptEnv))),
		AutoRenewCron:   strings.TrimSpace(envOrDefault("AUTO_RENEW_CRON", defaultAutoRenewCron)),
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Addr) == "" {
		return fmt.Errorf("APP_ADDR must not be empty")
	}
	if strings.TrimSpace(c.DBPath) == "" {
		return fmt.Errorf("APP_DB_PATH must not be empty")
	}
	if strings.TrimSpace(c.CertDir) == "" {
		return fmt.Errorf("APP_CERT_DIR must not be empty")
	}
	if strings.TrimSpace(c.AccountDir) == "" {
		return fmt.Errorf("APP_ACCOUNT_DIR must not be empty")
	}
	switch c.LetsEncryptEnv {
	case "staging", "production":
	default:
		return fmt.Errorf("LETSENCRYPT_ENV must be staging or production")
	}
	if _, err := usecase.ParseCronExpression(c.AutoRenewCron); err != nil {
		return err
	}

	return nil
}

func envOrDefault(key, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	return value
}
