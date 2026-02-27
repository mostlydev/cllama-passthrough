package cost

import "testing"

func TestExtractUsageFromJSON(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-1",
		"choices": [{"message": {"content": "hello"}}],
		"usage": {
			"prompt_tokens": 150,
			"completion_tokens": 42,
			"total_tokens": 192
		}
	}`)

	u, err := ExtractUsage(body)
	if err != nil {
		t.Fatal(err)
	}
	if u.PromptTokens != 150 {
		t.Errorf("expected 150 prompt tokens, got %d", u.PromptTokens)
	}
	if u.CompletionTokens != 42 {
		t.Errorf("expected 42 completion tokens, got %d", u.CompletionTokens)
	}
}

func TestExtractUsageMissing(t *testing.T) {
	body := []byte(`{"id": "chatcmpl-1", "choices": []}`)
	u, err := ExtractUsage(body)
	if err != nil {
		t.Fatal(err)
	}
	if u.PromptTokens != 0 || u.CompletionTokens != 0 {
		t.Errorf("expected zero usage when missing, got %+v", u)
	}
}

func TestExtractUsageFromSSE(t *testing.T) {
	// SSE stream: final data chunk before [DONE] contains usage
	stream := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":20,\"total_tokens\":120}}\n\n" +
		"data: [DONE]\n\n")
	u, err := ExtractUsageFromSSE(stream)
	if err != nil {
		t.Fatal(err)
	}
	if u.PromptTokens != 100 {
		t.Errorf("expected 100 prompt tokens, got %d", u.PromptTokens)
	}
	if u.CompletionTokens != 20 {
		t.Errorf("expected 20 completion tokens, got %d", u.CompletionTokens)
	}
}

func TestExtractUsageFromSSENoUsage(t *testing.T) {
	stream := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n")
	u, err := ExtractUsageFromSSE(stream)
	if err != nil {
		t.Fatal(err)
	}
	if u.PromptTokens != 0 {
		t.Errorf("expected 0, got %d", u.PromptTokens)
	}
}
