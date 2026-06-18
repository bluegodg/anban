# childweb 协作契约（前端 ⇄ 接线）

> 本文件给**两个 Codex / 两位协作者**读：一位优化 UI（结构/样式），一位维护接后端的接线逻辑。两边都先读这份，**各管各的 lane，别动对方文件、别改约定 ID**。这样 UI 可以自由迭代，接线几乎不用重做。
>
> 任一方的 Codex goal 提示词开头都应写："先读 `childweb/CONTRACT.md`，严格遵守文件归属与 DOM 契约。"

## 1. 文件归属（谁能改）

| 文件 | 归属 | 说明 |
|---|---|---|
| `index.html` | **UI 方** | 页面结构、标签、布局、文案。**可自由改样式/排版/加屏**，但见 §2 的 ID 契约 |
| `dist.css` / Tailwind / `fonts/` / `icons/` | **UI 方** | 视觉。`dist.css` 是编译产物，改 Tailwind 源后重新编译 |
| `app.js` | **接线方** | 事件绑定、调后端、渲染数据。UI 方不要改 |
| `api/client.js` | **接线方** | 后端 API 客户端（与 anban 后端契约对应） |
| `config.js` | **接线方** | baseURL / accessCode / deviceId |
| `integration-core.js` | **接线方** | 纯逻辑（映射/时间换算/数据整形），有单测 |
| `not-implemented.js` | **接线方** | 未实现功能统一提示 |
| `manifest.webmanifest` / `sw.js` | **接线方** | PWA |
| `smoke.test.mjs` | **接线方** | 测试 |
| `CONTRACT.md` / `README.md` | 共同 | 改了知会对方 |

**铁律**：UI 方不改 `app.js`/`api/`/`config.js`/`integration-core.js`；接线方不改 `index.html` 的视觉与布局。各自在自己文件里干活，冲突几乎为零。

## 2. DOM 契约（UI 方可以重排样式，但**不许改这些 id / data-* 的名字**）

`app.js` 靠**元素 id 和 `data-*` 属性**把行为挂到 UI 上。**只要这些名字不变，UI 怎么美化、怎么调整布局都不会断接线。**

### 2.1 唯一权威来源（改 ID 前必做）
手列的清单会过时。**改/删 `index.html` 里任何 `id=` 或 `data-*` 前，先 grep 确认 `app.js` 没在用它**：
```bash
# 列出 app.js 依赖的所有 id 与选择器（这就是契约的权威来源）
grep -oE "getElementById\(['\"][^'\"]+['\"]|querySelector(All)?\(['\"][^'\"]+['\"]|getAttribute\(['\"]data-[^'\"]+['\"]" childweb/app.js | sort -u
```
被引用到的 id/data-* = **受保护，不许改名/删除**；要改先和接线方约定。

### 2.2 当前受保护清单（按屏，截至本次；以 §2.1 grep 为准）
- **外壳/导航**：`screenInner`、`globalStatusBar`（含 `.status-bar-time`）、`spaToast`；导航 `.nav-link` + `data-nav`；各屏 section id：`s-login / s-home / s-message / s-warn / s-history / s-detail / s-family / s-mine / s-family-edit`。
- **登录**：`accessCode`、`loginBtn`、`loginBtnText`、`loginBtnIcon`。
- **首页**：`deviceStatusBadge`、`deviceStatusDot`、`deviceStatusLabel`、`statusTitle`、`statusDesc`、`statusTime`、`recentMsgList`、`greetingTriggerBtn`、`greetingStatusText`、`visionLookButton`、`visionStatusText`；快速留言 `quickMsgOverlay/quickMsgCard/quickMsgInput/quickMsgSend`；底部弹层 `bottomSheet/sheetOverlay/sheetInput` 等。
- **消息**：`chatArea`、`messagesContainer`、`messageInput`、发送按钮。
- **提醒/历史/详情**：`reminderList`、`reminderHourList/reminderMinuteList/reminderTimeDisplay/reminderTimePickerPanel`、`reminderSheetInput`、`sheetFreqDisplay/sheetFreqPickerPanel`、`historyList/historyEmpty`、详情 `detailName/detailTime/detailFreq/detailNote/detailIcon/detailIconWrap/detailTags/detailImportantToggle`、删除确认 `deleteConfirmOverlay/deleteConfirmCard`；提醒卡片上的 `data-name/data-time/data-freq/data-note/data-icon/data-iconcolor/data-important/data-reminder-id`。
- **家人/画像编辑**：表单字段 id（`editName/editAge/...` 一类）。
- **设置**：`settingsBaseURL`、`settingsDeviceId`、`saveConnectionBtn`。

> UI 方加**新控件**时：给它起个 id（沿用命名风格）→ 在 PR 里告诉接线方 → 接线方在 `app.js` 补上绑定。**不要自己去 app.js 接**。

## 3. 协作流程
1. **单一源**：所有人围着这个仓库的 `childweb/` 改，不再各自维护副本互发包。
2. **先同步再动手**：开工前 `git pull` 拿最新。
3. **分支 + PR**：各开分支，提 PR，**不直接推主分支**；对方 review 后合并。
4. **加新功能**：UI 方先把界面与 id 做好提 PR；接线方在后续/同 PR 里补 `app.js` 接线。
5. 改了本文件或文件归属，知会对方。

## 4. 后端契约（接线方对接 anban 后端）
`api/client.js` 的方法对应 anban 后端 `/api/*`：

- 新账号路径使用 `Authorization: Bearer <token>`。
- 未绑定账号的设备方法必须在 fetch 前短路。
- 旧演示路径继续使用 `X-Access-Code` 和显式 `deviceId`。
- 账号留言只发送 `text`，不可由前端提交可信 `fromName`。
- 消息页读取 `/api/timeline`。
