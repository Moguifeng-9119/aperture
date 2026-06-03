# Aperture + DeepSeek V4 配置指南

> 用 Aperture 智能路由 DeepSeek V4，简单问题走 flash（极便宜），复杂问题走 pro（极强），**API 费用立省 60%+**。

## 第一步：获取 API Key

1. 打开 [platform.deepseek.com](https://platform.deepseek.com)
2. 注册/登录 → 左侧菜单 → **API Keys**
3. 点击「创建 API Key」→ 复制保存（只显示一次）

> DeepSeek 新用户通常有免费额度，够你测试好几天。

## 第二步：生成配置（两种方式任选）

### 方式 A：向导一键生成（推荐）

```bash
./aperture -setup
```

跟着提示选：
```
  [5/5] 国产模型
    DeepSeek V4 | 通义千问 | Kimi | 智谱 GLM | MiniMax | 小米 MiMo
    启用国产模型? (y/n) [y]

    选择模型:
      [1] DeepSeek V4 — deepseek-v4-flash + deepseek-v4-pro
      [2] 通义千问 Qwen — qwen-plus (阿里云 DashScope)
      ...
    选择 (1-6) [1]            ← 选 1

      DeepSeek API Key sk-...  ← 粘贴你的 API Key
      ✓ DeepSeek V4 已配置

  默认模型 [deepseek-v4-flash]  ← 回车确认
  智能路由策略: [1] 规则路由 (推荐) ← 回车确认

  是否测试提供商连接? (y/n) [y] ← 回车测试
  测试 deepseek... ✓ 连接正常
```

搞定，`config.yaml` 已生成。

### 方式 B：手动写配置

创建 `config.yaml`：

```yaml
server:
  port: 8080

providers:
  - id: deepseek
    type: openai
    api_key: "sk-你的DeepSeek-API-Key"
    base_url: "https://api.deepseek.com"
    models:
      - id: deepseek-v4-flash       # 便宜快速，简单问题
        cost_per_1k_input: 0.00014
        cost_per_1k_output: 0.00028
        max_tokens: 1048576
      - id: deepseek-v4-pro         # 强推理，复杂问题
        cost_per_1k_input: 0.000435
        cost_per_1k_output: 0.00087
        max_tokens: 1048576

routing:
  default_model: deepseek-v4-flash
  default_provider: deepseek
  strategies:
    - name: rule
      enabled: true
```

## 第三步：启动

```bash
./aperture
# Aperture gateway listening on 0.0.0.0:8080
```

## 第四步：测试

### 简单问题 → 自动用 flash

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "你好，今天天气怎么样？"}]
  }'
```

响应头里可以看到路由决定：

```
X-Aperture-Model: deepseek-v4-flash
X-Aperture-Provider: deepseek
X-Aperture-Reason: rule:greeting matched → trivial → deepseek/deepseek-v4-flash
```

### 复杂问题 → 自动用 pro

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "帮我写一个线程安全的 LRU 缓存，Go 实现，要有过期机制"}]
  }'
```

响应头：

```
X-Aperture-Model: deepseek-v4-pro
X-Aperture-Reason: rule:code_generation matched → complex → deepseek/deepseek-v4-pro
```

### 如果你想强制用某个模型

```bash
# 不用 auto，直接指定模型名即可
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-v4-pro",
    "messages": [{"role": "user", "content": "1+1=?"}]
  }'
# 这个会直接用 pro，不走路由（当然这很浪费）
```

## 进阶：双模型智能路由

如果你同时有 DeepSeek 和其他提供商的 API Key，可以配置更精细的路由：

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

  # 如果你还有 OpenAI 或 Groq，可以混着用
  - id: openai
    type: openai
    api_key: "${OPENAI_API_KEY}"
    models:
      - id: gpt-4o-mini
        cost_per_1k_input: 0.00015
        cost_per_1k_output: 0.0006
        max_tokens: 128000

routing:
  default_model: deepseek-v4-flash
  default_provider: deepseek
  complexity_map:
    trivial:
      provider: deepseek
      model: deepseek-v4-flash
    simple:
      provider: deepseek
      model: deepseek-v4-flash
    moderate:
      provider: openai
      model: gpt-4o-mini        # 中等难度用 GPT-4o-mini
    complex:
      provider: deepseek
      model: deepseek-v4-pro    # 复杂任务用 DeepSeek V4 Pro
    expert:
      provider: openai
      model: gpt-4o             # 极难任务用 GPT-4o
```

这样配置后，Aperture 会在 DeepSeek V4 和 OpenAI 之间自动选择最合适的模型。

## 接入现有应用

把代码里的 `https://api.openai.com` 换成 `http://localhost:8080`，model 改成 `auto`：

### OpenAI SDK

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",  # 指向 Aperture
    api_key="not-needed"                   # Aperture 网关自己处理认证
)

response = client.chat.completions.create(
    model="auto",                          # 让 Aperture 选模型
    messages=[{"role": "user", "content": "解释量子计算"}]
)
print(response.choices[0].message.content)
```

### LangChain

```python
from langchain_openai import ChatOpenAI

llm = ChatOpenAI(
    model="auto",
    base_url="http://localhost:8080/v1",
    api_key="not-needed"
)
```

## 价格对比

假设你一个月有 100 万 token 的请求量，其中 80% 是"你好""今天天气"之类的简单问题，20% 是需要强推理的复杂任务：

| 方案 | 简单请求 (80 万 token) | 复杂请求 (20 万 token) | 月费用 |
|------|----------------------|----------------------|--------|
| 全用 V4 Pro | $0.435×0.8 + $0.87×0.8 = $1.04 | $0.435×0.2 + $0.87×0.2 = $0.26 | **$1.30** |
| **Aperture 路由** | flash: $0.14×0.8 + $0.28×0.8 = $0.34 | pro: $0.435×0.2 + $0.87×0.2 = $0.26 | **$0.60** |

**节省 54%**。实际场景中，简单请求的比例通常更高，节省效果更明显。

## Dashboard 看效果

浏览器打开 `http://localhost:8080/dashboard`，输入 Admin Key，可以看到：

- 每个模型处理了多少请求
- 累计节省了多少钱
- 实时请求流

---

有问题？[GitHub Issues](https://github.com/Moguifeng-9119/aperture/issues)
