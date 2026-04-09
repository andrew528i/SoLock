package jsonrpc

import (
	"encoding/json"
	"testing"

	"github.com/solock/solock/internal/application"
)

func newLockedApp() *application.App {
	return application.New(
		"/tmp/solock-test",
		nil,
		nil,
	)
}

func makeRequest(method string) *Request {
	return &Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  json.RawMessage(`{}`),
		ID:      json.RawMessage(`1`),
	}
}

func TestLockedVaultRejectsListEntries(t *testing.T) {
	app := newLockedApp()
	h := NewHandler(app, "test-token", make(chan struct{}))

	resp := h.Handle(makeRequest("list_entries"))

	if resp.Error == nil {
		t.Fatal("expected error response for list_entries when vault is locked")
	}
	if resp.Error.Code != ErrCodeLocked {
		t.Errorf("expected error code %d, got %d", ErrCodeLocked, resp.Error.Code)
	}
	if resp.Error.Message != "vault is locked" {
		t.Errorf("expected message 'vault is locked', got %q", resp.Error.Message)
	}
}

func TestLockedVaultRejectsAddGroup(t *testing.T) {
	app := newLockedApp()
	h := NewHandler(app, "test-token", make(chan struct{}))

	resp := h.Handle(makeRequest("add_group"))

	if resp.Error == nil {
		t.Fatal("expected error response for add_group when vault is locked")
	}
	if resp.Error.Code != ErrCodeLocked {
		t.Errorf("expected error code %d, got %d", ErrCodeLocked, resp.Error.Code)
	}
}

func TestLockedVaultAllowsStatus(t *testing.T) {
	app := newLockedApp()
	h := NewHandler(app, "test-token", make(chan struct{}))

	resp := h.Handle(makeRequest("status"))

	if resp.Error != nil {
		t.Fatalf("expected success for status when locked, got error: %s", resp.Error.Message)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("expected result to be map")
	}
	locked, ok := result["locked"].(bool)
	if !ok || !locked {
		t.Error("expected locked=true in status response")
	}
}

func TestLockedVaultAllowsShutdown(t *testing.T) {
	app := newLockedApp()
	shutdown := make(chan struct{}, 1)
	h := NewHandler(app, "test-token", shutdown)

	resp := h.Handle(makeRequest("shutdown"))

	if resp.Error != nil {
		t.Fatalf("expected success for shutdown when locked, got error: %s", resp.Error.Message)
	}
}

func TestLockedVaultRejectsSearchEntries(t *testing.T) {
	app := newLockedApp()
	h := NewHandler(app, "test-token", make(chan struct{}))

	resp := h.Handle(makeRequest("search_entries"))

	if resp.Error == nil {
		t.Fatal("expected error response for search_entries when vault is locked")
	}
	if resp.Error.Code != ErrCodeLocked {
		t.Errorf("expected error code %d, got %d", ErrCodeLocked, resp.Error.Code)
	}
}

func TestErrorResponseContainsCode(t *testing.T) {
	resp := errorResponse(json.RawMessage(`1`), ErrCodeLocked, "vault is locked")

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Error.Code != -32000 {
		t.Errorf("expected code -32000 in JSON, got %d", parsed.Error.Code)
	}
	if parsed.Error.Message != "vault is locked" {
		t.Errorf("expected message 'vault is locked', got %q", parsed.Error.Message)
	}
}
