---
name: sop-init
description: 为项目搭"右尺寸"的开发 SOP 骨架——固定公约 + 按本项目参数化的角色/结构。新项目起手或给老项目补 SOP 时用。先定档(T0/T1/T2)+ 端(ends)+ 协作人,生成 CLAUDE.md(含「Agent 工作约束」块)、docs/decisions、以及 T2 的角色/契约/协作文档。核心是右尺寸:宁可不足不要过度,会拒绝过度治理。
---

# sop-init

为项目搭开发 SOP 骨架。

**配套文件位置(本技能经软链装在 `~/.claude/skills/` 时仍按此绝对路径找)**:`SOP_HOME = /Users/sunchongsheng/code/sop-better/`。下文凡 `STANDARD.md` 指 `$SOP_HOME/STANDARD.md`,`templates/` 指 `$SOP_HOME/templates/`。

唯一真相源是 `STANDARD.md`,**先读它**,本技能只是执行流程,规则以 STANDARD 为准。

## 铁律(来自 STANDARD §1,任何档都生效)

- 人=要什么(目标/约束/验收)+ 否决权;AI=怎么做。不从 L0 翻成盲盒。
- 重瞄 brainstorming:只梳意图,架构 AI 提案、人评审。
- 跳 writing-plans,但 spec 验收要硬 + code review 必留。
- **🧱 客观顶嘴(协议化 · STANDARD §1.4)**:不许讨好;按「顶嘴协议」6 闸办(空夸违规 / 批准前先出反方 / 顶住一轮…)。用户选档超过实际需要**必须顶回去**,别"全都给建上"。
- **🗣 说人话(STANDARD §1.7)**:对人输出结论先行、少黑话、多打比方、短句分点;术语首次出现先解释。与顶嘴合为"沟通约束"。
- 右尺寸:重量由 `风险 × 验证成本` 定。

## 流程

1. **读 `STANDARD.md`**(§3 两轴 S×C + §4 约束块 + §2 参数)。
2. **定参数**(能从现有代码推断就推断:扫语言/目录/端数;推断不了就**一次问清**,别逐条挤牙膏):
   - `端数 S`:S0 单脚本 / S1 单端 / S2 多端 → 定契约
   - `协作结构 C`:C0 单人 / C1 双角色·小团队(issue+角色) / C2 多端多 agent → 定协作机器
   - `ends[]`:admin / backend / frontend / mobile / crawler …
   - `risk`:可逆低风险 / 线上不可逆
3. **右尺寸校验(顶嘴在这一步发力)**:按 `风险 × 验证成本` 反问"这档是不是太重了?"。若选档 > 实际需要,**明确建议降档并说理由**,不附和。
4. **按 STANDARD §3 两轴生成**(用 `templates/` 里的块,占位符按参数替换):
   - **总是**:`README.md`、`.gitignore`;涉密钥则 `.env.example`(密钥**绝不**进 git);**沟通约束块(顶嘴 + 说人话)**(S0·C0 附 README 末尾,否则放 CLAUDE.md)
   - **凡非 S0·C0**:`CLAUDE.md`(嵌 `agent-constraints.md` 的**标准块**[S0·C0 用极简块;S2 再加多端追加块] + **沟通约束块**;**块头 `{{SC}}` 填实际两轴如 `S1·C1`,严禁写"单人/T1"等会跟协作结构矛盾的词**)+ `docs/decisions/`(adr 模板 + `0001` 样例)+ 单一真相源声明
   - **C≥C1(有协作)**:issue 模板 + label 状态机 + 角色命令 + `collaboration.md`(`templates/collaboration.md`,按角色/端实例化)+ 状态标记 ✅🚧⏸️⬜
   - **S=S2(多端)**:`docs/contracts/README.md`(`templates/contracts-README.md`)+ 按 `ends[]` 的端角色
5. **不生成超过两轴的东西**(右尺寸硬约束):S1 不建 contracts;C0 不建 issue/角色机器;C1 不建跨端契约/worktree。
6. **报告**:列生成了什么 + 一句"为什么这档够用、没多给"。让 owner **扫 `CLAUDE.md` + 目录树**即可验收(便宜验证)。

## 禁止

- 写 writing-plans 式重型实施计划。**生成的任何文档(含协作/流程 SOP)都不许把开发流程建在 writing-plans 上**(违背 §1.3)——开发侧直接实现 + code review。
- **不许凭对用户其它项目(taoxi-geo 等)的记忆另写协作/流程文档**;协作文档一律用 `templates/`,否则会把旧习惯(尤其 writing-plans)偷带进来、自相矛盾(exp-002 根因)。
- 讨好式"为了全面两套都给你建上"——违反右尺寸 + 顶嘴铁律。
- 把密钥/凭据写进任何进 git 的文件。
