# 安伴后端「地基」实施计划（Foundation）

> **For agentic workers:** REQUIRED SUB-SKILL: 用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐 task 执行。步骤用 `- [ ]` 复选框追踪。

**Goal:** 从零搭出 `anban` Go monorepo 的「地基」——共享契约 + store + scheduler + xiaozhiclient（对 manager OpenAPI）+ childapi 骨架 + docker-compose，让 6 个业务域之后能**并行**对着稳定接口开干。

**Architecture:** 安伴是独立第三服务（[决策 C](../decisions/2026-05-29-server-architecture.md)），通过 manager 的认证 OpenAPI 驱动冻结的 xiaozhi。后端分层：`childapi`(北向) → `domains`(业务) → `xiaozhiclient`(南向) / `store`(数据) / `scheduler`(基建)，依赖纪律见[模块分解 §7](../specs/2026-05-28-module-decomposition-design.md)。本计划只做"地基"，不含任何业务域。

**Tech Stack:** Go 1.22+；gin（HTTP，与 xiaozhi 一致）；gorm + glebarez/sqlite（纯 Go、Windows 无 cgo 坑）；robfig/cron/v3（定时）；标准库 net/http（调 manager）。

**两套追踪别混：**
- **anban 仓库的 git commit**：每个 task 末尾在 anban 仓库内 `git commit`（Task 1 会 `git init`）。
- **计划进度**：在 `D:\Program\Project\AnBan-C\docs\plans\CHANGELOG.md` 勾选本计划的 F1–F10（Task 0 建好）。

