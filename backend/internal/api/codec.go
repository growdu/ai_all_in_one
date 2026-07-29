package api

import (
	"encoding/json"
	"io"
)

// jsonEncode 内部 helper（避免与别处 json 包名冲突）
func jsonEncode(w io.Writer, v any) error {
	return json.NewEncoder(w).Encode(v)
}
