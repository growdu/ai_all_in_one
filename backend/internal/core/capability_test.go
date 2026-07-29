package core

import "testing"

func TestModality_Valid(t *testing.T) {
	cases := []struct {
		m    Modality
		want bool
	}{
		{ModalityChat, true},
		{Modality("xxx"), false},
		{"", false},
	}
	for _, c := range cases {
		if got := c.m.Valid(); got != c.want {
			t.Errorf("Modality(%q).Valid() = %v, want %v", c.m, got, c.want)
		}
	}
}

func TestModality_String(t *testing.T) {
	if ModalityChat.String() != "chat" {
		t.Errorf("ModalityChat.String() = %q, want chat", ModalityChat.String())
	}
}
