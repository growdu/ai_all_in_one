package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ErrConvNotFound 会话不存在
var ErrConvNotFound = errors.New("conv not found")

// ConvRepo 会话仓储
type ConvRepo struct {
	store *FileStore
	mu    sync.RWMutex
}

// Conversation 会话
type Conversation struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"owner_user_id"`
	Title     string    `json:"title"`
	Model     string    `json:"model"`
	Mode      string    `json:"mode"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Pinned    bool      `json:"pinned"`
}

func NewConvRepo(store *FileStore) *ConvRepo {
	return &ConvRepo{store: store}
}

func (r *ConvRepo) Create(ownerID, model string) (*Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := &Conversation{
		ID:        generateConvID(),
		OwnerID:   ownerID,
		Title:     "新对话",
		Model:     model,
		Mode:      "single",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := r.persist(c); err != nil {
		return nil, err
	}
	return c, nil
}

func (r *ConvRepo) Get(id, ownerID string) (*Conversation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.getByID(id, ownerID)
}

func (r *ConvRepo) List(ownerID string, limit, offset int) ([]*Conversation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	all, err := r.readAll()
	if err != nil {
		return nil, err
	}
	var mine []*Conversation
	for _, c := range all {
		if c.OwnerID == ownerID {
			mine = append(mine, c)
		}
	}
	// 置顶优先 + 按 updated_at 倒序
	sort.Slice(mine, func(i, j int) bool {
		if mine[i].Pinned != mine[j].Pinned {
			return mine[i].Pinned
		}
		return mine[i].UpdatedAt.After(mine[j].UpdatedAt)
	})
	if offset >= len(mine) {
		return nil, nil
	}
	end := offset + limit
	if end > len(mine) {
		end = len(mine)
	}
	return mine[offset:end], nil
}

func (r *ConvRepo) UpdateTitle(id, ownerID, title string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, err := r.getByID(id, ownerID)
	if err != nil {
		return err
	}
	c.Title = title
	c.UpdatedAt = time.Now()
	return r.persist(c)
}

func (r *ConvRepo) Pin(id, ownerID string, pinned bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, err := r.getByID(id, ownerID)
	if err != nil {
		return err
	}
	c.Pinned = pinned
	c.UpdatedAt = time.Now()
	return r.persist(c)
}

func (r *ConvRepo) Delete(id, ownerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	all, err := r.readAll()
	if err != nil {
		return err
	}
	found := false
	for i, c := range all {
		if c.ID == id {
			if c.OwnerID != ownerID {
				return ErrConvNotFound
			}
			all = append(all[:i], all[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return ErrConvNotFound
	}
	if err := r.writeAll(all); err != nil {
		return err
	}
	msgRepo := NewMsgRepo(r.store)
	return msgRepo.deleteByConv(id, ownerID)
}

// ---- 内部 ----

func (r *ConvRepo) getByID(id, ownerID string) (*Conversation, error) {
	all, err := r.readAll()
	if err != nil {
		return nil, err
	}
	for _, c := range all {
		if c.ID == id && c.OwnerID == ownerID {
			return c, nil
		}
	}
	return nil, ErrConvNotFound
}

func (r *ConvRepo) readAll() ([]*Conversation, error) {
	data, err := os.ReadFile(r.convIndexPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Conversations []*Conversation `json:"_convs"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Conversations, nil
}

func (r *ConvRepo) writeAll(all []*Conversation) error {
	wrapper := struct {
		Conversations []*Conversation `json:"_convs"`
	}{Conversations: all}
	data, _ := json.MarshalIndent(wrapper, "", "  ")
	return os.WriteFile(r.convIndexPath(), data, 0o600)
}

func (r *ConvRepo) persist(c *Conversation) error {
	all, err := r.readAll()
	if err != nil {
		return err
	}
	found := false
	for i, existing := range all {
		if existing.ID == c.ID {
			all[i] = c
			found = true
			break
		}
	}
	if !found {
		all = append(all, c)
	}
	return r.writeAll(all)
}

func (r *ConvRepo) convIndexPath() string {
	return filepath.Join(filepath.Dir(r.store.index), "conv_index.json")
}

func generateConvID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "conv_" + hex.EncodeToString(b[:])
}
