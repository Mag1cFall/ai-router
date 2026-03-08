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

## 快速开始

### 1. 配置

```bash
cd api
cp config.example.yaml config.yaml
```

编辑 `config.yaml`，填入你的 provider：

```yaml
providers:
  - name: claude-aws
    protocol: claude
    endpoint: https://api.anthropic.com
    api_key: sk-ant-xxx

  - name: openai
    protocol: openai
    endpoint: https://api.openai.com/v1
    api_key: sk-xxx

routes:
  - match_model: "claude-*"
    provider: claude-aws
  - match_model: "gpt-*"
    provider: openai

server:
  port: 8446
```

### 2. 启动后端

```bash
cd api
go run ./cmd/server
# ✅ server listening on :8446 providers=claude-aws(claude),openai(openai)
```

### 3. 启动前端（可选）

```bash
cd web
npm install
npm run dev
# ✅ http://localhost:5173
```

前端是暗色管理面板，包含：
- **Provider 列表** — 显示已配置的后端提供商
- **Route 规则** — 模型匹配规则
- **实时请求日志** — 每条请求的协议徽章、模型、状态码、延迟

> 前端自动把 `/api` 代理到后端 `:8446`，开箱即用。

### 4. 发送请求

路由器监听 `:8446`，根据路径自动检测协议，根据模型名路由到后端：

```bash
# 用 OpenAI 协议调 Claude 模型（自动转译）
curl localhost:8446/v1/chat/completions \
  -d '{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"你好"}]}'

# 用 Claude 协议调 Claude 模型（直通）
curl localhost:8446/v1/messages \
  -d '{"model":"claude-sonnet-4-6","max_tokens":1024,"messages":[{"role":"user","content":"你好"}]}'

# 用 Gemini 协议调 OpenAI 模型（自动转译）
curl localhost:8446/v1beta/models/gpt-5.4:generateContent \
  -d '{"contents":[{"role":"user","parts":[{"text":"你好"}]}]}'
```

### 5. 集成测试

```bash
# 启动后端后，运行端到端测试：
go run tests/e2e/main.go
```

### 6. Docker 部署

```bash
# 构建并启动
docker compose up -d

# 仅后端
docker compose up api -d

# 挂载自定义配置
# 编辑 api/config.yaml 后：
docker compose up api -d
```

## 端点

| 协议 | 路径 |
|---|---|
| OpenAI | `/v1/chat/completions` |
| Claude | `/v1/messages` |
| Gemini | `/v1beta/models/{model}:generateContent` |
| Gemini 流式 | `/v1beta/models/{model}:streamGenerateContent` |

**管理 API**

| 路径 | 说明 |
|---|---|
| `GET /healthz` | 健康检查 |
| `GET /api/providers` | 已配置的 provider 列表 |
| `GET /api/routes` | 路由规则 |
| `GET /api/logs` | 最近请求日志 |

## 路线图

- [x] 3×3 协议全矩阵转译
- [x] SSE 跨协议流式转译
- [x] Thinking / Reasoning 双向转译
- [x] 连接池 & 并发优化
- [x] Vue TS 管理面板
- [x] Docker & CI/CD
- [ ] 负载均衡 & 权重路由
- [ ] 多租户 & 限流
