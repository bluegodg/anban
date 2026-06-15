# 子女端 Web 接入计划：stitch UI → 安伴（前端-only）— 2026-06-15

> 给执行者（含 Codex goal 模式）：你在 **anban-code** 仓库工作。子女端新 UI 源码**已拷入本仓库 `childweb/`**——一个纯 HTML + Tailwind + 原生 JS 的单文件 SPA（设计工具 stitch 导出，目前"好看但没接后端"，数据全是 localStorage mock）。本期任务：**把它接到安伴后端 + 做成可"添加到主屏"的 PWA App**。**只动前端，不改后端契约、不碰 xiaozhi。** 缺后端的功能按"该功能未实现"处理。全程 TDD，保持现有测试全绿。

---

## 1. 资产盘点

### 1.1 `childweb/`（你要接的 UI，已在本仓库）
- `index.html`（约 2786 行）：`<head>` 内联 `<style>` + `<body>` 多个 `<section class="spa-section" id="s-XXX">` + 文件尾一大段**内联 `<script>`**（非模块，含大量 `onclick="fn()"` 调用挂在 `window` 上的全局函数）。
- `dist.css`：**已编译好的 Tailwind 产物，别动**（无需重新构建）。`app.css` 是空的 Tailwind 入口，忽略。
- `fonts/`：Be Vietnam Pro 变体字 + Material Symbols 图标字体（`fonts.css`/`icons.css` 引用）。
- `server.js`：极简静态服务器（端口 3001），可沿用或替换。
- `tailwind.config.js`：Tailwind 配置（dist.css 已编译，除非要加新类否则不必跑）。

**9 个屏（section id）**：`s-login / s-home / s-message / s-warn(提醒) / s-history(历史提醒) / s-detail(提醒详情) / s-family(家人画像) / s-mine(设置) / s-family-edit(编辑画像)`。

**路由 / 生命周期**：hash 路由。全局 `SPA` 对象持 `sectionIds`(route→domId)、`currentSection`、`initialized`。`navigateTo(name)` 与 `window.hashchange` → `showSection(name)`；`showSection` 懒加载调用 `window['init'+Capitalized(name)]()`（如 `initHome`、`initFamilyEdit`，连字符转驼峰）。初始加载：若 `localStorage.anban_session` 存在则跳过登录直达 home。

**现有 mock 数据键**：`anban_session`（登录标志）、`anban_messages`、`anban_reminders`、`anban_family_profile`。

**各屏数据点（要替换成 API 调用的位置）**：
- `initLogin().handleLogin()`（约 1455 行）：现在是**假验证**——1.5s 后 `localStorage.setItem('anban_session','1')` 直接进 home。
- `initHome()`（约 1492）：状态卡文案、最近留言列表、环境温湿度（内部 `fetchWeather()` 调 `api.open-meteo.com`，按**子女手机**定位）、快速留言弹窗 `sendQuickMsg`。
- `initMessage()`（约 1932）：`loadMessages()` 读 localStorage 渲染气泡；`handleSend()` 写 localStorage。
- `initWarn()`（约 2000）：时间选择器 + 频率选择器（每天/仅一次/工作日/周末/自定义日历）；`saveReminder()`、`loadSavedReminders()`、`handleToggle()`（开关）、`openDetail()`。
- `initHistory()`：历史提醒（每次进入刷新）。
- `initFamily()`（约 2377）：内置 `defaultProfile` 渲染。`initFamilyEdit()`（约 2498）：`populateForm()` / `collectFormData()`（收集 name/age/livingSituation/occupation/aiPortrait）/ `renderHobbies/Habits/Health/Dos/Donts`；`saveBtn` 写 localStorage。

**⚠️ 手机外壳мокап**：外层 `.phone-frame` / `.phone-screen`（固定 430×932）是设计稿的"手机相框"。**App 化时要去掉**，让 `phone-screen` 铺满真实视口（`100vw / 100dvh`），保留内部布局。

