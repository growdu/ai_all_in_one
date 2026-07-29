package preprocessing

import (
	"context"
	"strings"
	"testing"

	"github.com/growdu/ai_all_in_one/backend/internal/core"
	"github.com/growdu/ai_all_in_one/backend/internal/storage"
)

func newTestPreprocessor() *Preprocessor {
	return NewPreprocessor(
		storage.NewFileStore(
			"/tmp/test-storage-files",
			"/tmp/test-storage-index.json",
			[]byte("0123456789abcdef0123456789abcdef"),
		),
		"default",
	)
}

func TestPreprocessor_TextSmallFile(t *testing.T) {
	p := newTestPreprocessor()
	meta, err := p.store.Put("default", "hello.txt", "text/plain", []byte("Hello, world!"))
	if err != nil {
		t.Fatal(err)
	}

	req := core.ChatRequest{
		Model: "x",
		Messages: []core.ChatMessage{
			{Role: "user", Content: "看下这个", Attachments: []string{meta.ID}},
		},
	}

	processed, info, err := p.Process(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(processed.Messages) != 1 {
		t.Fatalf("messages = %d", len(processed.Messages))
	}
	// 内容应当包含 file 内容
	if !strings.Contains(processed.Messages[0].Content, "Hello, world!") {
		t.Errorf("content = %q", processed.Messages[0].Content)
	}
	if !strings.Contains(processed.Messages[0].Content, "看下这个") {
		t.Errorf("content lost user message")
	}
	if info.Attachments[0].Truncated {
		t.Error("small text should not be truncated")
	}
}

func TestPreprocessor_TextTruncation(t *testing.T) {
	p := newTestPreprocessor()
	// 100KB 文本超 50KB 限制
	big := make([]byte, 100*1024)
	for i := range big {
		big[i] = 'A'
	}
	meta, _ := p.store.Put("default", "big.txt", "text/plain", big)

	req := core.ChatRequest{
		Model: "x",
		Messages: []core.ChatMessage{
			{Role: "user", Content: "看大文件", Attachments: []string{meta.ID}},
		},
	}

	processed, info, err := p.Process(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Attachments[0].Truncated {
		t.Error("100KB text should be truncated")
	}
	if !strings.Contains(processed.Messages[0].Content, "truncated") {
		t.Error("truncation notice should be in content")
	}
}

func TestPreprocessor_MultipleAttachments(t *testing.T) {
	p := newTestPreprocessor()
	m1, _ := p.store.Put("default", "a.txt", "text/plain", []byte("file A content"))
	m2, _ := p.store.Put("default", "b.txt", "text/plain", []byte("file B content"))

	req := core.ChatRequest{
		Model: "x",
		Messages: []core.ChatMessage{
			{Role: "user", Content: "两个文件", Attachments: []string{m1.ID, m2.ID}},
		},
	}

	processed, info, err := p.Process(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Attachments) != 2 {
		t.Errorf("info.attachments = %d", len(info.Attachments))
	}
	if !strings.Contains(processed.Messages[0].Content, "file A content") {
		t.Error("missing A")
	}
	if !strings.Contains(processed.Messages[0].Content, "file B content") {
		t.Error("missing B")
	}
}

func TestPreprocessor_NoAttachments(t *testing.T) {
	p := newTestPreprocessor()
	req := core.ChatRequest{
		Model: "x",
		Messages: []core.ChatMessage{
			{Role: "user", Content: "纯文本"},
		},
	}

	processed, info, err := p.Process(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if processed.Messages[0].Content != "纯文本" {
		t.Errorf("content changed without attachments: %q", processed.Messages[0].Content)
	}
	if len(info.Attachments) != 0 {
		t.Errorf("info should be empty, got %d", len(info.Attachments))
	}
}

func TestPreprocessor_OwnershipEnforced(t *testing.T) {
	p := newTestPreprocessor()
	meta, _ := p.store.Put("userA", "x.txt", "text/plain", []byte("userA 的文件"))

	req := core.ChatRequest{
		Model: "x",
		Messages: []core.ChatMessage{
			{Role: "user", Attachments: []string{meta.ID}},
		},
	}

	// userB 想读 userA 的文件 → 应当忽略（不注入） + 警告
	processed, info, _ := p.ProcessFor(context.Background(), req, "userB")
	if strings.Contains(processed.Messages[0].Content, "userA 的文件") {
		t.Error("userB should not see userA's file")
	}
	if len(info.Attachments) == 0 {
		t.Error("should report the skipped attachment")
	}
	if info.Attachments[0].Warnings == nil {
		t.Error("should have warning about ownership")
	}
}

func TestPreprocessor_SystemMessagePreserved(t *testing.T) {
	p := newTestPreprocessor()
	meta, _ := p.store.Put("default", "x.txt", "text/plain", []byte("file"))

	req := core.ChatRequest{
		Model: "x",
		Messages: []core.ChatMessage{
			{Role: "system", Content: "你是 Go 专家"},
			{Role: "user", Content: "看下", Attachments: []string{meta.ID}},
		},
	}

	processed, _, err := p.Process(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if processed.Messages[0].Role != "system" || processed.Messages[0].Content != "你是 Go 专家" {
		t.Error("system message should be unchanged")
	}
	if !strings.Contains(processed.Messages[1].Content, "file") {
		t.Error("user message should have file content")
	}
}
