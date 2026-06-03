<picture>
  <source media="(prefers-color-scheme: dark)">
  <h1 align="center">🔆 Aperture</h1>
</picture>

<p align="center">
  <strong>One gateway for all your LLMs.</strong><br>
  Route every request to the right model. Save 40-70% on API costs. Zero config changes in your app.
</p>

<p align="center">
  <a href="https://github.com/Moguifeng-9119/aperture/actions/workflows/ci.yml"><img src="https://github.com/Moguifeng-9119/aperture/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/Moguifeng-9119/aperture/releases"><img src="https://img.shields.io/github/v/release/Moguifeng-9119/aperture?color=blue" alt="Release"></a>
  <a href="https://pkg.go.dev/github.com/Moguifeng-9119/aperture"><img src="https://img.shields.io/badge/go-reference-00ADD8?logo=go" alt="Go Reference"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green" alt="License"></a>
</p>

---

## What is Aperture?

Your app talks to Aperture. Aperture decides which model to use. You save money.

```bash
# Before — your app sends everything to GPT-4o ($10/M tokens)
curl https://api.openai.com/v1/chat/completions -d '{"model":"gpt-4o",...}'

# After — Aperture routes "hello" → Llama 3 (free), "write code" → GPT-4o ($10)
curl http://localhost:8080/v1/chat/completions -d '{"model":"auto",...}'
```

A simple "hello" routes to a cheap model. A complex code review routes to GPT-4o. **Every decision is explained** in response headers so you can trust it.

### How routing works

| Tier | Strategy | Speed | Accuracy | When to use |
|:----:|----------|:-----:|:--------:|-------------|
| **1** | **Rules** — keywords, regex, token count | <0.1ms | ~80% | Zero config, works instantly |
| **2** | **Embeddings** — OpenAI text-embedding-3-small | ~200ms | ~90% | Set `OPENAI_API_KEY` for semantic matching |
| **3** | **ML** — custom ONNX classifier | <1ms | ~95% | Export data → train → deploy your own model |

No match? Request falls through to the next tier. Tiers 2 and 3 are **completely optional**.

---

## Quick Start

### Homebrew (macOS)
```bash
brew install Moguifeng-9119/tap/aperture
```

### Go install
```bash
go install github.com/Moguifeng-9119/aperture@latest
```

