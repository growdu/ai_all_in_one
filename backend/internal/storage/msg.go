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

// Message 消息
type Message struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	Attachments    []string  `json:"attachments,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// MsgRepo 消息仓储
type MsgRepo struct {
	store *FileStore
	mu    sync.RWMutex
}

func NewMsgRepo(store *FileStore) *MsgRepo {
	return &MsgRepo{store: store}
}

// Append 追加一条消息
func (r *MsgRepo) Append(convID, ownerID, role, content string, attachments []string) (*Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := &Message{
		ID:             generateMsgID(),
		ConversationID: convID,
		Role:           role,
		Content:        content,
		Attachments:    attachments,
		CreatedAt:      time.Now(),
	}
	if err := r.persist(m); err != nil {
		return nil, err
	}
	_ = ownerID // 1.0 简化：owner 校验在 ListByConv 时做
	return m, nil
}

// ListByConv 列出某会话的所有消息
func (r *MsgRepo) ListByConv(convID, ownerID string) ([]*Message, *Conversation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. 校验 conv 归属
	convRepo := NewConvRepo(r.store)
	conv, err := convRepo.Get(convID, ownerID)
	if err != nil {
		return nil, nil, err
	}

	// 2. 列消息
	all, err := r.readAll()
	if err != nil {
		return nil, nil, err
	}
	var mine []*Message
	for _, m := range all {
		if m.ConversationID == convID {
			mine = append(mine, m)
		}
	}
	sort.Slice(mine, func(i, j int) bool {
		return mine[i].CreatedAt.Before(mine[j].CreatedAt)
	})
	return mine, conv, nil
}

func (r *MsgRepo) deleteByConv(convID, ownerID string) error {
	all, err := r.readAll()
	if err != nil {
		return err
	}
	var kept []*Message
	for _, m := range all {
		if m.ConversationID == convID {
			continue // 删
		}
		kept = append(kept, m)
	}
	return r.writeAll(kept)
}

// ---- 内部 ----

func (r *MsgRepo) persist(m *Message) error {
	all, err := r.readAll()
	if err != nil {
		return err
	}
	found := false
	for i, existing := range all {
		if existing.ID == m.ID {
			all[i] = m
			found = true
			break
		}
	}
	if !found {
		all = append(all, m)
	}
	return r.writeAll(all)
}

func (r *MsgRepo) readAll() ([]*Message, error) {
	data, err := os.ReadFile(r.msgIndexPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Messages []*Message `json:"_msgs"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Messages, nil
}

func (r *MsgRepo) writeAll(all []*Message) error {
	wrapper := struct {
		Messages []*Message `json:"_msgs"`
	}{Messages: all}
	data, _ := json.MarshalIndent(wrapper, "", "  ")
	return os.WriteFile(r.msgIndexPath(), data, 0o600)
}

func (r *MsgRepo) msgIndexPath() string {
	return filepath.Join(filepath.Dir(r.store.index), "msg_index.json")
}

func generateMsgID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "msg_" + hex.EncodeToString(b[:])
}
