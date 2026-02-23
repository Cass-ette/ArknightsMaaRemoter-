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
	// 解析失败也返回空列表，避免 MAA 报错
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

	// 截图任务：base64 存到文件，避免数据库/JSON 膨胀
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
<style>
  * { box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; max-width: 900px; margin: 40px auto; padding: 0 20px; color: #333; }
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
</style>
</head>
<body>
<h1>MAA Remote</h1>
<p class="sub">控制面板 · 每 5 秒自动刷新</p>

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

function onTypeChange() {
  const t = document.getElementById('type').value;
  document.getElementById('params-wrap').style.display =
    t.startsWith('Settings-') ? 'inline' : 'none';
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
          '<td>' + new Date(t.created_at).toLocaleString('zh-CN') + '</td>' +
          '<td>' + typeName(t.type) + '</td>' +
          '<td><span class="' + t.status + '">' + t.status + '</span></td>' +
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
setInterval(load, 5000);
</script>
</body>
</html>
`
