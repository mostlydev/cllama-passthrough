package cost

import (
	"sort"
	"sync"
)

// CostEntry is one (agent, provider, model) cost bucket.
type CostEntry struct {
	AgentID           string
	Provider          string
	Model             string
	TotalInputTokens  int
	TotalOutputTokens int
	TotalCostUSD      float64
	RequestCount      int
}

type bucketKey struct {
	AgentID  string
	Provider string
	Model    string
}

// Accumulator aggregates per-request cost data in memory. Thread-safe.
type Accumulator struct {
	mu      sync.RWMutex
	buckets map[bucketKey]*CostEntry
}

func NewAccumulator() *Accumulator {
	return &Accumulator{buckets: make(map[bucketKey]*CostEntry)}
}

func (a *Accumulator) Record(agentID, provider, model string, inputTokens, outputTokens int, costUSD float64) {
	key := bucketKey{AgentID: agentID, Provider: provider, Model: model}
	a.mu.Lock()
	defer a.mu.Unlock()
	e, ok := a.buckets[key]
	if !ok {
		e = &CostEntry{AgentID: agentID, Provider: provider, Model: model}
		a.buckets[key] = e
	}
	e.TotalInputTokens += inputTokens
	e.TotalOutputTokens += outputTokens
	e.TotalCostUSD += costUSD
	e.RequestCount++
}

// ByAgent returns all cost entries for a given agent, sorted by model.
func (a *Accumulator) ByAgent(agentID string) []CostEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var out []CostEntry
	for _, e := range a.buckets {
		if e.AgentID == agentID {
			out = append(out, *e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Provider+"/"+out[i].Model < out[j].Provider+"/"+out[j].Model
	})
	return out
}

// All returns cost summaries grouped by agent, sorted by agent ID.
func (a *Accumulator) All() map[string][]CostEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	grouped := make(map[string][]CostEntry)
	for _, e := range a.buckets {
		grouped[e.AgentID] = append(grouped[e.AgentID], *e)
	}
	for k := range grouped {
		sort.Slice(grouped[k], func(i, j int) bool {
			return grouped[k][i].Provider+"/"+grouped[k][i].Model < grouped[k][j].Provider+"/"+grouped[k][j].Model
		})
	}
	return grouped
}

// TotalCost returns the sum of all recorded costs across all agents.
func (a *Accumulator) TotalCost() float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var total float64
	for _, e := range a.buckets {
		total += e.TotalCostUSD
	}
	return total
}
