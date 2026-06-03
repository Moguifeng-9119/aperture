# Aperture 配置指南

> 从零到跑通，覆盖所有支持的模型。不写一行 YAML，`./aperture -setup` 交互式生成配置。

---

## 目录

1. [5 分钟快速开始](#5-分钟快速开始)
2. [配置向导（推荐）](#配置向导推荐)
3. [手动配置](#手动配置)
4. [国产模型接入](#国产模型接入)
5. [多模型混合路由](#多模型混合路由)
6. [接入现有应用](#接入现有应用)
7. [Dashboard 使用](#dashboard-使用)
8. [常见问题](#常见问题)

---

## 5 分钟快速开始

```bash
# 1. 下载编译
git clone https://github.com/Moguifeng-9119/aperture.git
cd aperture
make build

# 2. 生成配置（交互式，一路回车）
./aperture -setup

# 3. 启动
./aperture

# 4. 测试
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"你好"}]}'
```

---

## 配置向导（推荐）

运行 `./aperture -setup`，逐项填写：

### 选择提供商

```
  [1/5] OpenAI           → 填 sk-...（跳过也行）
  [2/5] Anthropic        → 填 sk-ant-...
  [3/5] Groq             → 填 gsk_...
  [4/5] Ollama           → 本地，填 y + 模型名
  [5/5] 国产模型          → 选 DeepSeek / Qwen / Kimi / GLM / MiniMax / MiMo
```

每个都能跳过，至少配一个就行。

### 国产模型详情

```
      [1] DeepSeek V4 — deepseek-v4-flash + deepseek-v4-pro
      [2] 通义千问 Qwen — qwen-plus (阿里云 DashScope)
      [3] Kimi — moonshot-v1-8k (月之暗面)
      [4] 智谱 GLM — glm-4-flash + glm-4-plus
      [5] MiniMax — abab6.5s-chat
      [6] 小米 MiMo — mi-mo
```

### 路由策略

```
  智能路由策略:
    [1] 规则路由 - 基于关键词+正则，<0.1ms (推荐)
    [2] 嵌入路由 - 基于语义匹配 (需 OPENAI_API_KEY)
    [3] 都不需要，只用默认模型
```

选 [1] 就行，开箱即用。

### 管理员密码

设置后会保护 Dashboard 和 Admin API。留空则不校验。

### 连接测试

向导最后会自动测试每个提供商的连通性。

---

## 手动配置

如果你不想用向导，直接写 `config.yaml`：

### 最小配置（只配 DeepSeek）

```yaml
server:
  port: 8080

providers:
  - id: deepseek
    type: openai
    api_key: "sk-你的Key"
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

### 完整配置参考

见 [`config.example.yaml`](../config.example.yaml)，包含所有选项和注释。

---

## 国产模型接入

Aperture 支持 6 个国产模型，全部通过 OpenAI 兼容协议接入。

### DeepSeek V4

| 项目 | 值 |
|------|-----|
| API 地址 | `https://api.deepseek.com` |
| 获取 Key | [platform.deepseek.com](https://platform.deepseek.com) |
| 模型 | `deepseek-v4-flash`（便宜，0.14/百万）、`deepseek-v4-pro`（强推理，0.435/百万） |
| 上下文 | 1M token |

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

### 通义千问 Qwen（阿里云 DashScope）

| 项目 | 值 |
|------|-----|
| API 地址 | `https://dashscope.aliyuncs.com/compatible-mode/v1` |
| 获取 Key | [dashscope.console.aliyun.com](https://dashscope.console.aliyun.com) |
| 模型 | `qwen-plus` |

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

### Kimi（月之暗面 Moonshot）

| 项目 | 值 |
|------|-----|
| API 地址 | `https://api.moonshot.cn/v1` |
| 获取 Key | [platform.moonshot.cn](https://platform.moonshot.cn) |
| 模型 | `moonshot-v1-8k`、`moonshot-v1-32k` |

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

### 智谱 GLM-4（智谱 AI）

| 项目 | 值 |
|------|-----|
| API 地址 | `https://open.bigmodel.cn/api/paas/v4` |
| 获取 Key | [open.bigmodel.cn](https://open.bigmodel.cn) |
| 模型 | `glm-4-flash`（免费）、`glm-4-plus` |

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

| 项目 | 值 |
|------|-----|
| API 地址 | `https://api.minimax.chat/v1` |
| 获取 Key | [platform.minimax.chat](https://platform.minimax.chat) |
| 模型 | `abab6.5s-chat` |

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

### 小米 MiMo

| 项目 | 值 |
|------|-----|
| API 地址 | `https://api.mi.com/v1` |
| 获取 Key | 小米 AI 开放平台 |
| 模型 | `mi-mo` |

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

## 多模型混合路由

当你同时配了多个提供商，Aperture 可以在它们之间自动选择：

```yaml
providers:
  - id: deepseek     # 国产主力
    type: openai
    api_key: "${DEEPSEEK_API_KEY}"
    base_url: "https://api.deepseek.com"
    models:
      - id: deepseek-v4-flash
      - id: deepseek-v4-pro

  - id: openai       # 特定场景用
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
      model: deepseek-v4-flash     # 你好 → 最便宜
    simple:
      provider: openai
      model: gpt-4o-mini           # 简单问答 → GPT-4o-mini
    moderate:
      provider: deepseek
      model: deepseek-v4-pro       # 中等分析 → DeepSeek Pro
    complex:
      provider: openai
      model: gpt-4o                # 写代码 → GPT-4o
    expert:
      provider: openai
      model: gpt-4o                # 数学证明 → GPT-4o
```

**路由逻辑**：请求进来 → 规则引擎分析关键词 → 匹配复杂度 → 查表选模型。全程 < 0.1ms。

---

## 接入现有应用

### OpenAI Python SDK

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="not-needed"
)

response = client.chat.completions.create(
    model="auto",
    messages=[{"role": "user", "content": "解释量子计算"}]
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

### 任何 HTTP 客户端

把 API 地址从 `https://api.openai.com/v1/chat/completions` 改成 `http://localhost:8080/v1/chat/completions`，model 参数改成 `"auto"`。其他一切不变。

---

## Dashboard 使用

浏览器打开 `http://localhost:8080/dashboard`：

- **Overview**：总请求数、总费用、累计节省
- **Routing**：每个模型的路由次数、复杂分布
- **Keys**：管理 API Key
- **Log**：实时请求日志流

Dashboard 数据端点需要 `X-Admin-Key` 认证（你在 setup 里设置的密码）。

---

## 常见问题

### Q: 启动报 "no such provider"？
**A**: config.yaml 里没有配置任何提供商，或者 type 写错了。运行 `./aperture -setup` 重新生成。

### Q: DeepSeek 返回 401？
**A**: API Key 无效或过期。去 [platform.deepseek.com](https://platform.deepseek.com) 重新生成。

### Q: 国产模型连不上？
**A**: 检查 `base_url` 是否正确（参考上面的表格），部分模型在国内需要对应域名的网络访问权限。

### Q: 怎么强制用某个模型？
**A**: 把 model 参数从 `"auto"` 改成具体模型名，比如 `"deepseek-v4-pro"`。这样就绕过了路由。

### Q: 如何自定义规则？
**A**: 在 `config.yaml` 的 `routing.rules` 下添加自己的规则：

```yaml
routing:
  rules:
    - name: "legal_review"
      priority: 80
      keywords: ["合同", "合同", "法律", "条款", "协议", "合规"]
      assign_complexity: "expert"
```

---

有问题？[GitHub Issues](https://github.com/Moguifeng-9119/aperture/issues)
