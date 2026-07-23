---
name: sop-init
description: Use when 用户要给项目新建或增量补开发 SOP,需要按端数/协作人数/风险决定右尺寸结构,生成 Claude CLAUDE.md。
---

# sop-init

为项目搭开发 SOP 骨架。

**配套文件位置(本技能经软链装在 `~/.claude/skills/` 时仍按此绝对路径找)**:`SOP_HOME = /Users/sunchongsheng/code/sop-better/`。下文凡 `STANDARD.md` 指 `$SOP_HOME/STANDARD.md`,`master/` 指 `$SOP_HOME/master/`(母本,按触发分层;槽与收编规则见 `master/SLOTS.md`)。

唯一真相源是 `STANDARD.md`,**先读它**,本技能只是执行流程,规则以 STANDARD 为准。

## 铁律

**全部公约在 `STANDARD.md` §1(三分法 + 定义/建议/决策 + 各条 · 先读它、以它为准)**——本技能**不重抄**(重抄必漂移)。这里只列本技能**执行时**特有的硬约束:

- **右尺寸校验**:用户要的结构 > 实际有的端/人/协作需求 → 驳回、别预建(§1.6 + §1.9「不假民主」)。真一次性脚本 → 建议别 init(出范围);低风险小项目也不强制 issue / 独立分支 / PR / worktree。
- **增量幂等**:已有 SOP 只补缺的那层、不砸已有(§3.5)。
- 其余见下方「流程」「禁止」。

## 流程

1. **读 `STANDARD.md`**(§3 流程与结构按现实触发 + §3.5 演进 + §4 约束块 + §2 参数)。**判新建 or 加层**:目标已有 `CLAUDE.md` / docs(跑过)→ **增量模式**:测当前有几个端 / 几个人,**只补缺的那一层、绝不覆盖已有**,只为新加的端/人生成;只有 `AGENTS.md` 没有 `CLAUDE.md` = 旧格式(Codex 残留),先迁移成 `CLAUDE.md` 并删除 `AGENTS.md`,不保留桥接;加端若要返工(如接口写死在代码里)**标出来让 owner 抠,不自动重构**。
2. **定参数**(能从现有代码推断就推断:扫语言/目录/端数;推断不了就**一次问清**,别逐条挤牙膏):
   - `ends[]`:几个端?admin / backend / frontend / mobile / crawler … → **≥2 个端触发多端结构**(契约 + 每端身份文档);单端不建;worktree + 并行防撞只在真并行多 agent 时加(**与端数无关**,单端并行也触发)
   - `collaborators`:几个人?单人 / 人+多 agent / 多人 → **≥2 个不同的人触发协作 doc**;单人不建。另判断是否真有跨会话跟踪 / 角色交接 / 远端交付需求,决定是否需要 Issue / PR 指南
   - `risk`:可逆低风险 / 线上不可逆 → 默认撒手档、review 严格度
   - `house_style[]`:既有技术规范 / 参考项目,按端(如 "Go 后端 → go_dispatch_backend 仓本体")。指参照**本体**(纪律与理由见 STANDARD §2 该行 · exp-008);可给推断候选但**必须 owner 确认、不静默凭记忆填**。答"无" → 约束块占位写"无参照"(立栈走确认闸 · STANDARD §1.9)