### 1.2 现有 `web/`（**复用它的 API 客户端**，别重写 API 层）
`web/api/client.js` 是**已真机验证可用**的 ESM 客户端：`createAnbanClient({ baseURL, accessCode, fetchImpl })` 返回方法，全部自动带 `X-Access-Code` 头、解析 JSON、失败抛 `ApiError`。**直接复用**（复制到 `childweb/api/client.js` 或以模块引入）。方法：

| 方法 | 端点 |
|---|---|
| `sendMessage({deviceId, fromName, text})` | POST /api/messages |
| `listMessages({deviceId, status})` | GET /api/messages |
| `getStatus({deviceId})` | GET /api/device/status |
| `getHistory({deviceId, limit})` | GET /api/device/history |
| `createReminder({deviceId, scheduledAt, content, category})` | POST /api/reminders |
| `listReminders({deviceId, status})` / `deleteReminder(id)` / `ackReminder(id,{ackKind})` | …/api/reminders… |
| `getProfile({deviceId})` / `updateProfile({deviceId, fields})` | …/api/profile |
| `triggerGreeting / getGreetingSchedule / updateGreetingSchedule / captureVision / checkVisionPresence` | 其余域 |

`web/` 里其它 `*.js`（status-summary / history-view / message-result 等）是配合**旧 UI** 的纯逻辑/格式化助手——可借鉴算法，但新 UI 用 `client.js` 直接驱动即可，不必照搬其 DOM。

**⚠️ 模块桥接**：`client.js` 是 ESM（`export`），而 childweb 脚本是非模块 + 内联 `onclick`。做法：把 childweb 的 `<script>` 改成 `<script type="module">`，在模块内 `import { createAnbanClient }`，并把被 `onclick` 调用的函数显式挂到 `window`（现状本就大量挂 window，沿用）。

### 1.3 后端契约（已部署在演示服务器，端点今日真机验证过）
- 所有 `/api/*` 必须带头 `X-Access-Code: <访问码>`（子女登录框输入的就是它；缺/错 → 401）。childapi 已开 CORS 且允许 `X-Access-Code` 头。
- 演示设备 `deviceId = 9c:13:9e:8b:af:28`（childweb 默认用它；可在设置里改）。
- 关键形状：
  - `GET /api/device/status` → `{deviceId, online, lastInteractionAt, messages:[{messageId,status,queuedAt,playedAt}]}`（status: pending/played/...）。
  - `GET /api/device/history?deviceId=&limit=` → `{deviceId, messages:[{role,text,at}]}`，role 仅 `user`/`assistant`，已按时间正序。
  - `POST /api/messages` body `{deviceId, fromName, text}` → 201 `{messageId, status, ...}`。
  - `POST /api/reminders` body `{deviceId, scheduledAt(RFC3339 UTC，必须是将来时刻), content, category}` → 201 `{reminderId, status:"scheduled", text, ...}`；`category` 用 `med` 或 `custom`；列表 `GET /api/reminders?deviceId=[&status=]`，项含 `{reminderId, content, scheduledAt, status, ackKind, playedAt, ...}`。
  - `PUT /api/profile` body `{deviceId, fields:{...见 §3}}`；`GET /api/profile?deviceId=` → `{..., fields, prompt}`。

---

