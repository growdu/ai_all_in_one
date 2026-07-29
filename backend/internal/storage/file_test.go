package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestFileStore(t *testing.T) *FileStore {
	t.Helper()
	dir := t.TempDir()
	return NewFileStore(filepath.Join(dir, "files"), filepath.Join(dir, "keyring.json"), []byte("0123456789abcdef0123456789abcdef"))
}

func TestFileStore_PutGet(t *testing.T) {
	fs := newTestFileStore(t)
	meta, err := fs.Put("user1", "test.png", "image/png", []byte("fake-png-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if meta.ID == "" {
		t.Error("ID should be set")
	}
	if meta.Filename != "test.png" {
		t.Errorf("filename = %q", meta.Filename)
	}
	if meta.MimeType != "image/png" {
		t.Errorf("mime = %q", meta.MimeType)
	}
	if meta.Size != int64(len("fake-png-bytes")) {
		t.Errorf("size = %d", meta.Size)
	}
	if meta.OwnerID != "user1" {
		t.Errorf("owner = %q", meta.OwnerID)
	}

	got, err := fs.Get(meta.ID, "user1")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "fake-png-bytes" {
		t.Errorf("content = %q", got)
	}
}

func TestFileStore_GetNotFound(t *testing.T) {
	fs := newTestFileStore(t)
	_, err := fs.Get("nonexistent", "user1")
	if err == nil {
		t.Error("expected error")
	}
}

func TestFileStore_Ownership(t *testing.T) {
	fs := newTestFileStore(t)
	meta, _ := fs.Put("user1", "x.png", "image/png", []byte("data"))

	// user2 不能读 user1 的文件
	_, err := fs.Get(meta.ID, "user2")
	if err == nil {
		t.Error("user2 should not access user1's file")
	}
}

func TestFileStore_Delete(t *testing.T) {
	fs := newTestFileStore(t)
	meta, _ := fs.Put("user1", "x.png", "image/png", []byte("data"))

	if err := fs.Delete(meta.ID, "user1"); err != nil {
		t.Fatal(err)
	}
	_, err := fs.Get(meta.ID, "user1")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestFileStore_DeleteWrongOwner(t *testing.T) {
	fs := newTestFileStore(t)
	meta, _ := fs.Put("user1", "x.png", "image/png", []byte("data"))

	err := fs.Delete(meta.ID, "user2")
	if err == nil {
		t.Error("user2 should not delete user1's file")
	}
}

func TestFileStore_Truncation_Image(t *testing.T) {
	// 6MB 图片超 5MB 限制
	fs := newTestFileStore(t)
	big := make([]byte, 6*1024*1024)
	for i := range big {
		big[i] = byte(i % 256)
	}
	meta, err := fs.Put("user1", "big.png", "image/png", big)
	if err != nil {
		t.Fatal(err)
	}
	if !meta.Truncated {
		t.Error("expected truncated=true for 6MB image")
	}
	if meta.ProcessedSize > maxImageSize+1024 {
		t.Errorf("processed size = %d, want <= 5MB", meta.ProcessedSize)
	}
}

func TestFileStore_NoTruncation_SmallImage(t *testing.T) {
	fs := newTestFileStore(t)
	// 100KB image under 5MB limit → not truncated
	small := make([]byte, 100*1024)
	meta, err := fs.Put("user1", "small.png", "image/png", small)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Truncated {
		t.Error("100KB image should not be truncated (under 5MB limit)")
	}
}

func TestFileStore_TextSizeLimit(t *testing.T) {
	fs := newTestFileStore(t)
	// 100KB 文本
	big := make([]byte, 100*1024)
	for i := range big {
		big[i] = 'A'
	}
	meta, err := fs.Put("user1", "doc.txt", "text/plain", big)
	if err != nil {
		t.Fatal(err)
	}
	if !meta.Truncated {
		t.Error("expected truncated=true for 100KB text")
	}
}

func TestFileStore_SmallFile_NoTruncation(t *testing.T) {
	fs := newTestFileStore(t)
	small := []byte("hello world")
	meta, err := fs.Put("user1", "small.txt", "text/plain", small)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Truncated {
		t.Error("small file should not be truncated")
	}
}

func TestFileStore_UnsupportedMime(t *testing.T) {
	fs := newTestFileStore(t)
	_, err := fs.Put("user1", "x.exe", "application/x-msdownload", []byte("MZ"))
	if err == nil {
		t.Error("expected error for unsupported mime type")
	}
}

// 检查文件被存到正确路径
func TestFileStore_PhysicalFile(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore(filepath.Join(dir, "files"), filepath.Join(dir, "keyring.json"), []byte("0123456789abcdef0123456789abcdef"))
	meta, _ := fs.Put("user1", "x.png", "image/png", []byte("data"))
	expected := filepath.Join(dir, "files", meta.ID+".bin")
	if _, err := os.Stat(expected); err != nil {
		t.Errorf("physical file not at %s: %v", expected, err)
	}
}

func TestFileStore_ListByOwner(t *testing.T) {
	fs := newTestFileStore(t)
	fs.Put("user1", "a.png", "image/png", []byte("a"))
	fs.Put("user1", "b.png", "image/png", []byte("bb"))
	fs.Put("user2", "c.png", "image/png", []byte("ccc"))

	list := fs.ListByOwner("user1")
	if len(list) != 2 {
		t.Errorf("user1 list len = %d, want 2", len(list))
	}
	list2 := fs.ListByOwner("user2")
	if len(list2) != 1 {
		t.Errorf("user2 list len = %d, want 1", len(list2))
	}
}
