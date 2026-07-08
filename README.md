# cc-switch-server

CLI + Web panel for managing Claude Code's AI provider configuration on remote servers.

Swap between Anthropic, DeepSeek, OpenAI, Ollama (15+ presets) with a single command — no manual config editing. Non-Anthropic providers get an automatic local translation proxy that converts Anthropic API requests to OpenAI-compatible format.

## Features

- **One-command provider switching** — `cc-switch set deepseek`
- **15+ built-in presets** — DeepSeek, OpenAI, Anthropic, Groq, OpenRouter, Ollama, Zhipu, Qwen, SiliconFlow, Moonshot, Fireworks, Together, XAI, Baidu, NVIDIA NIM
- **Built-in format proxy** — Translates Anthropic ↔ OpenAI wire format automatically (streaming, tool calls, system prompts)
- **Web management panel** — Add, edit, and switch providers from a browser
- **Zero-dependency binary** — 11 MB statically linked, no runtime dependencies
- **Deploy anywhere** — systemd, Docker, or standalone binary

## Quick Start

```bash
# Download a pre-built binary or build from source
git clone https://github.com/Rookie629/cc-switch-server.git
cd cc-switch-server
make build-static

# Add a provider via interactive prompt
./cc-switch add

# Or use a preset with your API key
./cc-switch add --preset deepseek --key sk-your-key

# Switch to it
./cc-switch set deepseek

# Done — Claude Code now uses this provider
./cc-switch status
```

## Installation

### Standalone binary

```bash
make build-linux                       # cross-compile for Linux amd64
scp cc-switch web/* user@server:/opt/cc-switch/
ssh user@server
cd /opt/cc-switch
./cc-switch serve --host 0.0.0.0 --release
```

### From source

```bash
go install github.com/Rookie629/cc-switch-server@latest
# or
make build          # current platform
make build-static   # static binary (CGO_ENABLED=0)
make build-linux    # cross-compile for linux/amd64
make build-all      # all platforms → dist/
```

### Docker

```bash
docker compose up -d     # Web panel at http://localhost:9876
```

### systemd (production)

```bash
sudo make install
sudo systemctl start cc-switch-server
# Web panel: http://<server-ip>:9876
```

## Commands

| Command | Description |
|---------|-------------|
| `cc-switch list` | List all configured providers |
| `cc-switch status` | Show current active provider |
| `cc-switch set <name>` | Switch active provider (auto-starts proxy if needed) |
| `cc-switch add` | Interactive add |
| `cc-switch add --preset <name> --key <key>` | Add with preset |
| `cc-switch edit <name> --default-model <m>` | Modify a provider |
| `cc-switch remove <name>` | Delete a provider |
| `cc-switch models <name>` | List models for a provider |
| `cc-switch proxy-status` | Check proxy health |
| `cc-switch proxy-stop` | Stop the translation proxy |
| `cc-switch serve` | Start Web management panel |
| `cc-switch config-path` | Print data file location |
| `cc-switch backup` | Manually backup providers data |

### serve flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `9876` | Listen port |
| `--host` | `127.0.0.1` | Bind address (`0.0.0.0` for external access) |
| `--release` | `false` | Production mode (quiet logging) |
| `--data-dir` | `~/.cc-switch-server` | Data storage path |

## Project Structure

```
cc-switch-server/
├── main.go                          # Entry point, version injection
├── internal/
│   ├── api/router.go               # REST API + Web panel routes
│   ├── cli/                         # Cobra CLI commands
│   │   ├── add.go, edit.go, remove.go  # Provider CRUD
│   │   ├── set.go, list.go, status.go  # Activation & inspection
│   │   ├── proxy.go, proxy_daemon.go   # Proxy lifecycle
│   │   └── serve.go                    # Web server
│   ├── service/
│   │   ├── provider.go             # Provider business logic
│   │   ├── proxy.go                # Anthropic ↔ OpenAI translation proxy
│   │   └── config_writer.go        # Writes ~/.claude/settings.json
│   ├── store/
│   │   ├── models.go               # Data types
│   │   └── store.go                # JSON file persistence (atomic write + backups)
│   └── preset/presets.go           # 15+ built-in provider presets
├── web/                             # Frontend (vanilla JS, no build step)
│   ├── index.html
│   ├── app.js, style.css
├── deploy/
│   ├── cc-switch-server.service    # systemd unit
│   ├── install.sh, nginx.conf
│   └── README.md                   # Detailed deployment guide
├── Dockerfile, docker-compose.yml, Makefile
└── go.mod, go.sum
```

## How It Works

```
Claude Code                cc-switch proxy              Upstream API
     │                          │                          │
     │ Anthropic API ──────────>│                          │
     │ POST /v1/messages        │                          │
     │                          │ OpenAI Chat ───────────>│
     │                          │ POST /v1/chat/completions│
     │                          │                          │
     │                          │<───── OpenAI response ──│
     │<──── Anthropic response ─│                          │
     │                          │                          │
```

- **Anthropic-native providers** (api.anthropic.com) — no proxy needed, just writes `ANTHROPIC_AUTH_TOKEN` + `ANTHROPIC_BASE_URL` to `~/.claude/settings.json`
- **OpenAI-compatible providers** (DeepSeek, Ollama, etc.) — starts a background `proxy-daemon` that translates:
  - Tool definitions: `input_schema` → `function.parameters`
  - Content blocks: array/string `content` → plain text
  - Stream events: named SSE (`message_start`, `content_block_delta`) → OpenAI delta chunks and back
  - Tool calls: `tool_use` blocks → `message.tool_calls[]` and back
  - `system` field: both string and array-of-blocks formats

## Supported Providers (built-in presets)

| Preset | Type | Default Model |
|--------|------|---------------|
| `anthropic-official` | anthropic | claude-sonnet-4-6 |
| `deepseek` | openai_compatible | deepseek-chat |
| `openai` | openai | gpt-4o |
| `openrouter` | openai_compatible | anthropic/claude-sonnet-4-6 |
| `siliconflow` | openai_compatible | deepseek-ai/DeepSeek-V3 |
| `zhipu` | openai_compatible | glm-4-plus |
| `qwen` | openai_compatible | qwen-plus |
| `moonshot` | openai_compatible | moonshot-v1-32k |
| `groq`, `together`, `fireworks`, `xai-grok`, `baidu-ernie`, `nvidia-nim`, `ollama` | ... | ... |

## Data

| File | Purpose |
|------|---------|
| `~/.cc-switch-server/providers.json` | Provider configs (permissions: 600) |
| `~/.cc-switch-server/backups/` | Auto-backup before each write (keeps 10) |
| `~/.claude/settings.json` | Claude Code config (written by cc-switch) |
| `~/.cc-switch-server/proxy.pid` | Daemon proxy PID + port |

## Security

- `providers.json` stores API keys in plain text — set `chmod 600`
- Bind to `127.0.0.1` if only local access is needed; use `--host 0.0.0.0` behind Nginx for remote access
- HTTPS via Nginx reverse proxy + Let's Encrypt (see `deploy/nginx.conf`)
- Firewall-limit the web panel port from public access

## License

MIT
