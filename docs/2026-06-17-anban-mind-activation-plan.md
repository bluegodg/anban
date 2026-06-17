# 安伴心智"激活"计划：A 自主开口 + B 会话渗透 — 2026-06-17

> 给执行者(含 Codex goal)：你在 **anban-code** 仓库。AnBan Mind 的大脑已建好并在"纯观察"运行(见 [[anban-mind-deployed]] / `docs/superpowers/specs/2026-06-16-anban-mind-design.md`)。本计划只做**把它的两个输出口接通**：A=没人对话时自主轻声开口；B=对话时让心智状态影响设备回答。**只动 anban 层，不碰 xiaozhi、不改固件**。全程 TDD、小步、**每阶段真机看效果再继续**。不破坏已上线的：留言/提醒必达、#5 画像召回、现有 demo。

---

## 0. 现状与目标

- 现状(已查证)：mind 输入侧全建好(events/situation/selfstate/drives/thoughts/behavior/expression/reflection/life)，但**输出口都没通**：
  - 自主行动：idle loop 的 `long_silence` 念头被 `quiet_presence` 动机压成 `ActionWait`，从不 `speak`；
  - 会话渗透：mind 只**读**对话(history poller 入 `elder_spoke/assistant_spoke`)更新自身状态，**不回写**进对话。设备对话的"记得王阿姨/小宝"来自 **#5 profile/memory 的 role prompt**，不是 mind。
- "嘴"已存在：dispatcher 的 `greeting` executor → `greetingService.SpeakText` → `voiceGate`(10分钟配额) → `InjectSpeak(SkipLLM, AutoListen)`。
- 目标：A 让 mind 在"沉默够久 + 关心够高 + 配额允许 + 非深夜 + 设备在线"时**自主说一句**；B 让 mind 的"心境/牵挂"经 **role prompt 投影**软渗透进每轮对话。

## 1. 已定关键决策(执行者照此做，勿自行改方向)

1. **A 复用现有 `greeting` executor 开口**，不新造下发管道；话术先用**确定性模板**(不调 LLM)，保证稳。
2. **A 必须克制**：受现有 `ProactiveVoiceGate`(10分钟1条) + 一个**每设备冷却期**约束；深夜不说(沿用 situation 的 night 判断，时区已修为本地)；对话进行中不插话(沿用 conversation defer)。
3. **B 单一 prompt 所有者 = profile**：把 role prompt 装配统一成一个函数 `BuildPromptWith(fields, memoryFacts, mindContext)`；**profile 同时持久化 fields + memoryFacts + mindContext 三块**，任何一块更新都**读齐另外两块重建整段**再 `SetRolePrompt`。**mind 绝不自己调 SetRolePrompt**，只产出 `mindContext` 文本块交给 profile。这样三方(画像/记忆/心智)不再互相覆盖。
4. **保活前提**：A 的自主开口只在**设备在线**时能落地(WS"用完即挂"，离线时 InjectSpeak 失败属正常，静默跳过即可)。本计划不解决保活；真机验时设备需唤醒/供电。
5. **不碰 xiaozhi**：真正"逐轮实时影响下一句"需 xiaozhi 对话 hook → 违反方案C冻结、设计列为未来 → **本期不做**。B 只做 role-prompt 软渗透。

## 2. 必读(开工各读一次)

- `server/cmd/anban/main.go`(dispatcher、mind sinks、startMindLoops、greeting executor 装配)
- `server/internal/mind/{drives,behavior,expression,selfstate}/*.go`、`engine/engine.go`(TickIdle/runPipeline)
- `server/internal/domains/greeting/service.go`(SpeakText/play/voiceGate)
- `server/internal/domains/profile/service.go`(BuildPrompt/BuildPromptWithMemory/SyncMemoryFacts/Update)、`store.go`、`types.go`
- `server/internal/memory/service.go`(distill 如何经 profile.SyncMemoryFacts 写 prompt)
- `server/internal/proactive/*.go`(VoiceGate 接口)
- `AGENTS.md`、`docs/现状与交接-2026-06-14.md`

## 3. 阶段(每阶段独立小提交；测试绿 + 真机看效果，再开下一个)

