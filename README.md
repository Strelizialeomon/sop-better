# sop-better

一个 **Claude 技能仓**:为项目**搭、查、改**它的"开发 SOP",并且**按项目该有的重量右尺寸**——既补不足,更砍过度。

名字直说:make SOP better,一个**不断迭代**的开发 SOP 工具。形态照搬作者自己的 `google-design-skill`(`init / criticize / format`),换领域。

> 完整设计见 [`docs/superpowers/specs/2026-06-05-sop-better-design.md`](docs/superpowers/specs/2026-06-05-sop-better-design.md);档位模型简化(收成"一条流程+结构按现实长")见 [`2026-06-23-collapse-tiers-to-one-flow-design.md`](docs/superpowers/specs/2026-06-23-collapse-tiers-to-one-flow-design.md)(exp-012)。

---

## 两个命令(产品)

| 命令 | 干啥 |
|---|---|
| **/sop-init** | 给项目搭右尺寸 SOP 骨架:一条流程 + 按"几端/几人"长出的结构(结构文件 + 角色划分 + CLAUDE.md「Agent 工作约束」块) |
| **/sop-audit** | 给现有项目"体检",查"不合理"(**①过度治理 头号 ②结构错配 ③反驳缺失 ④结构缺失/凭据失真**)→ 出报告;**你说"改"它就接着修 + 开 PR**(默认只报告) |

**当前进度**:`/sop-init`、`/sop-audit` 都已建好装上。
> 没有单独的 `/sop-improve`:audit 你点头就改,"改"的活它包了——不为还没出现的需求养第三个命令(右尺寸)。

---

## 它为什么这么设计(一句话诊断)

作者用 AI 没爽感、没飞跃——因为只交了**打字**,没交**认知**;而现有那套高水位 SOP **全是控制机制**,**仪式吃掉了 AI 给的杠杆**。所以这工具的灵魂是两件事:

1. **把 SOP 右尺寸化**——对一个人指挥 AI,最大的"不合理"通常是**过度治理**,不是治理不足。
2. **把"人撒手、AI 接管"写进流程**——人管"要什么",AI 管"怎么做"。

---

## 核心模型:一条流程 + 结构按现实长

一份 SOP 里,**流程 + 人机分工恒定**(所有项目同一条),只有**结构**按项目实际长:

| 维度 | 管什么 | 变不变 |
|---|---|---|
| **流程** | 用哪些技能、跳哪些(brainstorm✓ / writing-plans✗ / code-review✓)+ issue+PR + 风险闸 | **恒定**(人人同一条) |
| **人机分工** | 人 = 要什么(目标/约束/验收)+ 否决权;AI = 怎么做(架构/设计/拆解/实现) | **恒定** |
| **结构** | 建哪些 docs / 角色划分 | **按现实长**(见下) |

**结构按两条正交触发长出来**(权威定义见 `STANDARD.md` §3,此处不重抄):**有第 2 个端 → 加契约/按端文档/worktree;有第 2 个人 → 加协作 doc;没有就别建**。旧的"两轴 9 宫格档位"(S0–S2 × C0–C2 / T0/T1/T2)已被 exp-012 收掉——它只是把这条生成规则预先枚举成了组合。

**贯穿线**:结构该多重、撒手梯子(L0–L4)该多高,都由 `风险 × 验证成本` 决定。

---

## 四条铁律

1. **audit 头号查"太重"**,不只是查"缺"。
2. **重瞄 brainstorming**:只梳"要什么",架构由 AI 提案、人评审不作者。
3. **砍 writing-plans 要补偿**:plan 越轻 → spec 验收越硬 + code review 越严(撒手后唯一安全网)。
4. **不从"永远 L0"翻成"永远盲盒"**:人不写架构,但看得见、能毙、高风险能跳进去。

---

## 怎么造它(狗粮法 · 自举)

sop-better **用它自己鼓吹的工作流来造**:重瞄的 brainstorming(人梳意图、AI 提架构)+ 跳 writing-plans + 留 code review。每造一块就是一次"撒手实验",记进 `experiments/`,把"什么能安全交给 AI"的护栏沉淀进 `PLAYBOOK.md`。

- `experiments/` —— exp-001(设计本项目,这套工作流的活样板)起的全部撒手实验日志;结晶沉淀在 `PLAYBOOK.md`。

撒手梯子、实验闭环、PLAYBOOK 护栏的完整说明,见 `PLAYBOOK.md` 与上面的 spec。

---

## 仓库结构

```
sop-better/
├── README.md
├── CLAUDE.md                    # 仓库家规(薄入口,纯指针)
├── STANDARD.md                  # 一条流程+结构按现实长的权威标准(audit 对照尺)
├── skills/{sop-init,sop-audit}/ # 命令实现(都已建 + 软链进 ~/.claude/skills)
├── master/                      # 母本:按触发分层(base + layer-collaborators/multiend/parallel-agents)的结构/约束/工作流
├── docs/superpowers/specs/      # 设计 spec
├── PLAYBOOK.md                  # 狗粮日志:撒手护栏
└── experiments/                 # 狗粮日志:exp-NNN
```
