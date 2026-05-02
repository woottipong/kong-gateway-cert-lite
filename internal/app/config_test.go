package app

import (
	"os"
	"testing"
)

func TestLoadConfigUsesDefaultAddrWhenEnvMissing(t *testing.T) {
	t.Setenv("APP_USERNAME", "")
	t.Setenv("APP_PASSWORD", "")
	previous, hadPrevious := os.LookupEnv("APP_ADDR")
	if err := os.Unsetenv("APP_ADDR"); err != nil {
		t.Fatalf("unset APP_ADDR: %v", err)
	}
	t.Cleanup(func() {
		if hadPrevious {
			_ = os.Setenv("APP_ADDR", previous)
			return
		}
		_ = os.Unsetenv("APP_ADDR")
	})

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected default config to load: %v", err)
	}
	if cfg.Addr != defaultAddr {
		t.Fatalf("expected default addr %q, got %q", defaultAddr, cfg.Addr)
	}
	if cfg.DBPath != defaultDBPath {
		t.Fatalf("expected default db path %q, got %q", defaultDBPath, cfg.DBPath)
	}
	if cfg.CertDir != defaultCertDir {
		t.Fatalf("expected default cert dir %q, got %q", defaultCertDir, cfg.CertDir)
	}
	if cfg.AccountDir != defaultAccountDir {
		t.Fatalf("expected default account dir %q, got %q", defaultAccountDir, cfg.AccountDir)
	}
	if cfg.LetsEncryptEnv != defaultLetsEncryptEnv {
		t.Fatalf("expected default letsencrypt env %q, got %q", defaultLetsEncryptEnv, cfg.LetsEncryptEnv)
	}
	if cfg.AutoRenewCron != defaultAutoRenewCron {
		t.Fatalf("expected default auto renew cron %q, got %q", defaultAutoRenewCron, cfg.AutoRenewCron)
	}
	if cfg.Username != "" {
		t.Fatalf("expected default username to be empty, got %q", cfg.Username)
	}
	if cfg.Password != "" {
		t.Fatal("expected default password to be empty")
	}
}

func TestLoadConfigRejectsEmptyAddr(t *testing.T) {
	t.Setenv("APP_ADDR", "")

	cfg, err := LoadConfig()
	if err == nil {
		t.Fatal("expected empty APP_ADDR to fail validation")
	}
	if cfg != (Config{}) {
		t.Fatalf("expected zero config on validation error, got %#v", cfg)
	}
}

func TestLoadConfigReadsAddrFromEnv(t *testing.T) {
	t.Setenv("APP_ADDR", "127.0.0.1:9090")
	t.Setenv("APP_DB_PATH", "/tmp/kong-cert-lite-test.db")
	t.Setenv("APP_USERNAME", "operator")
	t.Setenv("APP_PASSWORD", "secret")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected config to load: %v", err)
	}
	if cfg.Addr != "127.0.0.1:9090" {
		t.Fatalf("expected APP_ADDR override, got %q", cfg.Addr)
	}
	if cfg.DBPath != "/tmp/kong-cert-lite-test.db" {
		t.Fatalf("expected APP_DB_PATH override, got %q", cfg.DBPath)
	}
	if cfg.CertDir != defaultCertDir {
		t.Fatalf("expected default cert dir when env missing, got %q", cfg.CertDir)
	}
	if cfg.AccountDir != defaultAccountDir {
		t.Fatalf("expected default account dir when env missing, got %q", cfg.AccountDir)
	}
	if cfg.Username != "operator" {
		t.Fatalf("expected APP_USERNAME override, got %q", cfg.Username)
	}
	if cfg.Password != "secret" {
		t.Fatal("expected APP_PASSWORD override")
	}
	if cfg.DiscordWebhookURL != "" {
		t.Fatalf("expected default discord webhook to be empty, got %q", cfg.DiscordWebhookURL)
	}
	if cfg.DiscordNotifySuccess {
		t.Fatal("expected default discord success notification to be false")
	}
}

func TestLoadConfigReadsDiscordNotificationEnv(t *testing.T) {
	t.Setenv("DISCORD_WEBHOOK_URL", "https://discord.com/api/webhooks/example")
	t.Setenv("DISCORD_NOTIFY_SUCCESS", "true")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected config to load: %v", err)
	}
	if cfg.DiscordWebhookURL != "https://discord.com/api/webhooks/example" {
		t.Fatalf("expected discord webhook override, got %q", cfg.DiscordWebhookURL)
	}
	if !cfg.DiscordNotifySuccess {
		t.Fatal("expected discord success notifications enabled")
	}
}

func TestConfigValidateRejectsEmptyAddr(t *testing.T) {
	cfg := Config{Addr: " ", DBPath: "/tmp/app.db"}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected empty addr validation error")
	}
}

func TestConfigValidateRejectsEmptyDBPath(t *testing.T) {
	cfg := Config{Addr: ":8080", DBPath: " "}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected empty db path validation error")
	}
}

func TestConfigValidateRejectsEmptyCertDir(t *testing.T) {
	cfg := Config{Addr: ":8080", DBPath: "/tmp/app.db", CertDir: " ", AccountDir: "/tmp/accounts", LetsEncryptEnv: "staging"}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected empty cert dir validation error")
	}
}

func TestConfigValidateRejectsEmptyAccountDir(t *testing.T) {
	cfg := Config{Addr: ":8080", DBPath: "/tmp/app.db", CertDir: "/tmp/certs", AccountDir: " ", LetsEncryptEnv: "staging"}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected empty account dir validation error")
	}
}

func TestConfigValidateRejectsInvalidLetsEncryptEnv(t *testing.T) {
	cfg := Config{Addr: ":8080", DBPath: "/tmp/app.db", CertDir: "/tmp/certs", AccountDir: "/tmp/accounts", LetsEncryptEnv: "qa"}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid letsencrypt env validation error")
	}
}

func TestConfigValidateRejectsPartialBasicAuth(t *testing.T) {
	base := Config{
		Addr:           ":8080",
		DBPath:         "/tmp/app.db",
		CertDir:        "/tmp/certs",
		AccountDir:     "/tmp/accounts",
		LetsEncryptEnv: "staging",
		AutoRenewCron:  defaultAutoRenewCron,
	}

	cfg := base
	cfg.Username = "operator"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected username without password validation error")
	}

	cfg = base
	cfg.Password = "secret"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected password without username validation error")
	}
}

func TestConfigValidateRejectsInvalidAutoRenewCron(t *testing.T) {
	cfg := Config{
		Addr:           ":8080",
		DBPath:         "/tmp/app.db",
		CertDir:        "/tmp/certs",
		AccountDir:     "/tmp/accounts",
		LetsEncryptEnv: "staging",
		AutoRenewCron:  "not a cron",
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid auto renew cron validation error")
	}
}
