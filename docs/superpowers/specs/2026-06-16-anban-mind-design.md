# AnBan Mind 设计稿

- 日期：2026-06-16
- 状态：待用户审阅
- 范围：安伴心智系统设计，不进入代码实现计划

## 1. 设计目标

AnBan Mind 是安伴的统一心智层。它不是 `reminder`、`greeting`、`message` 的增强版，而是负责主动性、人性和动物性的核心系统。

它回答六个问题：

- 为什么想做。
- 何时做。
- 做什么。
- 怎么表达。
- 什么时候不做。
- 做完以后如何改变自己。

完整方向采用三条路线组合：

- 中央心智编排：所有事件先进入 Mind，再产生意图和行动。
- 会话人格渗透：普通 xiaozhi 对话也逐渐受到 AnBan 关系记忆、状态和语气约束影响。
- 自有生活流：AnBan 在用户不说话时也维护自己的状态、关注点和内心思流。

人格主轴：

- 家人型为主：默认权重 0.60。
- 宠物性点缀：默认权重 0.25。
- 管家能力藏在背后：默认权重 0.15。

## 2. 服务边界

方案 C 继续成立，但语义升级：

```text
xiaozhi = voice/device runtime
anban-code = companion mind
```

xiaozhi 负责：

- 设备连接。
- ASR/LLM/TTS。
- 说话中打断和自动聆听。
- MCP 工具执行。
- 对话历史。
- role prompt / RAG 等运行时能力。

anban-code 负责：

- AnBan Mind。
- 关系记忆。
- 主动行为编排。
- 子女端连接。
- 何时说、为什么说、说什么、是否不说。

心智所有权在 `anban-code`，xiaozhi 是身体、喉咙、听觉和工具运行时。

## 3. 总体数据流

```text
World Events
  -> Event Stream
  -> Situation Model
  -> Memory Retrieval
  -> SelfState Update
  -> Drive Activation
  -> Thought Generation
  -> Intention Formation
  -> Behavior Selection
  -> Expression Gate
  -> Action Execution
  -> Feedback
  -> Reflection
  -> Memory/SelfState/LifeState Update
```

关键原则：

- 事件不是规则触发器，而是心智输入。
- Thought 和 Speak 必须分离。
- Domain 是手脚，不是大脑。
- LLM 生成可能性，代码选择现实性。
- 沉默也是有意义的行为。

## 4. 模块结构

新增心智模块：

```text
server/internal/mind/
  engine/       # 心智主入口
  eventstream/  # 统一事件流
  situation/    # 当前处境模型
  memory/       # 心智级记忆视图
  selfstate/    # 安伴自身状态
  drives/       # 动机激活和竞争
  thoughts/     # 内心思流
  intention/    # 意图形成
  behavior/     # 行为选择
  expression/   # 表达闸门
  reflection/   # 事后反思
  life/          # 生活流和当天气质
  executors/     # 现有 domain 的执行器接口
```

Mind 对外只暴露主接口，不暴露内部细节：

```go
type Engine interface {
    Ingest(ctx context.Context, event Event) ([]Action, error)
    TickIdle(ctx context.Context, deviceID string, at time.Time) ([]Action, error)
    Reflect(ctx context.Context, deviceID string, window TimeWindow) error
}
```

依赖纪律：

- `childapi` 可以把子女端操作转成 Mind Event，但不直接碰 xiaozhi 或数据库。
- `scheduler` 只产生时间事件，不直接执行业务动作。
- `domains` 继续保持互不 import，只作为 action executor 被装配。
- `mind` 通过 executor interface 调用现有 domain 能力，接口由 `mind/executors` 或 `pkg/types` 承载。
- `xiaozhiclient` 仍是唯一懂 xiaozhi manager OpenAPI 的位置。
- `cmd/anban/main.go` 负责装配 Mind、executor、store、scheduler 和 xiaozhiclient，避免反向依赖。

## 5. 现有代码归位

现有模块不推倒，而是重新归位。

```text
internal/proactive
  -> expression gate 的打扰成本子系统

internal/memory
  -> mind memory 的事实沉淀底层

domains/greeting
  -> companionship / greeting executor

domains/reminder
  -> stewardship / reminder executor

domains/message
  -> family_bridge / message executor

domains/vision
  -> perception provider + observe executor

domains/profile
  -> prompt projection executor

domains/status
  -> child-facing read model

xiaozhiclient
  -> xiaozhi runtime adapter
```

以后不再让各 domain 自己代表主动性。主动性集中在 Mind：

```text
domain/scheduler/childapi/vision 产生 event
  -> mind.Ingest
  -> selected action
  -> executor
  -> domain/xiaozhiclient
```

## 6. 核心数据模型

核心对象：

```text
Event       # 发生了什么
Situation   # 当前是什么局面
Memory      # 记得什么
SelfState   # 安伴现在是什么状态
Drive       # 为什么想做
Thought     # 心里冒出什么念头
Intention   # 想达成什么
Action      # 实际准备做什么
Feedback    # 对方如何反应
Reflection  # 这次互动意味着什么
LifeState   # 今天/近期的气质和关注主题
```

