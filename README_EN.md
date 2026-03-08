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

## Quick Start

### 1. Configure

```bash
cd api
cp config.example.yaml config.yaml
# Edit config.yaml — fill in your API keys
```

### 2. Start Backend

```bash
cd api
go run ./cmd/server
# ✅ server listening on :8446
```

### 3. Start Dashboard (optional)

```bash
cd web
npm install
npm run dev
# ✅ http://localhost:5173
```

### 4. Send Requests

```bash
# OpenAI protocol → Claude backend (auto-translate)
curl localhost:8446/v1/chat/completions \
  -d '{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"Hello"}]}'

# Claude protocol → Claude backend (passthrough)
curl localhost:8446/v1/messages \
  -d '{"model":"claude-sonnet-4-6","max_tokens":1024,"messages":[{"role":"user","content":"Hello"}]}'
```

### 5. Integration Test

```bash
go run tests/e2e/main.go
```

### 6. Docker

```bash
docker compose up -d
```

## Endpoints

| Protocol | Path |
|---|---|
| OpenAI | `/v1/chat/completions` |
| Claude | `/v1/messages` |
| Gemini | `/v1beta/models/{model}:generateContent` |

**Management API**: `/healthz` · `/api/providers` · `/api/routes` · `/api/logs`

## Roadmap

- [x] 3×3 protocol matrix translation
- [x] SSE cross-protocol streaming
- [x] Thinking / Reasoning bidirectional
- [x] Connection pool & concurrency
- [x] Vue TS dashboard
- [x] Docker & CI/CD
- [ ] Load balancing & weighted routing
- [ ] Multi-tenancy & rate limiting