### Binary download
Pick your platform from the [latest release](https://github.com/Moguifeng-9119/aperture/releases/latest).

### Docker Compose (aperture + ollama, zero API keys)
```bash
git clone https://github.com/Moguifeng-9119/aperture.git && cd aperture
cp config.example.yaml config.yaml
docker compose up -d
# Ready at http://localhost:8080 — dashboard at /dashboard
```

### From source
```bash
git clone https://github.com/Moguifeng-9119/aperture.git && cd aperture
make build
cp config.example.yaml config.yaml
export OPENAI_API_KEY="sk-..."
./aperture --config config.yaml
```

### Try it
```bash
# Auto mode — let Aperture pick the best model
curl -s http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Write a Python function to sort an array"}]}' \
  | jq '.choices[0].message.content'

# Check which model was used
curl -sI http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Hello!"}]}' \
  | grep X-Aperture
# X-Aperture-Model: llama-3.1-8b-instant
# X-Aperture-Provider: groq
# X-Aperture-Reason: rule:greeting matched → trivial → groq/llama-3.1-8b-instant
```

---

## Features

### Routing
- **Explainable decisions** — every response includes `X-Aperture-*` headers with the reasoning
- **Progressive tiers** — start with fast rules, add embeddings/ML later
- **`"model": "auto"`** — drop-in replacement; existing code doesn't change

### Providers
- OpenAI (GPT-4o, GPT-4o-mini, any compatible API)
- Anthropic (Claude Opus, Sonnet, Haiku)
- Groq (Llama 3.1 at 1,200 tok/s)
- Ollama (local models — Qwen, DeepSeek, Mistral, etc.)

### Production
- **Fallback chain** — primary model fails? automatically retry with backup
- **Rate limiting** — per-API-key throttling
- **Cost analytics** — real-time dashboard with model-level cost breakdown
- **Prometheus metrics** — `/metrics` endpoint for Grafana dashboards
- **Graceful shutdown** — no dropped requests on restart
- **Single binary** — no Node.js, no Python, no dependencies

### Developer tools
- **A/B testing** — compare two routing strategies side-by-side, see agreement stats at `/admin/v1/ab-test/stats`
- **Training data export** — `GET /admin/v1/analytics/export` → JSONL for ML training
- **Dry-run routing** — `POST /admin/v1/routing/test` to see what model would be chosen
- **Admin dashboard** — built-in HTMX + Alpine.js UI at `/dashboard`

---

## Architecture

```
                  POST /v1/chat/completions
                  {"model": "auto", "messages": [...]}
                              │
                              ▼
          ┌───────────────────────────────────┐
          │            Pipeline               │
          │  Auth → Context → Route → Dispatch│
          └───────────────┬───────────────────┘
                          │
          ┌───────────────▼───────────────────┐
          │        Routing Engine             │
          │                                   │
          │  Tier 1: Rules     <0.1ms  80%    │
          │  Tier 2: Embed     ~200ms  90%    │
          │  Tier 3: ML        <1ms    95%    │
          └───────────────┬───────────────────┘
                          │
        ┌─────────────────┼─────────────────┐
        ▼                 ▼                  ▼
    OpenAI           Anthropic        Groq / Ollama
```

---

## API

### OpenAI-compatible (drop-in replacement)
```
POST   /v1/chat/completions    # supports stream=true
GET    /v1/models               # lists all models including "auto"
GET    /health                  # provider health status
```

### Admin (requires `X-Admin-Key` header)
```
GET    /admin/v1/health
GET    /admin/v1/analytics/summary         ?from=2024-01-01&to=2024-01-31
GET    /admin/v1/analytics/requests        ?page=1&per_page=50
GET    /admin/v1/analytics/export          (JSONL training data)
POST   /admin/v1/routing/test              { "messages": [...] }
GET    /admin/v1/keys
POST   /admin/v1/keys                      { "name": "production" }
DELETE /admin/v1/keys/{id}
GET    /admin/v1/ab-test/stats             (A/B agreement rate)
```

### Observability
```
GET    /metrics                # Prometheus text format
GET    /dashboard              # Built-in analytics UI
```

---

## Configuration

Minimal `config.yaml` to get started:

```yaml
server:
  port: 8080

providers:
  - id: openai
    type: openai
    api_key: "${OPENAI_API_KEY}"
  - id: ollama
    type: ollama
    base_url: "http://localhost:11434"

routing:
  default_model: gpt-4o-mini
  default_provider: openai
```

See [`config.example.yaml`](config.example.yaml) for all options including fallback chains, custom rules, rate limits, and per-model pricing.

---

## Development

```bash
make build        # build binary → ./aperture
make dev          # go run .
make test         # run all tests
make test-cover   # tests + coverage HTML
make lint         # go vet
make docker-build # build Docker image
```

---

## Roadmap

| Done | Milestone |
|:----:|-----------|
| ✓ | Core proxy + OpenAI adapter |
| ✓ | 4 providers + rule-based routing + streaming |
| ✓ | Embeddings + cost analytics + SQLite + dashboard |
| ✓ | Fallback chain + retry + rate limiter + cost wiring |
| ✓ | Real embeddings (OpenAI) + A/B testing + training export |
| ✓ | Docker Compose + CI + multi-platform releases |

| Next | Planned |
|:----:|---------|
| — | OpenTelemetry tracing + structured error codes |
| — | Helm chart + load testing suite |
| — | Token-aware routing (estimate cost before dispatching) |
| — | Multi-tenancy + caching layer |
| — | Plugin system (WASM / Go plugins) |

---

## License

MIT © [Moguifeng-9119](https://github.com/Moguifeng-9119)
