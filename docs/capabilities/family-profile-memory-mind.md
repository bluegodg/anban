# 家人资料、专属记忆与心智上下文能力契约

## CAPABILITY

子女端“家人”界面负责管理陪伴对象资料和专属记忆；安伴后端把这些内容与心智系统生成的状态上下文合并为安伴托管上下文块，写入 xiaozhi manager agent prompt。设备继续由 xiaozhi 负责基础语音对话，安伴负责让对话更像“知道这个人、记得这个家、会主动关心”。

## CONSTRAINTS

- xiaozhi 仍是冻结上游；安伴只通过 `xiaozhiclient` 调 manager OpenAPI。
- xiaozhi manager agent prompt 的人工可编辑内容视为风格层，描述语气、性格、偏向。
- 陪伴对象资料、专属记忆、心智心境属于安伴托管上下文，不应散落在风格层里。
- 安伴写 prompt 时只能替换 `ANBAN_CONTEXT` 托管块；已存在的风格层文字必须保留。
- 旧版整段安伴画像 prompt 会被迁移为托管块，避免继续保留“王阿姨”等旧演示对象事实。
- 资料和记忆写操作只能由家庭管理员执行；普通成员只读。

## IMPLEMENTATION CONTRACT

- Actor: 家庭管理员可编辑陪伴对象资料、添加/编辑/删除专属记忆。
- Actor: 普通家庭成员可查看资料和专属记忆。
- Surface: `childweb/` 家人页是当前子女端。
- Surface: `GET/POST/PUT/DELETE /api/memory/facts` 管理分条记忆。
- Surface: `GET/PUT /api/profile` 管理陪伴对象资料。
- Data: profile 持久化 `fields`、`memoryFacts`、`mindContext`，统一生成安伴托管上下文。
- Data: memory facts 带 `source=manual|dialogue`，手动和自动沉淀共用同一事实库。
- Runtime: `xiaozhiclient.SetRolePrompt` 将托管上下文写入 manager agent `custom_prompt` 的标记块。
- Runtime: mind engine 通过装配层读取 profile/memory 摘要，生成 `mindContext` 时可参考陪伴对象资料和长期记忆。

## NON-GOALS

- 不修改 xiaozhi core、manager 源码或设备固件。
- 不把安伴记忆库等同于 xiaozhi 内置 MemoryProvider；安伴记忆库是可控、可见、可管理的产品事实库。
- 不在本切片实现 AI 自动生成或自动改写“AI 认知画像”；当前仍以管理员可编辑画像为主。

## OPEN QUESTIONS

- AI 认知画像后续是否独立成字段，还是继续复用 `health` 文本中的 `AI画像：` 前缀。
- 自动沉淀事实的冲突处理、过期策略、同义合并规则尚未产品化。
- 是否需要在子女端展示每条记忆的来源、时间和“由对话自动生成”标记。

## HANDOFF

当前可继续按小切片推进：下一步优先做 AI 认知画像的自动生成/更新机制，或做记忆事实的冲突合并与来源展示。
