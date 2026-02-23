# ArknightsMaaRemoter

MAA 远程控制后端，基于 [MAA 远程控制协议](https://maa.plus)。

实现了协议要求的两个端点，并附带一个 Web 控制面板。MAA 每秒自动轮询，支持截图、一键长草、停止任务等全部标准任务类型。

## 快速开始

```bash
# 安装依赖
go mod tidy

# 启动服务（默认 :8080）
go run .

# 打开控制面板
# http://localhost:8080
```

MAA 配置填写：
- 获取任务端点：`http://localhost:8080/maa/getTask`
- 汇报任务端点：`http://localhost:8080/maa/reportStatus`
- 用户标识符：随意填写（单机用途不校验）

## 配置

通过环境变量配置：

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | `8080` | 监听端口 |
| `ADMIN_TOKEN` | （空） | 管理端点的 Bearer Token，不设置则无鉴权 |

示例：

```bash
PORT=9090 ADMIN_TOKEN=your-secret go run .
```

## 任务类型

| 类型 | 说明 |
|------|------|
| `LinkStart` | 一键长草（全部） |
| `LinkStart-Base` / `-WakeUp` / `-Combat` / `-Recruiting` / `-Mall` / `-Mission` / `-AutoRoguelike` / `-Reclamation` | 单独执行子功能 |
| `CaptureImageNow` | 立刻截图（不等队列） |
| `CaptureImage` | 排队截图 |
| `HeartBeat` | 心跳，返回当前执行的任务 ID |
| `StopTask` | 停止当前任务 |
| `Toolbox-GachaOnce` / `Toolbox-GachaTenTimes` | 牛牛抽卡 |
| `Settings-ConnectionAddress` / `Settings-Stage1` | 修改 MAA 配置（需 params） |

## 文件说明

```
main.go            入口，路由注册
handler/handler.go HTTP 处理器（MAA 协议 + 管理接口 + 控制面板）
store/store.go     内存任务队列，自动持久化到 tasks.json
screenshots/       截图文件（自动创建）
tasks.json         任务历史持久化文件（自动创建）
```

---

## 升级到远程控制

当前是本地自托管模式（MAA 和服务端在同一台机器）。以下方案可以将服务暴露到公网，实现从任意位置控制 MAA，**代码无需修改**。

### 方案一：Cloudflare Tunnel（推荐，免费，无需公网 IP）

适合：有 Cloudflare 账号，不想折腾服务器的情况。

```bash
# 1. 安装 cloudflared
# https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/

# 2. 登录并创建隧道（只需执行一次）
cloudflared tunnel login
cloudflared tunnel create maa-remote

# 3. 创建配置文件 ~/.cloudflared/config.yml
tunnel: <你的 tunnel ID>
credentials-file: /path/to/<tunnel-id>.json
ingress:
  - hostname: maa.yourdomain.com
    service: http://localhost:8080
  - service: http_status:404

# 4. 启动隧道
cloudflared tunnel run maa-remote
```

MAA 端填写：`https://maa.yourdomain.com/maa/getTask`

**优点**：免费、有 HTTPS、无需端口映射、支持自定义域名。

---

### 方案二：VPS 部署（完全控制）

适合：已有 VPS，想长期稳定运行。

```bash
# 构建 Linux 二进制
GOOS=linux GOARCH=amd64 go build -o maa-remote .

# 上传到 VPS
scp maa-remote user@your-vps:/opt/maa-remote/

# 在 VPS 上运行（systemd 服务）
# /etc/systemd/system/maa-remote.service
[Unit]
Description=MAA Remote

[Service]
ExecStart=/opt/maa-remote/maa-remote
WorkingDirectory=/opt/maa-remote
Environment=PORT=8080
Environment=ADMIN_TOKEN=your-secret-token
Restart=always

[Install]
WantedBy=multi-user.target
```

建议在 VPS 上用 Nginx 做反代并配置 HTTPS（Let's Encrypt）：

```nginx
server {
    listen 443 ssl;
    server_name maa.yourdomain.com;
    # ssl_certificate / ssl_certificate_key 省略...

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        # 截图任务响应体较大，放宽限制
        client_max_body_size 100m;
    }
}
```

---

### 方案三：frp 内网穿透（有公网 IP 的朋友帮忙中转）

适合：有一台有公网 IP 的服务器，但不想把业务跑在上面。

```toml
# frpc.toml（本地机器）
serverAddr = "your-server-ip"
serverPort = 7000

[[proxies]]
name = "maa-remote"
type = "tcp"
localIP = "127.0.0.1"
localPort = 8080
remotePort = 18080
```

MAA 端填写：`http://your-server-ip:18080/maa/getTask`

---

### 安全建议（暴露公网前务必确认）

- [ ] 设置 `ADMIN_TOKEN` 环境变量保护管理接口
- [ ] 使用 HTTPS（Cloudflare Tunnel 自带；VPS 方案用 Nginx + Let's Encrypt）
- [ ] MAA 协议端点（`/maa/*`）无需鉴权，这是协议要求，正常现象
- [ ] 截图体积可达数十 MB，确认反代的 `client_max_body_size` 足够大
