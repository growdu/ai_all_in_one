package security

import (
	"encoding/base64"
	"time"
)

// base64Encode 编码为 base64（标准）
func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// base64Decode 解码
func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// nowUnix 当前 unix 时间（秒）
func nowUnix() int64 {
	return time.Now().Unix()
}