### A1 — 让心智在空闲时能"决定开口"(后端逻辑)
- 在 `drives`/`behavior` 调整：当 `long_silence` 且 `concern`(或 care 强度)超过阈值、且距上次自主开口超过冷却期时，产出一个 **`ActionSpeak` + executor `greeting`** 的候选(而非永远 `wait`)；否则仍 `wait`/安静。
- 候选 `Text` 用**模板**(随机/轮换几句温和 check-in，如"我在这儿呢，刚想起你，今天还顺心吗？")。
- 保持 `expression.Gate` 现有克制(night 压制、conversation 延后)。
- TDD：沉默+高 concern→产出 speak；沉默+低 concern 或冷却期内→仍 wait；night→压制；conversation→延后。**不真发网络**(executor 用 fake)。

### A2 — 接线 + 配额 + 冷却 + 真机调
- `main.go`：idle loop 产出的 speak 动作经 dispatcher `greeting` executor 真下发；确认走 `voiceGate`；加**每设备自主开口冷却**(config: `ANBAN_MIND_PROACTIVE_COOLDOWN`，默认如 30m)与"仅白天"开关。
- **真机验**(设备唤醒守着)：静置一段(可临时把 idle interval/沉默阈值调小)，确认它**只在合适时机轻声说一句、不连环唠叨、深夜不说、对话中不插话**；据实际节奏调阈值/冷却。

### B1 — 统一 role prompt 装配(不改变现有对外行为，先防回归)
- `profile`：新增持久化字段 `MemoryFacts []string`、`MindContext string`；实现 `BuildPromptWith(fields, memoryFacts, mindContext)`(沿用 1500 rune 上限，心境块放末尾、最先被截断)；`Update`、`SyncMemoryFacts` 改为**读齐三块重建**(修掉"profile.Update 会清掉记忆事实"的隐患)。
- **回归测试**：只设 fields → prompt 与现在等价(#5 不变)；设 fields+facts → 含画像与近期记忆；三块都设 → 都在且不超长。**此阶段 mindContext 仍为空，对外无变化。**

### B2 — 心智把"心境"投影进 prompt(软渗透)
- 新增 `profile.SyncMindContext(deviceID, mindContext)`：读齐 fields+facts，套上新 mindContext 重建并 `SetRolePrompt`。
- mind 侧：在 `reflection`/`life` tick 后，由**确定性逻辑**从 `SelfState`+近期 open loops/episodic 生成一段**简短心境块**(如"最近你较挂念老人(concern 偏高)，语气更关切些；今天聊过：累、养花。")，经 main.go 装配调用 `SyncMindContext`(**不在 mind 包里 import profile**，走 main.go 或 pkg/types 接口，守架构)。
- TDD：给定 SelfState/事件 → 生成预期心境块文本(确定性)；缺数据→空块(降级，不写)。
- **真机验**：跟设备聊几轮(让 concern/warmth 漂移)→ 等一次 tick → 再对话，观察**回答语气/关切度随心境变化**，且**画像召回(问"孙子叫啥"→小宝)仍正常**。

> B3(逐轮实时 hook)= 未来/超范围，不做。

## 4. 铁律

1. 只动 anban-code(server/ + docs)。不改 xiaozhi/固件/其架构。
2. 不破坏：留言/提醒必达、#5 画像召回、架构守护测试(domains 不 import mind；编排在 cmd/childapi/scheduler/adapter)。**mind 不得直接 import profile/greeting**——经 main.go 装配或 pkg/types 接口。
3. mind 自主开口必须过 `voiceGate` + 冷却；失败/设备离线静默跳过，不报错刷屏。
4. 纯增量、不新引依赖、不提交密钥；conventional commits 小步提交；每阶段追加 `docs/REALTIME_CHANGELOG.md`。
5. 全程：`server/` 下 `GOPROXY=https://goproxy.cn,direct GOSUMDB=off CGO_ENABLED=0 go test ./...` 全绿。

## 5. 完成判据

- A：设备在线静置 → 合适时机自主轻声开口一次(配额+冷却+非深夜+不插话)，有单测锁逻辑、真机验过节奏。
- B：对话回答能体现心智心境(关切度随 concern/warmth 变)，且 **#5 画像召回不回归**；role prompt 三块(画像/记忆/心境)统一装配、互不覆盖、不超 1500。
- 五套测试全绿；架构守护不红；留言/提醒仍必达；REALTIME_CHANGELOG 每阶段更新。

## 6. 卡住怎么办

不要猜、不要即兴改 mind 整体设计或动 xiaozhi。把问题写进 `docs/decisions/` 新文件(卡点 + 选项 + 建议)，提交并在结尾点名，等人工裁决。任一测试做不绿就停下如实报告。**B1 若发现统一装配会改变现有 #5 prompt 输出，先停下确认**(这是已上线功能)。
