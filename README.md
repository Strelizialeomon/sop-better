# sop-better

一个 **Claude 技能仓**:为项目**搭、查、改**它的"开发 SOP",并且**按项目该有的重量分档**——既补不足,更砍过度。

名字直说:make SOP better,一个**不断迭代**的开发 SOP 工具。形态照搬作者自己的 `google-design-skill`(`init / criticize / format`),换领域。

> 完整设计见 [`docs/superpowers/specs/2026-06-05-sop-better-design.md`](docs/superpowers/specs/2026-06-05-sop-better-design.md)。

---

## 两个命令(产品)

| 命令 | 干啥 |
|---|---|
| **/sop-init** | 选档 → 给项目搭三维 SOP 骨架(结构文件 + 角色划分 + CLAUDE.md「Agent 工作约束」块) |
| **/sop-audit** | 给现有项目"体检",查"不合理"(**①过度治理 头号 ②档位错配 ③反驳缺失 ④结构缺失/凭据失真**)→ 出报告;**你说"改"它就接着修 + 开 PR**(默认只报告) |

**当前进度**:`/sop-init`、`/sop-audit` 都已建好装上。
> 没有单独的 `/sop-improve`:audit 你点头就改,"改"的活它包了——不为还没出现的需求养第三个命令(右尺寸)。

---

## 它为什么这么设计(一句话诊断)

作者用 AI 没爽感、没飞跃——因为只交了**打字**,没交**认知**;而现有那套高水位 SOP **全是控制机制**,**仪式吃掉了 AI 给的杠杆**。所以这工具的灵魂是两件事:

1. **把 SOP 右尺寸化**——对一个人指挥 AI,最大的"不合理"通常是**过度治理**,不是治理不足。
2. **把"人撒手、AI 接管"写进流程**——人管"要什么",AI 管"怎么做"。

---

## 核心模型:三维 SOP × 两轴档位

一份 SOP 是**三个维度**,且都随档位伸缩:

| 维度 | 管什么 |
|---|---|
| **结构** | 建哪些 docs / 角色划分 |
| **流程** | 用哪些技能、跳哪些(brainstorm✓ / writing-plans✗ / code-review✓) |
| **人机分工** | 人 = 要什么(目标/约束/验收)+ 否决权;AI = 怎么做(架构/设计/拆解/实现) |

**档位 = 两根独立的轴**(权威定义见 `STANDARD.md` §3,此处不重抄):**契约看 S(端数),协作机器看 C(协作结构)**。T0/T1/T2 只是常见 (S,C) 组合的简称——旧的一维标量档位已被 exp-002 证伪、废弃。

**贯穿线**:SOP 档与撒手梯子(L0–L4)是同一原理的两把尺,都由 `风险 × 验证成本` 决定该多重。

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
├── STANDARD.md                  # 两轴×三维的权威标准(audit 对照尺)
├── skills/{sop-init,sop-audit}/ # 命令实现(都已建 + 软链进 ~/.claude/skills)
├── templates/                   # 各档结构 + 约束块 + issue/PR 工作流模板
├── docs/superpowers/specs/      # 设计 spec
├── PLAYBOOK.md                  # 狗粮日志:撒手护栏
└── experiments/                 # 狗粮日志:exp-NNN
```
