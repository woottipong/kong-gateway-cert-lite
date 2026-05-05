package kong

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"kong-cert-lite/internal/domain"
)

type AdminClient struct {
	client *http.Client
}

type certificatePayload struct {
	Cert string   `json:"cert"`
	Key  string   `json:"key"`
	SNIs []string `json:"snis,omitempty"`
	Tags []string `json:"tags,omitempty"`
}

type certificateResponse struct {
	ID string `json:"id"`
}

func NewAdminClient(client *http.Client) *AdminClient {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	configured := *client
	configured.Transport = insecureTransport(client.Transport)

	return &AdminClient{client: &configured}
}

func (c *AdminClient) CheckConnection(ctx context.Context, target domain.KongTarget) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.AdminURL, nil)
	if err != nil {
		return "", fmt.Errorf("build Kong Admin API request: %w", err)
	}
	applyAuthHeader(req, target)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Sprintf("GET %s failed: %v", target.AdminURL, err), fmt.Errorf("Kong Admin API unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if readErr != nil {
		return "", fmt.Errorf("read Kong Admin API response: %w", readErr)
	}

	detail := fmt.Sprintf("GET %s -> %s", target.AdminURL, resp.Status)
	if snippet := strings.TrimSpace(string(body)); snippet != "" {
		detail += "\n" + snippet
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return detail, fmt.Errorf("Kong Admin API returned %s", resp.Status)
	}

	return detail, nil
}

func (c *AdminClient) SyncCertificate(ctx context.Context, target domain.KongTarget, existingKongCertificateID string, certPEM string, keyPEM string, snis []string, tags []string) (string, string, error) {
	payloadBytes, err := json.Marshal(certificatePayload{
		Cert: certPEM,
		Key:  keyPEM,
		SNIs: snis,
		Tags: tags,
	})
	if err != nil {
		return "", "", fmt.Errorf("marshal Kong certificate payload: %w", err)
	}

	method := http.MethodPost
	endpoint := strings.TrimRight(target.AdminURL, "/") + "/certificates"
	if strings.TrimSpace(existingKongCertificateID) != "" {
		method = http.MethodPatch
		endpoint += "/" + existingKongCertificateID
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", "", fmt.Errorf("build Kong sync request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	applyAuthHeader(req, target)

	resp, err := c.client.Do(req)
	if err != nil {
		return existingKongCertificateID, fmt.Sprintf("%s %s failed: %v", method, endpoint, err), fmt.Errorf("Kong certificate sync failed: %w", err)
	}
	defer resp.Body.Close()

	detail := fmt.Sprintf("%s %s -> %s", method, endpoint, resp.Status)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return existingKongCertificateID, detail, fmt.Errorf("Kong certificate sync returned %s", resp.Status)
	}

	var decoded certificateResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		if errors.Is(err, io.EOF) && strings.TrimSpace(existingKongCertificateID) != "" {
			return existingKongCertificateID, detail, nil
		}
		return existingKongCertificateID, detail, fmt.Errorf("parse Kong sync response: %w", err)
	}
	if decoded.ID == "" {
		decoded.ID = existingKongCertificateID
	}
	if decoded.ID == "" {
		return "", detail, fmt.Errorf("Kong sync response missing certificate id")
	}

	return decoded.ID, detail, nil
}

func applyAuthHeader(req *http.Request, target domain.KongTarget) {
	if target.AuthType == domain.KongTargetAuthTypeCustomHeader && target.AuthHeaderName != "" {
		req.Header.Set(target.AuthHeaderName, target.AuthHeaderValue)
	}
}

func insecureTransport(base http.RoundTripper) http.RoundTripper {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if base, ok := base.(*http.Transport); ok && base != nil {
		transport = base.Clone()
	}
	if transport.TLSClientConfig != nil {
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	} else {
		transport.TLSClientConfig = &tls.Config{}
	}
	transport.TLSClientConfig.InsecureSkipVerify = true // #nosec G402 -- Kong Admin API is expected to be private.

	return transport
}
