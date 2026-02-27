package provider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryFromEnv(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test-openai")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")

	r := NewRegistry("")
	r.LoadFromEnv()

	p, err := r.Get("openai")
	if err != nil {
		t.Fatalf("openai: %v", err)
	}
	if p.APIKey != "sk-test-openai" {
		t.Errorf("expected openai key, got %q", p.APIKey)
	}
	if p.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("unexpected openai base URL: %q", p.BaseURL)
	}
	if p.Auth != "bearer" {
		t.Errorf("expected openai auth=bearer, got %q", p.Auth)
	}
	if p.APIFormat != "openai" {
		t.Errorf("expected openai api_format=openai, got %q", p.APIFormat)
	}

	p, err = r.Get("anthropic")
	if err != nil {
		t.Fatalf("anthropic: %v", err)
	}
	if p.APIKey != "sk-ant-test" {
		t.Errorf("expected anthropic key, got %q", p.APIKey)
	}
	if p.Auth != "x-api-key" {
		t.Errorf("expected anthropic auth=x-api-key, got %q", p.Auth)
	}
	if p.APIFormat != "anthropic" {
		t.Errorf("expected anthropic api_format=anthropic, got %q", p.APIFormat)
	}
}

func TestRegistryFromFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "providers.json")
	err := os.WriteFile(configPath, []byte(`{
		"providers": {
			"ollama": {"base_url": "http://ollama:11434/v1", "auth": "none"},
			"openrouter": {"base_url": "https://openrouter.ai/api/v1", "api_key": "sk-or-test"}
		}
	}`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	r := NewRegistry(dir)
	if err := r.LoadFromFile(); err != nil {
		t.Fatalf("load from file: %v", err)
	}

	p, err := r.Get("ollama")
	if err != nil {
		t.Fatalf("get ollama: %v", err)
	}
	if p.BaseURL != "http://ollama:11434/v1" {
		t.Errorf("unexpected ollama URL: %q", p.BaseURL)
	}
	if p.Auth != "none" {
		t.Errorf("expected auth=none for ollama, got %q", p.Auth)
	}
}

func TestRegistryEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "providers.json")
	err := os.WriteFile(configPath, []byte(`{
		"providers": {
			"openai": {"base_url": "https://api.openai.com/v1", "api_key": "sk-from-file"}
		}
	}`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("OPENAI_API_KEY", "sk-from-env")

	r := NewRegistry(dir)
	if err := r.LoadFromFile(); err != nil {
		t.Fatal(err)
	}
	r.LoadFromEnv() // env wins

	p, err := r.Get("openai")
	if err != nil {
		t.Fatal(err)
	}
	if p.APIKey != "sk-from-env" {
		t.Errorf("env should override file, got %q", p.APIKey)
	}
}

func TestRegistryUnknownProvider(t *testing.T) {
	r := NewRegistry("")
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestSaveToFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	r.Set("openai", &Provider{Name: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "sk-x", Auth: "bearer"})

	if err := r.SaveToFile(); err != nil {
		t.Fatalf("save: %v", err)
	}

	r2 := NewRegistry(dir)
	if err := r2.LoadFromFile(); err != nil {
		t.Fatalf("load: %v", err)
	}
	p, err := r2.Get("openai")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if p.APIKey != "sk-x" {
		t.Fatalf("unexpected key: %q", p.APIKey)
	}
}
