package security

import (
	"bytes"
	"testing"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef") // 32 bytes
	plaintext := []byte("sk-doubao-12345-secret")

	cipher, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(cipher, plaintext) {
		t.Error("ciphertext should differ from plaintext")
	}
	if len(cipher) < len(plaintext) {
		t.Error("ciphertext should be >= plaintext (nonce + tag)")
	}

	got, err := Decrypt(key, cipher)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("decrypted = %q, want %q", got, plaintext)
	}
}

func TestEncrypt_EmptyPlaintext(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	cipher, err := Encrypt(key, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decrypt(key, cipher)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := []byte("0123456789abcdef0123456789abcdef")
	key2 := []byte("fedcba9876543210fedcba9876543210")
	cipher, _ := Encrypt(key1, []byte("secret"))

	_, err := Decrypt(key2, cipher)
	if err == nil {
		t.Error("expected error when decrypting with wrong key")
	}
}

func TestDeriveKey_FromPassphrase(t *testing.T) {
	// 1.0 阶段：用户主密钥是 32 字节 hex/base64
	// 简化版：直接用 []byte 作为 key，不做 KDF
	// 1.0 留作 future
	_ = DeriveKey
}

func TestEncrypt_NonceUnique(t *testing.T) {
	// 同一明文 + 同一 key 两次加密应得到不同密文（GCM nonce 随机）
	key := []byte("0123456789abcdef0123456789abcdef")
	plain := []byte("hello")
	c1, _ := Encrypt(key, plain)
	c2, _ := Encrypt(key, plain)
	if bytes.Equal(c1, c2) {
		t.Error("nonce should be random per encryption")
	}
}
