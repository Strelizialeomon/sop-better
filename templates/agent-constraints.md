<!-- templates/agent-constraints.md —— /sop-init 据档把对应块写进项目 CLAUDE.md。占位符:{{tier}} {{default_altitude}} {{ends}} -->

## === T0 块(一次性脚本) ===
```markdown
## Agent 工作约束(本项目 · T0 一次性脚本)
- 不跑 brainstorm / writing-plans,直接做。
- 改动小、可丢弃;出问题重写即可。
- Agent 必须客观顶嘴:有更简单的做法直说,别为"显得认真"加结构。
```

## === T1 块(单人认真) ===
```markdown
## Agent 工作约束(本项目 · T1 单人认真)
- 梳理思路:用 brainstorming,但只梳"要什么"(目标/约束/验收)。
  技术架构由 agent 提案,我评审 + 否决,不亲自设计。
- 不跑 writing-plans:spec 通过后直接实现(spec 必含可检验的验收标准)。
- 必跑 code review:撒手后的唯一安全网,不可省。
- Agent 必须客观顶嘴:有异议直说、标不确定与风险,禁止讨好附和。
- 默认撒手档 = {{default_altitude}};不可逆/高风险改动才升回我主导。
```

## === T2 块(多端 / 多 agent) ===
```markdown
## Agent 工作约束(本项目 · T2 多端 / 多 agent)
- 端划分:{{ends}}。每端有自己的 scope,跨端走 docs/contracts/ 的单一真相源。
- 梳理思路:各端独立 brainstorming,只梳"要什么";架构 agent 提案、我评审。
- 不跑 writing-plans:spec 通过直接实现(spec 验收标准必须可检验)。
- 必跑 code review;跨端契约 freeze 需我明确确认。
- Agent 必须客观顶嘴:禁止讨好;发现治理过重要主动喊"这里用不着这么重"。
- 默认撒手档 = {{default_altitude}};跨端不可逆改动升回我主导。
- 状态用标记:✅ 完成 / 🚧 进行 / ⏸️ 搁置 / ⬜ 未开始。
```
