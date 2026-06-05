# sop-better 设计 spec（v2）

- **日期**:2026-06-05
- **状态**:已与 owner 对齐(三维模型 + 先做 init,逐条 approve)
- **supersedes**:本 spec 取代同日 v1「撒手训练场」框架。v1 把整个 repo 当成"训练我工作模式"的日记本;v2 把它定成**一个会做 SOP 的工具**,训练场降级为"造这个工具时自己吃的狗粮"。
- **本 spec 由来**:用重瞄后的 brainstorming 产出——owner 梳理"要什么",架构由 AI 提案、owner 评审。未跑 writing-plans。

---

## 1. 这是什么

一个 **Claude 技能仓**:为项目**搭、查、改**它的"开发 SOP",并且**按项目该有的重量分档**——既补不足,更砍过度。形态照搬 owner 自己的 `google-design-skill`(`init / criticize / format` 三件套),换领域。

**不是**:静态规范文档、一刀切模板、给所有项目套同一套重型治理。

## 2. 为什么(诊断,压缩版)

owner 用 AI 没爽感、没飞跃——因为只交了**打字**,没交**认知**:设计/架构/取舍全自己扛。根因是习惯 + 没审视过的不信任 + "验证太贵所以只能提前控死"。更深一层:owner 现有那套高水位 SOP **全是控制机制**,为正确/可审计/多 agent 优化,**仪式吃掉了 AI 给的杠杆**。

→ 所以这工具的灵魂是:**把 SOP 右尺寸化 + 把"人该撒手、AI 该接管"的分工写进流程**。

## 3. 核心模型:三维 SOP × 分档

一份 SOP 不是一坨文档,是**三个维度**:

| 维度 | 管什么 |
|---|---|
| **结构** | 建哪些 docs / 角色划分(CLAUDE.md、docs/decisions、contracts、collaboration.md…) |
| **流程** | agent 用哪些技能、跳哪些(brainstorm✓ / writing-plans✗ / code-review✓…) |
| **人机分工** | 每步里谁拍板:**人 = 要什么(目标/约束/验收)+ 否决权;AI = 怎么做(架构/设计/拆解/实现)** |

三维都**随档位伸缩**:

| 档 | 结构 | 流程 | 人机分工 | 对标项目 |
|---|---|---|---|---|
| **T0 一次性脚本** | README + .env + .gitignore | 不 brainstorm,直接做 | 人给意图,AI 全包 | baozhang / jiandaoyun |
| **T1 单人认真** | + CLAUDE.md + docs/decisions(ADR)+ 单一真相源,直推 master | 轻 brainstorm(只梳意图)→ 建 → code review;**不跑 writing-plans** | 人=要什么+评审;AI=架构+实现 | llm_auto_report |
| **T2 多端/多 agent** | + 角色(coordination + scope agent)+ docs/contracts + collaboration.md + worktree + 状态标记 | 全套 + 跨端契约 handshake | 同上,跨端契约 freeze 需人确认 | taoxi-geo / go_dispatch / xreal |

**贯穿线**:这把"SOP 档(T0–T2)"与撒手梯子(L0–L4)是**同一原理的两把刻度尺**,都由 `风险 × 验证成本` 决定该多重。sop-better 就是把这条护栏做成工具。

## 4. 两个命令(原计划三件套,后收敛 · 见下注)

| 命令 | 干啥 | 诉求 |
|---|---|---|
| **/sop-init** | 选档 → 生成三维 SOP 骨架(结构文件 + CLAUDE.md「Agent 工作约束」块 + 角色划分) | 快速建骨架 |
| **/sop-audit** | 给现有项目体检,对照两轴查"不合理"(过度治理 头号 / 档错配 / 顶嘴缺 / 结构缺·凭据失真),出双轨报告;**默认只报告,owner 说"改 / go"才走 issue/PR 工作流修 + 开 PR** | 检查 + 按需改 |

