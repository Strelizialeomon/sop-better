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
- 右尺寸:重量由 `风险 × 验证成本` 定。

## 流程

1. **读 `STANDARD.md`**(§3 矩阵 + §4 约束块 + §2 参数)。
2. **定参数**(能从现有代码推断就推断:扫语言/目录/端数;推断不了就**一次问清**,别逐条挤牙膏):
   - `tier`:T0 一次性脚本 / T1 单人认真 / T2 多端·多 agent
   - `ends[]`:admin / backend / frontend / mobile / crawler …
   - `collaborators`:单人 / 人+多 agent / 多人
   - `risk`:可逆低风险 / 线上不可逆
3. **右尺寸校验(顶嘴在这一步发力)**:按 `风险 × 验证成本` 反问"这档是不是太重了?"。若选档 > 实际需要,**明确建议降档并说理由**,不附和。
4. **按 STANDARD §3 矩阵生成**(用 `templates/` 里的块,占位符按参数替换):
   - **所有档**:`README.md`、`.gitignore`;涉密钥则 `.env.example`(密钥**绝不**进 git);**「顶嘴协议」块**(T0 附在 README 末尾,T1+ 放进 CLAUDE.md)
   - **T1+**:`CLAUDE.md`(嵌 `templates/agent-constraints.md` **对应档块 + 顶嘴协议块**)+ `docs/decisions/`(`templates/adr-template.md` + `0001` 样例)+ 单一真相源声明
   - **T2**:按 `ends[]` 实例化**角色划分** + `docs/project/collaboration.md`(`templates/collaboration.md`)+ `docs/contracts/README.md`(`templates/contracts-README.md`)+ 状态标记约定(✅🚧⏸️⬜)
5. **不生成超过该档的东西**(右尺寸硬约束):T1 不出现 coordination/scope/contracts/worktree。
6. **报告**:列生成了什么 + 一句"为什么这档够用、没多给"。让 owner **扫 `CLAUDE.md` + 目录树**即可验收(便宜验证)。

## 禁止

- 写 writing-plans 式重型实施计划。
- 讨好式"为了全面两套都给你建上"——违反右尺寸 + 顶嘴铁律。
- 把密钥/凭据写进任何进 git 的文件。
