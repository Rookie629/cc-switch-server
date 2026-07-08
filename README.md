# cc-switch

一键切换 Claude Code 的 AI 提供商（DeepSeek、OpenAI 等）。

## 快速开始

```bash
# 1. 添加提供商
./cc-switch add --preset deepseek --key sk-your-api-key

# 2. 切换
./cc-switch set deepseek

# 3. 查看状态
./cc-switch status
```

## 常用命令

```bash
./cc-switch list              # 列出所有提供商
./cc-switch status            # 当前使用的提供商
./cc-switch set <name>        # 切换提供商
./cc-switch add               # 交互式添加
./cc-switch add --preset deepseek --key <key>   # 用预设添加
./cc-switch edit <name> --default-model <model> # 修改
./cc-switch remove <name>     # 删除
./cc-switch proxy-status      # 代理状态
```

## 内建预设

`anthropic-official` `deepseek` `openai` `groq` `openrouter` `siliconflow` `zhipu` `qwen` `ollama` ...

## 部署到服务器

```bash
# 编译
make build-linux

# 上传
scp cc-switch web/* root@server:/opt/cc-switch/

# 服务器上直接运行
cd /opt/cc-switch
./cc-switch serve --host 0.0.0.0 --port 9876 --release
```

## 原理

非 Anthropic 提供商（DeepSeek 等）自动启动本地翻译代理，将 Claude Code 的 Anthropic API 请求转换为 OpenAI 兼容格式。
