# cc-switch-server 部署指南

## 项目概述

cc-switch-server 是一个轻量级 CLI + Web 工具，用于在服务器上管理 Claude Code 的 AI 提供商配置。

**核心功能：**
- 管理多个 AI API 提供商（Anthropic 官方、DeepSeek、OpenAI、Ollama 等 15+ 预设）
- 一键切换 Claude Code 使用的 AI 提供商
- 内置 Anthropic ↔ OpenAI 格式翻译代理（非 Anthropic 提供商自动启用）
- Web 管理面板 + CLI 命令行双模式
- 数据持久化 + 自动备份

## 部署方式

### 方式一：systemd 服务（推荐）

适合有 systemd 的 Linux 服务器（Ubuntu 18.04+、Debian 10+、CentOS 7+）。

```bash
# 1. 在开发机上构建静态二进制
cd cc-switch-server
make build-linux

# 2. 拷贝到服务器
scp cc-switch web/* user@server:/tmp/

# 3. 在服务器上安装
ssh user@server
sudo mkdir -p /opt/cc-switch-server/web /var/lib/cc-switch-server/backups
sudo cp /tmp/cc-switch /opt/cc-switch-server/
sudo cp /tmp/*.html /tmp/*.css /tmp/*.js /opt/cc-switch-server/web/
sudo chmod +x /opt/cc-switch-server/cc-switch

# 4. 注册 systemd 服务
sudo cp deploy/cc-switch-server.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable cc-switch-server
sudo systemctl start cc-switch-server

# 5. 验证
sudo systemctl status cc-switch-server
curl http://127.0.0.1:9876/api/providers
```

**管理命令：**
```bash
sudo systemctl start cc-switch-server     # 启动
sudo systemctl stop cc-switch-server      # 停止
sudo systemctl restart cc-switch-server   # 重启
sudo systemctl status cc-switch-server    # 状态
sudo journalctl -u cc-switch-server -f    # 查看日志
```

### 方式二：Docker 部署

适合容器化环境。

```bash
# 构建镜像
docker build -t cc-switch-server:latest .

# 运行容器
docker run -d \
  --name cc-switch-server \
  --restart unless-stopped \
  -p 9876:9876 \
  -v cc-switch-data:/var/lib/cc-switch-server \
  -v $HOME/.claude:/home/ccswitch/.claude:rw \
  cc-switch-server:latest

# 或使用 docker compose
docker compose up -d
```

**Docker 管理命令：**
```bash
docker compose up -d              # 启动
docker compose down               # 停止
docker compose logs -f            # 查看日志
docker compose exec cc-switch cc-switch list    # 执行 CLI 命令
```

### 方式三：独立二进制

适合无 systemd 的环境或手动部署。

```bash
# 构建
make build-linux

# 拷贝到服务器任意目录
scp cc-switch user@server:/usr/local/bin/
ssh user@server chmod +x /usr/local/bin/cc-switch

# 直接运行
cc-switch serve --host 0.0.0.0 --port 9876 --release
```

可以配合 `nohup`、`screen`、`tmux` 或 `supervisor` 保持后台运行。

## 配置

### systemd 服务配置

编辑 `/etc/systemd/system/cc-switch-server.service`，修改以下参数：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--host` | `0.0.0.0` | 监听地址，`127.0.0.1` 仅本地访问 |
| `--port` | `9876` | Web 面板端口 |
| `--data-dir` | `/var/lib/cc-switch-server` | 数据存储路径 |
| `User` | `youruser` | 运行用户（需要能访问 `~/.claude/`） |
| `WorkingDirectory` | `/opt/cc-switch-server` | 工作目录（web/ 在此下） |

### 环境变量

| 变量 | 说明 |
|------|------|
| `GIN_MODE=release` | 生产模式（关闭 debug 日志） |
| `TZ=Asia/Shanghai` | 时区设置 |

## 使用

```bash
# 查看所有提供商
cc-switch list

# 查看当前使用的提供商
cc-switch status

# 添加新提供商（交互式）
cc-switch add

# 添加新提供商（预设）
cc-switch add --preset deepseek --key sk-your-key

# 切换提供商
cc-switch set deepseek

# 查看某提供商的模型列表
cc-switch models deepseek

# 查看代理状态
cc-switch proxy-status

# 查看数据文件路径
cc-switch config-path

# 启动 Web 面板
cc-switch serve

# 生产模式启动
cc-switch serve --release --host 0.0.0.0 --port 9876
```

## 反向代理（Nginx）

如果需要通过域名访问或添加 HTTPS，配置 Nginx：

```bash
sudo cp deploy/nginx.conf /etc/nginx/sites-available/cc-switch-server
sudo ln -s /etc/nginx/sites-available/cc-switch-server /etc/nginx/sites-enabled/
# 编辑配置文件，修改 server_name
sudo nginx -t
sudo systemctl reload nginx
```

## 数据文件

| 文件 | 说明 |
|------|------|
| `~/.cc-switch-server/providers.json` | 提供商配置（含 API Key） |
| `~/.cc-switch-server/backups/` | 自动备份（保留最近 10 份） |
| `~/.claude/settings.json` | Claude Code 配置（由 cc-switch 写入） |
| `~/.claude/settings.json.cc-switch-backup` | 切换前的自动备份 |

## 构建

```bash
# 查看所有构建目标
make help

# 当前平台构建
make build

# 静态链接（无依赖）
make build-static

# Linux amd64 交叉编译
make build-linux

# 多平台构建
make build-all        # 输出到 dist/

# Docker 镜像
make docker

# 安装到系统
sudo make install     # 安装到 /opt/cc-switch-server + systemd
sudo make uninstall   # 卸载
```

## 安全建议

1. **API Key 存储**：providers.json 包含明文 API Key，建议设置文件权限 `chmod 600`
2. **网络访问**：如只需本机访问，改为 `--host 127.0.0.1`
3. **HTTPS**：生产环境建议配置 Nginx 反向代理 + Let's Encrypt 证书
4. **防火墙**：限制 Web 面板端口的公网访问
5. **定期备份**：`~/.cc-switch-server/backups/` 已自动保留最近 10 份
