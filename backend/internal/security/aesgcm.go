// Package security 提供加密、Key 管理、JWT 等安全原语。
//
// 1.0 阶段：
//   - AES-256-GCM：用户 Key 加密落库
//   - Keyring：用户 Key CRUD 封装
//   - JWT：用户 token（2.0 完善，1.0 简化）
//
// 详见 docs/backend/02-provider.md §八。
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// Encrypt 用 AES-256-GCM 加密 plaintext
// 输出格式：nonce(12B) || ciphertext || tag(16B)
func Encrypt(key, plaintext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes (got %d)", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("read nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	// 拼成 nonce + ciphertext（GCM.Seal 已包含 tag）
	return append(nonce, ciphertext...), nil
}

// Decrypt 用 AES-256-GCM 解密
// 输入格式：nonce(12B) || ciphertext || tag(16B)
func Decrypt(key, ciphertext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes (got %d)", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	ns := gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, errors.New("ciphertext too short")
	}
	nonce := ciphertext[:ns]
	body := ciphertext[ns:]
	plaintext, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	return plaintext, nil
}

// DeriveKey 从 passphrase 派生 32 字节 key
// 1.0 阶段保留接口；当前实现直接返回（待 2.0 接 scrypt/argon2）
func DeriveKey(passphrase string) ([]byte, error) {
	// 1.0 简化：直接截断/补齐到 32 字节
	// 警告：这不是真正的 KDF，2.0 必须替换
	if len(passphrase) == 0 {
		return nil, errors.New("empty passphrase")
	}
	key := make([]byte, 32)
	copy(key, passphrase)
	return key, nil
}
