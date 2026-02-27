package agentctx

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AgentContext holds the per-agent mounted contract and metadata files.
type AgentContext struct {
	AgentID     string
	AgentsMD    []byte
	ClawdapusMD []byte
	Metadata    map[string]any
}

// Load reads an agent's context files from contextRoot/<agentID>/.
func Load(contextRoot, agentID string) (*AgentContext, error) {
	dir := filepath.Join(contextRoot, agentID)

	agentsMD, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		return nil, fmt.Errorf("load agent context %q: AGENTS.md: %w", agentID, err)
	}

	clawdapusMD, err := os.ReadFile(filepath.Join(dir, "CLAWDAPUS.md"))
	if err != nil {
		return nil, fmt.Errorf("load agent context %q: CLAWDAPUS.md: %w", agentID, err)
	}

	metaRaw, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		return nil, fmt.Errorf("load agent context %q: metadata.json: %w", agentID, err)
	}

	var meta map[string]any
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		return nil, fmt.Errorf("load agent context %q: parse metadata: %w", agentID, err)
	}

	return &AgentContext{
		AgentID:     agentID,
		AgentsMD:    agentsMD,
		ClawdapusMD: clawdapusMD,
		Metadata:    meta,
	}, nil
}

// MetadataToken returns metadata["token"] when present and a string.
func (a *AgentContext) MetadataToken() string {
	if a == nil {
		return ""
	}
	tok, _ := a.Metadata["token"].(string)
	return tok
}
