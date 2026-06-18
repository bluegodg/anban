# decisions/

> 一次性、重要的技术或方向决策的记录。每份决策一页纸，独立成文件。
> 区别于 specs/（设计文档，描述"打算怎么做"）和 plans/（实施计划，描述"怎么动手做"）：decisions/ 记录"为什么这么选的"。

## 文件命名

`YYYY-MM-DD-<topic>.md`，例如：
- `2026-06-08-memory-module.md` —— W1 周五的记忆模块路线选择
- `2026-06-15-degrade-level.md` —— W2 中期是否触发降级的判断

## 单份决策模板

```markdown
# <决策主题>

- 日期：YYYY-MM-DD
- 状态：已决策 / 待审 / 已推翻
- 决策人：<谁>

## 上下文

<这个决策出现的背景，为什么现在要拍板>

## 选项

- A：<...>
- B：<...>
- C：<...>

## 选择

选 <X>。

## 理由

<2-3 段，列具体事实和工程判断>

## 风险与止损

<这个决策如果错了，什么时候/怎么发现，怎么撤回>
```

## 已落定的决策

- [x] [`2026-05-29-server-architecture.md`](./2026-05-29-server-architecture.md) —— 服务端架构选 **C**（安伴做独立第三服务 + 冻结 xiaozhi），经真代码深读证实。
- [x] [`2026-06-16-scheme-c-repo-boundary-and-deployment.md`](./2026-06-16-scheme-c-repo-boundary-and-deployment.md) —— 设备到手后的仓库边界、两服务部署口径、Gate A/B/C/D 和可插拔验收。
- [x] [`2026-06-18-vision-vlm-and-device-verification-blocker.md`](./2026-06-18-vision-vlm-and-device-verification-blocker.md) —— “看一眼·原图”当前阻塞在设备离线与 xiaozhi VLM 模型/endpoint 不可用，暂不宣布端到端完成。

## 当前待写的决策

- [ ] `<YYYY-MM-DD>-memory-module.md` —— W1 周五前完成，路线 A/B/C 选一（见 PRD §5 / §7.4）