## 2. 功能↔后端映射 & "未实现"清单
| 屏 / 动作 | 接法 | 状态 |
|---|---|---|
| 登录(访问码) | 用输入的访问码调 `getStatus({deviceId})` 探测：2xx → 存访问码+session 进 home；401 → 错误提示 | ✅ |
| 首页·状态卡 | `getStatus`（online/lastInteractionAt） | ✅ |
| 首页·最近留言 / 消息屏 | `getHistory`（老人↔设备对话气泡）+ `listMessages`/status 的留言播报态；发送→`sendMessage` | ✅ |
| 提醒·列表/创建/删除/详情/历史 | `listReminders` / `createReminder` / `deleteReminder` / `listReminders({status})` | ✅(见 §4 取舍) |
| 家人·画像查看/保存 | `getProfile` / `updateProfile`（见 §3 映射）；保存后设备人设会更新 | ✅ |
| ❌ 环境温湿度 | 无后端 | "未实现"（或保留 open-meteo 但注明是天气、非室内） |
| ❌ 消息图片/语音附件 | 无后端 | "未实现" |
| ❌ 新设备激活 / 忘记访问码 | 无后端 | "未实现" |
| ❌ 提醒"重要(强制播报)" | 后端无该字段 | "未实现"/忽略 |
| ❌ 提醒重复频率(每天/工作日/自定义) | 后端只单次 | 见 §4 |
| ❌ 设置里声纹等高级项 | 无后端 | "未实现" |

统一加一个 `notImplemented(featureName)` → `showToast('该功能未实现')`，所有未实现入口都走它。

---

## 3. 画像字段映射（stitch → anban `profile.Fields`，有损；**不要改后端 Fields**）
anban `Fields`：`Name, Nickname, Children[], Grandchildren[], Hobbies[], Schedule(text), Health(text), Taboos[]`。

| stitch 字段 | anban 字段 | 处理 |
|---|---|---|
| name | Name（同时填 Nickname） | ✅ |
| hobbies[] | Hobbies[] | ✅ |
| habits[{icon,text}] | Schedule | 把各 text 用换行拼成文本 |
| health[{name,detail}] | Health | 拼成文本 |
| donts[] | Taboos[] | ✅ |
| dos[] | 并入 Schedule 文本 | 降维 |
| aiPortrait | 并入 Health 文本（或单独段） | 降维 |
| age / livingSituation / occupation | anban 无对应字段 | 仅前端 localStorage 保留+展示，不回写后端 |

- `getProfile` 回填表单：尽力把 Schedule/Health 文本拆回展示即可（拼接是单向有损的，不要求完美还原）。
- 抽成纯函数 `mapStitchProfileToFields(stitchData)` 与 `mapFieldsToStitchProfile(fields)`，便于单测。

---

## 4. 提醒"频率/重要"取舍（最大功能落差，务必按此处理）
后端 reminder 是**单次** `scheduledAt`（必须将来时刻）：
- **"仅一次"**：UI 选的时间 → 拼成"今天或明天该时刻"的 RFC3339 UTC（若今天该时刻已过则顺延到明天）→ 正常 `createReminder`。抽纯函数 `nextOccurrenceUTC(hh, mm, now)` 做单测。
- **"每天 / 工作日 / 周末 / 自定义"**：后端不支持重复 → 本期**"该功能未实现"**（频率可选但提交时提示，或仅落下一次单点并说明）。
- **"重要(强制播报)"**：后端无字段 → 忽略 + 标"未实现"。

---

## 5. App 化（PWA）
- **去手机外壳**：移除/旁路 `.phone-frame` 与 `.phone-screen` 的固定尺寸包裹，让其占满 `100vw / 100dvh`；状态栏мокап可保留为简单顶栏或隐藏。
- **PWA**：新增 `childweb/manifest.webmanifest`（`name:"安伴"`、`display:"standalone"`、`theme_color`/`background_color` 取暖色 #F78C6B/#FAF6F0、`icons` 192+512）；`index.html` 加 `<link rel="manifest">` 与 iOS 的 `<meta name="apple-mobile-web-app-capable">` 等；加最小 `childweb/sw.js`（缓存静态壳；对 `/api/*` 用 network-first，别缓存）。手机浏览器"添加到主屏"即全屏 App 体感。
- **图标**：生成/放入 192×192、512×512 PNG（可用安伴主色 + nest_eco_leaf 图标风格的占位图）。
- **部署**：`childweb/` 作为静态目录服务（沿用 `server.js` 或并入安伴现有静态服务）。

