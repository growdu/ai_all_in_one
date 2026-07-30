package storage

import (
	"testing"
	"time"
)

func newTestRepoStore(t *testing.T) *FileStore {
	t.Helper()
	dir := t.TempDir()
	return NewFileStore(
		dir+"/files",
		dir+"/file_index.json",
		[]byte("0123456789abcdef0123456789abcdef"),
	)
}

func TestConvRepo_CreateListGet(t *testing.T) {
	fs := newTestRepoStore(t)
	repo := NewConvRepo(fs)

	// 初始：空
	list, _ := repo.List("user1", 10, 0)
	if len(list) != 0 {
		t.Errorf("initial list = %d", len(list))
	}

	// 新建 3 个
	c1, _ := repo.Create("user1", "mock-echo")
	time.Sleep(10 * time.Millisecond)
	c2, _ := repo.Create("user1", "gpt-4o")
	time.Sleep(10 * time.Millisecond)
	repo.Create("user2", "doubao-1-5-pro-32k")

	// user1 应看到 2 个
	list, _ = repo.List("user1", 10, 0)
	if len(list) != 2 {
		t.Errorf("user1 list = %d, want 2", len(list))
	}

	// 按 updated_at 倒序：c2 在前
	if list[0].ID != c2.ID {
		t.Errorf("list[0] = %s, want %s", list[0].ID, c2.ID)
	}

	// Get
	got, err := repo.Get(c1.ID, "user1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != c1.ID {
		t.Errorf("got = %s", got.ID)
	}

	// 跨用户不能 Get
	_, err = repo.Get(c1.ID, "user2")
	if err == nil {
		t.Error("user2 should not access user1's conv")
	}
}

func TestConvRepo_UpdateTitle(t *testing.T) {
	fs := newTestRepoStore(t)
	repo := NewConvRepo(fs)
	c, _ := repo.Create("user1", "x")

	if err := repo.UpdateTitle(c.ID, "user1", "新标题"); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.Get(c.ID, "user1")
	if got.Title != "新标题" {
		t.Errorf("title = %q", got.Title)
	}
}

func TestConvRepo_Delete(t *testing.T) {
	fs := newTestRepoStore(t)
	repo := NewConvRepo(fs)
	c, _ := repo.Create("user1", "x")
	msgRepo := NewMsgRepo(fs)
	msgRepo.Append(c.ID, "msg-1", "user", "", nil)
	msgRepo.Append(c.ID, "msg-2", "assistant", "hello", nil)

	if err := repo.Delete(c.ID, "user1"); err != nil {
		t.Fatal(err)
	}
	_, err := repo.Get(c.ID, "user1")
	if err == nil {
		t.Error("expected error after delete")
	}
	// 消息应也被删（级联）
	_, _, err = msgRepo.ListByConv(c.ID, "user1")
	if err == nil {
		t.Error("messages should be deleted too")
	}
}

func TestConvRepo_DeleteWrongOwner(t *testing.T) {
	fs := newTestRepoStore(t)
	repo := NewConvRepo(fs)
	c, _ := repo.Create("user1", "x")

	err := repo.Delete(c.ID, "user2")
	if err == nil {
		t.Error("user2 should not delete user1's conv")
	}
}

func TestConvRepo_Pin(t *testing.T) {
	fs := newTestRepoStore(t)
	repo := NewConvRepo(fs)
	c, _ := repo.Create("user1", "x")

	if err := repo.Pin(c.ID, "user1", true); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.Get(c.ID, "user1")
	if !got.Pinned {
		t.Error("should be pinned")
	}

	if err := repo.Pin(c.ID, "user1", false); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.Get(c.ID, "user1")
	if got.Pinned {
		t.Error("should be unpinned")
	}
}

func TestMsgRepo_AppendList(t *testing.T) {
	fs := newTestRepoStore(t)
	convRepo := NewConvRepo(fs)
	msgRepo := NewMsgRepo(fs)

	c, _ := convRepo.Create("user1", "x")
	msgRepo.Append(c.ID, "msg-1", "user", "hi", nil)
	msgRepo.Append(c.ID, "msg-2", "assistant", "hello", nil)
	msgRepo.Append(c.ID, "msg-3", "user", "how are you?", []string{"file_xxx"})

	msgs, _, err := msgRepo.ListByConv(c.ID, "user1")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Errorf("msgs = %d, want 3", len(msgs))
	}
	if msgs[0].Content != "hi" {
		t.Errorf("msgs[0] = %q", msgs[0].Content)
	}
	if msgs[2].Attachments[0] != "file_xxx" {
		t.Errorf("attachments lost")
	}
}

func TestMsgRepo_Ownership(t *testing.T) {
	fs := newTestRepoStore(t)
	convRepo := NewConvRepo(fs)
	msgRepo := NewMsgRepo(fs)

	c, _ := convRepo.Create("user1", "x")
	msgRepo.Append(c.ID, "m", "user", "x", nil)

	// user2 应当不能读
	_, _, err := msgRepo.ListByConv(c.ID, "user2")
	if err == nil {
		t.Error("user2 should not see user1's messages")
	}
}
