package logging

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestLogRequestEmitsJSON(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)
	l.LogRequest("tiverton", "openai/gpt-4o")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, buf.String())
	}
	if entry["claw_id"] != "tiverton" {
		t.Errorf("expected claw_id=tiverton, got %v", entry["claw_id"])
	}
	if entry["type"] != "request" {
		t.Errorf("expected type=request, got %v", entry["type"])
	}
	if entry["model"] != "openai/gpt-4o" {
		t.Errorf("expected model, got %v", entry["model"])
	}
	if _, ok := entry["intervention"]; !ok {
		t.Errorf("expected intervention field to be present")
	}
}

func TestLogResponseIncludesLatency(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)
	l.LogResponse("tiverton", "openai/gpt-4o", 200, 1250)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry["type"] != "response" {
		t.Errorf("expected type=response")
	}
	if entry["latency_ms"].(float64) != 1250 {
		t.Errorf("expected latency_ms=1250, got %v", entry["latency_ms"])
	}
	if entry["status_code"].(float64) != 200 {
		t.Errorf("expected status_code=200, got %v", entry["status_code"])
	}
}
