<h1 align="center">Aperture</h1>

<p align="center">
  <strong>One gateway for all your LLMs.</strong><br>
  Route every request to the right model.<br>
  Save 40–70% on API costs. Zero code changes.
</p>

<p align="center">
  <a href="https://github.com/Moguifeng-9119/aperture/actions/workflows/ci.yml"><img src="https://github.com/Moguifeng-9119/aperture/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/Moguifeng-9119/aperture/releases"><img src="https://img.shields.io/github/v/release/Moguifeng-9119/aperture" alt="Release"></a>
  <a href="https://pkg.go.dev/github.com/Moguifeng-9119/aperture"><img src="https://img.shields.io/badge/go-reference-00ADD8?logo=go" alt="Go Reference"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green" alt="License"></a>
</p>

---

## What is Aperture?

Aperture is an **OpenAI-compatible LLM gateway** that analyzes each request and routes it to the optimal model. It replaces your single-model API call with **intelligent multi-model routing** — without changing your application code.

```
Before:  Every request → GPT-4o                    ($10.00 / 1M tokens)
After:   "hello" → Llama 3.1 8B                   ($0.05 / 1M tokens)
         "write a function" → GPT-4o               ($10.00 / 1M tokens)
         "explain this code" → Claude 3 Haiku      ($0.25 / 1M tokens)
         ─────────────────────────────────────────────────────────
         Average savings: 40–70%
```

Every routing decision includes an `X-Aperture-Reason` header so you can see exactly **why** a model was chosen.

### Three tiers, one gateway

| Tier | How it works | Latency | Accuracy | Setup |
|:----:|-------------|:-----:|:--------:|--------|
| **1** | Rules — keywords, regex, token count | < 0.1ms | ~80% | None — built in |
| **2** | Embeddings — OpenAI text-embedding-3-small | ~200ms | ~90% | Set `OPENAI_API_KEY` |
| **3** | ML — train your own ONNX classifier | < 1ms | ~95% | Export data → train → deploy |

Tiers 2 and 3 are optional. Tiers cascade: if Tier 1 isn't confident enough, Tier 2 tries. Still not sure? Tier 3 has the final say.

---

## Quick Start

```bash
# Clone, build, run
git clone https://github.com/Moguifeng-9119/aperture.git && cd aperture
make build && cp config.example.yaml config.yaml

# Set your API keys
export OPENAI_API_KEY="sk-..."
export APERTURE_ADMIN_KEY="choose-a-password"

# Start the gateway
./aperture
# → Listening on :8080 · Dashboard at /dashboard · Metrics at /metrics
```

**Try auto-routing:**

```bash
# A greeting — routes to a cheap model
curl -s http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Hello!"}]}'

# A code request — routes to a powerful model
curl -s http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Write a Python function to parse JSON"}]}'
```

**See what model handled your request:**

```bash
curl -sI http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Hello!"}]}' \
  | grep X-Aperture

# X-Aperture-Model: llama-3.1-8b-instant
# X-Aperture-Provider: groq
# X-Aperture-Reason: rule:greeting matched → trivial → groq/llama-3.1-8b-instant
# X-Aperture-Saving-USD: 0.009850
```

### Other install options

| Method | Command |
|--------|---------|
| **Go install** | `go install github.com/Moguifeng-9119/aperture@latest` |
| **Binary** | Download from [releases](https://github.com/Moguifeng-9119/aperture/releases/latest) |
| **Docker Compose** | `docker compose up -d` (includes Ollama for local models) |
| **Helm** | `helm install aperture ./deploy/helm/aperture` |
| **Homebrew** | `brew install Moguifeng-9119/tap/aperture` |

---

## Features

### Supported providers

OpenAI · Anthropic · Groq · Ollama

All with streaming (SSE) support. Add new providers by implementing a single Go interface.

### Production ready

| Feature | Description |
|---------|-------------|
| **Fallback chain** | Primary provider fails → auto retries with backup models |
| **Rate limiting** | Per-API-key throttling, configurable RPM |
| **Graceful shutdown** | In-flight requests complete before server stops |
| **Single binary** | No Node.js, no Python, no Docker required |
| **Prometheus metrics** | `/metrics` endpoint for Grafana & alerting |
| **Structured errors** | Typed error codes with HTTP status mapping |
| **Distributed tracing** | OpenTelemetry-style span hierarchy per pipeline stage |
| **Docker image** | `docker run -p 8080:8080 -v ./config.yaml:/app/config.yaml aperture` |
| **Helm chart** | `helm install aperture ./deploy/helm/aperture` |

### Observability & analytics

| Feature | Description |
|---------|-------------|
| **Explainable routing** | Every response includes `X-Aperture-Model`, `X-Aperture-Provider`, `X-Aperture-Reason`, `X-Aperture-Saving-USD` |
| **Cost dashboard** | Real-time per-model cost tracking at `/dashboard` |
| **Admin API** | Full CRUD for API keys, routing test endpoint, analytics queries |

### ML & experimentation

| Feature | Description |
|---------|-------------|
| **A/B testing** | Compare two routing strategies; view agreement stats at `/admin/v1/ab-test/stats` |
| **Training data export** | `GET /admin/v1/analytics/export` returns JSONL for model training |
| **Dry-run routing** | `POST /admin/v1/routing/test` shows which model would be selected |
| **Custom ML classifier** | Export data, train with `tools/train_model.py`, deploy as Tier 3 |

---

## API

### OpenAI-compatible endpoints

```
POST   /v1/chat/completions    → stream=true supported
GET    /v1/models               → lists all models + "auto"
GET    /health                  → per-provider health status
```

### Admin API (requires `X-Admin-Key` header)

```
GET    /admin/v1/health
GET    /admin/v1/analytics/summary       ?from=2024-01-01&to=2024-01-31
GET    /admin/v1/analytics/requests      ?page=1&per_page=50
GET    /admin/v1/analytics/export        → JSONL training data
POST   /admin/v1/routing/test            → { "messages": [...] }
GET    /admin/v1/ab-test/stats           → A/B agreement rate
GET    /admin/v1/keys                    → list API keys
POST   /admin/v1/keys                    → { "name": "my-key" }
DELETE /admin/v1/keys/{id}
```

### Observability

```
GET    /metrics                 → Prometheus text format
GET    /dashboard               → built-in analytics UI
```

---

## Configuration

The shortest working config:

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

Full reference including fallback chains, custom routing rules, rate limits, per-model pricing, and embedding settings: **[config.example.yaml](config.example.yaml)**.

---

## Development

```bash
make build        # → ./aperture
make dev          # go run .
make test         # run all tests
make test-cover   # tests + coverage.html
make lint         # go vet ./...
make docker-build # build Docker image
```

---

## Roadmap

| Done | Next |
|------|------|
| Core proxy + OpenAI adapter | Token-aware cost estimation |
| 4 providers + rule routing + streaming | Multi-tenancy + caching |
| Embeddings + cost analytics + SQLite | Load testing suite (k6) |
| Dashboard + admin API + Prometheus | ONNX model quality metrics |
| Fallback/retry + rate limiter + Docker Compose | Plugin system (WASM) |
| Real embeddings (OpenAI) + A/B testing | |
| Structured errors + tracing + Helm chart | |

---

## License

MIT © [Moguifeng-9119](https://github.com/Moguifeng-9119)
