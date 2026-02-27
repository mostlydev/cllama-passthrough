package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Provider holds auth and routing config for one LLM provider.
type Provider struct {
	Name      string `json:"name,omitempty"`
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key,omitempty"`
	Auth      string `json:"auth,omitempty"`       // "bearer" (default), "none", "x-api-key"
	APIFormat string `json:"api_format,omitempty"` // "openai" (default), "anthropic"
}

// Registry manages known providers; it is safe for concurrent use.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]*Provider
	authDir   string
}

var knownProviders = map[string]string{
	"openai":     "https://api.openai.com/v1",
	"anthropic":  "https://api.anthropic.com/v1",
	"openrouter": "https://openrouter.ai/api/v1",
	"ollama":     "http://ollama:11434/v1",
}

var envKeyMap = map[string]string{
	"OPENAI_API_KEY":     "openai",
	"ANTHROPIC_API_KEY":  "anthropic",
	"OPENROUTER_API_KEY": "openrouter",
}

var envBaseURLMap = map[string]string{
	"OPENAI_BASE_URL":     "openai",
	"ANTHROPIC_BASE_URL":  "anthropic",
	"OPENROUTER_BASE_URL": "openrouter",
	"OLLAMA_BASE_URL":     "ollama",
}

func NewRegistry(authDir string) *Registry {
	return &Registry{
		providers: make(map[string]*Provider),
		authDir:   authDir,
	}
}

// LoadFromFile reads providers.json from the auth directory.
func (r *Registry) LoadFromFile() error {
	if r.authDir == "" {
		return nil
	}
	path := filepath.Join(r.authDir, "providers.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read providers.json: %w", err)
	}

	var cfg struct {
		Providers map[string]Provider `json:"providers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse providers.json: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for name, p := range cfg.Providers {
		n := normalizeName(name)
		if n == "" {
			continue
		}
		cp := p
		cp.Name = n
		if cp.BaseURL == "" {
			cp.BaseURL = knownProviders[n]
		}
		if cp.Auth == "" {
			cp.Auth = defaultAuth(n)
		}
		if cp.APIFormat == "" {
			cp.APIFormat = defaultAPIFormat(n)
		}
		r.providers[n] = &cp
	}

	return nil
}

// LoadFromEnv overlays known provider keys/base URLs from env vars.
// Values from env win over file values.
func (r *Registry) LoadFromEnv() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for envKey, provName := range envBaseURLMap {
		v := strings.TrimSpace(os.Getenv(envKey))
		if v == "" {
			continue
		}
		p, ok := r.providers[provName]
		if !ok {
			p = &Provider{Name: provName, Auth: defaultAuth(provName), APIFormat: defaultAPIFormat(provName)}
		}
		p.BaseURL = v
		r.providers[provName] = p
	}

	for envKey, provName := range envKeyMap {
		v := strings.TrimSpace(os.Getenv(envKey))
		if v == "" {
			continue
		}
		p, ok := r.providers[provName]
		if !ok {
			p = &Provider{Name: provName, BaseURL: knownProviders[provName], Auth: defaultAuth(provName), APIFormat: defaultAPIFormat(provName)}
		}
		if p.BaseURL == "" {
			p.BaseURL = knownProviders[provName]
		}
		if p.Auth == "" {
			p.Auth = defaultAuth(provName)
		}
		if p.APIFormat == "" {
			p.APIFormat = defaultAPIFormat(provName)
		}
		p.APIKey = v
		r.providers[provName] = p
	}
}

func (r *Registry) Set(name string, p *Provider) {
	n := normalizeName(name)
	if n == "" || p == nil {
		return
	}
	cp := *p
	cp.Name = n
	if cp.BaseURL == "" {
		cp.BaseURL = knownProviders[n]
	}
	if cp.Auth == "" {
		cp.Auth = defaultAuth(n)
	}
	if cp.APIFormat == "" {
		cp.APIFormat = defaultAPIFormat(n)
	}
	r.mu.Lock()
	r.providers[n] = &cp
	r.mu.Unlock()
}

func (r *Registry) Delete(name string) bool {
	n := normalizeName(name)
	if n == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.providers[n]; !ok {
		return false
	}
	delete(r.providers, n)
	return true
}

func (r *Registry) Get(name string) (*Provider, error) {
	n := normalizeName(name)
	r.mu.RLock()
	p, ok := r.providers[n]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown provider: %q", name)
	}
	cp := *p
	return &cp, nil
}

func (r *Registry) All() map[string]*Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]*Provider, len(r.providers))
	for k, v := range r.providers {
		cp := *v
		out[k] = &cp
	}
	return out
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.providers))
	for k := range r.providers {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// SaveToFile writes providers.json back to authDir for UI edits.
func (r *Registry) SaveToFile() error {
	if r.authDir == "" {
		return fmt.Errorf("no auth directory configured")
	}
	if err := os.MkdirAll(r.authDir, 0o700); err != nil {
		return fmt.Errorf("create auth dir: %w", err)
	}

	r.mu.RLock()
	providers := make(map[string]Provider, len(r.providers))
	for name, p := range r.providers {
		providers[name] = Provider{
			Name:      "",
			BaseURL:   p.BaseURL,
			APIKey:    p.APIKey,
			Auth:      p.Auth,
			APIFormat: p.APIFormat,
		}
	}
	r.mu.RUnlock()

	cfg := struct {
		Providers map[string]Provider `json:"providers"`
	}{Providers: providers}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal providers.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(r.authDir, "providers.json"), data, 0o600); err != nil {
		return fmt.Errorf("write providers.json: %w", err)
	}
	return nil
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func defaultAuth(provider string) string {
	switch normalizeName(provider) {
	case "ollama":
		return "none"
	case "anthropic":
		return "x-api-key"
	default:
		return "bearer"
	}
}

func defaultAPIFormat(provider string) string {
	if normalizeName(provider) == "anthropic" {
		return "anthropic"
	}
	return "openai"
}
