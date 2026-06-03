# Aperture

Intelligent multi-model LLM routing gateway — route each request to the optimal model based on complexity, saving 40-70% on API costs without sacrificing quality.

[![CI](https://github.com/Moguifeng-9119/aperture/actions/workflows/ci.yml/badge.svg)](https://github.com/Moguifeng-9119/aperture/actions/workflows/ci.yml)
[![Go 1.22+](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Tests](https://img.shields.io/badge/tests-141%20passing-brightgreen)](https://github.com/Moguifeng-9119/aperture/actions)

## Why Aperture?

Every LLM request is different. A simple "hello" doesn't need GPT-4o, and a complex code review shouldn't go to a tiny model. Aperture analyzes each request and routes it to the right model automatically.

**Key differentiators:**
- **Explainable routing** — every decision comes with a reason and cost estimate in response headers
- **Progressive intelligence** — start with fast rule-based routing, upgrade to embeddings and ML as you grow
- **Cost analytics** — built-in dashboard showing real-time cost savings and model usage
- **Single binary** — zero dependencies, deploy anywhere. One `aperture` binary is all you need
- **OpenAI-compatible** — drop-in replacement for `/v1/chat/completions`, works with any OpenAI SDK

## Features

- [x] OpenAI-compatible API (`/v1/chat/completions`, `/v1/models`)
- [x] Multi-provider: OpenAI, Anthropic, Groq, Ollama
- [x] Streaming (SSE) and non-streaming responses
- [x] Tier 1: Rule-based routing (keywords, regex, token count)
- [x] Tier 2: Real embedding routing (OpenAI text-embedding-3-small, keyword vector fallback)
- [x] Tier 3: ML classifier (local ONNX, trainable)
- [x] Built-in admin dashboard (HTMX + Alpine.js + Chart.js)
- [x] Admin API (analytics, API key CRUD, routing test)
- [x] Cost tracking and savings calculation
- [x] Multi-turn conversation context
- [x] API key authentication + rate limiting
- [x] Pipeline fallback chain + retry on provider errors
- [x] A/B testing: compare two routing strategies side-by-side
- [x] Training data export (JSONL format for ML training)
- [x] Docker Compose (aperture + ollama)
- [x] Prometheus metrics endpoint (`/metrics`)
- [x] Graceful shutdown
- [x] Single binary build + Docker image + goreleaser

## Quick Start

### Prerequisites
- Go 1.22+ (for building from source)
- One or more API keys: OpenAI, Anthropic, Groq, or Ollama (local)

### Build from source
```bash
git clone https://github.com/Moguifeng-9119/aperture.git
cd aperture
make build
```

### Configure
```bash
cp config.example.yaml config.yaml
# Edit config.yaml with your API keys, or use environment variables:
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
export APERTURE_ADMIN_KEY="your-admin-key"
```

### Run
```bash
./aperture --config config.yaml
# Aperture gateway listening on 0.0.0.0:8080
# Dashboard at http://localhost:8080/dashboard
```

### Use
```bash
# Direct model access (bypasses routing)
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-your-key" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# Auto-routing mode — let Aperture choose the best model
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Write a function to sort an array"}]
  }'
# Response headers include:
#   X-Aperture-Model: gpt-4o
#   X-Aperture-Provider: openai
#   X-Aperture-Reason: rule:code_generation matched → complexity=complex → openai/gpt-4o
```

## Architecture

```
Client (OpenAI SDK) → Aperture Gateway (:8080)
                         │
                    ┌─────▼──────┐
                    │  Pipeline   │
                    │  1. Auth    │
                    │  2. Context │
                    │  3. Route   │
                    │  4. Dispatch│
                    │  5. Record  │
                    └─────┬──────┘
                          │
              ┌───────────▼───────────┐
              │   Routing Engine      │
              │  Tier 1: Rules        │  <0.1ms, ~80% coverage
              │  Tier 2: Embeddings   │  ~5ms, semantic matching
              │  Tier 3: ML (ONNX)    │  ~1ms, custom trained
              └───────────┬───────────┘
                          │
        ┌─────────────────┼─────────────────┐
        ▼                 ▼                  ▼
    OpenAI          Anthropic           Groq / Ollama
```

## API Reference

### Chat Completions (OpenAI-compatible)
```
POST /v1/chat/completions
  Header: Authorization: Bearer <api-key>
  Header: X-Conversation-Id (optional, for multi-turn)
  Body: {
    "model": "auto" | "gpt-4o" | "claude-3-haiku" | ...,
    "messages": [...],
    "stream": false
  }
  Response Headers:
    X-Aperture-Model         → model used
    X-Aperture-Provider      → provider used
    X-Aperture-Reason        → routing decision reason
    X-Aperture-Saving-USD    → estimated cost saving
```

### List Models
```
GET /v1/models
```

### Health Check
```
GET /health
```

### Admin API (requires X-Admin-Key)
```
GET    /admin/v1/health
GET    /admin/v1/analytics/summary?from=&to=&project_id=
GET    /admin/v1/analytics/requests?page=&per_page=
GET    /admin/v1/analytics/export?from=&to=          (JSONL training data)
POST   /admin/v1/routing/test          { "messages": [...] }
GET    /admin/v1/keys
POST   /admin/v1/keys                  { "name": "my-key" }
DELETE /admin/v1/keys/{id}
GET    /admin/v1/ab-test/stats         (A/B comparison stats)
```

### Metrics
```
GET /metrics     (Prometheus text format)
```

## Roadmap

### Completed
| Version | Feature | |
|---------|---------|:--:|
| **v0.1** | Core proxy + OpenAI adapter + Docker + goreleaser | ✓ |
| **v0.2** | 4 providers (OpenAI, Anthropic, Groq, Ollama) + rule-based routing + streaming | ✓ |
| **v0.3** | Embedding classification + cost analytics + SQLite + dashboard UI + admin API | ✓ |
| **v0.4** | Fallback chain + retry + rate limiter + model cost wiring + Docker Compose + CI | ✓ |
| **v0.5** | Real embeddings (OpenAI) + A/B testing + training data export | ✓ |
| **—** | 14 test packages, 4 provider tests, CI (Go 1.25, go vet, build) | ✓ |

### Next (v0.6)
| Feature | Effort |
|---------|:------:|
| OpenTelemetry distributed tracing (span per pipeline stage) | M |
| Structured error codes (replacing raw error strings) | M |
| Helm chart for Kubernetes deployment | M |
| Load testing suite (vegeta or k6 scripts) | M |
| Request log search/filter in dashboard | M |

### Future (v0.7 — v1.0)
| Version | Feature |
|---------|---------|
| **v0.7** | Token-aware routing: estimate cost BEFORE dispatching |
| **v0.8** | Multi-tenancy: project isolation, per-project budgets, usage quotas |
| **v0.9** | Caching layer: cache identical/similar requests to skip API calls |
| **v1.0** | Plugin system: user-defined Strategy implementations via Go plugin or WASM |

## Development

```bash
make build        # build binary
make dev          # go run
make test         # run all tests
make test-cover   # run tests + generate coverage HTML
make lint         # go vet
make docker-build # build Docker image
make docker-run   # run with Docker
```

### Docker Compose (local dev)
```bash
docker compose up -d    # aperture + ollama, ready at :8080
```

## License

MIT

## License

MIT
