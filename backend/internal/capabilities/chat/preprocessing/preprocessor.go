// Package preprocessing 实现了 docs/architecture/02-input-processing.md §三
// 描述的附件预处理：把 file_id 解析为内容，注入到 message 中。
package preprocessing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/growdu/ai_all_in_one/backend/internal/core"
	"github.com/growdu/ai_all_in_one/backend/internal/storage"
)

// ProcessedRequest 预处理后的请求
type ProcessedRequest struct {
	Messages  []core.ChatMessage
	Info      *core.ProcessingInfo
}

// Preprocessor 附件预处理器
type Preprocessor struct {
	store       *storage.FileStore
	defaultUser string
}

// NewPreprocessor 创建 Preprocessor
func NewPreprocessor(store *storage.FileStore, defaultUser string) *Preprocessor {
	return &Preprocessor{store: store, defaultUser: defaultUser}
}

// Process 预处理（用 defaultUser）
func (p *Preprocessor) Process(ctx context.Context, req core.ChatRequest) (*ProcessedRequest, *core.ProcessingInfo, error) {
	return p.ProcessFor(ctx, req, p.defaultUser)
}

// ProcessFor 预处理（指定 owner）
func (p *Preprocessor) ProcessFor(ctx context.Context, req core.ChatRequest, ownerID string) (*ProcessedRequest, *core.ProcessingInfo, error) {
	info := &core.ProcessingInfo{}
	out := core.ChatRequest{
		Model:    req.Model,
		Messages: make([]core.ChatMessage, len(req.Messages)),
	}
	copy(out.Messages, req.Messages)

	for i, msg := range out.Messages {
		if len(msg.Attachments) == 0 {
			continue
		}
		// 收集 file 内容
		var fileBlocks []string
		for _, fileID := range msg.Attachments {
			meta, data, err := p.store.GetMetaAndContent(fileID, ownerID)
			if err != nil {
				// owner 不匹配 / 文件不存在 → 警告但不阻塞
				info.Attachments = append(info.Attachments, core.AttachmentInfo{
					FileID:   fileID,
					Type:     "unknown",
					Warnings: []string{"file not found or access denied"},
				})
				continue
			}
			// 1.0 简化：所有文件按文本注入（图片标注 mime + 占位）
			block := formatFileBlock(meta, data)
			fileBlocks = append(fileBlocks, block)

			info.Attachments = append(info.Attachments, core.AttachmentInfo{
				FileID:        meta.ID,
				Type:          meta.MimeType,
				OriginalSize:  int(meta.Size),
				ProcessedSize: int(meta.ProcessedSize),
				Truncated:     meta.Truncated,
			})
		}

		// 注入：附件内容 + 用户原内容
		// 格式：
		//   <附件内容>
		//
		//   ---
		//
		//   <原用户消息>
		if len(fileBlocks) > 0 {
			var b strings.Builder
			b.WriteString(strings.Join(fileBlocks, "\n\n"))
			b.WriteString("\n\n---\n\n")
			b.WriteString(msg.Content)
			out.Messages[i].Content = b.String()
		}
	}
	return &ProcessedRequest{
		Messages: out.Messages,
		Info:     info,
	}, info, nil
}

// formatFileBlock 格式化单文件块
func formatFileBlock(meta *storage.FileMeta, data []byte) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[附件: %s (%s, %d 字节", meta.Filename, meta.MimeType, meta.Size)
	if meta.Truncated {
		fmt.Fprintf(&b, " → 截断到 %d 字节", meta.ProcessedSize)
	}
	b.WriteString(")]\n")
	if isImage(meta.MimeType) {
		// 1.0 简化：图片不能塞给所有模型
		// 标注 mime，前端可将来升级为 vision 直传
		fmt.Fprintf(&b, "  类型: image（1.0 阶段未做 vision 抽取，请用户描述图片内容）\n")
	} else {
		// 文本类（text/*、application/pdf、application/json）
		b.WriteString(string(data))
	}
	return b.String()
}

func isImage(mime string) bool { return strings.HasPrefix(mime, "image/") }

// 避免 unused import
var _ = time.Now
