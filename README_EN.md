<div align="center">

# AI-Router

**OpenAI · Claude · Gemini — Any In, Any Out**

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go)](https://go.dev)

[中文](README.md) | [English](README_EN.md)

</div>

---

High-performance AI API protocol router. Accepts any of the three protocols, routes by model name, translates bidirectionally.

| Client ↓ / Backend → | OpenAI | Claude | Gemini |
|---|---|---|---|
| **OpenAI** | passthrough | ✅ | ✅ |
| **Claude** | ✅ | passthrough | ✅ |
| **Gemini** | ✅ | ✅ | passthrough |

## Usage

```bash
cd api
cp config.example.yaml config.yaml   # fill in your API keys
go run ./cmd/server                   # :8446
```

```bash
# OpenAI protocol → Claude backend
curl localhost:8446/v1/chat/completions \
  -d '{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"Hello"}]}'

# Claude protocol → Gemini backend
curl localhost:8446/v1/messages \
  -d '{"model":"gemini-3-pro-preview","max_tokens":1024,"messages":[{"role":"user","content":"Hello"}]}'

# Gemini protocol → OpenAI backend
curl localhost:8446/v1beta/models/gpt-5.4:generateContent \
  -d '{"contents":[{"role":"user","parts":[{"text":"Hello"}]}]}'
```

## Endpoints

| Protocol | Path |
|---|---|
| OpenAI | `/v1/chat/completions` |
| Claude | `/v1/messages` |
| Gemini | `/v1beta/models/{model}:generateContent` |
| Gemini Stream | `/v1beta/models/{model}:streamGenerateContent` |

## Roadmap

- [ ] SSE cross-protocol streaming translation
- [ ] Connection pool & concurrency optimization
- [ ] Vue TS dashboard
- [ ] Docker / K8s / CI-CD
- [ ] Metrics & logging
- [ ] Load balancing & weighted routing
- [ ] Multi-tenancy & rate limiting

