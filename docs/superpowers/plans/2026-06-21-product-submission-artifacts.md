# 安伴成果物提交文档修订 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 按成果物要求修订 `product/商业计划书_安伴.docx` 与 `product/product-intro.docx`，保留并复核 `product/demo.mp4`，产出内容真实、可打开且逐页视觉检查通过的提交文件。

**Architecture:** 使用一个临时 Python 构建脚本对既有 DOCX 做局部、确定性编辑：商业计划书校准架构、功能、隐私、市场、财务和团队章节；产品说明书更新当前功能、测试、运行说明和无障碍属性。每个文件独立生成到临时路径，经过结构审计、WPS/PDF 渲染、逐页 PNG 检查后再替换 `product/` 中的最终文件。

**Tech Stack:** Bundled Python、python-docx、OOXML、WPS COM/PDF、pypdfium2、FFmpeg/ffprobe。

---

### Task 1: 固定基线与事实清单

**Files:**
- Read: `product/商业计划书_安伴.docx`
- Read: `product/product-intro.docx`
- Read: `product/demo.mp4`
- Read: `docs/agent-memory.md`
- Read: `docs/capabilities/family-profile-memory-mind.md`
- Read: `docs/REALTIME_CHANGELOG.md`
- Read: `server/cmd/anban/main.go`
- Read: `server/internal/childapi/server.go`
- Read: `server/internal/config/config.go`
- Create: `%TEMP%/anban-doc-revision/baseline/`

- [ ] **Step 1: 复制两份原始 DOCX 到临时基线目录**

  使用 PowerShell `Copy-Item -LiteralPath` 复制，不在 `product/` 中留下备份文件。

- [ ] **Step 2: 提取段落、表格、图片、超链接、核心属性和字段信息**

  输出 JSON，内容至少包含段落索引、样式、表格单元格、图片数量、外链、作者、创建时间、修订标记和批注部件。

- [ ] **Step 3: 复核当前事实**

  证据必须确认：AnBan 独立于 xiaozhi；当前前端为 `childweb/`；`web/` 为旧版兼容；视觉原图会留存；OpenMemory 当前为兼容读取；生产自动视觉轮询关闭；Go 全包测试、childweb 60/60、web 80/80。

- [ ] **Step 4: 复核外部入口和视频**

  `git ls-remote origin main` 必须成功；线上首页、`/health` 和 APK 返回 HTTP 200；`ffprobe` 必须确认视频为 1920x1080 H.264/AAC 且可读取时长。

### Task 2: 编写确定性 DOCX 修订器

**Files:**
- Create: `%TEMP%/anban-doc-revision/revise_docs.py`
- Create: `%TEMP%/anban-doc-revision/revision_manifest.json`

- [ ] **Step 1: 建立安全替换和插入函数**

  实现 `replace_paragraph_text`、`insert_paragraph_after`、`insert_table_after`、`insert_picture_after`、`set_image_alt_text`、`set_repeat_table_header`、`set_update_fields` 和 `scrub_core_properties`。每次目标匹配必须恰好一次；零次或多次匹配立即失败。

- [ ] **Step 2: 定义商业计划书修订清单**

  清单必须覆盖：方案 C 架构、真实功能列表、视觉留存与权限、记忆与四层上下文、Mind、PWA/APK、系统架构图、来源附录、三年测算、团队能力与岗位分工，以及绝对化措辞降级。

- [ ] **Step 3: 定义产品说明书修订清单**

  清单必须覆盖：60/60、`web/` 旧版标签、账户与绑定、角色权限、时间线、语音输入、问候计划、视觉历史、PWA/APK、本地环境变量、HTTP/PWA 边界、表题修正、图片替代文本和元数据。

- [ ] **Step 4: 生成修订清单 JSON**

  每一项记录文档、操作、锚点、修改后摘要和成功状态；构建结束时所有项目必须为成功。

### Task 3: 修订商业计划书

**Files:**
- Modify: `product/商业计划书_安伴.docx`
- Read: `output/imagegen/anban-system-architecture.png`
- Read: `output/imagegen/anban-server-four-layer-context.png`

- [ ] **Step 1: 替换过时技术描述**

  所有“AnBan 接管设备音频流/ASR/LLM/TTS”“基于 xiaozhi 服务扩展开发”“图片不保存”“已实现向量检索”等表述必须删除或改为当前事实。

- [ ] **Step 2: 补充已实现产品能力**

  将账户、绑定、角色、时间线、问候计划、视觉历史、AI 画像、Mind、PWA 和 APK 写入产品与技术章节，并区分当前能力与未来规划。

- [ ] **Step 3: 插入架构图与四层上下文图**

  两张图片使用现有源文件，宽度不超过正文宽度；添加说明性题注和有意义的替代文本。