### Event

Event 是生活中发生的一件事，不是直接命令。

典型类型：

```text
elder_spoke
assistant_spoke
child_message_received
reminder_created
reminder_due
reminder_acknowledged
greeting_requested
presence_seen
presence_absent
vision_observation
device_online
device_offline
long_silence
profile_updated
memory_distilled
action_executed
feedback_observed
```

要求：

- 所有业务入口先产生 Event。
- Event 带来源、时间、摘要、payload、显著性、置信度。
- Event 不直接等于 Action。

### Situation

Situation 是当前处境模型，描述外部局面。

字段方向：

```text
time_of_day
elder_presence
interaction_mode
activity_level
emotional_tone
social_context
open_loops
constraints
```

例子：

```text
现在是晚上。
老人很久没有互动。
下午提醒没有语音确认。
设备在线。
此刻适合观察，不适合长篇打扰。
```

### Memory

记忆分层：

```text
profile_fact      # 家庭画像事实
preference        # 偏好
routine           # 作息与习惯
episodic          # 共同经历
relationship      # 关系记忆
emotional_trace   # 情绪痕迹
open_loop         # 未完成事项
behavior_lesson   # 行为经验
```

事实记忆让它聪明，自传式和关系记忆让它像同一个存在。

### SelfState

SelfState 是动物性和连续性的核心。

```text
warmth       # 亲近感
concern      # 担心程度
curiosity    # 好奇心
playfulness  # 玩闹倾向
energy       # 活跃程度
quietness    # 保持安静倾向
patience     # 陪伴耐心
confidence   # 对当前判断的确信度
```

这些值缓慢漂移，并影响行为选择。

### Drive

Drive 是行为动机。

核心动机：

```text
companionship   # 维持陪伴感
care            # 照看和关心
curiosity       # 好奇与观察
play            # 轻松互动
stewardship     # 提醒、安排、任务完成
family_bridge   # 连接子女和老人
quiet_presence  # 安静陪着
```

多个 drive 竞争，不由单条规则独裁。

### Thought

Thought 是内心念头，不是对外话语。

字段方向：

```text
source_events
related_memories
drive
content
emotional_tone
urgency
care_value
novelty
interruption_cost
intimacy
status
```

绝大多数 Thought 不会说出口，只影响状态、意图和未来行为。

### Intention

Intention 是 Thought 到 Action 的桥。

典型类型：

```text
check_in
comfort
share
remind
ask
observe
wait
notify_child
update_memory
sync_prompt
```

一个 Thought 可以生成多个 Intention。

### Action

Action 是外部或内部可执行动作。

类型：

```text
speak
wait
listen
call_mcp_tool
send_child_notification
update_role_prompt
archive_memory
schedule_recheck
subtle_expression
```

Action 通过 executor 执行，并产生 Feedback。

## 7. 运行循环

AnBan Mind 同时运行四条循环。

### Realtime Loop

事件到达时运行。

```text
new event
  -> normalize
  -> update situation
  -> retrieve memory
  -> update selfstate
  -> activate drives
  -> generate thoughts
  -> choose immediate action or defer
```

目标：不漏事、不机械、不乱插话。

### Idle Loop

用户不说话时运行。

```text
scan recent events
  -> detect open loops
  -> update slow selfstate
  -> generate idle thoughts
  -> score urges
  -> maybe schedule action
```

Idle Loop 让 AnBan 在沉默时仍然有内部活动。

### Reflection Loop

互动后运行。

```text
collect event/action/feedback
  -> summarize episode
  -> extract memory candidates
  -> update relationship memory
  -> adjust selfstate baselines
  -> update behavior lessons
```

Reflection 让 AnBan 在时间中变化。

### Life Loop

低频运行，维护当天气质。

```text
today_theme
lingering_thoughts
social_energy
care_focus
playfulness_trend
relationship_temperature
```

Life Loop 不编造生活经历，只基于真实事件形成当天状态和关注主题。

## 8. 人格混合模型

人格不是单个 prompt，而是权重、状态和行为偏置。

```text
family  = 0.60
pet     = 0.25
steward = 0.15
```

职责：

```text
family decides tone
steward decides capability
pet decides liveliness
```

家人型负责：

- 记得旧事。
- 关心当下。
- 说话自然。
- 懂得不打扰。

宠物性负责：

- 状态感。
- 亲近反应。
- 好奇心。
- 小幅度不可预测性。
- 安静陪着。

管家性负责：

- 提醒。
- 汇报。
- 工具调用。
- 子女连接。
- 记忆和 prompt 同步。

三者不平权竞争。家人型是底色，宠物性是生命感，管家性是能力。

## 9. 行为选择系统

事件不直接触发行为。行为来自候选竞争。

```text
thoughts + drives + selfstate + situation + memory
  -> candidate intentions
  -> candidate actions
  -> scoring
  -> expression gate
  -> execute / wait / suppress
```

评分公式：

