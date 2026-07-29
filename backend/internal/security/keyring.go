package security

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Keyring 用户 Key 加密存储
//
// 1.0 阶段：JSON 文件 + AES-256-GCM
//   - 0 CGO 依赖，零部署门槛
//   - 文件放在 data_dir/keysring.json，chmod 0600
//   - 1.0 用户量小，文件方案足够
//
// 2.0 阶段：换 modernc.org/sqlite（接入时改）
//   - 详见 docs/backend/02-provider.md §九点二
type Keyring struct {
	path string
	key  []byte
	mu   sync.RWMutex
}

// KeyEntry 单个 provider 的 key
type KeyEntry struct {
	Provider   string `json:"provider"`
	Ciphertext string `json:"ciphertext"` // base64 of nonce+ciphertext
	UpdatedAt  int64  `json:"updated_at"`
}

// File schema: { "version": 1, "entries": { "doubao": {...}, ... } }
type keyringFile struct {
	Version int                  `json:"version"`
	Entries map[string]KeyEntry  `json:"entries"`
}

// NewKeyring 打开/创建 keyring
func NewKeyring(path string, masterKey []byte) (*Keyring, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("master key must be 32 bytes")
	}
	// 确保父目录存在
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	// 文件不存在则创建空文件
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		empty := keyringFile{Version: 1, Entries: map[string]KeyEntry{}}
		if err := saveKeyring(path, &empty); err != nil {
			return nil, err
		}
	}
	return &Keyring{path: path, key: masterKey}, nil
}

func saveKeyring(path string, f *keyringFile) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func loadKeyring(path string) (*keyringFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f keyringFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	if f.Entries == nil {
		f.Entries = map[string]KeyEntry{}
	}
	return &f, nil
}

// Put 存一个 provider 的 key（plaintext）
func (k *Keyring) Put(provider, plaintext string) error {
	_, err := k.PutWithMeta(provider, plaintext)
	return err
}

// PutWithMeta 存并返回 entry（含 updated_at）
func (k *Keyring) PutWithMeta(provider, plaintext string) (KeyEntry, error) {
	cipher, err := Encrypt(k.key, []byte(plaintext))
	if err != nil {
		return KeyEntry{}, err
	}
	encoded := base64Encode(cipher)

	k.mu.Lock()
	defer k.mu.Unlock()
	f, err := loadKeyring(k.path)
	if err != nil {
		return KeyEntry{}, err
	}
	entry := KeyEntry{
		Provider:   provider,
		Ciphertext: encoded,
		UpdatedAt:  nowUnix(),
	}
	f.Entries[provider] = entry
	if err := saveKeyring(k.path, f); err != nil {
		return KeyEntry{}, err
	}
	return entry, nil
}

// Get 取一个 provider 的明文 key
// 未配置返回 ErrKeyNotFound
func (k *Keyring) Get(provider string) (string, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	f, err := loadKeyring(k.path)
	if err != nil {
		return "", err
	}
	entry, ok := f.Entries[provider]
	if !ok {
		return "", ErrKeyNotFound
	}
	cipher, err := base64Decode(entry.Ciphertext)
	if err != nil {
		return "", err
	}
	plain, err := Decrypt(k.key, cipher)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// Delete 删一个 provider 的 key
func (k *Keyring) Delete(provider string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	f, err := loadKeyring(k.path)
	if err != nil {
		return err
	}
	if _, ok := f.Entries[provider]; !ok {
		return ErrKeyNotFound
	}
	delete(f.Entries, provider)
	return saveKeyring(k.path, f)
}

// Has 检查 provider 是否已配 key
func (k *Keyring) Has(provider string) bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	f, err := loadKeyring(k.path)
	if err != nil {
		return false
	}
	_, ok := f.Entries[provider]
	return ok
}

// List 列出已配 key 的 provider
func (k *Keyring) List() []string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	f, err := loadKeyring(k.path)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(f.Entries))
	for n := range f.Entries {
		names = append(names, n)
	}
	return names
}

// ErrKeyNotFound provider 未配置 key
var ErrKeyNotFound = errors.New("key not found")
