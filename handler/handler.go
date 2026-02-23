package handler

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"ArknightsMaaRemoter/store"
)

type Handler struct {
	store *store.Store
}

func New(s *store.Store) *Handler {
	return &Handler{store: s}
}

// ── MAA 协议类型 ──────────────────────────────────────────────

type getTaskReq struct {
	User   string `json:"user"`
	Device string `json:"device"`
}

type taskItem struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Params string `json:"params,omitempty"`
}

type getTaskResp struct {
	Tasks []taskItem `json:"tasks"`
}

type reportReq struct {
	User    string `json:"user"`
	Device  string `json:"device"`
	Task    string `json:"task"`
	Status  string `json:"status"`
	Payload string `json:"payload"`
}

// ── MAA 协议端点 ──────────────────────────────────────────────

// GetTask 是 MAA 每秒轮询的获取任务端点
func (h *Handler) GetTask(c *gin.Context) {
	var req getTaskReq
	_ = c.ShouldBindJSON(&req)

	pending := h.store.Pending()
	items := make([]taskItem, 0, len(pending))
	for _, t := range pending {
		items = append(items, taskItem{
			ID:     t.ID,
			Type:   t.Type,
			Params: t.Params,
		})
	}

	c.JSON(http.StatusOK, getTaskResp{Tasks: items})
}

// ReportStatus 接收 MAA 的任务执行结果
func (h *Handler) ReportStatus(c *gin.Context) {
	var req reportReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}

	payload := req.Payload
	if req.Payload != "" && req.Status == "SUCCESS" {
		t := h.store.Get(req.Task)
		if t != nil && isScreenshotTask(t.Type) {
			if path, err := saveScreenshot(req.Task, req.Payload); err == nil {
				payload = path
			}
		}
	}

	h.store.Complete(req.Task, req.Status, payload)
	c.JSON(http.StatusOK, gin.H{})
}

func isScreenshotTask(taskType string) bool {
	return taskType == "CaptureImage" || taskType == "CaptureImageNow"
}

func saveScreenshot(taskID, b64data string) (string, error) {
	if err := os.MkdirAll("screenshots", 0755); err != nil {
		return "", err
	}
	filename := fmt.Sprintf("screenshots/%s_%s.png",
		time.Now().Format("20060102_150405"), taskID[:8])
	data, err := base64.StdEncoding.DecodeString(b64data)
	if err != nil {
		return "", err
	}
	return filename, os.WriteFile(filename, data, 0644)
}

// ── 管理端点 ──────────────────────────────────────────────────

type submitTaskReq struct {
	Type   string `json:"type" binding:"required"`
	Params string `json:"params"`
}