**前置人/环境约定：**
- anban 仓库根目录建议 `D:\Program\Project\anban\`（与 AnBan-C 文档仓、xiaozhi 研究仓平级，互不嵌套）。
- 需要一台能跑的 xiaozhi（core+manager）——若还没起，Task 9 的 docker-compose 负责；拿 API Token 见 Task 5 的前置说明。

---

## 文件结构（地基交付后 anban/server 长这样）

```
anban/
├── go.mod                         # module github.com/<org>/anban
├── .gitignore
├── README.md
├── docker-compose.yml             # anban + xiaozhi(core+manager)
├── server/
│   ├── cmd/anban/main.go          # 装配 + 启动（Task 8）
│   ├── internal/
│   │   ├── config/config.go       # 从 env 读配置（Task 3）
│   │   ├── store/store.go         # gorm+sqlite 连接 + AutoMigrate（Task 4）
│   │   ├── xiaozhiclient/
│   │   │   ├── client.go          # Client 接口 + 类型（Task 5）
│   │   │   ├── http_client.go     # 真实现：InjectSpeak 对 manager（Task 5）
│   │   │   ├── http_client_test.go# httptest 假 manager 测 InjectSpeak（Task 5）
│   │   │   └── fake.go            # FakeClient：各域并行开发用（Task 5）
│   │   ├── scheduler/
│   │   │   ├── scheduler.go        # Scheduler 接口 + cron/一次性实现（Task 6）
│   │   │   └── scheduler_test.go
│   │   └── childapi/
│   │       ├── server.go           # gin 路由 + 域路由占位（Task 7）
│   │       ├── accesscode.go       # 访问码中间件（Task 7）
│   │       └── accesscode_test.go
│   └── pkg/types/types.go         # 跨模块共享类型（Task 2）
```

> 业务域 `internal/domains/*` 与前端 `web/` **不在本计划**——见末尾 §Roadmap。

---

## Task 0：建计划进度追踪

**Files:** Modify `D:\Program\Project\AnBan-C\docs\plans\CHANGELOG.md`

- [ ] **Step 1：在 CHANGELOG.md 末尾追加本计划的勾选块**

把下面整块追加到 `D:\Program\Project\AnBan-C\docs\plans\CHANGELOG.md` 文件末尾：

```markdown

## 2026-05-29 安伴后端地基计划（docs/plans/2026-05-29-anban-backend-foundation.md）

- [ ] F1  anban monorepo scaffold（go.mod/目录/.gitignore/README/git init）
- [ ] F2  pkg/types 共享类型
- [ ] F3  config 从 env 读配置
- [ ] F4  store：gorm+sqlite 连接 + AutoMigrate
- [ ] F5  xiaozhiclient：Client 接口 + 真 InjectSpeak + FakeClient
- [ ] F6  scheduler：cron + 一次性定时
- [ ] F7  childapi：gin 骨架 + 访问码中间件 + 域路由占位
- [ ] F8  cmd/anban/main.go 装配启动
- [ ] F9  docker-compose（anban + xiaozhi）
- [ ] F10 端到端冒烟：anban 起 + /health + InjectSpeak 打通
```

- [ ] **Step 2：标记**

本 task 无 anban 代码，无需 git commit。完成后把上面 F0 不存在——直接进 Task 1。

---

## Task 1：anban monorepo scaffold

**Files:**
- Create: `D:\Program\Project\anban\go.mod`
- Create: `D:\Program\Project\anban\.gitignore`
- Create: `D:\Program\Project\anban\README.md`

- [ ] **Step 1：建目录 + go module**

在 PowerShell 跑（逐条）：
```powershell
New-Item -ItemType Directory -Force "D:\Program\Project\anban\server\cmd\anban"
New-Item -ItemType Directory -Force "D:\Program\Project\anban\server\internal\config"
New-Item -ItemType Directory -Force "D:\Program\Project\anban\server\internal\store"
New-Item -ItemType Directory -Force "D:\Program\Project\anban\server\internal\xiaozhiclient"
New-Item -ItemType Directory -Force "D:\Program\Project\anban\server\internal\scheduler"
New-Item -ItemType Directory -Force "D:\Program\Project\anban\server\internal\childapi"
New-Item -ItemType Directory -Force "D:\Program\Project\anban\server\internal\domains"
New-Item -ItemType Directory -Force "D:\Program\Project\anban\server\pkg\types"
Set-Location "D:\Program\Project\anban\server"
go mod init github.com/anban/anban/server
```
> 若有正式 GitHub org，把 `github.com/anban/anban/server` 换成真实 module path（如 `github.com/<org>/anban/server`），后续 import 前缀随之改。

- [ ] **Step 2：写 `.gitignore`（仓库根 `D:\Program\Project\anban\.gitignore`）**

```gitignore
# Go
/server/anban
*.exe
*.test
*.out
# 本地数据/密钥
*.db
*.sqlite
.env
.env.local
# IDE
.idea/
.vscode/
# codegraph 等本地工具
.codegraph/
```

- [ ] **Step 3：写 `README.md`（仓库根）**

```markdown
# anban

安伴后端（Go）+ 子女端前端。独立第三服务，通过 manager OpenAPI 驱动冻结的 xiaozhi。
架构见 AnBan-C 文档仓：docs/specs/2026-05-28-module-decomposition-design.md。

## 本地起
1. `cp .env.example .env` 填 MANAGER_BASE_URL / MANAGER_API_TOKEN / ANBAN_ACCESS_CODE
2. `cd server && go run ./cmd/anban`
3. 健康检查：`curl http://localhost:8090/health`
```

- [ ] **Step 4：git init + 首次提交**

```powershell
Set-Location "D:\Program\Project\anban"
git init
git add .
git commit -m "chore: scaffold anban monorepo"
```

- [ ] **Step 5：标记** CHANGELOG F1 → `[x]`。

---

## Task 2：pkg/types 共享类型

**Files:** Create `D:\Program\Project\anban\server\pkg\types\types.go`

**为什么**：跨模块共享的、与具体域无关的小类型。**保持极小**——只放真正被多处用的。

- [ ] **Step 1：写 `server/pkg/types/types.go`**

```go
// Package types 放跨模块共享、与具体业务域无关的小类型。
// 纪律：这里只允许放被两个及以上模块共用的类型；任何域专属类型放到该域自己的 types.go。
package types

import "errors"

// ErrNotImplemented 供尚未实现的接口方法返回（地基期 FakeClient / 占位用）。
var ErrNotImplemented = errors.New("anban: not implemented")

// DeviceID 是 xiaozhi 侧的设备标识（= manager 的 device_name）。
type DeviceID string
```

- [ ] **Step 2：编译自检**

```powershell
Set-Location "D:\Program\Project\anban\server"; go build ./...
```
预期：无输出、退出码 0。

- [ ] **Step 3：提交**

```powershell
git add server/pkg/types/types.go; git commit -m "feat(types): add shared cross-module types"
```

- [ ] **Step 4：标记** CHANGELOG F2 → `[x]`。

---

## Task 3：config 从环境变量读配置

**Files:**
- Create: `D:\Program\Project\anban\server\internal\config\config.go`
- Create: `D:\Program\Project\anban\server\internal\config\config_test.go`
- Create: `D:\Program\Project\anban\.env.example`

- [ ] **Step 1：写失败测试 `internal/config/config_test.go`**

```go
package config

import "testing"

func TestLoadFailsWhenManagerTokenMissing(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when ANBAN_MANAGER_API_TOKEN missing, got nil")
	}
}

func TestLoadOKWithDefaults(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_123")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ListenAddr != ":8090" {
		t.Fatalf("ListenAddr default = %q, want :8090", c.ListenAddr)
	}
	if c.DBDSN != "anban.db" {
		t.Fatalf("DBDSN default = %q, want anban.db", c.DBDSN)
	}
}
```

- [ ] **Step 2：跑测试确认失败**

```powershell
Set-Location "D:\Program\Project\anban\server"; go test ./internal/config/
```
预期：FAIL（`Load` 未定义，编译错误）。

- [ ] **Step 3：写实现 `internal/config/config.go`**

```go
// Package config 从环境变量装载安伴后端运行所需配置。
package config

import (
	"fmt"
	"os"
)

type Config struct {
	ManagerBaseURL  string // xiaozhi manager 根地址，如 http://localhost:8080
	ManagerAPIToken string // manager 签发的 API Token（X-API-Token）
	DBDSN           string // sqlite 文件路径
	AccessCode      string // 子女端访问码（简化登录）
	ListenAddr      string // 安伴 HTTP 监听地址
}

func Load() (Config, error) {
	c := Config{
		ManagerBaseURL:  os.Getenv("ANBAN_MANAGER_BASE_URL"),
		ManagerAPIToken: os.Getenv("ANBAN_MANAGER_API_TOKEN"),
		AccessCode:      os.Getenv("ANBAN_ACCESS_CODE"),
		DBDSN:           envOr("ANBAN_DB_DSN", "anban.db"),
		ListenAddr:      envOr("ANBAN_LISTEN_ADDR", ":8090"),
	}
	if c.ManagerBaseURL == "" {
		return Config{}, fmt.Errorf("config: ANBAN_MANAGER_BASE_URL 必填")
	}
	if c.ManagerAPIToken == "" {
		return Config{}, fmt.Errorf("config: ANBAN_MANAGER_API_TOKEN 必填")
	}
	if c.AccessCode == "" {
		return Config{}, fmt.Errorf("config: ANBAN_ACCESS_CODE 必填")
	}
	return c, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
```

- [ ] **Step 4：跑测试确认通过**

```powershell
go test ./internal/config/
```
预期：PASS（ok）。

- [ ] **Step 5：写 `.env.example`（仓库根 `D:\Program\Project\anban\.env.example`）**

```dotenv
ANBAN_MANAGER_BASE_URL=http://localhost:8080
ANBAN_MANAGER_API_TOKEN=请填_manager签发的APIToken
ANBAN_ACCESS_CODE=demo
ANBAN_DB_DSN=anban.db
ANBAN_LISTEN_ADDR=:8090
```

- [ ] **Step 6：提交**

```powershell
git add server/internal/config/ .env.example; git commit -m "feat(config): load config from env with validation"
```

- [ ] **Step 7：标记** CHANGELOG F3 → `[x]`。

---

## Task 4：store（gorm + sqlite 连接 + AutoMigrate）

**Files:**
- Create: `D:\Program\Project\anban\server\internal\store\store.go`
- Create: `D:\Program\Project\anban\server\internal\store\store_test.go`

**为什么**：各业务域的 `store.go` 通过这个共享包拿 `*gorm.DB` 并注册自己的表。共享 store **不含任何业务表逻辑**（模块分解 §3 注）。

- [ ] **Step 1：装依赖**

```powershell
Set-Location "D:\Program\Project\anban\server"
go get gorm.io/gorm@latest
go get github.com/glebarez/sqlite@latest
```

- [ ] **Step 2：写失败测试 `internal/store/store_test.go`**

```go
package store

import "testing"

type sample struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

func TestOpenAndAutoMigrate(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.AutoMigrate(&sample{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := s.DB.Create(&sample{Name: "x"}).Error; err != nil {
		t.Fatalf("Create: %v", err)
	}
	var got sample
	if err := s.DB.First(&got).Error; err != nil {
		t.Fatalf("First: %v", err)
	}
	if got.Name != "x" {
		t.Fatalf("Name = %q, want x", got.Name)
	}
}
```

- [ ] **Step 3：跑测试确认失败**

```powershell
go test ./internal/store/
```
预期：FAIL（`Open` 未定义）。

- [ ] **Step 4：写实现 `internal/store/store.go`**

```go
// Package store 提供安伴自有 DB 的连接与迁移。
// 纪律：只有本包知道数据库存在；只放连接/迁移/通用助手，不放任何域的业务表逻辑。
package store

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type Store struct {
	DB *gorm.DB
}

// Open 打开一个 sqlite 库。dsn 为文件路径，或 ":memory:" 用于测试。
func Open(dsn string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return &Store{DB: db}, nil
}

// AutoMigrate 由各域在装配时传入自己的模型，建表/改表。
func (s *Store) AutoMigrate(models ...any) error {
	return s.DB.AutoMigrate(models...)
}
```

- [ ] **Step 5：跑测试确认通过**

```powershell
go test ./internal/store/
```
预期：PASS。

- [ ] **Step 6：提交**

```powershell
git add server/internal/store/ go.mod go.sum; git commit -m "feat(store): sqlite connection + AutoMigrate helper"
```

- [ ] **Step 7：标记** CHANGELOG F4 → `[x]`。

---

## Task 5：xiaozhiclient（南向适配器：接口 + 真 InjectSpeak + FakeClient）

**Files:**
- Create: `D:\Program\Project\anban\server\internal\xiaozhiclient\client.go`
- Create: `D:\Program\Project\anban\server\internal\xiaozhiclient\http_client.go`
- Create: `D:\Program\Project\anban\server\internal\xiaozhiclient\http_client_test.go`
- Create: `D:\Program\Project\anban\server\internal\xiaozhiclient\fake.go`

**前置（人工，一次性）：拿 manager 的 API Token**
1. 起好 xiaozhi（core+manager，见 Task 9）。
2. 登录 manager 控制台 → 用户区 → **API Token** 页签发一个 token（路径 `/api/user/api-tokens`）。
3. 填进 anban 的 `.env`：`ANBAN_MANAGER_API_TOKEN=<刚签发的>`。
> 真实请求体/鉴权头已对 xiaozhi 源码核实：`POST /api/open/v1/devices/inject-message`，body `{device_id, message, skip_llm, auto_listen}`，鉴权头 `X-API-Token: <token>`（见 `manager/backend/controllers/user.go` `InjectMessage` 与 `middleware/openapi_auth.go`）。

- [ ] **Step 1：写接口与共享类型 `internal/xiaozhiclient/client.go`**

```go
// Package xiaozhiclient 是安伴唯一懂 xiaozhi 的地方：封装 manager 的 OpenAPI(/api/open/v1, X-API-Token)。
// 纪律：只有本包 import 网络/manager 细节；它不反向 import 任何 domain。
package xiaozhiclient

import (
	"context"
	"encoding/json"
	"time"
)

// Client 是各业务域唯一可见的南向接口（域只依赖它，不碰 HTTP 细节）。
type Client interface {
	// InjectSpeak 让指定设备说一段话（主动播报）。message/reminder/greeting 用。
	InjectSpeak(ctx context.Context, deviceID, text string, opts InjectOptions) error
	// GetDeviceStatus 读设备在线/最近互动。status 域用。
	GetDeviceStatus(ctx context.Context, deviceID string) (DeviceStatus, error)
	// GetHistory 读近 N 条对话历史（只读）。status / 子女端深度用。
	GetHistory(ctx context.Context, deviceID string, limit int) ([]HistoryMessage, error)
	// SetRolePrompt 把家庭画像写成 xiaozhi 人设 prompt。profile 域用。
	SetRolePrompt(ctx context.Context, deviceID, prompt string) error
	// CallDeviceMCPTool 远程调设备已注册的 MCP 工具（如拍照）。vision 域用。
	CallDeviceMCPTool(ctx context.Context, deviceID, tool string, args map[string]any) (json.RawMessage, error)
}

// InjectOptions 对应 manager inject-message 的可选参数。
type InjectOptions struct {
	SkipLLM    bool  // true=直接念原话；false=过 LLM 润色
	AutoListen *bool // 非 nil 时控制"播完是否自动续听"；nil=用服务端默认
}

type DeviceStatus struct {
	DeviceID     string
	Online       bool
	LastActiveAt time.Time
}

type HistoryMessage struct {
	Role string // "user" | "assistant"
	Text string
	At   time.Time
}
```

- [ ] **Step 2：写失败测试 `internal/xiaozhiclient/http_client_test.go`**（用 httptest 假冒 manager，断言路径/鉴权头/请求体）

```go
package xiaozhiclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInjectSpeakSendsCorrectRequest(t *testing.T) {
	var gotPath, gotToken string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotToken = r.Header.Get("X-API-Token")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"message":"ok"}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	skip := false
	auto := true
	err := c.InjectSpeak(context.Background(), "dev-001", "妈，该吃药了", InjectOptions{SkipLLM: skip, AutoListen: &auto})
	if err != nil {
		t.Fatalf("InjectSpeak: %v", err)
	}
	if gotPath != "/api/open/v1/devices/inject-message" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotToken != "tok_abc" {
		t.Fatalf("X-API-Token = %q", gotToken)
	}
	if gotBody["device_id"] != "dev-001" || gotBody["message"] != "妈，该吃药了" {
		t.Fatalf("body = %v", gotBody)
	}
	if gotBody["auto_listen"] != true {
		t.Fatalf("auto_listen = %v, want true", gotBody["auto_listen"])
	}
}

func TestInjectSpeakErrorsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"无效或已过期的API Token"}`))
	}))
	defer srv.Close()
	c := NewHTTPClient(srv.URL, "bad")
	if err := c.InjectSpeak(context.Background(), "d", "hi", InjectOptions{}); err == nil {
		t.Fatal("expected error on 401, got nil")
	}
}
```

- [ ] **Step 3：跑测试确认失败**

```powershell
go test ./internal/xiaozhiclient/
```
预期：FAIL（`NewHTTPClient` 未定义）。

- [ ] **Step 4：写真实现 `internal/xiaozhiclient/http_client.go`**

```go
package xiaozhiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/anban/anban/server/pkg/types"
)

// HTTPClient 是 Client 的真实现：对 manager 的 /api/open/v1。
type HTTPClient struct {
	baseURL string
	token   string
	hc      *http.Client
}

func NewHTTPClient(baseURL, token string) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		token:   token,
		hc:      &http.Client{Timeout: 10 * time.Second},
	}
}

// 确保 *HTTPClient 实现了 Client 接口（编译期检查）。
var _ Client = (*HTTPClient)(nil)

type injectReq struct {
	DeviceID   string `json:"device_id"`
	Message    string `json:"message"`
	SkipLlm    bool   `json:"skip_llm"`
	AutoListen *bool  `json:"auto_listen,omitempty"`
}

func (c *HTTPClient) InjectSpeak(ctx context.Context, deviceID, text string, opts InjectOptions) error {
	body, err := json.Marshal(injectReq{
		DeviceID:   deviceID,
		Message:    text,
		SkipLlm:    opts.SkipLLM,
		AutoListen: opts.AutoListen,
	})
	if err != nil {
		return err
	}
	_, err = c.do(ctx, http.MethodPost, "/api/open/v1/devices/inject-message", body)
	return err
}

// do 发一个带 X-API-Token 的请求；2xx 返回响应体，否则返回错误（含状态码与响应片段）。
func (c *HTTPClient) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Token", c.token)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("xiaozhi manager %s %s -> %d: %s", method, path, resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// 以下 4 个方法在地基期返回 ErrNotImplemented，由各自的域 follow-on 计划据真实端点补齐
// （GetDeviceStatus→status；GetHistory→status/深度；SetRolePrompt→profile；CallDeviceMCPTool→vision）。
func (c *HTTPClient) GetDeviceStatus(ctx context.Context, deviceID string) (DeviceStatus, error) {
	return DeviceStatus{}, types.ErrNotImplemented
}

func (c *HTTPClient) GetHistory(ctx context.Context, deviceID string, limit int) ([]HistoryMessage, error) {
	return nil, types.ErrNotImplemented
}

func (c *HTTPClient) SetRolePrompt(ctx context.Context, deviceID, prompt string) error {
	return types.ErrNotImplemented
}

func (c *HTTPClient) CallDeviceMCPTool(ctx context.Context, deviceID, tool string, args map[string]any) (json.RawMessage, error) {
	return nil, types.ErrNotImplemented
}
```

> **范围说明（非占位符违规）**：`InjectSpeak` 是 W1/W2 关键路径（被 3 个域用），故在地基**完整实现 + 测试**。其余 4 个方法的真实端点形状须各域开工时按需对源码核实，因此它们在地基显式返回 `types.ErrNotImplemented`，并在 §Roadmap 的对应域计划里有"实现 xiaozhiclient.XXX"的首个 task。接口已冻结，域可并行对接口编码。

- [ ] **Step 5：写 `internal/xiaozhiclient/fake.go`（各域并行开发/测试用）**

```go
package xiaozhiclient

import (
	"context"
	"encoding/json"
)

// FakeClient 实现 Client，把调用记录在内存里，供各域并行开发与单测使用。
type FakeClient struct {
	InjectCalls []InjectCall
}

type InjectCall struct {
	DeviceID string
	Text     string
	Opts     InjectOptions
}

var _ Client = (*FakeClient)(nil)

func (f *FakeClient) InjectSpeak(ctx context.Context, deviceID, text string, opts InjectOptions) error {
	f.InjectCalls = append(f.InjectCalls, InjectCall{DeviceID: deviceID, Text: text, Opts: opts})
	return nil
}
func (f *FakeClient) GetDeviceStatus(ctx context.Context, deviceID string) (DeviceStatus, error) {
	return DeviceStatus{DeviceID: deviceID, Online: true}, nil
}
func (f *FakeClient) GetHistory(ctx context.Context, deviceID string, limit int) ([]HistoryMessage, error) {
	return nil, nil
}
func (f *FakeClient) SetRolePrompt(ctx context.Context, deviceID, prompt string) error { return nil }
func (f *FakeClient) CallDeviceMCPTool(ctx context.Context, deviceID, tool string, args map[string]any) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
```

- [ ] **Step 6：跑测试确认通过**

```powershell
go test ./internal/xiaozhiclient/
```
预期：PASS（两条用例都过）。

- [ ] **Step 7：提交**

```powershell
git add server/internal/xiaozhiclient/; git commit -m "feat(xiaozhiclient): Client interface + real InjectSpeak + fake"
```

- [ ] **Step 8：标记** CHANGELOG F5 → `[x]`。

---

## Task 6：scheduler（cron + 一次性定时）

**Files:**
- Create: `D:\Program\Project\anban\server\internal\scheduler\scheduler.go`
- Create: `D:\Program\Project\anban\server\internal\scheduler\scheduler_test.go`

**为什么**：greeting/reminder 域注册"到点干啥"。地基只提供机制；具体任务内容由域填。

- [ ] **Step 1：装依赖**

```powershell
Set-Location "D:\Program\Project\anban\server"; go get github.com/robfig/cron/v3@latest
```

- [ ] **Step 2：写失败测试 `internal/scheduler/scheduler_test.go`**

```go
package scheduler

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestScheduleAtFires(t *testing.T) {
	s := New()
	s.Start()
	defer s.Stop()

	var fired atomic.Int32
	_, err := s.ScheduleAt(time.Now().Add(50*time.Millisecond), func() { fired.Add(1) })
	if err != nil {
		t.Fatalf("ScheduleAt: %v", err)
	}
	time.Sleep(150 * time.Millisecond)
	if fired.Load() != 1 {
		t.Fatalf("fired = %d, want 1", fired.Load())
	}
}

func TestCancelStopsOneShot(t *testing.T) {
	s := New()
	s.Start()
	defer s.Stop()

	var fired atomic.Int32
	id, _ := s.ScheduleAt(time.Now().Add(100*time.Millisecond), func() { fired.Add(1) })
	s.Cancel(id)
	time.Sleep(200 * time.Millisecond)
	if fired.Load() != 0 {
		t.Fatalf("fired = %d, want 0 (cancelled)", fired.Load())
	}
}
```

- [ ] **Step 3：跑测试确认失败**

```powershell
go test ./internal/scheduler/
```
预期：FAIL（`New` 未定义）。

- [ ] **Step 4：写实现 `internal/scheduler/scheduler.go`**

```go
// Package scheduler 提供定时能力：cron 周期任务 + 一次性定时任务。
// 纪律：只提供机制；任务内容由各域以闭包传入。
package scheduler

import (
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type JobID string

type Scheduler struct {
	cron     *cron.Cron
	mu       sync.Mutex
	oneShots map[JobID]*time.Timer
	seq      int64
}

func New() *Scheduler {
	return &Scheduler{
		cron:     cron.New(),
		oneShots: make(map[JobID]*time.Timer),
	}
}

func (s *Scheduler) Start() { s.cron.Start() }
func (s *Scheduler) Stop() {
	s.cron.Stop()
	s.mu.Lock()
	for _, t := range s.oneShots {
		t.Stop()
	}
	s.oneShots = make(map[JobID]*time.Timer)
	s.mu.Unlock()
}

// RegisterCron 注册一个 cron 表达式周期任务（如每天 8 点："0 8 * * *"）。
func (s *Scheduler) RegisterCron(spec string, fn func()) (JobID, error) {
	eid, err := s.cron.AddFunc(spec, fn)
	if err != nil {
		return "", err
	}
	return JobID("cron-" + itoa(int64(eid))), nil
}

// ScheduleAt 在指定时刻触发一次 fn（用于一次性提醒）。
func (s *Scheduler) ScheduleAt(t time.Time, fn func()) (JobID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	id := JobID("once-" + itoa(s.seq))
	d := time.Until(t)
	if d < 0 {
		d = 0
	}
	timer := time.AfterFunc(d, func() {
		s.mu.Lock()
		delete(s.oneShots, id)
		s.mu.Unlock()
		fn()
	})
	s.oneShots[id] = timer
	return id, nil
}

// Cancel 取消一次性任务（cron 任务的取消地基期不需要，留待按需扩展）。
func (s *Scheduler) Cancel(id JobID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.oneShots[id]; ok {
		t.Stop()
		delete(s.oneShots, id)
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
```

- [ ] **Step 5：跑测试确认通过**

```powershell
go test ./internal/scheduler/
```
预期：PASS。

- [ ] **Step 6：提交**

```powershell
git add server/internal/scheduler/ go.mod go.sum; git commit -m "feat(scheduler): cron + one-shot timer mechanism"
```

- [ ] **Step 7：标记** CHANGELOG F6 → `[x]`。

---

## Task 7：childapi（gin 骨架 + 访问码中间件 + 域路由占位）

**Files:**
- Create: `D:\Program\Project\anban\server\internal\childapi\accesscode.go`
- Create: `D:\Program\Project\anban\server\internal\childapi\accesscode_test.go`
- Create: `D:\Program\Project\anban\server\internal\childapi\server.go`

**为什么**：北向边界。地基提供 `/health` + 访问码鉴权 + 各域的路由分组占位（返回 501），让前端能立刻对着接口形状开发。

- [ ] **Step 1：装依赖**

```powershell
Set-Location "D:\Program\Project\anban\server"; go get github.com/gin-gonic/gin@latest
```

- [ ] **Step 2：写失败测试 `internal/childapi/accesscode_test.go`**

```go
package childapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAccessCodeMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequireAccessCode("secret"))
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// 缺码 → 401
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w1.Code != http.StatusUnauthorized {
		t.Fatalf("no code: status = %d, want 401", w1.Code)
	}

	// 正确码 → 200
	w2 := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Access-Code", "secret")
	r.ServeHTTP(w2, req)
	if w2.Code != http.StatusOK {
		t.Fatalf("with code: status = %d, want 200", w2.Code)
	}
}
```

- [ ] **Step 3：跑测试确认失败**

```powershell
go test ./internal/childapi/
```
预期：FAIL（`RequireAccessCode` 未定义）。

- [ ] **Step 4：写实现 `internal/childapi/accesscode.go`**

```go
package childapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireAccessCode 校验请求头 X-Access-Code 是否等于配置的访问码（简化登录）。
func RequireAccessCode(code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("X-Access-Code") != code {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "访问码无效"})
			return
		}
		c.Next()
	}
}
```

- [ ] **Step 5：跑测试确认通过**

```powershell
go test ./internal/childapi/
```
预期：PASS。

- [ ] **Step 6：写 `internal/childapi/server.go`（路由骨架 + 域占位）**

```go
package childapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Deps 是 childapi 装配所需的依赖（各域 handler 之后注入这里）。
// 地基期为空骨架；域 follow-on 计划往这里加自己的 handler 字段并注册路由。
type Deps struct {
	AccessCode string
}

func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	// 健康检查无需访问码
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 子女端 API：全部走访问码
	api := r.Group("/api", RequireAccessCode(d.AccessCode))

	// —— 各业务域路由占位（域 follow-on 计划逐个替换为真 handler）——
	// 返回 501，让前端先对着 URL 形状开发。
	notImpl := func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "未实现（地基占位）"})
	}
	api.POST("/messages", notImpl)            // message 域
	api.GET("/messages", notImpl)             // message 域
	api.POST("/reminders", notImpl)           // reminder 域
	api.GET("/reminders", notImpl)            // reminder 域
	api.POST("/greetings/trigger", notImpl)   // greeting 域
	api.GET("/profile", notImpl)              // profile 域
	api.PUT("/profile", notImpl)              // profile 域
	api.GET("/status", notImpl)               // status 域

	return r
}
```

- [ ] **Step 7：编译自检**

```powershell
go build ./...
```
预期：无输出、退出码 0。

- [ ] **Step 8：提交**

```powershell
git add server/internal/childapi/ go.mod go.sum; git commit -m "feat(childapi): gin skeleton + access-code mw + domain route stubs"
```

- [ ] **Step 9：标记** CHANGELOG F7 → `[x]`。

---

## Task 8：cmd/anban/main.go 装配启动

**Files:** Create `D:\Program\Project\anban\server\cmd\anban\main.go`

- [ ] **Step 1：写 `server/cmd/anban/main.go`**

```go
package main

import (
	"log"

	"github.com/anban/anban/server/internal/childapi"
	"github.com/anban/anban/server/internal/config"
	"github.com/anban/anban/server/internal/scheduler"
	"github.com/anban/anban/server/internal/store"
	"github.com/anban/anban/server/internal/xiaozhiclient"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	st, err := store.Open(cfg.DBDSN)
	if err != nil {
		log.Fatalf("数据库打开失败: %v", err)
	}
	_ = st // 地基期各域尚未注册模型；域接入时在此 st.AutoMigrate(...)

	xc := xiaozhiclient.NewHTTPClient(cfg.ManagerBaseURL, cfg.ManagerAPIToken)
	_ = xc // 地基期各域尚未接入；域接入时把 xc 注入各域 service

	sch := scheduler.New()
	sch.Start()
	defer sch.Stop()

	r := childapi.NewRouter(childapi.Deps{AccessCode: cfg.AccessCode})

	log.Printf("anban 启动，监听 %s（manager=%s）", cfg.ListenAddr, cfg.ManagerBaseURL)
	if err := r.Run(cfg.ListenAddr); err != nil {
		log.Fatalf("HTTP 服务退出: %v", err)
	}
}
```

> `_ = st` / `_ = xc` 是地基期的有意占位：装配链已通，域接入时去掉下划线、把依赖注入各域。这不是 TODO 占位符——地基的职责就是"接好线、能启动"，域逻辑属于 follow-on 计划。

- [ ] **Step 2：编译 + 启动自检（用临时 env）**

```powershell
Set-Location "D:\Program\Project\anban\server"
$env:ANBAN_MANAGER_BASE_URL="http://localhost:8080"; $env:ANBAN_MANAGER_API_TOKEN="dummy"; $env:ANBAN_ACCESS_CODE="demo"
go build ./...
```
预期：编译通过。（真正起服务在 Task 10 冒烟。）

- [ ] **Step 3：提交**

```powershell
git add server/cmd/anban/main.go; git commit -m "feat(cmd): wire config+store+xiaozhiclient+scheduler+childapi"
```

- [ ] **Step 4：标记** CHANGELOG F8 → `[x]`。

---

## Task 9：docker-compose（anban + xiaozhi）

**Files:**
- Create: `D:\Program\Project\anban\server\Dockerfile`
- Create: `D:\Program\Project\anban\docker-compose.yml`

**为什么**：一条命令把 anban + xiaozhi(core+manager) 一起起，路演/联调用。

- [ ] **Step 1：写 `server/Dockerfile`**

```dockerfile
# 多阶段构建：纯 Go（glebarez/sqlite 无需 cgo）
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/anban ./cmd/anban

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/anban /app/anban
EXPOSE 8090
ENTRYPOINT ["/app/anban"]
```

- [ ] **Step 2：写 `docker-compose.yml`（仓库根）**

```yaml
services:
  # xiaozhi：core+manager。镜像/构建方式按 xiaozhi 仓库的 doc/docker.md 填。
  # 这里占位为 image；首次本地联调也可直接在宿主机跑 xiaozhi，不进 compose。
  xiaozhi:
    image: xiaozhi-esp32-server-golang:local
    ports:
      - "8080:8080"   # manager REST/WS
      - "8000:8000"   # core ws（端口以 xiaozhi 配置为准）
    # build / volumes / env 按 xiaozhi 仓库 doc 填

  anban:
    build:
      context: ./server
    depends_on:
      - xiaozhi
    ports:
      - "8090:8090"
    environment:
      ANBAN_MANAGER_BASE_URL: "http://xiaozhi:8080"
      ANBAN_MANAGER_API_TOKEN: "${ANBAN_MANAGER_API_TOKEN}"
      ANBAN_ACCESS_CODE: "${ANBAN_ACCESS_CODE:-demo}"
      ANBAN_DB_DSN: "/data/anban.db"
    volumes:
      - anban_data:/data

volumes:
  anban_data:
```

> xiaozhi 的 `image`/`build`/`env`/端口按其仓库 `doc/docker.md`、`doc/docker_compose.md` 填实（本计划不深入 xiaozhi 部署细节，那属于 0 阶段任务 0.1）。

- [ ] **Step 3：提交**

```powershell
Set-Location "D:\Program\Project\anban"; git add server/Dockerfile docker-compose.yml; git commit -m "chore(deploy): docker-compose for anban + xiaozhi"
```

- [ ] **Step 4：标记** CHANGELOG F9 → `[x]`。

---

## Task 10：端到端冒烟（spine 打通）

**Files:** 无新文件（验证 + 记录）。

- [ ] **Step 1：起 anban（宿主机，连一台已运行的 xiaozhi）**

```powershell
Set-Location "D:\Program\Project\anban\server"
$env:ANBAN_MANAGER_BASE_URL="http://localhost:8080"
$env:ANBAN_MANAGER_API_TOKEN="<填真 token>"
$env:ANBAN_ACCESS_CODE="demo"
go run ./cmd/anban
```

- [ ] **Step 2：健康检查 + 访问码（另开一个 PowerShell）**

```powershell
# 健康检查（无需码）
(Invoke-WebRequest http://localhost:8090/health).Content
# 期望：{"status":"ok"}

# 无访问码访问 /api/* → 401
try { Invoke-WebRequest http://localhost:8090/api/status } catch { $_.Exception.Response.StatusCode.value__ }
# 期望：401

# 带访问码 → 501（占位，证明中间件放行、路由在）
(Invoke-WebRequest http://localhost:8090/api/status -Headers @{ "X-Access-Code"="demo" } -SkipHttpErrorCheck).StatusCode
# 期望：501
```

- [ ] **Step 3：用一次性小程序验证 `InjectSpeak` 真打通**（确认设备在线后）

在 `server/cmd/anban` 临时建 `D:\Program\Project\anban\server\cmd\smoke\main.go`：
```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/anban/anban/server/internal/xiaozhiclient"
)

func main() {
	xc := xiaozhiclient.NewHTTPClient(os.Getenv("ANBAN_MANAGER_BASE_URL"), os.Getenv("ANBAN_MANAGER_API_TOKEN"))
	auto := true
	err := xc.InjectSpeak(context.Background(), os.Getenv("SMOKE_DEVICE_ID"), "安伴地基冒烟测试，您好呀", xiaozhiclient.InjectOptions{SkipLLM: true, AutoListen: &auto})
	if err != nil {
		log.Fatalf("InjectSpeak 失败: %v", err)
	}
	log.Println("InjectSpeak 成功——设备应当开口")
}
```
跑：
```powershell
$env:SMOKE_DEVICE_ID="<一台在线设备的 device_id>"
go run ./cmd/smoke
```
预期：日志"InjectSpeak 成功"，且**真设备/假设备开口说话**。失败则按错误信息（401=token/鉴权头、404=路径、设备不在线等）排查。

- [ ] **Step 4：清理冒烟程序 + 提交**

```powershell
Remove-Item -Recurse -Force "D:\Program\Project\anban\server\cmd\smoke"
Set-Location "D:\Program\Project\anban"; git add -A; git commit -m "test: foundation smoke verified (manual)"
```

- [ ] **Step 5：标记** CHANGELOG F10 → `[x]`，并在 CHANGELOG 末尾写一行"2026-05-29 安伴后端地基完成"。

---

## 自检（writing-plans 内部）

**1. Spec 覆盖**（对照模块分解文档）：
- §2 五层：childapi(T7) / domains(本计划不含，Roadmap) / xiaozhiclient(T5) / store(T4) / scheduler(T6) ✓
- §2.1 五方法：InjectSpeak 真实现(T5)；其余 4 个接口已定 + 地基返回 ErrNotImplemented + Roadmap 注明域计划补齐 ✓（有意范围裁剪，非占位符）
- §7 依赖纪律：childapi 不碰 store/xiaozhiclient（T7 路由占位无直调）；只有 xiaozhiclient 懂 manager（T5）；只有 store 懂 DB（T4）✓
- 仓库/monorepo/docker-compose(②)：T1/T9 ✓

**2. 占位符扫描**：`_ = st`/`_ = xc`（T8）与 4 个 `ErrNotImplemented`（T5）均为**有意的地基边界**并已书面说明理由，非"TODO/待填"。无裸 TODO。✓

**3. 类型一致性**：`xiaozhiclient.Client` 5 方法签名在 client.go / http_client.go / fake.go 三处一致；`InjectOptions{SkipLLM,AutoListen}` 一致；module path `github.com/anban/anban/server` 在所有 import 一致（若改 org 需全局替换，已在 T1 注明）。✓

---

## Roadmap：地基之后的并行 follow-on 计划（本计划不展开）

地基（F1–F10）合并后，下列计划可**并行**开工（对齐模块分解 §6 责任田）。每个都是独立 writing-plans 产物：

| 计划 | 负责人 | 依赖地基的什么 | 首个 task 通常是 |
|---|---|---|---|
| `message` 域 | 成员 B | xiaozhiclient.InjectSpeak（已实现）+ store + childapi | 建留言表 + POST/GET /api/messages |
| `reminder`+`greeting` 域 | 成员 A | InjectSpeak + scheduler（已实现）+ store | 提醒表 + scheduler 注册 + 应答跟踪 |
| `profile` 域 | 成员 D | **需先实现** xiaozhiclient.SetRolePrompt（读真 role/agent 端点） | 画像表 + 写人设 |
| `status` 域 | 成员 D | **需先实现** xiaozhiclient.GetDeviceStatus / GetHistory | 状态聚合 + GET /api/status |
| `vision` 域 | 成员 C | **需先实现** xiaozhiclient.CallDeviceMCPTool（设备拍照 MCP 工具，档②） | 采帧→VLM 判定→触发；最不确定、可降级 |
| `web` 前端 | 成员 E | childapi 路由形状（占位已在）| 对着 /api/* 起页面，连假数据再切真 |

> 4 个"需先实现"的 xiaozhiclient 方法仍归组长（守南向缝），由对应域计划的首个 task 触发组长补齐——接口已冻结，不阻塞域内其余并行开发。
