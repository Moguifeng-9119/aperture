# aperture

Claude Code → 任意模型代理。只做两件事：**协议翻译** + **关键词路由**。

## 是什么

```
Claude Code (Anthropic 格式) → aperture (:8080) → DeepSeek / OpenAI / ... (OpenAI 格式)
```

Claude Code 说 Anthropic 格式的 API，大部分模型说 OpenAI 格式。aperture 在中间翻译。

## 为什么精简

[v0.8.0 及之前](https://github.com/Moguifeng-9119/aperture/issues/1) 有 dashboard、admin API、ML 路由、A/B 测试、K8s 部署等 47 个源文件，个人无法维护。

v1.0 砍到 3 个文件：[main.go](main.go) + [proxy.go](proxy.go) + [router.go](router.go)。旧版完整代码在 tag [`v0.8.0-archived`](https://github.com/Moguifeng-9119/aperture/releases/tag/v0.8.0-archived)。

## 快速开始

```bash
# 1. 配置
cp config.example.yaml config.yaml
# 编辑 config.yaml — 填入 API key

# 2. 启动
go run . --config config.yaml

# 3. 配置 Claude Code (settings.json)
# "ANTHROPIC_BASE_URL": "http://localhost:8080"
```

## 路由规则

`config.yaml` 的 `routing.rules` 里配关键词：

```yaml
rules:
  - name: "debug"
    keywords: ["debug", "bug", "error", "not working"]
    model: "deepseek-v4-pro"       # 复杂 → 贵模型
    provider: "deepseek"

  - name: "simple"
    keywords: ["hello", "explain", "what is"]
    model: "deepseek-v4-flash"     # 简单 → 便宜模型
    provider: "deepseek"
```

没命中规则走 `routing.default_model`。

## 对比

| | v0.8.0 | v1.0 |
|------|:--:|:--:|
| 源文件 | 47 | 3 |
| Dashboard | ✅ | ❌ |
| Admin API | ✅ | ❌ |
| ML 路由 | ✅ | ❌ |
| K8s/Helm | ✅ | ❌ |
| 协议翻译 | ✅ | ✅ |
| 关键词路由 | ✅ | ✅ |

## License

MIT
