package logging

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// Logger writes structured JSON logs suitable for claw audit ingestion.
type Logger struct {
	mu  sync.Mutex
	enc *json.Encoder
}

type entry struct {
	TS           string  `json:"ts"`
	ClawID       string  `json:"claw_id,omitempty"`
	Type         string  `json:"type"`
	Model        string  `json:"model,omitempty"`
	LatencyMS    *int64  `json:"latency_ms,omitempty"`
	StatusCode   *int    `json:"status_code,omitempty"`
	Intervention *string `json:"intervention"`
	Error        string  `json:"error,omitempty"`
}

func New(w io.Writer) *Logger {
	if w == nil {
		w = io.Discard
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return &Logger{enc: enc}
}

func (l *Logger) LogRequest(clawID, model string) {
	l.log(entry{
		TS:           time.Now().UTC().Format(time.RFC3339),
		ClawID:       clawID,
		Type:         "request",
		Model:        model,
		Intervention: nil,
	})
}

func (l *Logger) LogResponse(clawID, model string, statusCode int, latencyMS int64) {
	l.log(entry{
		TS:           time.Now().UTC().Format(time.RFC3339),
		ClawID:       clawID,
		Type:         "response",
		Model:        model,
		LatencyMS:    ptrI64(latencyMS),
		StatusCode:   ptrInt(statusCode),
		Intervention: nil,
	})
}

func (l *Logger) LogError(clawID, model string, statusCode int, latencyMS int64, err error) {
	errText := ""
	if err != nil {
		errText = err.Error()
	}
	l.log(entry{
		TS:           time.Now().UTC().Format(time.RFC3339),
		ClawID:       clawID,
		Type:         "error",
		Model:        model,
		LatencyMS:    ptrI64(latencyMS),
		StatusCode:   ptrInt(statusCode),
		Intervention: nil,
		Error:        errText,
	})
}

func (l *Logger) LogIntervention(clawID, model, reason string) {
	reasonCopy := reason
	l.log(entry{
		TS:           time.Now().UTC().Format(time.RFC3339),
		ClawID:       clawID,
		Type:         "intervention",
		Model:        model,
		Intervention: &reasonCopy,
	})
}

func (l *Logger) log(e entry) {
	if l == nil || l.enc == nil {
		return
	}
	l.mu.Lock()
	_ = l.enc.Encode(e)
	l.mu.Unlock()
}

func ptrInt(v int) *int {
	return &v
}

func ptrI64(v int64) *int64 {
	return &v
}
