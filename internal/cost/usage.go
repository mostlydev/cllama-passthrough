package cost

import (
	"bytes"
	"encoding/json"
)

// Usage holds token counts from an OpenAI-compatible response.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ExtractUsage parses usage from a non-streamed JSON response body.
func ExtractUsage(body []byte) (Usage, error) {
	var resp struct {
		Usage *Usage `json:"usage"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return Usage{}, err
	}
	if resp.Usage == nil {
		return Usage{}, nil
	}
	return *resp.Usage, nil
}

// ExtractUsageFromSSE scans SSE data lines for the last one containing a "usage" field.
// OpenAI streams include usage in the final data chunk before "data: [DONE]".
func ExtractUsageFromSSE(stream []byte) (Usage, error) {
	var lastUsage Usage
	for _, line := range bytes.Split(stream, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		payload := bytes.TrimPrefix(line, []byte("data: "))
		if bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}
		var chunk struct {
			Usage *Usage `json:"usage"`
		}
		if json.Unmarshal(payload, &chunk) == nil && chunk.Usage != nil {
			lastUsage = *chunk.Usage
		}
	}
	return lastUsage, nil
}
