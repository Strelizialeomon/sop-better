---
name: sop-init
description: 为项目搭"右尺寸"的开发 SOP 骨架——固定公约 + 按本项目参数化的角色/结构。新项目起手或给老项目补 SOP 时用。先定:几个端(ends)+ 几个人(协作人)+ 风险,生成 CLAUDE.md(含「Agent 工作约束」块)、docs/decisions、以及多端的角色/契约/协作文档。核心是右尺寸:宁可不足不要过度,会拒绝过度治理。
---

# sop-init

为项目搭开发 SOP 骨架。

**配套文件位置(本技能经软链装在 `~/.claude/skills/` 时仍按此绝对路径找)**:`SOP_HOME = /Users/sunchongsheng/code/sop-better/`。下文凡 `STANDARD.md` 指 `$SOP_HOME/STANDARD.md`,`templates/` 指 `$SOP_HOME/templates/`。

唯一真相源是 `STANDARD.md`,**先读它**,本技能只是执行流程,规则以 STANDARD 为准。

## 铁律

**全部公约在 `STANDARD.md` §1(三分法 + 定义/建议/决策 + 各条 · 先读它、以它为准)**——本技能**不重抄**(重抄必漂移)。这里只列本技能**执行时**特有的硬约束:

- **右尺寸校验**:用户要的结构 > 实际有的端/人 → 驳回、别预建(§1.5 + §1.9「不假民主」)。真一次性脚本 → 建议别 init(出范围)。
- **增量幂等**:已有 SOP 只补缺的那层、不砸已有(§3.5)。
- 其余见下方「流程」「禁止」。

## 流程

1. **读 `STANDARD.md`**(§3 一条流程+结构按现实长 + §3.5 演进 + §4 约束块 + §2 参数)。**判新建 or 加层**:目标已有 CLAUDE.md/docs(跑过)→ **增量模式**:测当前有几个端 / 几个人,**只补缺的那一层、绝不覆盖已有**,只为新加的端/人生成;加端若要返工(如接口写死在代码里)**标出来让 owner 抠,不自动重构**。
2. **定参数**(能从现有代码推断就推断:扫语言/目录/端数;推断不了就**一次问清**,别逐条挤牙膏):
   - `ends[]`:几个端?admin / backend / frontend / mobile / crawler … → **≥2 个端触发多端结构**(契约 + 每端身份文档 + worktree);单端不建
   - `collaborators`:几个人?单人 / 人+多 agent / 多人 → **≥2 个不同的人触发协作 doc**;单人不建(issue+PR 不看这条、归恒定流程)
   - `risk`:可逆低风险 / 线上不可逆 → 默认撒手档、review 严格度
   - `house_style[]`:既有技术规范 / 参考项目,按端(如 "Go 后端 → go_dispatch_backend 仓本体")。指参照**本体**(纪律与理由见 STANDARD §2 该行 · exp-008);可给推断候选但**必须 owner 确认、不静默凭记忆填**。答"无" → 约束块占位写"无参照"(立栈走确认闸 · STANDARD §1.9)
3. **右尺寸校验(反驳在这一步发力)**:反问"这项目真需要一份 SOP 吗?多端 / 多人结构是真有、还是在预建?"。真一次性脚本 → 建议别 init(出范围);凭空的多端 / 多人结构 → **明确建议砍掉并说理由**,不附和。
4. **生成(一条流程 + 结构按现实长 · 用 `templates/` 里的块,占位符按参数替换):**
   - **总是(所有跑 sop-init 的项目,含单人单端)**:`README.md`、`.gitignore`;涉密钥则 `.env.example`(密钥**绝不**进 git);`CLAUDE.md`(嵌 `agent-constraints.md` 的**标准块 + 沟通约束块**;**块头 `{{proj}}` 填项目一句话描述,别写档位编号 / "单人"等窄化词**)+ `docs/decisions/`(adr 模板 + `0001` 样例 + **升级触发条件 ADR**:"加第 2 个端 → 补契约/按端文档/worktree;加第 2 个人 → 补协作 doc")+ 单一真相源声明 + **issue/PR 工作流(`templates/issue-pr-workflow.md` —— 全生命周期 + 凭据保真)+ issue 模板 + label 状态机(评估中/开发中/已完成 + `待业务确认` 阻塞 flag)+ 状态标记 ✅🚧⏸️⬜**(issue+PR 恒定、人人都走含单人)
   - **有第 2 个人(≥2 个不同的人)才加**:`templates/collaboration.md` 的**基线双角色段**(业务↔开发 handoff:起需求 / 收口标准 / 交棒)+ 角色命令。单人 → 不建(角色变焦在公约里恒定)。
   - **有第 2 个端(≥2 端)才加**:`docs/contracts/README.md`(`templates/contracts-README.md`)+ **`templates/multiend-contracts.md`**(契约握手 + firmness 三级 + req-doc 语义级/实施层边界)+ **按 `ends[]` 给每端生成 `templates/end-role-claude.md`** → 落 `<end_dir>/CLAUDE.md`(端级身份文档:身份 + 本端 local〔scope/技术栈/常读文件/实施层词汇〕+ 指向项目根 CLAUDE.md 工作约束块)。**靠 harness 自动加载 cwd 最近 CLAUDE.md = 进端即定身份**(治"多端各端不知自己是谁");纪律:**端文件指针不复述通用红线**(复述=漂移源 · STANDARD §1.8)。
     - **且 owner 确认"真并行多 agent"才加**(多端里的可选项,不默认):worktree —— **`templates/worktree-isolation.md`**(布局/HEAD race trap/setup+维护/起手按-ref-验/反转条件)+ `adr-template.md` 记 worktree ADR;**外加 `collaboration.md` 的「多端多 agent 追加段」**(角色 + 6+1 骨架 + 消息总线 + scope 隔离 + coordination)。串行 / 单 agent → 都不发(过度治理)。
   - **注**:`collaboration.md` 在「有第 2 个人」或「多端真并行多 agent」**任一**命中时生成,带对应段(可只有其一——如 taoxi-geo 单人多端多 agent 只有追加段)。
5. **没有的端/人不预建**(右尺寸硬约束):单端不建 contracts/按端文档/worktree;单人不建协作 doc/角色命令;真一次性脚本压根不跑 sop-init(出范围)。
6. **报告**:列生成了什么 + 一句"为什么这档够用、没多给"。让 owner **扫 `CLAUDE.md` + 目录树**即可验收(便宜验证)。

## 禁止

- 写 writing-plans 式重型实施计划。**生成的任何文档(含协作/流程 SOP)都不许把开发流程建在 writing-plans 上**(违背 §1.3)——开发侧直接实现 + code review。
- **不许凭对用户其它项目(taoxi-geo 等)的记忆另写协作/流程文档**;协作文档一律用 `templates/`,否则会把旧习惯(尤其 writing-plans)偷带进来、自相矛盾(exp-002 根因)。
- 讨好式"为了全面两套都给你建上"——违反右尺寸 + 反驳铁律。
- **预建还没有的端 / 人 / 角色的结构**(为想象建仪式 = 最坏的过度治理;真加了才补 · STANDARD §3.5)。
- **增量模式下覆盖/重建已有结构**(只补缺的,不砸 owner 已有的)。
- 把密钥/凭据写进任何进 git 的文件。
