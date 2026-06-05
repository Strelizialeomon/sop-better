<!-- templates/collaboration.md —— C1(双角色/小团队)轻量协作。C2 多端·多并行 agent 改用 collaboration-c2.md。生成为 docs/project/collaboration.md。占位符:{{ends}} {{collaborators}} -->

# 协作约定(本项目)

> 本文件是"谁干什么、怎么不撞车"的真相源。跨端事实定义在 `docs/contracts/`,这里只管协作流。

## 业务端起需求(对话 → 收口 → issue + req doc)

1. **对话**:业务方跟 agent 聊需求,agent **主动建议补盲区**(可行性 / 怎么拆 · 照亮不代选),业务方拍——循环到收口。
2. **收口标准**(三样钉死才算,不是聊到完美):**① 验收(怎么算对)· ② 主流程(哪几步)· ③ 范围(要啥 / 不要啥)**。
3. **落凭据**:起需求 issue(轻量:idea + assignee + link)+ req doc(`docs/requirements/`)。**req doc 只写"要什么"(语义级:做什么 / 验收 / 主流程 / 跨端换什么数据),绝不碰"怎么做"**——技术留给 dev 端 brainstorm。
4. **交棒**:收口后球交 agent 实施,业务方只在高风险闸 / 联调露面。

## 角色与边界({{ends}})

每端一个 scope agent,只动自己 scope 的代码:

{{#each ends}}
- **scope:{{this}}** —— 负责 {{this}} 端;跨端需求走 contracts + handshake,不直接改别人的代码。
{{/each}}

- **协作形态**:{{collaborators}}
- (单人 + 多 agent 时)主窗口 = 你;scope agent = 派出去的实现者。**派单 prompt 必须自包含**——subagent 看不到主窗口上下文。

## 不撞车规则

- 每个 scope 独立工作;并行任务并行派,不串行。
- 跨端改动先在 `docs/contracts/` 对齐字段/接口,再各自实现。
- **永不阻塞**:卡住了主动汇报进度,不默默卡死别人。

## 红线

- 不擅自 commit / push(需 owner 明确指令)。
- 不跑 writing-plans;spec 验收要硬 + code review 必留(见各端 CLAUDE.md)。
- 治理过重要主动喊停——右尺寸 > 全面。

<!-- 注:worktree 物理隔离是可选项,仅当多 agent 真的频繁本地冲突时才上;否则别加,属过度治理。 -->
