<!-- master/layer-parallel-agents/coordination-multiend.md —— 触发:多端 且 真并行多 agent(两个触发都命中才发;单端并行只发 parallel-agents.md)。
     $sop-init 把这段追加进 docs/project/collaboration.md:若也有第 2 个人,接在 layer-collaborators 双角色段之后;单人多端多 agent 则单独成 collaboration.md。
     端-agnostic 的防撞约定(起手报告 / 消息总线 / 取活认领 / escalate / 遇阻处置 / close-keyword)在 parallel-agents.md,本文不复述——本文只装"端"的事。
     拆自原 coordination.md(exp-047):那份一半端-agnostic 一半按端,单端并行项目整发过度、整剪漏防撞。占位符:{{ends}} -->

# 多端多 agent 协调(本项目)

> 单端并行不用这份(那是 `parallel-agents.md` 的事);多端各端一个 scope agent + 一个 coordination(产品方向)+ owner 时才加(蒸馏自 taoxi-geo 829 行 SOP)。

## 角色(多端)

> 每端的**身份 + 边界**写在各端 `<端目录>/CLAUDE.md`(端级文档 · `$sop-init` 按 `ends[]` 生成 · Claude Code 自动加载)。此处只给跨端协作骨架,**不复述各端边界**(单一真相源 · STANDARD §1.7)。

| 角色 | scope | 干啥 |
|---|---|---|
| Coordination | docs | 出"在做什么" + 起跨端 req doc · **不出契约、不写端代码** |
| 各端 scope agent | {{ends}} | 端内代码 + 端内 spec/ADR + 自决实施 · 遇不合理自己解决 + 评论 · **边界见各端 agent 指令文件** |
| Owner | — | 提需求 + 审 req + 联调 + merge · **不当通讯人** |

**scope agent ≠ 执行 PM 派的细 task;= 高权限程序员,收到方向后自决实施。**

> **身份靠"进哪个端的目录"自动定**:Claude Code 自动加载 cwd 最近的 `CLAUDE.md`——在任务 worktree 里 cd 进 `<端目录>/` 就自动是该端 scope agent;**主仓根 = coordination;任务 worktree 根 = 端身份未定、cd 进端目录才定**(worktree 根上的是任务 agent,不是 coordination)。**端身份 ⊥ worktree**(worktree 按 issue/任务切、非按端)。不靠声明、不靠猜(细则见 `worktree-isolation.md`)。
>
> **错座位护栏**:端内活(端内 spec / 端代码)归 scope agent、在任务 worktree 的该端目录产出;coordination(主仓)只产**跨端 req doc**。**救场**——spec 已误产在主仓:别"释放分支",按 req-doc 交接走 `push → doc PR(Refs)→ owner merge 进 master → scope agent 新建任务 worktree 从 origin/master 另切实施分支`。

## 6+1 流程骨架(轻 · 无硬 gate)

1. **起需求**:跨端 → coord 起 req doc;单端上下文明确 → 该 scope agent 自起(不必经 coord 中转)。req doc 写"做什么 + 怎么算对 + 主流程 + 形态 + 跨端换什么数据",**不写实施层**(见 `multiend-contracts.md`)。
2. **取活**:scope label 有 open issue 即"待干"(认领纪律见 `parallel-agents.md`)。
3. **细化**:scope agent 跟 owner 定端内方案 → 端内 spec doc → 自检 + 评论 announce(不再单独整体批)。
4. **开发**:切 `feat/issue-N` 分支(**名字钉死,不自决**——分支名就是抢坑闸,加 slug / 换前缀即失效,见 `parallel-agents.md` 约定 3);实施 + 进展 / 变更写 issue 评论;push + PR(默认 `Refs #N`,最终收口才 `Closes #N`);**过新眼睛 review**(STANDARD §1.2)再示意 merge。
5. **联调**:owner 跑;单端 bug → 该 agent;跨端 mismatch → 评论拍板改哪端。
6. **收口**:别把"做完了"手抄进多个 doc(单一真相源 · STANDARD §1.7)。收工书挡按根 `CLAUDE.md`「取活与书挡」条,不复述。
- **+1 回改 req doc**(可选 · 有重大 deviation 才值)。

## 按端 scope 隔离

- 不动别端代码、不动别人起的 req doc / 契约(要改走 issue 评论提)。
- 识别到要改 req doc / 跨端(含 owner 当面加的扩范围)→ 按 `parallel-agents.md` 的 escalate 约定上交 **coord**;只"做不做 / 优先级"留 owner 拍。
