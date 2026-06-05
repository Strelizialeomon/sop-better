<!-- templates/collaboration.md —— 仅 T2。生成为 docs/project/collaboration.md。占位符:{{ends}} {{collaborators}} -->

# 协作约定(本项目)

> 本文件是"谁干什么、怎么不撞车"的真相源。跨端事实定义在 `docs/contracts/`,这里只管协作流。

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
