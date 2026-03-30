// 入站请求协议识别：根据 HTTP 方法、路径和请求体判断 OpenAI/Claude/Gemini
package detect

import (
	"encoding/json"
	"strings"

	"github.com/Mag1cFall/ai-router/api/internal/config"
	"github.com/tidwall/gjson"
)

type Protocol = config.ProviderProtocol

const (
	ProtocolUnknown = config.ProtocolUnknown
	ProtocolOpenAI  = config.ProtocolOpenAI
	ProtocolClaude  = config.ProtocolClaude
	ProtocolGemini  = config.ProtocolGemini
)

// FromRequest 根据请求方法、路径和请求体内容判断入站协议
func FromRequest(method, path string, body []byte) Protocol {
	path = normalizePath(path)

	switch {
	case method == "POST" && (path == "/v1/chat/completions" || path == "/v1/responses"):
		if len(body) == 0 {
			return ProtocolUnknown
		}
		return ProtocolOpenAI
	case method == "GET" && path == "/v1/models":
		return ProtocolOpenAI
	case method == "POST" && path == "/v1/messages":
		return ProtocolClaude
	case method == "POST" && isGeminiPath(path):
		return ProtocolGemini
	default:
		return ProtocolUnknown
	}
}

// ExtractModelName 从请求路径（Gemini）或请求体 model 字段提取模型名
func ExtractModelName(path string, body []byte) string {
	path = normalizePath(path)
	if isGeminiPath(path) {
		prefix := "/models/"
		idx := strings.Index(path, prefix)
		if idx < 0 {
			return ""
		}
		rest := path[idx+len(prefix):]
		colon := strings.Index(rest, ":")
		if colon < 0 {
			return ""
		}
		return rest[:colon]
	}

	if len(body) == 0 || !json.Valid(body) {
		return ""
	}

	return gjson.GetBytes(body, "model").String()
}

// IsStreaming 判断请求体是否开启流式输出
func IsStreaming(body []byte) bool {
	if len(body) == 0 || !json.Valid(body) {
		return false
	}
	return gjson.GetBytes(body, "stream").Bool()
}

// normalizePath 删除查询参数并去除末尾断斜杠
func normalizePath(path string) string {
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}
	if path == "" {
		return "/"
	}
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
		if path == "" {
			return "/"
		}
	}
	return path
}

// isGeminiPath 判断路径是否为 Gemini generateContent 或 streamGenerateContent
func isGeminiPath(path string) bool {
	if !strings.HasPrefix(path, "/v1beta/models/") && !strings.HasPrefix(path, "/v1/models/") {
		return false
	}
	idx := strings.Index(path, "/models/")
	if idx < 0 {
		return false
	}
	rest := path[idx+len("/models/"):]
	colon := strings.Index(rest, ":")
	if colon <= 0 {
		return false
	}
	action := rest[colon+1:]
	return action == "generateContent" || action == "streamGenerateContent"
}
