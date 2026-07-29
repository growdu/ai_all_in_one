package mockprovider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/growdu/ai_all_in_one/backend/internal/core"
)

func TestProvider_Name(t *testing.T) {
	if New().Name() != "mock" {
		t.Error("Name != mock")
	}
}

func TestProvider_Modality(t *testing.T) {
	if New().Modality() != core.ModalityChat {
		t.Error("Modality != chat")
	}
}

func TestProvider_ListModels(t *testing.T) {
	models := New().ListModels()
	if len(models) < 2 {
		t.Errorf("expected >=2 models, got %d", len(models))
	}
	for _, m := range models {
		if m.Provider != "mock" {
			t.Errorf("model %s provider = %q, want mock", m.ID, m.Provider)
		}
	}
}

func TestProvider_ChatComplete(t *testing.T) {
	p := New()
	req := core.ChatRequest{
		Model: "mock-echo",
		Messages: []core.ChatMessage{
			{Role: "user", Content: "hello"},
		},
	}
	resp, err := p.ChatComplete(context.Background(), req, "fake-key")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Content, "hello") {
		t.Errorf("content = %q, want echo of hello", resp.Content)
	}
	if resp.Provider != "mock" {
		t.Errorf("provider = %q", resp.Provider)
	}
}

func TestProvider_ChatStream(t *testing.T) {
	p := New()
	req := core.ChatRequest{
		Model:    "mock-echo",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
	}
	chunks, errs, closer, err := p.ChatStream(context.Background(), req, "key")
	if err != nil {
		t.Fatal(err)
	}
	defer closer.Close()

	var got string
	for c := range chunks {
		got += c.Delta
	}
	if !strings.Contains(got, "hi") {
		t.Errorf("stream content = %q, want echo of hi", got)
	}
	// errs 应当被 close
	for range errs {
	}
	_ = http.StatusOK // 占位防 unused
	_ = httptest.NewRecorder
	_ = io.Discard
}