// AdminAuth 是可选的 Bearer Token 认证中间件
// 通过环境变量 ADMIN_TOKEN 配置，不设置则不鉴权
func (h *Handler) AdminAuth() gin.HandlerFunc {
	token := os.Getenv("ADMIN_TOKEN")
	return func(c *gin.Context) {
		if token == "" {
			c.Next()
			return
		}
		if c.GetHeader("Authorization") != "Bearer "+token {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

// SubmitTask 向队列添加一个任务
func (h *Handler) SubmitTask(c *gin.Context) {
	var req submitTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	t := h.store.Add(req.Type, req.Params)
	c.JSON(http.StatusOK, t)
}

// ListTasks 返回所有任务列表（最新在前）
func (h *Handler) ListTasks(c *gin.Context) {
	c.JSON(http.StatusOK, h.store.All())
}

// GetScreenshot 提供截图文件下载
func (h *Handler) GetScreenshot(c *gin.Context) {
	id := c.Param("id")
	t := h.store.Get(id)
	if t == nil || t.Payload == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.File(t.Payload)
}

// Dashboard 提供简单的 Web 控制面板
func (h *Handler) Dashboard(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, dashboardHTML)
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>MAA Remote</title>
<link rel="icon" href="https://prts.wiki/favicon.ico">
<style>
  * { box-sizing: border-box; }
  html { background-image: url('/static/bkg7.png'); background-size: cover; background-attachment: fixed; background-position: center; min-height: 100%; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; max-width: 900px; margin: 40px auto; padding: 0 20px; color: #333; }
  #top-btn { position: fixed; right: 24px; bottom: 40px; cursor: pointer; opacity: 0.85; transition: opacity 0.2s; }
  #top-btn:hover { opacity: 1; }
  #top-btn img { width: 48px; height: 48px; display: block; }
  h1 { font-size: 22px; margin-bottom: 4px; }
  .sub { color: #888; font-size: 13px; margin-bottom: 24px; }
  .toolbar { display: flex; gap: 8px; flex-wrap: wrap; align-items: center; margin-bottom: 16px; }
  select, input, button { padding: 7px 12px; border: 1px solid #ddd; border-radius: 6px; font-size: 14px; }
  input[type=password] { width: 180px; }
  button { background: #2563eb; color: #fff; border-color: #2563eb; cursor: pointer; }
  button:hover { background: #1d4ed8; }
  button.secondary { background: #f3f4f6; color: #374151; border-color: #d1d5db; }
  button.secondary:hover { background: #e5e7eb; }
  table { width: 100%; border-collapse: collapse; font-size: 13px; }
  th { background: #f9fafb; padding: 9px 12px; text-align: left; border-bottom: 2px solid #e5e7eb; }
  td { padding: 9px 12px; border-bottom: 1px solid #f3f4f6; }
  tr:hover td { background: #f9fafb; }
  .PENDING { color: #92400e; background: #fef3c7; padding: 2px 7px; border-radius: 4px; font-size: 11px; }
  .SUCCESS { color: #065f46; background: #d1fae5; padding: 2px 7px; border-radius: 4px; font-size: 11px; }
  .FAILED  { color: #991b1b; background: #fee2e2; padding: 2px 7px; border-radius: 4px; font-size: 11px; }
  .id { font-family: monospace; font-size: 11px; color: #6b7280; }
  a { color: #2563eb; }
  #params-wrap { display: none; }
  .hint { font-size: 12px; color: #9ca3af; margin-left: 4px; }
  .title-bar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px; }
  .title-links { display: flex; align-items: center; gap: 16px; }
  .icon-link { display: flex; align-items: center; gap: 6px; color: #374151; text-decoration: none; font-size: 13px; transition: color 0.15s; }
  .icon-link:hover { color: #111; }
  .icon-link svg { width: 20px; height: 20px; fill: currentColor; flex-shrink: 0; }
  .qq-info { display: flex; align-items: center; gap: 6px; color: #374151; font-size: 13px; }
  .qq-info svg { width: 20px; height: 20px; fill: #12B7F5; flex-shrink: 0; }
</style>
</head>
<body>
<div class="title-bar">
  <div>
    <h1>MAA Remote</h1>
    <p class="sub">控制面板 · 每 2 秒自动刷新</p>
  </div>
  <div class="title-links">
    <a href="https://github.com/Cass-ette/ArknightsMaaRemoter-" target="_blank" class="icon-link" title="GitHub 仓库">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"/></svg>
      GitHub
    </a>
    <span class="qq-info">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M21.395 15.035a39.548 39.548 0 0 0-.803-2.264c.396-1.108.656-2.476.656-4.148C21.248 3.473 17.824 0 12 0S2.752 3.473 2.752 8.623c0 1.805.302 3.268.757 4.396a32.5 32.5 0 0 0-.802 2.016C1.054 19.456.682 23.394 2.07 23.933c1.048.415 2.698-.68 4.186-2.777.929.299 2.14.63 3.3.832C9.956 23.5 10.96 24 12 24c1.04 0 2.044-.5 2.444-2.012 1.16-.202 2.37-.533 3.3-.832 1.488 2.096 3.138 3.192 4.186 2.777 1.388-.539 1.017-4.477-.535-8.898zm-7.256 4.806c-.437.116-1.216.179-2.139.179s-1.702-.063-2.139-.179c-.265-.07-.376-.172-.399-.229-.095-.233.274-.61.663-.907.406-.31.903-.48 1.416-.48.08 0 .16.01.239.027.16.035.32.053.48.053.16 0 .32-.018.48-.053a1.86 1.86 0 0 1 .239-.027c.513 0 1.01.17 1.416.48.389.297.758.674.663.907-.023.057-.134.159-.399.229z"/></svg>
      1804534439
    </span>
    <a href="https://space.bilibili.com/480210191" target="_blank" class="icon-link" title="bilibili 主页">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" style="fill:#00aeec"><path d="M17.813 4.653h.854c1.51.054 2.769.578 3.773 1.574 1.004.995 1.524 2.249 1.56 3.76v7.36c-.036 1.51-.556 2.769-1.56 3.773s-2.262 1.524-3.773 1.56H5.333c-1.51-.036-2.769-.556-3.773-1.56S.036 18.858 0 17.347v-7.36c.036-1.511.556-2.765 1.56-3.76 1.004-.996 2.262-1.52 3.773-1.574h.774l-1.174-1.12a1.234 1.234 0 0 1-.373-.906c0-.356.124-.658.373-.907l.027-.027c.267-.249.573-.373.92-.373.347 0 .653.124.92.373L9.653 4.44c.071.071.134.142.187.213h4.267a.836.836 0 0 1 .16-.213l2.853-2.747c.267-.249.573-.373.92-.373.347 0 .662.151.929.4.267.249.391.551.391.907 0 .355-.124.657-.373.906zM5.333 7.24c-.746.018-1.373.276-1.88.773-.506.498-.769 1.13-.786 1.894v7.52c.017.764.28 1.395.786 1.893.507.498 1.134.756 1.88.773h13.334c.746-.017 1.373-.275 1.88-.773.506-.498.769-1.129.786-1.893v-7.52c-.017-.765-.28-1.396-.786-1.894-.507-.497-1.134-.755-1.88-.773zM8 11.107c.373 0 .684.124.933.373.25.249.383.569.4.96v1.173c-.017.391-.15.711-.4.96-.249.25-.56.374-.933.374s-.684-.125-.933-.374c-.25-.249-.383-.569-.4-.96V12.44c0-.373.129-.689.386-.947.258-.257.574-.386.947-.386zm8 0c.373 0 .684.124.933.373.25.249.383.569.4.96v1.173c-.017.391-.15.711-.4.96-.249.25-.56.374-.933.374s-.684-.125-.933-.374c-.25-.249-.383-.569-.4-.96V12.44c.017-.391.15-.711.4-.96.249-.249.56-.373.933-.373z"/></svg>
      bilibili
    </a>
  </div>
</div>

<div id="top-btn" onclick="window.scrollTo({top:0,behavior:'smooth'})">
  <img src="/static/top.png" alt="top">
</div>

<div class="toolbar">
  <select id="type" onchange="onTypeChange()">
    <optgroup label="一键长草">
      <option value="LinkStart">一键长草 (LinkStart)</option>
      <option value="LinkStart-Base">基建 (LinkStart-Base)</option>
      <option value="LinkStart-WakeUp">唤醒登录 (LinkStart-WakeUp)</option>
      <option value="LinkStart-Combat">刷关卡 (LinkStart-Combat)</option>
      <option value="LinkStart-Recruiting">公开招募 (LinkStart-Recruiting)</option>
      <option value="LinkStart-Mall">商店 (LinkStart-Mall)</option>
      <option value="LinkStart-Mission">日常任务 (LinkStart-Mission)</option>
      <option value="LinkStart-AutoRoguelike">自动肉鸽 (LinkStart-AutoRoguelike)</option>
      <option value="LinkStart-Reclamation">生息演算 (LinkStart-Reclamation)</option>
    </optgroup>
    <optgroup label="截图">
      <option value="CaptureImageNow">立刻截图 (CaptureImageNow)</option>
      <option value="CaptureImage">排队截图 (CaptureImage)</option>
    </optgroup>
    <optgroup label="控制">
      <option value="HeartBeat">心跳检测 (HeartBeat)</option>
      <option value="StopTask">停止当前任务 (StopTask)</option>
    </optgroup>
    <optgroup label="工具箱">
      <option value="Toolbox-GachaOnce">牛牛抽卡单次 (Toolbox-GachaOnce)</option>
      <option value="Toolbox-GachaTenTimes">牛牛抽卡十连 (Toolbox-GachaTenTimes)</option>
    </optgroup>
    <optgroup label="设置">
      <option value="Settings-ConnectionAddress">修改连接地址 (Settings-ConnectionAddress)</option>
      <option value="Settings-Stage1">修改关卡 (Settings-Stage1)</option>
    </optgroup>
  </select>
  <span id="params-wrap">
    <input id="params" type="text" placeholder="参数值" style="width:160px" />
  </span>
  <button onclick="submit()">下发任务</button>
  <input id="token" type="password" placeholder="Admin Token（可选）" />
  <button class="secondary" onclick="load()">刷新</button>
  <span class="hint" id="status"></span>
</div>

<table>
  <thead>
    <tr>
      <th>时间</th>
      <th>类型</th>
      <th>状态</th>
      <th>Task ID</th>
      <th>操作</th>
    </tr>
  </thead>
  <tbody id="tasks"></tbody>
</table>

<script>
const TIME_ICON = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAABIAAAAUCAYAAACAl21KAAAACXBIWXMAAD2EAAA9hAHVrK90AAAF+mlUWHRYTUw6Y29tLmFkb2JlLnhtcAAAAAAAPD94cGFja2V0IGJlZ2luPSLvu78iIGlkPSJXNU0wTXBDZWhpSHpyZVN6TlRjemtjOWQiPz4gPHg6eG1wbWV0YSB4bWxuczp4PSJhZG9iZTpuczptZXRhLyIgeDp4bXB0az0iQWRvYmUgWE1QIENvcmUgNS42LWMxNDUgNzkuMTYzNDk5LCAyMDE4LzA4LzEzLTE2OjQwOjIyICAgICAgICAiPiA8cmRmOlJERiB4bWxuczpyZGY9Imh0dHA6Ly93d3cudzMub3JnLzE5OTkvMDIvMjItcmRmLXN5bnRheC1ucyMiPiA8cmRmOkRlc2NyaXB0aW9uIHJkZjphYm91dD0iIiB4bWxuczp4bXA9Imh0dHA6Ly9ucy5hZG9iZS5jb20veGFwLzEuMC8iIHhtbG5zOmRjPSJodHRwOi8vcHVybC5vcmcvZGMvZWxlbWVudHMvMS4xLyIgeG1sbnM6cGhvdG9zaG9wPSJodHRwOi8vbnMuYWRvYmUuY29tL3Bob3Rvc2hvcC8xLjAvIiB4bWxuczp4bXBNTT0iaHR0cDovL25zLmFkb2JlLmNvbS94YXAvMS4wL21tLyIgeG1sbnM6c3RFdnQ9Imh0dHA6Ly9ucy5hZG9iZS5jb20veGFwLzEuMC9zVHlwZS9SZXNvdXJjZUV2ZW50IyIgeG1wOkNyZWF0b3JUb29sPSJBZG9iZSBQaG90b3Nob3AgQ0MgMjAxOSAoV2luZG93cykiIHhtcDpDcmVhdGVEYXRlPSIyMDIwLTA2LTE0VDIwOjA5OjMzKzA4OjAwIiB4bXA6TW9kaWZ5RGF0ZT0iMjAyMC0wNi0xNFQyMDozMTo1MyswODowMCIgeG1wOk1ldGFkYXRhRGF0ZT0iMjAyMC0wNi0xNFQyMDozMTo1MyswODowMCIgZGM6Zm9ybWF0PSJpbWFnZS9wbmciIHBob3Rvc2hvcDpDb2xvck1vZGU9IjMiIHBob3Rvc2hvcDpJQ0NQcm9maWxlPSJzUkdCIElFQzYxOTY2LTIuMSIgeG1wTU06SW5zdGFuY2VJRD0ieG1wLmlpZDpiZTcwNmNjZi1mZWNmLTVmNDItYWJjNi1jYjA3MjI3NGY5M2YiIHhtcE1NOkRvY3VtZW50SUQ9ImFkb2JlOmRvY2lkOnBob3Rvc2hvcDo4MGU2Nzg3OS04ODNhLTRlNGUtOGY2Yi02MDM2NDQ4MGRkZmEiIHhtcE1NOk9yaWdpbmFsRG9jdW1lbnRJRD0ieG1wLmRpZDo4YTYzYzI3Ni01MjMwLTFhNDctODc0OS1lZjcxYmM5YmFkY2MiPiA8eG1wTU06SGlzdG9yeT4gPHJkZjpTZXE+IDxyZGY6bGkgc3RFdnQ6YWN0aW9uPSJjcmVhdGVkIiBzdEV2dDppbnN0YW5jZUlEPSJ4bXAuaWlkOjhhNjNjMjc2LTUyMzAtMWE0Ny04NzQ5LWVmNzFiYzliYWRjYyIgc3RFdnQ6d2hlbj0iMjAyMC0wNi0xNFQyMDowOTozMyswODowMCIgc3RFdnQ6c29mdHdhcmVBZ2VudD0iQWRvYmUgUGhvdG9zaG9wIENDIDIwMTkgKFdpbmRvd3MpIi8+IDxyZGY6bGkgc3RFdnQ6YWN0aW9uPSJzYXZlZCIgc3RFdnQ6aW5zdGFuY2VJRD0ieG1wLmlpZDpiZTcwNmNjZi1mZWNmLTVmNDItYWJjNi1jYjA3MjI3NGY5M2YiIHN0RXZ0OndoZW49IjIwMjAtMDYtMTRUMjA6MzE6NTMrMDg6MDAiIHN0RXZ0OnNvZnR3YXJlQWdlbnQ9IkFkb2JlIFBob3Rvc2hvcCBDQyAyMDE5IChXaW5kb3dzKSIgc3RFdnQ6Y2hhbmdlZD0iLyIvPiA8L3JkZjpTZXE+IDwveG1wTU06SGlzdG9yeT4gPC9yZGY6RGVzY3JpcHRpb24+IDwvcmRmOlJERj4gPC94OnhtcG1ldGE+IDw/eHBhY2tldCBlbmQ9InIiPz5z1LIYAAACwklEQVQ4y62US2gTURSGT17No0mTmKR5NG0ek0fzbt7QJm2alYJd1ARJcJO4qBAXRZeuClm6EJfixqVSBRGyMVsfCxciuhIsFER3ImorQu31vyNKZpKCCwc+Zpi5559z7n/OJcYY/Q9odnZ2DJvNNh0MBs+3220/njfUavVlk8mkMxqNZ/x+vxPfKBAISKCpqakxVCpVwuV2H+RyuQ2T0XhLp9O9MhgMFiJ67vP5mrFYjKLRqARaXFyUEI/HaX5+voYgJgjCOb1ev6tUKl8jI/yWPrjd7muhUIh4VqNMFFpYWGhyIafT+c1utx9ZrdZjLD7UaDRsZmbmusPhUOA9jUJcfZRIJEIej+cKF0IWDBkxrVbLzGYz4+/m5ubuYo1WHkeZTFpCNrukFoTgbR40CWT0AhmYYAKNQjqdVo5GpVLesVgsbGVl5bjRaLBqtcrLFDPDfr2FixZAoxACJKAEOoUrlUrdbLVaXwqFglgW3GSG6ekH8UQiki8UKJvLSRgTQurk9XoJNp9FKd9Hy0JJ20vZLOXy+XGhLD7IKRaLXGxHvj92m+1RNBIxhQSB5FC5XB6jUqkY0AL35ULY1DdwzIf+Ijli38BSsZw/QMSLfXkpF8Kmfka3p9fW1ggGiPBnrCdCqmRDQ9XW1wkOUbPZpNXV1RQCP9Xrdba8vCwRS6fTp2u12r8JIXiDB/X7fdbtdhk6m1v/FRyhjJ1MJkPJZJLgqDgNaI3fQi6Xi8K4Y7LFBXD/aqfTYXt7ezwDMROMxwGEfuDvuwjU8rHgppwohCMhg4/PuMg7sLm5Ke/s9xCpYN5OFsLUbyObn/v7+2wwGDDMEAuHw0yhUIyNCoRuTBTCiyrOoY+9Xo/x69LWFjtp3jg4nw5LpdKFv0J4WILVHdR/D9k8gVPDarUyREsMMf1DTP8kHmPPniLmIdZdxGmR+AVyGTWoivTwXwAAAABJRU5ErkJggg==';

const STATUS_NAMES = {
  'SUCCESS': '已完成',
  'FAILED':  '失败',
};

function statusBadge(s) {
  return '<span class="' + s + '">' + (STATUS_NAMES[s] || s) + '</span>';
}

const TYPE_NAMES = {
  'LinkStart':                   '一键长草',
  'LinkStart-Base':              '基建',
  'LinkStart-WakeUp':            '唤醒登录',
  'LinkStart-Combat':            '刷关卡',
  'LinkStart-Recruiting':        '公开招募',
  'LinkStart-Mall':              '商店',
  'LinkStart-Mission':           '日常任务',
  'LinkStart-AutoRoguelike':     '自动肉鸽',
  'LinkStart-Reclamation':       '生息演算',
  'CaptureImageNow':             '立刻截图',
  'CaptureImage':                '排队截图',
  'HeartBeat':                   '心跳检测',
  'StopTask':                    '停止当前任务',
  'Toolbox-GachaOnce':           '牛牛抽卡单次',
  'Toolbox-GachaTenTimes':       '牛牛抽卡十连',
  'Settings-ConnectionAddress':  '修改连接地址',
  'Settings-Stage1':             '修改关卡',
};

function typeName(type) {
  const zh = TYPE_NAMES[type];
  return zh ? zh + ' <span style="color:#9ca3af;font-size:11px">(' + type + ')</span>' : type;
}

const PARAMS_HINT = {
  'Settings-Stage1':             '关卡名，如 1-7、CE-6、S3-7',
  'Settings-ConnectionAddress':  'ADB 地址，如 127.0.0.1:5555',
};

function onTypeChange() {
  const t = document.getElementById('type').value;
  const isSettings = t.startsWith('Settings-');
  document.getElementById('params-wrap').style.display = isSettings ? 'inline' : 'none';
  if (isSettings) {
    document.getElementById('params').placeholder = PARAMS_HINT[t] || '参数值';
  }
}

function getHeaders() {
  const token = document.getElementById('token').value;
  const h = { 'Content-Type': 'application/json' };
  if (token) h['Authorization'] = 'Bearer ' + token;
  return h;
}

async function load() {
  document.getElementById('status').textContent = '加载中…';
  try {
    const r = await fetch('/admin/tasks', { headers: getHeaders() });
    if (r.status === 401) { document.getElementById('status').textContent = 'Token 错误'; return; }
    const tasks = await r.json();
    const tbody = document.getElementById('tasks');
    if (!tasks || tasks.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" style="color:#aaa;text-align:center">暂无任务</td></tr>';
    } else {
      tbody.innerHTML = tasks.map(t => {
        const isScreenshot = (t.type === 'CaptureImage' || t.type === 'CaptureImageNow');
        const action = (isScreenshot && t.status === 'SUCCESS')
          ? '<a href="/admin/screenshot/' + t.id + '" target="_blank">查看截图</a>'
          : '-';
        return '<tr>' +
          '<td><img src="' + TIME_ICON + '" style="width:16px;height:16px;vertical-align:middle;margin-right:5px">' + new Date(t.created_at).toLocaleString('zh-CN') + '</td>' +
          '<td>' + typeName(t.type) + '</td>' +
          '<td>' + statusBadge(t.status) + '</td>' +
          '<td class="id">' + t.id + '</td>' +
          '<td>' + action + '</td>' +
          '</tr>';
      }).join('');
    }
    document.getElementById('status').textContent = '已更新 ' + new Date().toLocaleTimeString('zh-CN');
  } catch(e) {
    document.getElementById('status').textContent = '请求失败';
  }
}

async function submit() {
  const type = document.getElementById('type').value;
  const params = document.getElementById('params').value;
  const body = { type };
  if (params) body.params = params;
  const r = await fetch('/admin/task', { method: 'POST', headers: getHeaders(), body: JSON.stringify(body) });
  if (r.status === 401) { alert('Token 错误'); return; }
  document.getElementById('params').value = '';
  load();
}

load();
setInterval(load, 2000);
</script>
</body>
</html>
`
