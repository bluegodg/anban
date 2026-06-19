# 家人资料、专属记忆与心智上下文能力契约

## CAPABILITY

子女端 `childweb/` 的“家人”界面负责管理陪伴对象资料和专属记忆；安伴后端持久化这些内容，并把它们连同 Mind 生成的心智上下文，通过 xiaozhi 已支持的长期记忆 provider 通道提供给设备每轮对话。xiaozhi manager agent `custom_prompt` 只保留设备的语气、性格和行为偏向，不再保存“蓝”“王阿姨”等个人资料。

## CONSTRAINTS

- xiaozhi 仍是冻结上游；不修改 core、manager 源码或设备固件。
- xiaozhi manager agent `custom_prompt` 是风格层，只能包含与具体陪伴对象无关的风格和行为约束。
- 陪伴对象资料、专属记忆和心智上下文由 AnBan 持有，不能写入 `custom_prompt`。
- AnBan 在同一后端进程内提供带独立 Bearer token 的 MemOS 兼容读取接口；不新增第三个记忆服务进程。
- xiaozhi 只按当前用户话语查询上下文；`add/message`、`flush`、`reset/memory` 兼容路由不接收或删除业务数据。对话沉淀继续由 AnBan 主动轮询 manager history 后完成。
- 未部署 AnBan 时，xiaozhi 保持原有基础对话能力；部署 AnBan 后才把目标 agent 切为 `memory_mode=long` 并配置该 provider。
- 资料和记忆写操作只能由家庭管理员执行；普通成员只读。

## IMPLEMENTATION CONTRACT

- Actor: 家庭管理员可编辑陪伴对象资料、添加/编辑/删除专属记忆。
- Actor: 普通家庭成员可查看资料和专属记忆。
- Surface: `GET/PUT /api/profile` 管理陪伴对象资料。
- Surface: `GET/POST/PUT/DELETE /api/memory/facts` 管理分条记忆。
- Surface: `POST /api/openmem/v1/search/memory` 是 xiaozhi 的受保护上下文读取口。
- Data: profile 持久化 `fields`、`memoryFacts`、`mindContext`，统一生成不含风格指令的陪伴对象上下文。
- Data: memory facts 带 `source=manual|dialogue`，手动和自动沉淀共用同一事实库。
- Runtime: 空 query 返回空，避免 xiaozhi 会话启动时缓存旧资料；每轮非空 query 都读取当前 profile，因此管理员修改后下一轮即可生效。
- Runtime: `xiaozhiclient.SetRolePrompt` 只允许写风格 prompt；检测到 `ANBAN_CONTEXT` 或陪伴对象/记忆/心智标签时直接拒绝。
- Runtime: manager 风格层要求称呼严格使用 profile 的“常用称呼”原文，不自行追加“阿姨”“奶奶”等后缀。
- Runtime: Mind engine 通过装配层读取 profile/memory 摘要，生成 `mindContext` 时显式参考陪伴对象资料和长期记忆。

## DEPLOYMENT

1. 在 AnBan 环境中设置至少 32 字节随机值 `ANBAN_MEMORY_PROVIDER_TOKEN`。
2. 在 xiaozhi manager 新建默认 memory config：provider=`memos`，`base_url` 指向 AnBan 的 `/api/openmem/v1`，`api_key` 使用同一随机值，`enable_search=true`。
3. 把目标 agent 的 `memory_mode` 改为 `long`，并把 `custom_prompt` 清理为纯风格文本。
4. 重启 xiaozhi core 以重新初始化 memory provider；AnBan 可独立重启。
5. 用带 Bearer token 的 `search/memory` 请求验证返回资料、记忆和心智上下文，再用真机询问陪伴对象姓名验证最终回答。

## NON-GOALS

- 不把 xiaozhi 的对话消息反向写入 AnBan provider；自动沉淀仍走既有 history poller。
- 不在本切片实现 AI 自动生成或自动改写“AI 认知画像”；当前仍以管理员可编辑画像为主。
- 不在本切片实现向量检索；当前上下文上限仍为 1500 rune，优先保证基础资料完整可用。

## OPEN QUESTIONS

- AI 认知画像后续是否独立成字段，还是继续复用 `health` 文本中的 `AI画像：` 前缀。
- 自动沉淀事实的冲突处理、过期策略、同义合并规则尚未产品化。
- 是否需要在子女端展示每条记忆的来源、时间和“由对话自动生成”标记。

## HANDOFF

本切片先完成严格分层和真机生效验证；下一小步再做 AI 认知画像自动生成/更新，不把两个阶段混在一次改动中。
