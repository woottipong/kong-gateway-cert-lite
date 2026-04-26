package acme

import (
	"context"
	"crypto"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/registration"

	"kong-cert-lite/internal/usecase"
)

const (
	letsEncryptStagingURL    = "https://acme-staging-v02.api.letsencrypt.org/directory"
	letsEncryptProductionURL = "https://acme-v02.api.letsencrypt.org/directory"
)

type LegoClient struct {
	accountDir      string
	letsEncryptEnv  string
	cloudflareToken string
}

type accountUser struct {
	Email        string                 `json:"email"`
	Registration *registration.Resource `json:"registration,omitempty"`
	key          crypto.PrivateKey
}

func NewLegoClient(accountDir string, letsEncryptEnv string, cloudflareToken string) *LegoClient {
	return &LegoClient{
		accountDir:      accountDir,
		letsEncryptEnv:  letsEncryptEnv,
		cloudflareToken: strings.TrimSpace(cloudflareToken),
	}
}

func (c *LegoClient) Issue(ctx context.Context, request usecase.ACMEIssueRequest) (usecase.ACMEIssueResult, error) {
	_ = ctx

	if strings.TrimSpace(c.cloudflareToken) == "" {
		return usecase.ACMEIssueResult{}, fmt.Errorf("cloudflare dns api token is not configured")
	}
	if strings.TrimSpace(request.Email) == "" {
		return usecase.ACMEIssueResult{}, fmt.Errorf("acme account email is required")
	}
	if len(request.Domains) == 0 {
		return usecase.ACMEIssueResult{}, fmt.Errorf("at least one domain is required for issue")
	}

	user, accountPath, err := c.loadOrCreateUser(request.Email)
	if err != nil {
		return usecase.ACMEIssueResult{}, err
	}

	config := lego.NewConfig(user)
	config.CADirURL = c.caDirectoryURL()
	config.Certificate.KeyType = certcrypto.RSA2048

	client, err := lego.NewClient(config)
	if err != nil {
		return usecase.ACMEIssueResult{}, fmt.Errorf("create lego client: %w", err)
	}

	providerConfig := cloudflare.NewDefaultConfig()
	providerConfig.AuthToken = c.cloudflareToken
	providerConfig.ZoneToken = c.cloudflareToken
	provider, err := cloudflare.NewDNSProviderConfig(providerConfig)
	if err != nil {
		return usecase.ACMEIssueResult{}, fmt.Errorf("configure cloudflare dns provider: %w", err)
	}
	if err := client.Challenge.SetDNS01Provider(provider); err != nil {
		return usecase.ACMEIssueResult{}, fmt.Errorf("configure dns challenge: %w", err)
	}

	if user.Registration == nil {
		registrationResource, err := client.Registration.ResolveAccountByKey()
		if err != nil {
			registrationResource, err = client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
			if err != nil {
				return usecase.ACMEIssueResult{}, fmt.Errorf("register acme account: %w", err)
			}
		}
		user.Registration = registrationResource
		if err := saveAccount(accountPath, user); err != nil {
			return usecase.ACMEIssueResult{}, err
		}
	}

	resource, err := client.Certificate.Obtain(certificate.ObtainRequest{
		Domains: request.Domains,
		Bundle:  true,
	})
	if err != nil {
		return usecase.ACMEIssueResult{}, fmt.Errorf("issue certificate: %w", err)
	}

	return usecase.ACMEIssueResult{
		FullChainPEM:  resource.Certificate,
		PrivateKeyPEM: resource.PrivateKey,
	}, nil
}

func (c *LegoClient) caDirectoryURL() string {
	if strings.EqualFold(strings.TrimSpace(c.letsEncryptEnv), "production") {
		return letsEncryptProductionURL
	}
	return letsEncryptStagingURL
}

func (c *LegoClient) loadOrCreateUser(email string) (*accountUser, string, error) {
	baseDir := filepath.Join(c.accountDir, strings.ToLower(strings.TrimSpace(c.letsEncryptEnv)), sanitizeEmail(email))
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return nil, "", fmt.Errorf("create acme account directory %s: %w", baseDir, err)
	}

	keyPath := filepath.Join(baseDir, "account.key")
	accountPath := filepath.Join(baseDir, "account.json")

	privateKey, err := loadOrCreatePrivateKey(keyPath)
	if err != nil {
		return nil, "", err
	}

	user := &accountUser{Email: email, key: privateKey}
	if _, err := os.Stat(accountPath); err == nil {
		accountBytes, err := os.ReadFile(accountPath)
		if err != nil {
			return nil, "", fmt.Errorf("read acme account file %s: %w", accountPath, err)
		}
		if err := json.Unmarshal(accountBytes, user); err != nil {
			return nil, "", fmt.Errorf("parse acme account file %s: %w", accountPath, err)
		}
		user.key = privateKey
	}

	return user, accountPath, nil
}

func loadOrCreatePrivateKey(path string) (crypto.PrivateKey, error) {
	if _, err := os.Stat(path); err == nil {
		keyBytes, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read acme account key %s: %w", path, err)
		}
		privateKey, err := certcrypto.ParsePEMPrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("parse acme account key %s: %w", path, err)
		}
		return privateKey, nil
	}

	privateKey, err := certcrypto.GeneratePrivateKey(certcrypto.EC256)
	if err != nil {
		return nil, fmt.Errorf("generate acme account key: %w", err)
	}

	pemBlock := certcrypto.PEMBlock(privateKey)
	if pemBlock == nil {
		return nil, fmt.Errorf("encode acme account key: unsupported private key type")
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(pemBlock), 0o600); err != nil {
		return nil, fmt.Errorf("write acme account key %s: %w", path, err)
	}

	return privateKey, nil
}

func saveAccount(path string, user *accountUser) error {
	accountBytes, err := json.MarshalIndent(&accountUser{
		Email:        user.Email,
		Registration: user.Registration,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal acme account: %w", err)
	}
	if err := os.WriteFile(path, accountBytes, 0o600); err != nil {
		return fmt.Errorf("write acme account file %s: %w", path, err)
	}
	return nil
}

func sanitizeEmail(email string) string {
	replacer := strings.NewReplacer("@", "_at_", "/", "_", "\\", "_", ":", "_", " ", "_")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(email)))
}

func (u *accountUser) GetEmail() string {
	return u.Email
}

func (u *accountUser) GetRegistration() *registration.Resource {
	return u.Registration
}

func (u *accountUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}
