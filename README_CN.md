<div align="center">

# Aperture · 光圈

**智能多模型 LLM 路由网关**

[![CI](https://github.com/Moguifeng-9119/aperture/actions/workflows/ci.yml/badge.svg)](https://github.com/Moguifeng-9119/aperture/actions)
[![Release](https://img.shields.io/github/v/release/Moguifeng-9119/aperture?color=6366f1)](https://github.com/Moguifeng-9119/aperture/releases)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)
[![README](https://img.shields.io/badge/README-EN-blue)](README.md)

*每个请求自动路由到最合适的模型，API 费用直降 40–70%，一行代码不用改。*

</div>

---

## 💡 一句话解释

你现在的做法：所有请求都打给 GPT-4o，一句"你好"和一封"帮我写个操作系统"花一样的钱。

Aperture 的做法：分析请求内容，自动把"你好"扔给便宜的模型，把硬骨头扔给 GPT-4o。**省下的钱是实实在在的。**

```
"你好"               →  Groq Llama 3.1 8B       ($0.05 / 百万 token)
"这个 SQL 慢在哪"     →  Claude 3 Haiku         ($0.25 / 百万 token)
"帮我写一个 REST API"  →  GPT-4o                 ($10.00 / 百万 token)
```

每个路由决定都会在响应头里写清楚原因，信不过随时可以关掉。

---

## 🚀 5 分钟跑起来

```bash
git clone https://github.com/Moguifeng-9119/aperture.git && cd aperture
make build
cp config.example.yaml config.yaml
export OPENAI_API_KEY="sk-..."
./aperture
# → 监听 :8080，Dashboard 在 /dashboard，Metrics 在 /metrics
```

```bash
# 自动路由模式 — model 填 auto，Aperture 替你选模型
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"用 Python 写一个排序函数"}]}'

# 看看到底用了哪个模型
curl -sI localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"你好"}]}' \
  | grep X-Aperture

# X-Aperture-Model: llama-3.1-8b-instant
# X-Aperture-Provider: groq
# X-Aperture-Reason: rule:greeting matched → trivial → groq/llama-3.1-8b-instant
# X-Aperture-Saving-USD: 0.009850
```

**接入方式：把 `https://api.openai.com` 改成 `http://localhost:8080`，model 改成 `auto`。完事。**

---

## ⚖️ 跟其他网关有什么不同？

| | Aperture | LiteLLM | Portkey | OneAPI |
|---|:---:|:---:|:---:|:---:|
| **智能路由** | ✓ 3 层（规则+嵌入+ML） | ✗ | ✗ | ✗ |
| **可解释路由** | ✓ `X-Aperture-Reason` | ✗ | ✗ | ✗ |
| **成本仪表盘** | ✓ 内置 | ✗ | ✓ 仅云端 | ✗ |
| **A/B 测试** | ✓ | ✗ | ✗ | ✗ |
| **单文件部署** | ✓ Go 编译 ~15MB | ✗ Python | ✗ 云端 | ✗ Node.js |
| **自托管** | ✓ | ✓ | ✗ | ✓ |
| **训练数据导出** | ✓ JSONL | ✗ | ✗ | ✗ |
| **开源协议** | MIT | MIT | Apache 2.0 | MIT |

> 其他网关只是"把你的请求转发给一个固定的模型"。Aperture 帮你**决定用哪个模型** — 这是本质区别。

---

<div align="center">
  <a href="https://skillicons.dev">
    <img src="https://skillicons.dev/icons?i=go,docker,kubernetes,prometheus,sqlite,htmx&theme=light" alt="技术栈" />
  </a>
</div>

## 🔧 功能一览

| 分类 | 说明 |
|------|------|
| **提供商** | OpenAI · Anthropic · Groq · Ollama · DeepSeek V4 · 通义千问 · Kimi · 智谱 GLM · MiniMax · 小米 MiMo · [配置教程 →](docs/deepseek-guide.md) |
| **路由引擎** | 三层：规则（关键词/正则/token 计数）→ 嵌入（OpenAI API）→ ML（ONNX 自定义模型） |
| **生产特性** | 失败自动回退链 · 请求重试 · 按 Key 限速 · 优雅退出 · 结构化错误码 |
| **运维面板** | 内置 Dashboard（HTMX + Alpine.js + Chart.js）· Prometheus `/metrics` |
| **开发者工具** | A/B 测试对比 · 路由测试端点 · 训练数据 JSONL 导出 |
| **部署** | 单二进制 · Docker Compose · Helm · goreleaser 6 平台交叉编译 |
| **API 兼容** | 完全兼容 OpenAI `/v1/chat/completions`，可无缝替换 |

---

## ⚙️ 最小配置

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

完整配置参考：[`config.example.yaml`](config.example.yaml) — 包含回退链、自定义规则、限速、模型定价、嵌入设置。

---

## 📊 项目数据

<div align="center">

| 指标 | 数据 |
|------|------|
| 测试包 | 15 个 |
| 提供商覆盖 | 4/4 |
| CI | test · lint · build · release |
| 发布平台 | linux (amd64/arm64) · darwin (amd64/arm64) · windows (amd64/arm64) |

</div>

## 🗺️ 路线图

| ✅ 已完成 | ⬜ 规划中 |
|-----------|-----------|
| 核心代理 + OpenAI 适配器 | Token 感知路由（请求前预估费用） |
| 4 个提供商 + 规则路由 + 流式 | 多租户 + 缓存层 |
| 嵌入分类 + 成本分析 + SQLite | 压测套件（k6） |
| Dashboard + Admin API + Prometheus | ONNX 模型质量指标 |
| 回退链 + 重试 + 限速 + Docker Compose | 插件系统（WASM） |
| 真实嵌入（OpenAI）+ A/B 测试 | |
| 结构化错误码 + 分布式追踪 + Helm | |
| CI/CD + 多平台发布 | |

---

## 🛠️ 开发

```bash
make build        # 编译 → ./aperture
make dev          # go run .
make test         # 跑测试
make test-cover   # 测试 + 覆盖率 HTML
make lint         # go vet ./...
make docker-build # 构建 Docker 镜像
```

---

<div align="center">

MIT License — 随便用，随便改，随便卖。

</div>
