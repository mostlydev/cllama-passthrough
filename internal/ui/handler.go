package ui

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"

	"github.com/mostlydev/cllama-passthrough/internal/agentctx"
	"github.com/mostlydev/cllama-passthrough/internal/cost"
	"github.com/mostlydev/cllama-passthrough/internal/provider"
)

//go:embed templates/*.html
var templateFS embed.FS

// UIOption configures optional Handler dependencies.
type UIOption func(*Handler)

// WithAccumulator attaches a cost accumulator to the UI handler,
// enabling the /costs dashboard and /costs/api endpoint.
func WithAccumulator(acc *cost.Accumulator) UIOption {
	return func(h *Handler) {
		h.accumulator = acc
	}
}

// WithContextRoot sets the agent context directory for pod member listing.
func WithContextRoot(root string) UIOption {
	return func(h *Handler) {
		h.contextRoot = root
	}
}

type Handler struct {
	registry    *provider.Registry
	accumulator *cost.Accumulator
	contextRoot string
	tpl         *template.Template
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

// -- costs page types --

type costsPageData struct {
	TotalCostUSD  float64
	TotalRequests int
	TotalTokens   int
	Agents        []agentCostRow
}

type agentCostRow struct {
	AgentID        string
	TotalRequests  int
	TotalTokensIn  int
	TotalTokensOut int
	TotalCostUSD   float64
	Models         []modelCostRow
}

type modelCostRow struct {
	Provider  string
	Model     string
	Requests  int
	TokensIn  int
	TokensOut int
	CostUSD   float64
}

// -- pod page types --

type podPageData struct {
	PodName string
	Members []podMemberRow
}

type podMemberRow struct {
	AgentID       string
	Service       string
	Type          string
	TotalRequests int
	TotalCostUSD  float64
	Models        []string // models seen in live traffic
}

// -- costs API types --

type costsAPIResponse struct {
	TotalCostUSD float64                     `json:"total_cost_usd"`
	Agents       map[string]agentAPIResponse `json:"agents"`
}

type agentAPIResponse struct {
	TotalCostUSD  float64            `json:"total_cost_usd"`
	TotalRequests int                `json:"total_requests"`
	Models        []modelAPIResponse `json:"models"`
}

type modelAPIResponse struct {
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	InputTokens int     `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	CostUSD     float64 `json:"cost_usd"`
	Requests    int     `json:"requests"`
}

func NewHandler(reg *provider.Registry, opts ...UIOption) http.Handler {
	if reg == nil {
		reg = provider.NewRegistry("")
	}
	tpl := template.Must(template.ParseFS(templateFS, "templates/*.html"))
	h := &Handler{registry: reg, tpl: tpl}
	for _, o := range opts {
		o(h)
	}
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/":
		h.renderIndex(w, "", http.StatusOK)
		return
	case r.Method == http.MethodPost && r.URL.Path == "/providers":
		h.handleProviderUpdate(w, r)
		return
	case r.Method == http.MethodGet && r.URL.Path == "/pod":
		h.renderPod(w)
		return
	case r.Method == http.MethodGet && r.URL.Path == "/costs":
		h.renderCosts(w)
		return
	case r.Method == http.MethodGet && r.URL.Path == "/costs/api":
		h.handleCostsAPI(w)
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
	_ = h.tpl.ExecuteTemplate(w, "index.html", pageData{Providers: rows, Error: errText})
}

func (h *Handler) renderCosts(w http.ResponseWriter) {
	data := h.buildCostsPageData()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tpl.ExecuteTemplate(w, "costs.html", data)
}

func (h *Handler) handleCostsAPI(w http.ResponseWriter) {
	resp := h.buildCostsAPIResponse()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)
}

func (h *Handler) buildCostsPageData() costsPageData {
	if h.accumulator == nil {
		return costsPageData{}
	}

	grouped := h.accumulator.All()
	agentIDs := make([]string, 0, len(grouped))
	for id := range grouped {
		agentIDs = append(agentIDs, id)
	}
	sort.Strings(agentIDs)

	var agents []agentCostRow
	for _, id := range agentIDs {
		entries := grouped[id]
		row := agentCostRow{AgentID: id}
		for _, e := range entries {
			row.TotalRequests += e.RequestCount
			row.TotalTokensIn += e.TotalInputTokens
			row.TotalTokensOut += e.TotalOutputTokens
			row.TotalCostUSD += e.TotalCostUSD
			row.Models = append(row.Models, modelCostRow{
				Provider:  e.Provider,
				Model:     e.Model,
				Requests:  e.RequestCount,
				TokensIn:  e.TotalInputTokens,
				TokensOut: e.TotalOutputTokens,
				CostUSD:   e.TotalCostUSD,
			})
		}
		agents = append(agents, row)
	}

	var totalReqs, totalToks int
	for _, a := range agents {
		totalReqs += a.TotalRequests
		totalToks += a.TotalTokensIn + a.TotalTokensOut
	}

	return costsPageData{
		TotalCostUSD:  h.accumulator.TotalCost(),
		TotalRequests: totalReqs,
		TotalTokens:   totalToks,
		Agents:        agents,
	}
}

func (h *Handler) buildCostsAPIResponse() costsAPIResponse {
	resp := costsAPIResponse{
		Agents: make(map[string]agentAPIResponse),
	}
	if h.accumulator == nil {
		return resp
	}

	resp.TotalCostUSD = h.accumulator.TotalCost()
	grouped := h.accumulator.All()
	for id, entries := range grouped {
		agent := agentAPIResponse{}
		for _, e := range entries {
			agent.TotalRequests += e.RequestCount
			agent.TotalCostUSD += e.TotalCostUSD
			agent.Models = append(agent.Models, modelAPIResponse{
				Provider:     e.Provider,
				Model:        e.Model,
				InputTokens:  e.TotalInputTokens,
				OutputTokens: e.TotalOutputTokens,
				CostUSD:      e.TotalCostUSD,
				Requests:     e.RequestCount,
			})
		}
		resp.Agents[id] = agent
	}
	return resp
}

func (h *Handler) renderPod(w http.ResponseWriter) {
	data := h.buildPodPageData()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tpl.ExecuteTemplate(w, "pod.html", data)
}

func (h *Handler) buildPodPageData() podPageData {
	var members []podMemberRow
	var podName string

	if h.contextRoot != "" {
		agents, err := agentctx.ListAgents(h.contextRoot)
		if err == nil {
			for _, a := range agents {
				if podName == "" && a.Pod != "" {
					podName = a.Pod
				}
				m := podMemberRow{
					AgentID: a.AgentID,
					Service: a.Service,
					Type:    a.Type,
				}

				// merge live cost data if accumulator available
				if h.accumulator != nil {
					entries := h.accumulator.ByAgent(a.AgentID)
					seen := make(map[string]bool)
					for _, e := range entries {
						m.TotalRequests += e.RequestCount
						m.TotalCostUSD += e.TotalCostUSD
						modelKey := fmt.Sprintf("%s/%s", e.Provider, e.Model)
						if !seen[modelKey] {
							m.Models = append(m.Models, modelKey)
							seen[modelKey] = true
						}
					}
				}

				members = append(members, m)
			}
		}
	}

	sort.Slice(members, func(i, j int) bool {
		return members[i].AgentID < members[j].AgentID
	})

	return podPageData{PodName: podName, Members: members}
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
