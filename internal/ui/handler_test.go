package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mostlydev/cllama-passthrough/internal/provider"
)

func TestUIListsProviders(t *testing.T) {
	reg := provider.NewRegistry(t.TempDir())
	reg.Set("openai", &provider.Provider{Name: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "sk-test", Auth: "bearer"})
	h := NewHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "openai") {
		t.Error("expected provider name in response")
	}
}

func TestUIUpsertProvider(t *testing.T) {
	authDir := t.TempDir()
	reg := provider.NewRegistry(authDir)
	h := NewHandler(reg)

	form := url.Values{}
	form.Set("name", "openrouter")
	form.Set("base_url", "https://openrouter.ai/api/v1")
	form.Set("api_key", "sk-or-test")
	form.Set("auth", "bearer")

	req := httptest.NewRequest(http.MethodPost, "/providers", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d body=%s", w.Code, w.Body.String())
	}

	p, err := reg.Get("openrouter")
	if err != nil {
		t.Fatalf("provider not saved in memory: %v", err)
	}
	if p.APIKey != "sk-or-test" {
		t.Fatalf("expected api key saved, got %q", p.APIKey)
	}

	data, err := os.ReadFile(filepath.Join(authDir, "providers.json"))
	if err != nil {
		t.Fatalf("providers.json missing: %v", err)
	}
	if !strings.Contains(string(data), "openrouter") {
		t.Fatalf("providers.json missing provider entry: %s", string(data))
	}
}

func TestUIDeleteProvider(t *testing.T) {
	reg := provider.NewRegistry(t.TempDir())
	reg.Set("openai", &provider.Provider{Name: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "sk-test", Auth: "bearer"})
	h := NewHandler(reg)

	form := url.Values{}
	form.Set("name", "openai")
	form.Set("action", "delete")

	req := httptest.NewRequest(http.MethodPost, "/providers", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}

	_, err := reg.Get("openai")
	if err == nil {
		t.Fatalf("expected provider to be deleted")
	}
}

func TestMaskKey(t *testing.T) {
	if got := maskKey(""); got != "" {
		t.Fatalf("expected empty mask, got %q", got)
	}
	if got := maskKey("abcd"); got != "****" {
		t.Fatalf("expected short key mask, got %q", got)
	}
	if got := maskKey("sk-example-1234"); got != "sk-e...1234" {
		t.Fatalf("unexpected mask: %q", got)
	}
}

func TestNotFound(t *testing.T) {
	h := NewHandler(provider.NewRegistry(t.TempDir()))
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		b, _ := io.ReadAll(w.Result().Body)
		t.Fatalf("expected 404, got %d body=%s", w.Code, string(b))
	}
}
