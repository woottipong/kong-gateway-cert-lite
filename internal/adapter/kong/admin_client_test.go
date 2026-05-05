package kong

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kong-cert-lite/internal/domain"
)

func TestAdminClientSkipsTLSVerificationByDefault(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Fatalf("expected root path, got %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewAdminClient(&http.Client{Timeout: 2 * time.Second})
	target := domain.KongTarget{
		AdminURL: server.URL,
	}

	if _, err := client.CheckConnection(context.Background(), target); err != nil {
		t.Fatalf("expected TLS verification to be skipped by default: %v", err)
	}
}
