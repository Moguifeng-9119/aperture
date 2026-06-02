# Aperture

Intelligent multi-model LLM routing gateway — route each request to the optimal model based on complexity, saving 40-70% on API costs without sacrificing quality.

## Why Aperture?

Every LLM request is different. A simple "hello" doesn't need GPT-4o, and a complex code review shouldn't go to a tiny model. Aperture analyzes each request and routes it to the right model automatically.

**Key differentiators:**
- **Explainable routing** — every decision comes with a reason and cost estimate
- **Progressive intelligence** — start with simple rules, upgrade to embeddings and ML as you grow
- **Cost analytics** — real-time dashboard showing exactly how much you're saving
- **Single binary** — no dependencies, deploy anywhere
- **OpenAI-compatible** — drop-in replacement, works with any OpenAI SDK

## Quick Start

### Prerequisites
- Go 1.22+ (for building from source)
- An OpenAI API key (Anthropic, Groq, Ollama support coming in Phase 2)

### Build from source
```bash
git clone https://github.com/2144983846/aperture.git
cd aperture
make build
```

### Configure
```bash
cp config.example.yaml config.yaml
# Edit config.yaml with your API keys
export OPENAI_API_KEY="sk-..."
```

### Run
```bash
./aperture --config config.yaml
# Aperture gateway listening on 0.0.0.0:8080
```

### Use
```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-your-key" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
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
              │  Tier 1: Rules        │
              │  Tier 2: Embeddings   │
              │  Tier 3: ML (ONNX)    │
              └───────────────────────┘
```

## Roadmap

| Phase | Feature | Status |
|-------|---------|--------|
| 1 | Core proxy + OpenAI adapter | In progress |
| 2 | Multi-provider + rule-based routing | Planned |
| 3 | Embedding classification + cost analytics | Planned |
| 4 | Dashboard UI | Planned |
| 5 | ML classifier + production hardening | Planned |

## License

MIT
