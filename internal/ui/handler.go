package ui

import (
	"embed"
	"html/template"
	"net/http"
	"sort"
	"strings"

	"github.com/mostlydev/cllama-passthrough/internal/provider"
)

//go:embed templates/*.html
var templateFS embed.FS

type Handler struct {
	registry *provider.Registry
	tpl      *template.Template
}

type providerRow struct {
	Name      string
	BaseURL   string
	Auth      string
	MaskedKey string
}

type pageData struct {
	Providers []providerRow
	Error     string
}

func NewHandler(reg *provider.Registry) http.Handler {
	if reg == nil {
		reg = provider.NewRegistry("")
	}
	tpl := template.Must(template.ParseFS(templateFS, "templates/index.html"))
	return &Handler{registry: reg, tpl: tpl}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/":
		h.renderIndex(w, "", http.StatusOK)
		return
	case r.Method == http.MethodPost && r.URL.Path == "/providers":
		h.handleProviderUpdate(w, r)
		return
	default:
		http.NotFound(w, r)
		return
	}
}

func (h *Handler) handleProviderUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderIndex(w, "invalid form body", http.StatusBadRequest)
		return
	}

	name := strings.ToLower(strings.TrimSpace(r.FormValue("name")))
	if name == "" {
		h.renderIndex(w, "provider name is required", http.StatusBadRequest)
		return
	}

	action := strings.ToLower(strings.TrimSpace(r.FormValue("action")))
	switch action {
	case "delete":
		h.registry.Delete(name)
	default:
		baseURL := strings.TrimSpace(r.FormValue("base_url"))
		auth := strings.ToLower(strings.TrimSpace(r.FormValue("auth")))
		if auth == "" {
			auth = "bearer"
		}
		h.registry.Set(name, &provider.Provider{
			Name:    name,
			BaseURL: baseURL,
			APIKey:  strings.TrimSpace(r.FormValue("api_key")),
			Auth:    auth,
		})
	}

	if err := h.registry.SaveToFile(); err != nil {
		h.renderIndex(w, "failed to persist providers.json: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) renderIndex(w http.ResponseWriter, errText string, status int) {
	all := h.registry.All()
	names := make([]string, 0, len(all))
	for name := range all {
		names = append(names, name)
	}
	sort.Strings(names)

	rows := make([]providerRow, 0, len(names))
	for _, name := range names {
		p := all[name]
		rows = append(rows, providerRow{
			Name:      p.Name,
			BaseURL:   p.BaseURL,
			Auth:      p.Auth,
			MaskedKey: maskKey(p.APIKey),
		})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = h.tpl.Execute(w, pageData{Providers: rows, Error: errText})
}

func maskKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
