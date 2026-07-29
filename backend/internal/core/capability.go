// Package core 定义跨角色共享的核心抽象。
//
// 详细设计见 ../../../docs/backend/02-provider.md §二 与
// ../../../docs/architecture/00-overview.md。
package core

import "fmt"

// Modality 模态枚举：用户可见的功能大类。
// 详见 docs/architecture/00-overview.md。
type Modality string

const (
	ModalityChat     Modality = "chat"
	ModalityMusic    Modality = "music"
	ModalityVideo    Modality = "video"
	ModalityImage    Modality = "image"
	ModalityTTS      Modality = "tts"
	ModalityEmbedding Modality = "embedding"
)

// Valid 判断 m 是否为已知模态
func (m Modality) Valid() bool {
	switch m {
	case ModalityChat, ModalityMusic, ModalityVideo, ModalityImage, ModalityTTS, ModalityEmbedding:
		return true
	}
	return false
}

// String 实现 fmt.Stringer
func (m Modality) String() string {
	return string(m)
}

// ParseModality 从字符串解析模态，未知返回错误
func ParseModality(s string) (Modality, error) {
	m := Modality(s)
	if !m.Valid() {
		return "", fmt.Errorf("unknown modality: %q", s)
	}
	return m, nil
}
