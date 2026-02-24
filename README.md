# ArknightsMaaRemoter

MAA 远程控制后端，基于 [MAA 远程控制协议](https://maa.plus)。

实现了协议要求的两个端点，并附带一个 Web 控制面板。MAA 每秒自动轮询，支持截图、一键长草、停止任务等全部标准任务类型。

**适合人群**：已经在电脑上把 MAA 配置好了，但是经常不在电脑旁边、或者懒得专门走过去开一次的人。这个项目只负责远程触发任务，具体的功能配置（打哪个关卡、招募怎么选、肉鸽难度等）还是要在电脑上的 MAA 里提前设置好。

参考视频（MAA 远程控制功能介绍）：[BV1Wmh8zQE7d](https://www.bilibili.com/video/BV1Wmh8zQE7d/)

---

## 使用指南

### 第一步：获取程序

**方式一（推荐）：下载启动器**

1. 从 [Releases](https://github.com/Cass-ette/ArknightsMaaRemoter-/releases/latest) 下载以下文件：
   - `ArknightsMaaRemoter.exe`
     
2. 如果下载有问题双击 `启动器.bat` 即可启动，浏览器会自动打开控制面板。

> 如果没有 `ArknightsMaaRemoter.exe`，启动器会自动从 GitHub 下载最新版本。

**方式二：从源码构建（需要已安装 Go 1.21+）**

```bash
git clone https://github.com/Cass-ette/ArknightsMaaRemoter-.git
cd ArknightsMaaRemoter-
go mod tidy
go run .
```

---

### 第二步：配置 MAA

打开 MAA → 设置 → 远程控制，填写以下内容：

| 字段 | 填写值 |
|------|--------|
| 获取任务端点 | `http://localhost:8080/maa/getTask` |
| 汇报任务端点 | `http://localhost:8080/maa/reportStatus` |
| 用户标识符 | 随意填写（单机模式不校验） |
| 轮询间隔 | 1000（毫秒，默认值即可） |

填写完毕后点击「启动」，MAA 会开始每秒轮询任务队列。

---

### 第三步：使用控制面板

打开浏览器访问 `http://localhost:8080`，即可看到控制面板。

- **下发任务**：点击任务类型按钮，立即加入队列，MAA 下次轮询时会自动取走执行
- **任务状态**：
  - `等待中` — 任务已下发，等待 MAA 取走执行
  - `已完成` — MAA 已完成任务并回调
  - `失败` — 任务执行失败或被手动中止
- **截图查看**：执行截图任务后，可在任务列表点击对应条目查看截图
- **自动刷新**：页面每 2 秒自动刷新，无需手动操作

---

### 管理员 Token（可选）

如果需要保护控制面板（多人局域网环境或暴露公网时），可设置 `ADMIN_TOKEN` 环境变量：

**Windows 命令行启动：**
```cmd
set ADMIN_TOKEN=your-secret-token
ArknightsMaaRemoter.exe
```

**Windows 环境变量（永久）：**
系统属性 → 高级 → 环境变量 → 新建 `ADMIN_TOKEN`，值填你的密码。

设置后，下发任务、查看任务列表、获取截图等管理接口均需在请求头中携带：
```
Authorization: Bearer your-secret-token
```
控制面板会自动处理，直接在浏览器中使用无需额外操作。

> MAA 的轮询端点（`/maa/getTask`、`/maa/reportStatus`）无需 Token，这是协议规定的。

---

## 任务类型说明

| 任务 | 说明 |
|------|------|
| 一键长草（全部） | 执行全部已启用的长草子任务 |
| 开始唤醒 / 基建换班 / 自动战斗 | 单独执行对应子功能 |
| 自动公招 / 购物 / 日常任务 | 单独执行对应子功能 |
| 自动肉鸽 / 生息演算 | 单独执行对应子功能 |
| 立刻截图 | 插队截图，不等待队列 |
| 排队截图 | 按顺序截图 |
| 心跳 | 返回当前 MAA 正在执行的任务 ID |
| 停止任务 | 中止 MAA 当前正在执行的任务 |
| 牛牛抽卡（单次/十连） | 工具箱功能 |

---

## 文件结构

```
ArknightsMaaRemoter.exe   主程序
启动器.bat                双击启动，自动下载主程序并打开浏览器
static/
  bkg7.png               控制面板背景图
  Top.png                返回顶部按钮图标
screenshots/             截图文件（运行后自动创建）
tasks.json               任务历史（运行后自动创建，重启不丢失）
```

---

## 升级到远程控制

当前是本机模式（MAA 和服务端在同一台电脑）。以下方案可将服务暴露到公网，实现从任意位置控制 MAA，**代码无需修改**。

### 方案一：Cloudflare Tunnel（推荐，免费，无需公网 IP）

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

MAA 端改填：`https://maa.yourdomain.com/maa/getTask`

**优点**：免费、自带 HTTPS、无需端口映射、支持自定义域名。

---

### 方案二：VPS 部署（完全控制）

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

建议用 Nginx 做反代并配置 HTTPS（Let's Encrypt）：

```nginx
server {
    listen 443 ssl;
    server_name maa.yourdomain.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        client_max_body_size 100m;
    }
}
```

---

### 方案三：frp 内网穿透

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

MAA 端改填：`http://your-server-ip:18080/maa/getTask`

---

### 安全建议（暴露公网前务必确认）

- [ ] 设置 `ADMIN_TOKEN` 环境变量保护管理接口
- [ ] 使用 HTTPS（Cloudflare Tunnel 自带；VPS 方案用 Nginx + Let's Encrypt）
- [ ] MAA 协议端点（`/maa/*`）无需鉴权，这是协议要求，正常现象
- [ ] 截图体积可达数十 MB，确认反代的 `client_max_body_size` 足够大

---

## 发布新版本

推送带 `v` 前缀的 tag，GitHub Actions 会自动构建并发布 Release：

```bash
git tag v1.0.0
git push origin v1.0.0
```