3. **右尺寸校验(反驳在这一步发力)**:反问"这项目真需要一份 SOP 吗?多端 / 多人结构是真有、还是在预建?任务是否真需要 Issue / PR / worktree?"。真一次性脚本 → 建议别 init(出范围);凭空的多端 / 多人结构或流程动作 → **明确建议砍掉并说理由**,不附和。
4. **生成(copy `master/` 的层 + 填槽 + 剪掉不触发的层 · 先读 `master/SLOTS.md`):** **每个 copy 出来的文件先删顶部 `<!-- … -->` 元注释**(那是给 `$sop-init` 的槽 / 触发说明、不进产物),再填槽——否则 `{{proj}}` 等会连头注里一起被填成乱码。
   - **agent 指令文件落点**:`master/base/CLAUDE.md` 是母本内容源,所有项目都落 `CLAUDE.md`。
   - **总是(所有项目,含单人单端)**:`README.md`、`.gitignore`;涉密钥则 `.env.example`(密钥**绝不**进 git);copy `master/base/` → 项目根 agent 指令文件(填 `{{proj}}`〔别写档位编号 / "单人"窄化词〕/ `{{house_style}}` / `{{default_altitude}}` / `{{risk_gate_items}}` / `{{prod_infra_note}}`)+ `docs/decisions/`(adr-template + 你补 `0001` 样例 + **升级触发 ADR**:"加第 2 个端 → 补契约/按端操作台;真并行多 agent → 补 worktree + 并行防撞(与端数无关);多端且并行 → 再补协调段;加第 2 个人 → 补协作 doc")+ 单一真相源声明。`docs/project/issue-pr-workflow.md` 可作为“触发时才用”的薄指南生成,但**不要**无条件创建 Issue 模板、label 状态机或把 Issue / PR 写成每个任务前置。
   - **有第 2 个人(≥2 个不同的人)才加**:copy `master/layer-collaborators/collaboration.md` → `docs/project/collaboration.md`(业务↔开发 handoff:起需求 / 收口标准 / 交棒)。单人 → 不建(角色变焦在 base 公约里恒定)。
   - **有第 2 个端(≥2 端)才加**:copy `master/layer-multiend/` → `docs/contracts/`(`contracts-README` + `multiend-contracts`:契约握手 + firmness 三级 + req-doc 语义级/实施层边界)+ **按 `ends[]` 给每端用 `end-role-claude` 落 `<端>/CLAUDE.md`**(端级操作台:身份 + scope/取活 + Step 3/Step 4 端速查 + 评论里程碑 + 本端 local〔技术栈/常读文件/实施层词汇〕+ 指向根 agent 指令文件工作约束块)+ **用 `multiend-constraints-block` 填根 `CLAUDE.md` 的 `{{multiend_constraints}}` 槽**(单端时该槽整行删 · 不中段动刀)。**靠 Claude Code 自动加载 cwd 所在的 `CLAUDE.md` = 进端即定身份**;纪律:**端文件指针不复述通用红线**(复述=漂移源 · STANDARD §1.7),但不能薄到只剩身份/常读文件桥接。
   - **owner 确认"真并行多 agent"才加(与端数无关,单端也触发,不默认)**:copy `worktree-isolation.md`(物理层:布局/HEAD race trap/创建+清理/起手按-ref-验/反转条件)+ `parallel-agents.md`(逻辑层:起手报告/消息总线/取活认领/escalate/close-keyword)→ `docs/project/`(两份互指,头部各写"我只治哪层、另一层看隔壁";根 `CLAUDE.md`「其它情景规则」行自带并行指针,增量补时核指针在)+ `adr-template` 记 worktree ADR;**若也多端**,再把 `coordination-multiend.md` 追加进 `docs/project/collaboration.md`(角色表 + 6+1 + 按端 scope 隔离;无第 2 个人则单独成 collaboration.md)。串行 / 单 agent → 不发(过度治理)。
   - **三触发自检(治 exp-012 挂错闸 · 必做)**:生成后点名验——单人·多端·并行 = base+multiend+parallel,**不要** collaborators;2 人·单端·串行 = base+collaborators,不要后两层;单人·多端·串行 = base+multiend,**不背** worktree;单人·单端·真并行 = base+parallel(worktree+parallel-agents),**不背** multiend / collaborators。**按触发分流、别拿"目录相邻"当门。**
   - **escalate 端-agnostic(exp-009)**:§1.9 carve-out 指针在 `layer-collaborators`(→业务方)与 `layer-parallel-agents/parallel-agents`(上交对的人:多端给 coord / 双角色给业务方 / 否则给 owner)两层都在,别只挂并行层。
5. **没有的端/人/流程不预建**(右尺寸硬约束):单端不建 contracts/按端操作台;单人不建协作 doc;无真并行 / 隔离需求不建 worktree;无跨会话跟踪 / 交接需求不建 Issue 机器;无远端交付 / 评审需求不强制分支 + PR;真一次性脚本压根不跑 `$sop-init`(出范围)。
6. **报告**:列生成了什么 + 一句"为什么这档够用、没多给"。让 owner **扫 agent 指令文件 + 目录树**即可验收(便宜验证)。

## 禁止

- 预写重型实施计划(把完整实现方案写死再执行)。**生成的任何文档(含协作/流程 SOP)都不许把开发流程建在重型实施计划上**(违背 §1.2)——开发侧直接实现 + code review。
- **不许凭对用户其它项目(taoxi-geo 等)的记忆另写协作/流程文档**;协作文档一律 copy `master/` 的层,否则会把旧习惯(尤其重型实施计划)偷带进来、自相矛盾(exp-002 根因)。
- **增量模式整份重 copy 已有层 = 砸 owner 已填的槽 / 正当宽放**:增量只对**新增的层**整份 copy,已有层只块/槽级补缺(§3.5)。
- 讨好式"为了全面两套都给你建上"——违反右尺寸 + 反驳铁律。
- 生成或保留 `AGENTS.md` 桥接文件——本工具已回 Claude Code 规范,旧 `AGENTS.md` 是待迁移的 Codex 残留。
- **预建还没有的端 / 人 / 角色的结构**(为想象建仪式 = 最坏的过度治理;真加了才补 · STANDARD §3.5)。
- 因为 agent 能自动执行,就给所有小改强制 Issue / 独立分支 / PR / worktree 或额外本地编排工具。
- **增量模式下覆盖/重建已有结构**(只补缺的,不砸 owner 已有的)。
- 把密钥/凭据写进任何进 git 的文件。