```text
score =
  drive_strength
  + care_value
  + relationship_fit
  + situation_fit
  + memory_relevance
  + timing_fit
  + novelty
  + personality_bias
  - interruption_cost
  - repetition_cost
  - uncertainty_penalty
  - fatigue_cost
```

表达闸门输出：

```text
execute_now
schedule_later
wait_for_better_moment
silently_update_state
ask_for_more_context
notify_child
suppress
```

有限不可预测性采用 controlled variability：

- 先用评分筛掉不合理候选。
- 只在分数接近的候选之间轻微采样。
- 采样受 SelfState 和人格权重影响。
- 硬约束先过滤。

## 10. LLM 职责

原则：

```text
LLM 负责理解、生成、总结、表达。
代码负责状态、约束、评分、执行、审计。
```

LLM 子能力：

```text
Extractor          # 抽取事实、偏好、关系记忆
Interpreter        # 理解当前处境
ThoughtGenerator   # 生成候选内心想法
Reflector          # 总结互动意义
Renderer           # 渲染自然表达
```

LLM 不直接决定外部副作用。它提供结构化候选，Mind Engine 负责最终选择。

每个 LLM 调用必须：

- 输入明确。
- 输出 JSON schema。
- 字段有范围。
- 失败可降级。
- 可测试。

降级策略：

```text
Extractor 失败 -> 不新增记忆
Interpreter 失败 -> 用规则版 situation
ThoughtGenerator 失败 -> 用模板 thought 或只记录事件
Reflector 失败 -> 不做长期更新
Renderer 失败 -> 用短模板表达
```

## 11. 与 xiaozhi 的交互

当前保持方案 C：

```text
AnBan Mind -> xiaozhiclient -> xiaozhi manager OpenAPI -> device
```

继续使用已有能力：

```text
InjectSpeak
GetDeviceStatus
GetHistory
SetRolePrompt
CallDeviceMCPTool
```

未来如 xiaozhi 暴露更多接口，可以扩展：

```text
GetSessionState
ListMCPTools
GetRecentToolCalls
GetRAGContext
```

如果未来需要影响每轮普通对话，优先增加薄 hook：

```text
xiaozhi session event -> AnBan Mind context -> xiaozhi response hint
```

但心智所有权仍留在 `anban-code`。

## 12. 数据持久化

新增心智表：

```text
mind_events
mind_situations
mind_memories
mind_self_states
mind_thoughts
mind_intentions
mind_actions
mind_feedback
mind_reflections
mind_life_states
```

保留现有业务表：

```text
messages
reminders
greetings
profiles
memory_facts
status_snapshots
```

关系：

```text
mind_actions.executor_ref -> domain 具体记录
mind_events.source_ref -> 来自哪个 domain/xiaozhi/childapi
mind_memories.evidence_event_ids -> 支撑该记忆的事件
```

Mind 表保存为什么，domain 表保存执行细节。

## 13. 测试策略

测试重点：

- 事件进入 eventstream 后不会被 domain 私自直接执行。
- `reminder_due` 会生成 thought/intention/action，而不是必然 speak。
- 夜晚 `long_silence` 可以只更新 concern，不打扰老人。
- 同样场景下 SelfState 不同，行为选择不同但仍在合理范围。
- LLM 失败时 fallback action 可执行。
- Reflection 能把反馈更新到 memory/selfstate。
- Expression gate 能解释为什么说或不说。

测试层次：

```text
unit tests:
  scoring, gate, state update, memory selection

integration tests:
  event -> action -> executor

golden tests:
  LLM JSON 输出结构和降级路径

simulation tests:
  一天生活事件流，检查连续性、节奏和克制
```

## 14. 完整建设顺序

这不是缩小范围，而是完整系统的搭建顺序。

1. 统一事件流：所有入口进入 `mind_events`。
2. Action Executor 收编：domain 变成执行器。
3. Situation + SelfState：建立当前处境和安伴自身状态。
4. Drives：建立动机激活和竞争。
5. Thoughts + Intentions：建立内心思流和意图形成。
6. Behavior Selection + Expression Gate：建立行为评分、克制和显化。
7. Reflection + Relationship Memory：建立长期关系变化。
8. Life Loop + 会话渗透：建立持续存在感，并影响普通对话。

## 15. 非目标

本设计不展开：

- 公网安全、权限、隐私和合规策略。
- 子女端 UI 改版。
- xiaozhi core fork。
- 设备固件改造。
- 医疗诊断或健康建议能力。

这些可以另开设计文档。

## 16. 完成后的产品形态

AnBan 不再是：

```text
提醒 + 问候 + 留言 + 画像 + 视觉
```

而是：

```text
一个持续感知生活、
持续产生内心思流、
有记忆和自我状态、
由动机驱动行为、
能主动也能克制、
会在时间中变化的陪伴心智。
```

它应该让人感觉：

- 它不是定时任务。
- 它不是客服。
- 它不是单纯助手。
- 它在用户不说话时也仍然存在。
- 它有时候开口，有时候安静。
- 它能记得共同经历，并慢慢形成相处方式。