> **没有独立 `/sop-improve`**:最初照搬 google-design-skill 设了 init/audit/improve 三件套,但 audit 选了"owner 点头就改",改的活被它包了 → improve 与它撞车,砍掉(右尺寸,exp-003)。

## 5. 关键原则(逐条都是 owner 拍过的)

1. **audit 头号查"太重",不是"缺"**:对一个人指挥 AI,过度治理(coordination/scope label/回溯 req doc/audit 补口)比治理不足更常见、更伤杠杆。审查器必须敢喊"你用不着这么重"。
2. **重瞄 brainstorming**:继续用它梳理思路,但**只梳"要什么"**;技术架构由 AI 提案,人**评审不作者**。默认 brainstorming 会把人脑内的代码一点点抽出来——那正是病根,本工具反其道。
3. **砍 writing-plans 的耦合补偿**:plan 是验证检查点,砍了,重量压到两处且必须同时加硬——**spec 验收标准要够硬**(AI 才知道"做完")+ **code review 铁定保留**(撒手后唯一安全网)。「plan 越轻 → 验收越硬 + review 越严」。
4. **不从 L0 翻成盲盒**:"不让我定架构"= AI 提案、人评审否决,**不是**人退场。人不**写**架构,但看得见、能毙、高风险能跳进去。定规矩这种高 stakes 事,人该主导。

## 6. /sop-init 的标志性产物:CLAUDE.md「Agent 工作约束」块

每档生成一段(T1 示例):

```markdown
## Agent 工作约束(本项目 · T1 档)
- 梳理思路:用 brainstorming,但只梳"要什么"(目标/约束/验收)。
  技术架构由 agent 提案,我评审 + 否决,不亲自设计。
- 不跑 writing-plans:spec 通过后直接实现(spec 必含可检验的验收标准)。
- 必跑 code review:撒手后的唯一安全网,不可省。
- 默认撒手档 = L2;不可逆/高风险改动才升回我主导。
```

## 7. 仓库结构(v2)

```
sop-better/
├── README.md                    # 工具是什么 + 三维模型 + 怎么用 + 怎么造(狗粮法)
├── docs/superpowers/specs/      # 本 spec
├── skills/                      # ← 新增:三件套技能实现(init 先行)
│   └── sop-init/
├── templates/                   # ← 新增:各档的结构 + CLAUDE.md 约束块模板
├── STANDARD.md                  # ← 新增:三维 × 分档的权威标准(audit 的对照尺)
├── PLAYBOOK.md                  # 狗粮日志:撒手实验沉淀的护栏(降级为"造这工具的方法记录")
└── experiments/                 # 狗粮日志:exp-NNN
```

## 8. 范围与先后

- **先做 `/sop-init`**:自包含、立刻能用,且它生成的就是 audit 要对照的"标准";`/sop-audit` 第二步(已建)。**不做独立 /sop-improve**——audit 经 owner go 链改即可。
- **YAGNI**:不上自动化平台;**不养独立 /sop-improve**(audit 经 go 链改);不顺手治别的项目。
- **安全待办(挂名另开)**:`py-script`/`jiandaoyun`/`vpn-tutorial/CREDENTIALS.md`/`llm_auto_report` 硬编码密钥——真实风险,与本项目无关。

## 9. 狗粮注(自举)

`experiments/` + `PLAYBOOK.md` 不删,降级为**造 sop-better 时自己吃狗粮的记录**。**exp-001(设计本项目)就是"重瞄 brainstorming + 跳 writing-plans"工作流的活样板**——证明这套流程跑得通,我们不是在固化没验证过的东西。造 `/sop-init` = 下一次撒手实验(exp-002,目标 L3)。

## 10. 成功标准

- `/sop-init` 能为一个新项目按选定档,几分钟生成可用的三维 SOP 骨架(含 Agent 约束块);
- 生成物**右尺寸**——T1 项目不会被塞进 T2 的重型仪式;
- 后续 `/sop-audit` 能对一个真实项目(如某个 taoxi-* )喊出"这里过度治理 / 这里档错了";
- owner 用它起的新项目,默认撒手档从 L0 上移。
