# Aperture Configuration Guide

> From zero to running, covering all supported models. No YAML required — `./aperture -setup` generates everything interactively.

---

## Table of Contents

1. [5-Minute Quick Start](#5-minute-quick-start)
2. [Setup Wizard (Recommended)](#setup-wizard-recommended)
3. [Manual Configuration](#manual-configuration)
4. [Chinese Domestic Models](#chinese-domestic-models)
5. [Multi-Model Hybrid Routing](#multi-model-hybrid-routing)
6. [Integrating Existing Apps](#integrating-existing-apps)
7. [Dashboard](#dashboard)
8. [FAQ](#faq)

---

## 5-Minute Quick Start

```bash
# 1. Clone and build
git clone https://github.com/Moguifeng-9119/aperture.git
cd aperture
make build

# 2. Generate config (interactive, just hit Enter)
./aperture -setup

# 3. Start
./aperture

# 4. Test
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Hello!"}]}'
```

---

## Setup Wizard (Recommended)

Run `./aperture -setup` and answer the prompts:

### Choose Providers

```
  [1/5] OpenAI           → enter sk-... (or skip)
  [2/5] Anthropic        → enter sk-ant-...
  [3/5] Groq             → enter gsk_...
  [4/5] Ollama           → local, enter y + model name
  [5/5] Chinese Models   → DeepSeek / Qwen / Kimi / GLM / MiniMax / MiMo
```

Skip any you don't need. At least one is required.

### Chinese Models Detail

```
      [1] DeepSeek V4 — deepseek-v4-flash + deepseek-v4-pro
      [2] Qwen (Alibaba DashScope)
      [3] Kimi (Moonshot)
      [4] GLM-4 (Zhipu AI)
      [5] MiniMax
      [6] Xiaomi MiMo
```

### Routing Strategy

```
  Routing strategy:
    [1] Rules — keyword+regex based, <0.1ms (recommended)
    [2] Embeddings — semantic matching (requires OPENAI_API_KEY)
    [3] None — just use the default model
```

Choose [1] — it works out of the box.

### Admin Password

Protects the Dashboard and Admin API. Leave empty to skip auth.

### Connection Test

The wizard tests connectivity to each provider at the end.

---

## Manual Configuration

If you prefer writing YAML directly:

### Minimal Config (DeepSeek only)

```yaml
server:
  port: 8080

providers:
  - id: deepseek
    type: openai
    api_key: "sk-your-key"
    base_url: "https://api.deepseek.com"
    models:
      - id: deepseek-v4-flash
        cost_per_1k_input: 0.00014
        cost_per_1k_output: 0.00028
        max_tokens: 1048576
      - id: deepseek-v4-pro
        cost_per_1k_input: 0.000435
        cost_per_1k_output: 0.00087
        max_tokens: 1048576

routing:
  default_model: deepseek-v4-flash
  default_provider: deepseek
```

### Full Reference

See [`config.example.yaml`](../config.example.yaml) for all options with comments.

---

## Chinese Domestic Models

Aperture supports 6 Chinese models, all via OpenAI-compatible protocol.

### DeepSeek V4

| Item | Value |
|------|-------|
| API URL | `https://api.deepseek.com` |
| Get Key | [platform.deepseek.com](https://platform.deepseek.com) |
| Models | `deepseek-v4-flash` (cheap, $0.14/M), `deepseek-v4-pro` (powerful, $0.435/M) |
| Context | 1M tokens |

```yaml
providers:
  - id: deepseek
    type: openai
    api_key: "${DEEPSEEK_API_KEY}"
    base_url: "https://api.deepseek.com"
    models:
      - id: deepseek-v4-flash
        cost_per_1k_input: 0.00014
        cost_per_1k_output: 0.00028
        max_tokens: 1048576
      - id: deepseek-v4-pro
        cost_per_1k_input: 0.000435
        cost_per_1k_output: 0.00087
        max_tokens: 1048576
```

### Qwen (Alibaba DashScope)

| Item | Value |
|------|-------|
| API URL | `https://dashscope.aliyuncs.com/compatible-mode/v1` |
| Get Key | [dashscope.console.aliyun.com](https://dashscope.console.aliyun.com) |
| Models | `qwen-plus` |

```yaml
providers:
  - id: qwen
    type: openai
    api_key: "${DASHSCOPE_API_KEY}"
    base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
    models:
      - id: qwen-plus
        cost_per_1k_input: 0.0008
        cost_per_1k_output: 0.002
        max_tokens: 131072
```

### Kimi (Moonshot)

| Item | Value |
|------|-------|
| API URL | `https://api.moonshot.cn/v1` |
| Get Key | [platform.moonshot.cn](https://platform.moonshot.cn) |
| Models | `moonshot-v1-8k`, `moonshot-v1-32k` |

```yaml
providers:
  - id: moonshot
    type: openai
    api_key: "${MOONSHOT_API_KEY}"
    base_url: "https://api.moonshot.cn/v1"
    models:
      - id: moonshot-v1-8k
        cost_per_1k_input: 0.012
        cost_per_1k_output: 0.012
        max_tokens: 8192
```

### GLM-4 (Zhipu AI)

| Item | Value |
|------|-------|
| API URL | `https://open.bigmodel.cn/api/paas/v4` |
| Get Key | [open.bigmodel.cn](https://open.bigmodel.cn) |
| Models | `glm-4-flash` (free), `glm-4-plus` |

```yaml
providers:
  - id: zhipu
    type: openai
    api_key: "${ZHIPU_API_KEY}"
    base_url: "https://open.bigmodel.cn/api/paas/v4"
    models:
      - id: glm-4-flash
        cost_per_1k_input: 0.0
        cost_per_1k_output: 0.0
        max_tokens: 131072
      - id: glm-4-plus
        cost_per_1k_input: 0.007
        cost_per_1k_output: 0.007
        max_tokens: 131072
```

### MiniMax

| Item | Value |
|------|-------|
| API URL | `https://api.minimax.chat/v1` |
| Get Key | [platform.minimax.chat](https://platform.minimax.chat) |
| Models | `abab6.5s-chat` |

```yaml
providers:
  - id: minimax
    type: openai
    api_key: "${MINIMAX_API_KEY}"
    base_url: "https://api.minimax.chat/v1"
    models:
      - id: abab6.5s-chat
        cost_per_1k_input: 0.001
        cost_per_1k_output: 0.001
        max_tokens: 245760
```

### Xiaomi MiMo

| Item | Value |
|------|-------|
| API URL | `https://api.mi.com/v1` |
| Get Key | Xiaomi AI Platform |
| Models | `mi-mo` |

```yaml
providers:
  - id: mimo
    type: openai
    api_key: "${XIAOMI_API_KEY}"
    base_url: "https://api.mi.com/v1"
    models:
      - id: mi-mo
        cost_per_1k_input: 0.001
        cost_per_1k_output: 0.002
        max_tokens: 131072
```

---

## Multi-Model Hybrid Routing

When you have multiple providers configured, Aperture chooses the best model automatically:

```yaml
providers:
  - id: deepseek
    type: openai
    api_key: "${DEEPSEEK_API_KEY}"
    base_url: "https://api.deepseek.com"
    models:
      - id: deepseek-v4-flash
      - id: deepseek-v4-pro

  - id: openai
    type: openai
    api_key: "${OPENAI_API_KEY}"
    models:
      - id: gpt-4o-mini
      - id: gpt-4o

routing:
  default_model: deepseek-v4-flash
  default_provider: deepseek
  complexity_map:
    trivial:
      provider: deepseek
      model: deepseek-v4-flash     # "Hello" → cheapest
    simple:
      provider: openai
      model: gpt-4o-mini           # Simple Q&A → GPT-4o-mini
    moderate:
      provider: deepseek
      model: deepseek-v4-pro       # Analysis → DeepSeek Pro
    complex:
      provider: openai
      model: gpt-4o                # Code generation → GPT-4o
    expert:
      provider: openai
      model: gpt-4o                # Math proofs → GPT-4o
```

**How it works**: Request arrives → rule engine analyzes keywords → matches complexity level → looks up model in the map. Under 0.1ms.

---

## Integrating Existing Apps

### OpenAI Python SDK

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="not-needed"
)

response = client.chat.completions.create(
    model="auto",
    messages=[{"role": "user", "content": "Explain quantum computing"}]
)
```

### OpenAI Node.js SDK

```js
import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "http://localhost:8080/v1",
  apiKey: "not-needed",
});

const response = await client.chat.completions.create({
  model: "auto",
  messages: [{ role: "user", content: "Hello!" }],
});
```

### LangChain

```python
from langchain_openai import ChatOpenAI

llm = ChatOpenAI(model="auto", base_url="http://localhost:8080/v1")
```

### Any HTTP Client

Change your API endpoint from `https://api.openai.com/v1/chat/completions` to `http://localhost:8080/v1/chat/completions`, and set `model` to `"auto"`. Everything else stays the same.

---

## Dashboard

Open `http://localhost:8080/dashboard` in your browser:

- **Overview** — total requests, total cost, cumulative savings
- **Routing** — model distribution, complexity breakdown
- **Keys** — manage API keys
- **Log** — real-time request stream

Dashboard data endpoints require `X-Admin-Key` authentication (the password you set during setup).

---

## FAQ

### Q: "no such provider" on startup?
**A**: No providers configured in config.yaml, or the type is wrong. Run `./aperture -setup` to regenerate.

### Q: DeepSeek returns 401?
**A**: Invalid or expired API key. Regenerate at [platform.deepseek.com](https://platform.deepseek.com).

### Q: Chinese models not connecting?
**A**: Double-check the `base_url` against the tables above. Some models require network access to their specific domains.

### Q: How to force a specific model?
**A**: Change `model` from `"auto"` to the specific model name, e.g. `"deepseek-v4-pro"`. This bypasses routing.

### Q: How to add custom routing rules?
**A**: Add rules under `routing.rules` in config.yaml:

```yaml
routing:
  rules:
    - name: "legal_review"
      priority: 80
      keywords: ["contract", "NDA", "legal", "compliance", "agreement"]
      assign_complexity: "expert"
```

---

Questions? [GitHub Issues](https://github.com/Moguifeng-9119/aperture/issues)
