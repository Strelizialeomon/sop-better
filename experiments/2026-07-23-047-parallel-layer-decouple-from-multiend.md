# exp-047 · 并行层与多端解耦:触发去绑端 + coordination 拆两份 + 原生 worktree 收编(xhs-analysis 实测回灌 · #25)

- **日期**:2026-07-23
- **触发**:issue #25——`$sop-init` 在 xhs-analysis(单端 · 单人 · 真并行多 agent)实测,并行层因"前提:已多端"被整份剪掉,多 agent 逻辑层防撞一条没给;owner 一句「你为什么没有写端和端之间如何协作」就问出洞,只好现补。根因在母本,不在该次执行。
- **决策**:按 #25 的 F1–F3 落——触发去绑端(F1)、coordination.md 拆两份(F2)、原生 `EnterWorktree` 收编为首选(F3)。

---

## 病灶一:并行层触发写成「已多端」,但它治的病(HEAD race / 逻辑撞车)跟端数无关

- 母本内部本就不自洽:STANDARD §3 口诀端-agnostic,§3 正文「这些端要…」与 SLOTS「前提:已多端」把它收窄回多端子选项。
- 实测证据:xhs-analysis 单端(从未有第 2 个端)照样撞出跨 PR 逻辑冲突(其 commit bfe47eb 修两处)。
- **反直觉结晶(已进 STANDARD 举例)**:单端并行反而更容易撞——多端"你只动你端的代码"就是天然边界,单端没有,所有 agent 在同一堆目录里刨。把并行层挂在多端门后,恰好把最需要它的项目挡在外面。

## 病灶二:coordination.md 是不可分割的一整块,一半端-agnostic 一半按端

`$sop-init` 面对「单端+真并行」只有两个都错的选择:整份发 = 装没有端的多端协调机器(过度治理 §5.1);整份剪 = 漏逻辑层防撞(结构缺失 §5.4)。本次实测选了整剪,被 owner 问出来。

## 改了什么

- **F2 拆分**(`master/layer-parallel-agents/`):
  - 新 `parallel-agents.md`(端-agnostic · 真并行即发):起手一句话报告 / issue 消息总线(含"方案变更必须回写 issue") / 取活先认领 / escalate carve-out / close-keyword + HEAD race 两坑 / worktree 选项。**蒸自 xhs-analysis 已合并实测版**(其 PR #2),非凭空写。
  - 新 `coordination-multiend.md`(多端 且 并行才发):角色表 / 身份靠 cwd + 错座位护栏 / 6+1 骨架 / 按端 scope 隔离;防撞约定全降为指向 parallel-agents.md 的指针。
  - 删 `coordination.md`;命名混淆一并治:**端↔端走 contracts,人↔人走 collaboration.md,agent↔agent 走 parallel-agents.md**(owner 实测中就把后两者混了)。
- **F1 触发去绑端**:SLOTS 层表、STANDARD §3 正文 + §3.5 开始最小/升级触发 ADR、sop-init SKILL(采集/生成/三触发自检补"单人·单端·并行"例)全部改为「真并行多 agent,与端数无关;多端且并行再加协调段」;STANDARD 触发举例补第 4 条(单人+单端+真并行)。
- **F3 原生收编**(`worktree-isolation.md`):优先原生 `EnterWorktree`(自动 `.claude/worktrees/<task>/` 建目录+分支+切会话;默认 baseRef=fresh 从 origin/<默认分支> 新建,正合"不猜基线";ExitWorktree remove 有拒删未提交改动闸)——**已按工具自述核实,非转述**。手动 `git worktree add` 降 fallback。**新增 caveat(#25 未提,本次核出)**:原生退出清理只管本会话原生建的 worktree,手动建 / 跨会话遗留(含 keep 下来的)仍走"合并即清"手动 checklist。反转条件表该行标记已执行。
- **去绑端顺带**:worktree-isolation / end-role 内按端表述加〔多端〕限定或普化("coordination 窗口"→"主仓窗口(多端时即 coordination)");残留引用清扫(end-role / layer-collaborators / STANDARD 6 处)。

## 反向验收 / 自审

- **净增行数(诚实报)**:layer-parallel-agents 目录 删 46(coordination.md)+ 新 31+48 = 净 **+33 行**;worktree-isolation 原生段 +6。超出"持平"——但这是 P1 结构错配的修复:多出的行主要是 xhs-analysis 实测蒸来的逻辑层防撞(取活认领 / 方案变更回写 issue 是旧版没有的护栏)+ 两份互指头注。以 audit 镜头自查:不是为想象预建,是为已实测发生的形态(单端并行)补缺。
- **承重墙逐条核**(exp-006):HEAD race、错座位救场、6+1、scope 隔离、escalate carve-out、close-keyword、起手 freshness 全部在位,只是换了住址;拆分无删义。
- **单一真相源**:防撞约定只住 parallel-agents.md,coordination-multiend 全指针;三触发正交自此名实相符(SLOTS 表不再有"前提:已多端")。

## 复验 / 未验证

- **正向验证已有一半**:xhs-analysis 就是"修正后触发"的手工预演(其结构 = base + worktree-isolation + parallel-agents,无 multiend),owner 未再问出洞——但那是人工补的,母本改后未重跑 `$sop-init` machine-path 复验。
- #25 建议的验证:下次 `$sop-init` 碰「单端+真并行」项目按新触发生成,看 owner 还问不问得出「端和端怎么协作」;并入 #15 集中 sweep。
- 原生 worktree 的 `.claude/worktrees/` 忽略状态未逐项目验(母本已写 check-ignore 前验,兜底)。
