package security

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func newTestKeyring(t *testing.T) (*Keyring, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "keyring.json")
	kr, err := NewKeyring(path, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	return kr, path
}

func TestKeyring_PutGet(t *testing.T) {
	kr, _ := newTestKeyring(t)
	if err := kr.Put("doubao", "sk-test-12345"); err != nil {
		t.Fatal(err)
	}
	got, err := kr.Get("doubao")
	if err != nil {
		t.Fatal(err)
	}
	if got != "sk-test-12345" {
		t.Errorf("got = %q, want sk-test-12345", got)
	}
}

func TestKeyring_Overwrite(t *testing.T) {
	kr, _ := newTestKeyring(t)
	kr.Put("doubao", "first")
	kr.Put("doubao", "second")
	got, _ := kr.Get("doubao")
	if got != "second" {
		t.Errorf("got = %q, want second (overwritten)", got)
	}
}

func TestKeyring_Delete(t *testing.T) {
	kr, _ := newTestKeyring(t)
	kr.Put("doubao", "x")
	if err := kr.Delete("doubao"); err != nil {
		t.Fatal(err)
	}
	_, err := kr.Get("doubao")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestKeyring_DeleteNotFound(t *testing.T) {
	kr, _ := newTestKeyring(t)
	err := kr.Delete("never-existed")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestKeyring_Has(t *testing.T) {
	kr, _ := newTestKeyring(t)
	if kr.Has("doubao") {
		t.Error("empty keyring should not have doubao")
	}
	kr.Put("doubao", "x")
	if !kr.Has("doubao") {
		t.Error("after Put, Has should be true")
	}
}

func TestKeyring_List(t *testing.T) {
	kr, _ := newTestKeyring(t)
	kr.Put("doubao", "a")
	kr.Put("openai", "b")
	kr.Put("claude", "c")
	list := kr.List()
	if len(list) != 3 {
		t.Errorf("list len = %d, want 3", len(list))
	}
	// 检查三个都在（顺序无关）
	set := make(map[string]bool)
	for _, n := range list {
		set[n] = true
	}
	for _, want := range []string{"doubao", "openai", "claude"} {
		if !set[want] {
			t.Errorf("missing %s in list", want)
		}
	}
}

func TestKeyring_Persistence(t *testing.T) {
	// 重新打开同一个文件，key 应该还在
	dir := t.TempDir()
	path := filepath.Join(dir, "keyring.json")
	key := []byte("0123456789abcdef0123456789abcdef")

	kr1, _ := NewKeyring(path, key)
	kr1.Put("doubao", "persistent-value")
	kr1.Put("openai", "another-value")

	// 重新打开
	kr2, err := NewKeyring(path, key)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := kr2.Get("doubao"); got != "persistent-value" {
		t.Errorf("after reopen: got = %q, want persistent-value", got)
	}
	if got, _ := kr2.Get("openai"); got != "another-value" {
		t.Errorf("after reopen: openai got = %q, want another-value", got)
	}
}

func TestKeyring_FilePermissions(t *testing.T) {
	// 1.0 关键安全要求：keyring 文件 0600
	kr, path := newTestKeyring(t)
	kr.Put("doubao", "x")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	mode := info.Mode().Perm()
	if mode != 0o600 {
		t.Errorf("file mode = %o, want 0600", mode)
	}
}

func TestKeyring_WrongMasterKey(t *testing.T) {
	// 用 key1 存，用 key2 读 → 应当返回 Decrypt 错误（不是 panic）
	dir := t.TempDir()
	path := filepath.Join(dir, "keyring.json")
	key1 := []byte("0123456789abcdef0123456789abcdef")
	key2 := []byte("fedcba9876543210fedcba9876543210")

	kr1, _ := NewKeyring(path, key1)
	kr1.Put("doubao", "secret")

	// 重新打开用 key2：load 成功（只是 read），Get 时 decrypt 失败
	kr2, _ := NewKeyring(path, key2)
	_, err := kr2.Get("doubao")
	if err == nil {
		t.Error("expected error decrypting with wrong master key")
	}
}

// ---- concurrency ----

func TestKeyring_ConcurrentPut(t *testing.T) {
	kr, _ := newTestKeyring(t)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			kr.Put("doubao", "value-"+string(rune('A'+n%26)))
		}(i)
	}
	wg.Wait()
	// 最终读能读到其中一个值
	if _, err := kr.Get("doubao"); err != nil {
		t.Fatal(err)
	}
}
