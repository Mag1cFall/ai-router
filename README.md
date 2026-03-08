<div align="center">

# AI-Router

**OpenAI · Claude · Gemini — 三进三出**

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go)](https://go.dev)

[中文](README.md) | [English](README_EN.md)

</div>

---

高性能 AI API 协议路由器。接收三种协议中的任意一种，按模型名路由，双向自动转译。

| 客户端 ↓ / 后端 → | OpenAI | Claude | Gemini |
|---|---|---|---|
| **OpenAI** | 直通 | ✅ | ✅ |
| **Claude** | ✅ | 直通 | ✅ |
| **Gemini** | ✅ | ✅ | 直通 |

## 使用

```bash
cd api
cp config.example.yaml config.yaml   # 填入 API key
go run ./cmd/server                   # :8446
```

```bash
# OpenAI 协议 → Claude 后端
curl localhost:8446/v1/chat/completions \
  -d '{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"你好"}]}'

# Claude 协议 → Gemini 后端
curl localhost:8446/v1/messages \
  -d '{"model":"gemini-3-pro-preview","max_tokens":1024,"messages":[{"role":"user","content":"你好"}]}'

# Gemini 协议 → OpenAI 后端
curl localhost:8446/v1beta/models/gpt-5.4:generateContent \
  -d '{"contents":[{"role":"user","parts":[{"text":"你好"}]}]}'
```

## 端点

| 协议 | 路径 |
|---|---|
| OpenAI | `/v1/chat/completions` |
| Claude | `/v1/messages` |
| Gemini | `/v1beta/models/{model}:generateContent` |
| Gemini 流式 | `/v1beta/models/{model}:streamGenerateContent` |

## 路线图

- [ ] SSE 跨协议流式转译
- [ ] 连接池 & 并发优化
- [ ] Vue TS 管理面板
- [ ] Docker / K8s / CI-CD
- [ ] 指标 & 日志
- [ ] 负载均衡 & 权重路由
- [ ] 多租户 & 限流