- [ ] **Step 4: 重写市场来源与口径**

  采用国家统计局 2025 年人口公报、国务院养老服务意见和网信办拟人化互动治理文件；删除无法追溯的“83%”“120%”等数字。TAM/SAM/SOM 采用人口基数与明确业务假设分层，不伪装成第三方统计。

- [ ] **Step 5: 扩充三年财务测算**

  保留既有价格带，新增保守测算表和敏感性说明；设备销量、订阅率、ASP、硬件成本、服务成本、获客成本、退换/售后均标记为内部测算假设，不表述为已实现收入。

- [ ] **Step 6: 完成团队章节**

  使用“产品与用户研究、AI/后端、子女端与移动端、硬件与集成、运营与合规”五类岗位分工，不写虚构姓名、学历、公司经历或融资经历。

- [ ] **Step 7: 新增数据来源与参考资料**

  写入可点击的官方链接、发布日期、访问日期和对应正文口径；市场测算项明确标注“安伴内部情景测算”。

### Task 4: 修订产品说明书

**Files:**
- Modify: `product/product-intro.docx`

- [ ] **Step 1: 更新验证证据与版本信息**

  将 childweb 59/59 更新为 60/60；保留 Go 全包通过和 web 80/80；更新文档修订日期和核心属性。

- [ ] **Step 2: 更新产品功能清单**

  补充账户注册/登录、设备绑定、管理员/成员权限、时间线、浏览器语音输入、问候计划、视觉历史/重分析/删除、PWA 和 APK。

- [ ] **Step 3: 校准技术与运行说明**

  明确 `childweb/` 是当前前端、`web/` 是旧版兼容；本地运行写明 `.env.example`、Manager Base URL、API Token、访问码等配置类别；注明 HTTP 线上入口不保证浏览器 PWA 安装，移动端优先 APK。

- [ ] **Step 4: 修复题注和无障碍信息**

  将错误的“图 1”改为“表 1”；为十张图片写入能说明内容的替代文本；保留现有暖橙视觉系统。

### Task 5: 结构审计与内容验收

**Files:**
- Verify: `product/商业计划书_安伴.docx`
- Verify: `product/product-intro.docx`
- Create: `%TEMP%/anban-doc-revision/audit/`

- [ ] **Step 1: 运行敏感词与事实回归扫描**

  旧错误短语、59/59、空团队标题、TBD/TODO、密钥样式和访问码不得出现；源码、线上地址、APK、60/60、方案 C、Mind 和数据来源必须出现。

- [ ] **Step 2: 运行 OOXML 结构审计**

  两份文档必须可重新打开；批注、插入、删除、移动修订计数均为 0；商业计划书至少含两张图片，产品说明书所有图片均有替代文本。

- [ ] **Step 3: 运行无障碍审计**

  检查标题层级、图片替代文本、表头标记和超链接文本；只对确认是表头的第一行设置重复表头。

- [ ] **Step 4: 核对成果要求矩阵**

  商业计划书七项要求逐项有正文证据；产品说明书包含产品详情、源码和可演示记录；视频与线上地址作为并行证据。

### Task 6: 渲染、逐页检查与迭代

**Files:**
- Create: `%TEMP%/anban-doc-revision/render/business/`
- Create: `%TEMP%/anban-doc-revision/render/intro/`

- [ ] **Step 1: 将两份 DOCX 转换为 PDF 和逐页 PNG**

  优先使用打包渲染器；本机缺少 LibreOffice 时使用 WPS COM 转 PDF，再用 bundled `pypdfium2` 以 100% 以上分辨率生成逐页 PNG。

- [ ] **Step 2: 检查每一页**

  逐页确认无裁切、重叠、乱码、表格溢出、图片失真、异常空白页和错误页码；同时检查标题、表格、图片、来源与结尾页。

- [ ] **Step 3: 修复并重新渲染**

  任一页面失败即修改构建脚本并重新生成两份最终 DOCX；最后一次修改后必须重新完成结构审计和全页检查。

### Task 7: 最终完成审计

**Files:**
- Verify: `product/商业计划书_安伴.docx`
- Verify: `product/product-intro.docx`
- Verify: `product/demo.mp4`

- [ ] **Step 1: 重跑 Go 与前端测试**

  `go test -count=1 ./...`、`npm test --prefix childweb`、`npm test --prefix web` 必须通过，并记录实际测试数。

- [ ] **Step 2: 重跑公网和 Git 入口检查**

  GitHub `main` 可读取；首页、健康检查和 APK 返回 HTTP 200。

- [ ] **Step 3: 检查最终工作树**

  只暂存修订规格、实施计划和三个 `product/` 成果物；不暂存 `server/anban-linux` 或其他用户文件。

- [ ] **Step 4: 提交修订成果**

  提交信息使用 `docs(product): revise submission documents`，提交后确认分支为 `feat/product-docs-revision` 且工作树仅剩原有无关文件。
