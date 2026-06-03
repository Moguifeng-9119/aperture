<div align="center">

# Aperture

**Intelligent Multi-Model LLM Routing Gateway**

[![CI](https://github.com/Moguifeng-9119/aperture/actions/workflows/ci.yml/badge.svg)](https://github.com/Moguifeng-9119/aperture/actions)
[![Release](https://img.shields.io/github/v/release/Moguifeng-9119/aperture?color=6366f1)](https://github.com/Moguifeng-9119/aperture/releases)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)
[![中文文档](https://img.shields.io/badge/README-中文-red)](README_CN.md)

*Route every request to the right model. Save 40–70% on API costs. Zero code changes.*

</div>

---

<h3 align="center">💡 The Problem</h3>

Your app sends **every request** to the same expensive model. "Hello" costs the same as "Write a production-grade auth system in Rust."

<h3 align="center">🔆 Aperture's Solution</h3>

Aperture sits between your app and LLM providers, analyzes each request, and routes it to the **optimal model**:

```
"Hello!"           →  Groq Llama 3.1 8B       ($0.05 / 1M tokens)
"Explain this SQL" →  Claude 3 Haiku         ($0.25 / 1M tokens)
"Write an API"     →  GPT-4o                 ($10.00 / 1M tokens)
```

Every decision is **explained** in response headers. You can trust it — or override it.

<div align="center">

| Tier 1: Rules | Tier 2: Embeddings | Tier 3: ML |
|:---:|:---:|:---:|
| `< 0.1ms` | `~200ms` | `< 1ms` |
| ~80% accuracy | ~90% accuracy | ~95% accuracy |
| Built in, zero config | Set `OPENAI_API_KEY` | Train your own ONNX model |

*Tiers cascade. Start with Tier 1. Add 2 and 3 when you need more accuracy.*

</div>

---

## ⚖️ Why not LiteLLM / Portkey / OneAPI?

| | Aperture | LiteLLM | Portkey | OneAPI |
|---|:---:|:---:|:---:|:---:|
| **Auto routing** | ✓ 3-tier (rules+embed+ML) | ✗ | ✗ | ✗ |
| **Explainable decisions** | ✓ `X-Aperture-Reason` | ✗ | ✗ | ✗ |
| **Cost dashboard** | ✓ built-in | ✗ | ✓ cloud only | ✗ |
| **A/B testing** | ✓ | ✗ | ✗ | ✗ |
| **Single binary** | ✓ Go, ~15MB | ✗ Python | ✗ cloud | ✗ Node.js |
| **Self-hosted** | ✓ | ✓ | ✗ | ✓ |
| **Training data export** | ✓ JSONL | ✗ | ✗ | ✗ |
| **ARM64 builds** | ✓ | ✗ | ✗ | ✗ |
| **License** | MIT | MIT | Apache 2.0 | MIT |

> Aperture is the only gateway that **decides which model to use** — others just proxy to a pre-configured model.

---

## 🚀 Quick Start

> 📖 **配置教程**：[中文](docs/guide.md) · [English](docs/guide-en.md) — 覆盖所有模型接入、多模型混合路由、SDK 集成

### 启动后打开

| 地址 | 用途 |
|------|------|
| `http://localhost:8080/dashboard` | 📊 实时仪表盘 — 请求量、费用、延迟、模型分布 |
| `http://localhost:8080/health` | ✅ 健康检查 → `{"status":"ok"}` |
| `http://localhost:8080/metrics` | 📈 Prometheus 指标 |
| `http://localhost:8080/v1/chat/completions` | 🤖 OpenAI 格式 API |
| `http://localhost:8080/v1/messages` | 🤖 Anthropic 格式 API（Claude Code 用） |

```bash
git clone https://github.com/Moguifeng-9119/aperture.git
cd aperture
make build

# 生成配置（交互式问答，推荐）
./aperture -setup

# 或手动写配置
cp config.example.yaml config.yaml
# 编辑 config.yaml，填 API Key
```

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Write a function to parse JSON"}]}'
```

Check which model was chosen:

```bash
curl -sI localhost:8080/v1/chat/completions -d '{"model":"auto","messages":[{"role":"user","content":"Hello!"}]}' | grep X-Aperture

# X-Aperture-Model: llama-3.1-8b-instant
# X-Aperture-Provider: groq
# X-Aperture-Reason: rule:greeting matched → trivial → groq/llama-3.1-8b-instant
```

<div align="center">

| Install | Command |
|---------|---------|
| **Go** | `go install github.com/Moguifeng-9119/aperture@latest` |
| **Docker** | `docker compose up -d` |
| **Helm** | `helm install aperture ./deploy/helm/aperture` |
| **Binary** | [Latest release](https://github.com/Moguifeng-9119/aperture/releases/latest) |

</div>

---

<div align="center">
  <a href="https://skillicons.dev">
    <img src="https://skillicons.dev/icons?i=go,docker,kubernetes,prometheus,sqlite,htmx,yaml&theme=light" alt="Tech Stack" />
  </a>
</div>

## 🔧 Tech Stack & Features

| Category | Spec |
|----------|------|
| **Language** | Go 1.25 — single binary, no runtime dependencies |
| **Providers** | OpenAI · Anthropic · Groq · Ollama · DeepSeek V4 · Qwen · Kimi · GLM · MiniMax · MiMo |
| **Routing** | 3-tier: Rules (regex/keyword/token) → Embeddings (OpenAI API) → ML (ONNX) |
| **Storage** | SQLite (WAL mode) — embedded, zero ops |
| **Dashboard** | HTMX + Alpine.js + Chart.js — embedded in binary |
| **Observability** | Prometheus `/metrics` · Structured error codes · Span tracing |
| **Deployment** | Docker · Docker Compose · Helm · goreleaser (6 platforms) |
| **API** | OpenAI-compatible `/v1/chat/completions` — drop-in replacement |

<div align="center">
  <a href="https://github.com/Jurredr/github-widgetbox">
    <img src="https://github-widgetbox.vercel.app/api/skills?tools=git,docker,kubernetes,nginx,redis,aws,vercel&includeNames=true&theme=nautilus" alt="GitHub WidgetBox" />
  </a>
</div>

---

## 📖 API

### OpenAI-compatible
```
POST   /v1/chat/completions    → stream=true supported
GET    /v1/models               → all models + "auto"
GET    /health                  → per-provider health
```

### Admin (requires `X-Admin-Key`)
```
GET    /admin/v1/analytics/summary       ?from=&to=
GET    /admin/v1/analytics/requests      ?page=&per_page=
GET    /admin/v1/analytics/export        → JSONL training data
POST   /admin/v1/routing/test            → { "messages": [...] }
GET    /admin/v1/ab-test/stats           → A/B agreement rate
CRUD   /admin/v1/keys                    → API key management
```

### Observability
```
GET    /metrics                 → Prometheus format
GET    /dashboard               → built-in analytics UI
```

---

## ⚙️ Config

```yaml
server:
  port: 8080

providers:
  - id: openai
    type: openai
    api_key: "${OPENAI_API_KEY}"

routing:
  default_model: gpt-4o-mini
  default_provider: openai
```

Full reference: **[config.example.yaml](config.example.yaml)** — fallback chains, custom rules, rate limits, per-model pricing, embedding settings.

---

## 📊 Project Status

<div align="center">

| Metric | Value |
|--------|-------|
| **Test packages** | 15 |
| **Providers covered** | 4/4 |
| **CI** | test · lint · build · release |
| **Platforms** | linux (amd64/arm64) · darwin (amd64/arm64) · windows (amd64/arm64) |

</div>

## 🗺️ Roadmap

<div align="center">

| ✅ Done | ⬜ Next |
|---------|---------|
| Core proxy + OpenAI adapter | Token-aware cost estimation |
| 4 providers + rule routing + streaming | Multi-tenancy + caching |
| Embeddings + analytics + SQLite | Load testing suite (k6) |
| Dashboard + admin API + Prometheus | ONNX model quality metrics |
| Fallback/retry + rate limiter + Docker Compose | Plugin system (WASM) |
| Real embeddings (OpenAI) + A/B testing | |
| Structured errors + tracing + Helm chart | |
| CI/CD + goreleaser + multi-platform releases | |

</div>

---

## 🛠️ Development

```bash
make build        # → ./aperture
make dev          # go run .
make test         # run all tests
make test-cover   # tests + coverage.html
make lint         # go vet ./...
make docker-build # build Docker image
```

---

---

## Community

🐛 **Bug?** [Open an issue](https://github.com/Moguifeng-9119/aperture/issues/new?template=bug_report.yml) — structured template, takes 30 seconds

💡 **Feature idea?** [Request it](https://github.com/Moguifeng-9119/aperture/issues/new?template=feature_request.yml)

💬 **Questions?** [Discussions](https://github.com/Moguifeng-9119/aperture/discussions) — ask anything

---

<div align="center">

MIT License — use it, fork it, ship it. No strings attached.

</div>