---

## 6. 运行配置（baseURL / accessCode / deviceId）
加一个轻量 `config`（读写 localStorage）：
- `accessCode`：登录页输入 → `X-Access-Code`。
- `baseURL`：安伴后端地址；演示默认可填同源相对路径或 `http://<server>:8090`；设置页可改。
- `deviceId`：默认 `9c:13:9e:8b:af:28`；设置页可改。
所有 API 调用统一从该 config 取值构造 `createAnbanClient`。

---

## 7. 分阶段计划（每阶段一个独立小提交；TDD；每阶段在 `docs/REALTIME_CHANGELOG.md` 追加记录）
1. **P1 脚手架**：`childweb` 脚本改 `type="module"`、引入 `api/client.js`、建 `config`（baseURL/accessCode/deviceId←localStorage）、加统一 `notImplemented()`。保证页面照常渲染（先不改各屏逻辑）。
2. **P2 登录**：`handleLogin` 改真校验（`getStatus` 探测，401 报错，成功存 accessCode+session）。
3. **P3 首页**：状态卡←`getStatus`；最近留言←`getHistory`/`listMessages`。
4. **P4 消息**：`loadMessages`←`getHistory`(+留言态)；`handleSend`←`sendMessage`；附件→`notImplemented`。
5. **P5 提醒**：`loadSavedReminders`←`listReminders`；`saveReminder`←`createReminder`（§4 仅一次映射）；删除←`deleteReminder`；详情/历史←`listReminders({status})`；重复/重要→`notImplemented`。
6. **P6 家人画像**：`populateForm`←`getProfile`；保存←`updateProfile`（§3 映射）。
7. **P7 App 化**：去手机外壳 + PWA(manifest/sw/图标) + 设置页(baseURL/deviceId)。
8. **P8 收尾**：环境温湿度等未实现项统一处理；补冒烟测试；更新本仓库 `childweb/README.md` 写明启动/部署/已实现与未实现清单。

---

## 8. 验证 & 纪律（完成判据）
- **不破坏现有测试**：`server/` 下 `GOPROXY=https://goproxy.cn,direct GOSUMDB=off CGO_ENABLED=0 go build ./... && go vet ./... && go test ./...` 全绿；`npm test --prefix web` 仍 80/80。
- **childweb 纯逻辑加测**：把"画像映射、单次提醒时间换算 `nextOccurrenceUTC`、history/留言→气泡的数据整形、API 错误→提示文案"抽成可测纯函数，加 `childweb/smoke.test.mjs`（`node --test` 风格，仿 `web/smoke.test.mjs`）。
- **联调事实**（见 `docs/现状与交接-2026-06-14.md` 与 OpenAPI 联调记录）：访问码=后端 `ANBAN_ACCESS_CODE`；deviceId=`9c:13:9e:8b:af:28`；设备睡眠时主动类不一定可达（保活是已知**固件**问题，与本前端无关，**不要在前端/后端去"修保活"**）。
- **铁律**：只动前端；不改后端 API 契约（缺的就"未实现"）；不碰 xiaozhi；不提交任何密钥；中文 UTF-8；conventional commits 小步提交。
- **不要做**：不要重写 `web/api/client.js` 的 API 逻辑（复用）；不要为对齐 UI 字段去改后端 `profile.Fields`；不要引入重型前端框架（保持原生 JS + Tailwind 产物）；不要动 `dist.css`（除非新增类并理解 Tailwind 构建）。

---

## 9. 目标终态
`childweb/` = 接好安伴后端、可"添加到主屏"的子女端 App：登录(访问码)→首页(设备状态/最近对话)→消息(留言收发)→提醒(单次创建/列表/删除)→家人(画像查看/保存，并联动设备人设)。未接后端的功能优雅地提示"该功能未实现"。现有 `web/` 与 server 测试不受影响。
