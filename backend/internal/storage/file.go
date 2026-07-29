// Package storage 提供文件存储抽象。
//
// 1.0 阶段：本地文件系统（data_dir/files/）
// 2.0 阶段：可换 S3 / OSS / MinIO
package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FileMeta 文件元信息
type FileMeta struct {
	ID            string    `json:"id"`
	OwnerID       string    `json:"owner_user_id"`
	Filename      string    `json:"filename"`
	MimeType      string    `json:"mime_type"`
	Size          int64     `json:"size"`
	ProcessedSize int64     `json:"processed_size"`
	Truncated     bool      `json:"truncated"`
	Source        string    `json:"source"`
	CreatedAt     time.Time `json:"created_at"`
}

// ErrFileNotFound 文件不存在或无权限
var ErrFileNotFound = errors.New("file not found")

// ErrUnsupportedMime 不支持的 mime 类型
var ErrUnsupportedMime = errors.New("unsupported file type")

// 1.0 大小限制
const (
	maxImageSize int64 = 5 * 1024 * 1024 // 5MB
	maxDocSize   int64 = 50 * 1024      // 50KB
	truncHead          = 30 * 1024      // 截断时保留头部
	truncTail          = 20 * 1024      // 截断时保留尾部
)

// FileStore 本地文件系统实现
type FileStore struct {
	dir     string
	index   string
	key     []byte // 1.0 阶段占位（未用，2.0 接 AES-GCM 加密 owner 字段）
	mu      sync.RWMutex
	indexes map[string]FileMeta
}

// NewFileStore 创建 FileStore
func NewFileStore(dir, indexPath string, key []byte) *FileStore {
	fs := &FileStore{
		dir:     dir,
		index:   indexPath,
		key:     key,
		indexes: make(map[string]FileMeta),
	}
	_ = os.MkdirAll(dir, 0o700)
	if data, err := os.ReadFile(indexPath); err == nil {
		_ = fs.loadIndex(data)
	}
	return fs
}

// Put 存文件，自动按类型 truncate
func (s *FileStore) Put(ownerID, filename, mimeType string, data []byte) (*FileMeta, error) {
	if !isSupportedMime(mimeType) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedMime, mimeType)
	}

	truncated := false
	processed := data
	maxSize := maxDocSize
	if isImage(mimeType) {
		maxSize = maxImageSize
	}
	if int64(len(data)) > maxSize {
		processed = truncateBytes(data, maxSize)
		truncated = true
	}

	id := generateID()
	if err := os.WriteFile(s.pathFor(id), processed, 0o600); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	meta := FileMeta{
		ID:            id,
		OwnerID:       ownerID,
		Filename:      filename,
		MimeType:      mimeType,
		Size:          int64(len(data)),
		ProcessedSize: int64(len(processed)),
		Truncated:     truncated,
		Source:        "user_upload",
		CreatedAt:     time.Now(),
	}
	s.mu.Lock()
	s.indexes[id] = meta
	s.persistLocked()
	s.mu.Unlock()
	return &meta, nil
}

// Get 取文件内容（带 owner 校验）
func (s *FileStore) Get(id, ownerID string) ([]byte, error) {
	s.mu.RLock()
	meta, ok := s.indexes[id]
	s.mu.RUnlock()
	if !ok || meta.OwnerID != ownerID {
		return nil, ErrFileNotFound
	}
	return os.ReadFile(s.pathFor(id))
}

// GetMeta 取元信息
func (s *FileStore) GetMeta(id, ownerID string) (*FileMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	meta, ok := s.indexes[id]
	if !ok || meta.OwnerID != ownerID {
		return nil, ErrFileNotFound
	}
	return &meta, nil
}

// GetMetaAndContent 一次返回元信息和内容（避免双重读锁）
func (s *FileStore) GetMetaAndContent(id, ownerID string) (*FileMeta, []byte, error) {
	s.mu.RLock()
	meta, ok := s.indexes[id]
	s.mu.RUnlock()
	if !ok || meta.OwnerID != ownerID {
		return nil, nil, ErrFileNotFound
	}
	data, err := os.ReadFile(s.pathFor(id))
	if err != nil {
		return nil, nil, err
	}
	return &meta, data, nil
}

// Delete 删文件
func (s *FileStore) Delete(id, ownerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta, ok := s.indexes[id]
	if !ok || meta.OwnerID != ownerID {
		return ErrFileNotFound
	}
	_ = os.Remove(s.pathFor(id))
	delete(s.indexes, id)
	s.persistLocked()
	return nil
}

// ListByOwner 列某 owner 的所有文件
func (s *FileStore) ListByOwner(ownerID string) []FileMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []FileMeta
	for _, m := range s.indexes {
		if m.OwnerID == ownerID {
			out = append(out, m)
		}
	}
	return out
}

func (s *FileStore) pathFor(id string) string {
	return filepath.Join(s.dir, id+".bin")
}

func (s *FileStore) persistLocked() {
	wrapper := struct {
		Files map[string]FileMeta `json:"_files"`
	}{Files: s.indexes}
	data, _ := json.MarshalIndent(wrapper, "", "  ")
	_ = os.WriteFile(s.index, data, 0o600)
}

func (s *FileStore) loadIndex(data []byte) error {
	var wrapper struct {
		Files map[string]FileMeta `json:"_files"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return err
	}
	for k, v := range wrapper.Files {
		s.indexes[k] = v
	}
	return nil
}

// --- 工具函数 ---

func generateID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return "file_" + hex.EncodeToString(b[:])
}

func isImage(mime string) bool { return strings.HasPrefix(mime, "image/") }

func isSupportedMime(mime string) bool {
	if strings.HasPrefix(mime, "image/") {
		return true
	}
	if strings.HasPrefix(mime, "text/") {
		return true
	}
	if mime == "application/pdf" {
		return true
	}
	if mime == "application/json" {
		return true
	}
	return false
}

func truncateBytes(data []byte, maxSize int64) []byte {
	if int64(len(data)) <= maxSize {
		return data
	}
	if maxSize < int64(truncHead+truncTail) {
		return data[:maxSize]
	}
	head := data[:truncHead]
	tail := data[len(data)-truncTail:]
	out := make([]byte, 0, truncHead+truncTail+100)
	out = append(out, head...)
	out = append(out, []byte("\n\n... [truncated] ...\n\n")...)
	out = append(out, tail...)
	return out
}
