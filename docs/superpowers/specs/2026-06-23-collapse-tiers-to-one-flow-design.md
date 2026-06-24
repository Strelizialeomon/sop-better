# 把「两轴 9 宫格档位」收成「一条流程 + 结构按现实长」

> 状态：待 owner review。走本仓工作流（STANDARD §1.3）：跳 writing-plans、spec 带**可检验验收**、build 后派**独立新眼睛 review**、记 `exp-012`。
> **本 spec 由来**：重瞄 brainstorming——owner 以**业务角色**描述"想要的效果"（概念干净），架构由 agent 提案、owner 评审。未跑 writing-plans。

---

## 1. 要什么（owner 定的效果，逐条钉死）

- **概念干净**是唯一诉求（owner 明确：不是嫌输出长、不是嫌起手盘问、不是嫌 STANDARD 难改——日常用着行，就是"分这么多档"这个**概念**膈应）。
- 心智模型收成：**一条流程（恒定）+ 结构按"多端 / 多人"长**。
- **输出参考 media-ops 一致**：生成物的丰度不缩；media-ops 那种 CLAUDE.md 照样产。
- **小项目 = media-ops 流程减 worktree**：单端小项目跑同一条流程，只是没 worktree / 没契约。
- **一次性脚本不跑 sop-init**（owner 拍板）：真跑一次就扔的，压根不 init——出了工具范围，不给"缩水档"。
- **单人也走 issue+PR**（owner 接受这点成本）：人数不再决定"要不要 issue"。

## 2. 核心洞察（为什么这次合并是干净的）

STANDARD §1 公约开头白纸黑字：「**跟项目无关，solo 脚本和四端产品都一样**」。即**整条流程**（人机分工 / brainstorming 变焦 / 跳 plans+spec 可检验+新眼睛 review / 反驳 6 闸 / 右尺寸 / 单一真相源 / 说人话 / 凭据保真 / 自决·永不阻塞·立栈闸·缺交付物反弹 / 主动建议）**本来就不随档位变**。

档位（§2 参数 + §3 两轴）真正 gate 的**只有结构**：契约层、issue/角色机器、worktree、按端身份文档。

**结论**：「一条流程恒定」不是新发明，是把 §1 已有的事实摆上台面；档位只是**把"结构按现实长"这条生成规则，预先枚举成了 9 个组合**。删档位 = 删掉这层多余的枚举，回到生成规则本身。

## 3. 新模型

> **一条流程（恒定 · 所有项目 · 就是 §1 公约 + media-ops 那套执行面）**
> 起手 freshness → 单一真相源 → 重瞄 brainstorming（角色变焦）→ 跳 writing-plans 但 spec 验收可检验 → issue+PR → 低风险 agent 自动合 / 高风险人审 → 必过独立新眼睛 review → 高风险闸（线上不可逆清单，恒定）→ 公约（反驳 6 闸 + 说人话闸）。
>
> **结构按"实际有什么"长出来（绝不预建）：**
> - **有第 2 个端？** → 加 `docs/contracts/`（跨端真相源）+ 按端身份文档（`<端>/CLAUDE.md`）+ worktree 隔离。（= owner 说的"worktree 开关"）
> - **有第 2 个人？** → 加协作 doc（谁干啥不撞车）。
> - 单端单人 → 只跑那条流程。
>
> **删干净**：`S0/S1/S2`、`C0/C1/C2`、`T0/T1/T2` 全部词汇 + agent-constraints「极简块 / 直接做」。

**注**：两个"有没有"（端 / 人）**正交**——media-ops = 单端 + 2 人（有协作 doc、无契约）。owner 心智里的"单一开关"实为这两条；都服从同一句"没有就别建"（§3.5），故概念仍是**一条规则**而非两张表。

## 4. 护栏保全清单（exp-006 减法自审 · 本 spec 的命门）

删档位**不许**顺手删掉任一档曾经独自扛着的护栏。逐条去向：

| # | 旧护栏（出处） | 旧挂靠 | 去向 | 动作 |
|---|---|---|---|---|
| 1 | `docs/contracts/` 跨端真相源（§3 轴一 S2） | S2 | 挂「有第 2 个端」 | **KEEP** |
| 2 | issue 模板 + label 状态机（§3 轴二 C1+） | C1+ | 进恒定流程（人人有） | **KEEP·降为基线** |
| 3 | 角色命令 / 协作 doc 谁干啥不撞车（C1+） | C1+ | 挂「有第 2 个人」（条件触发，非档位） | **KEEP·条件化** |
| 4 | 按端身份文档 `<端>/CLAUDE.md`（end-role-claude · S2） | S2 | 挂「有第 2 个端」 | **KEEP** |
| 5 | worktree 隔离（worktree-isolation · C2 可选） | C2 | 挂「有第 2 个端·真并行」 | **KEEP** |
| 6 | 6+1 骨架 + 消息总线（collaboration-c2 · C2） | C2 | 并入协作 doc（多人/多端协调） | **KEEP·合并** |
| 7 | 默认撒手档 + review 严格度 by risk（§2 risk） | risk 参数 | 不动（risk 参数留存，驱动自动合范围） | **KEEP** |
| 8 | 升级触发条件 ADR（§3.5） | 全档 | 简化为「加端→多端 / 加人→多人」触发 | **KEEP·简化** |
| 9 | 绝不为还没有的端预建结构（§3.5 初心） | 全档 | 升级为「绝不为还没有的端/人预建」 | **KEEP·泛化** |
| 10 | SOP 规则按需删/合/降级 · 反向齿轮（§3.5） | 全档 | 不动（对 STANDARD 自己递归生效） | **KEEP** |
| 11 | house_style 立栈对齐（§2 + §1.9） | 正交 | 不动 | **KEEP** |
| 12 | 凭据双书挡：开工 surface `待业务确认` / 收工推 doc（§1.8，标 C≥C1） | C≥C1 | 人人 issue+PR 后适用所有项目 | **KEEP·扩面** |

§1 公约其余各条**本就 tier-independent**（§2 核心洞察），不在删改面内，自动留存。

## 5. 一处「主动反转」（不是丢失 · owner 已 bless）

不是零改动——有一条旧规矩被**有意反转**，必须显式记录：

- **§3.3 C0「单人单 agent → 不要角色划分、不要 issue 状态机」** → 反转为「**单人也走 issue+PR**」。
- **依据**：§1.5「**AI 自动跑的仪式 → 人几乎不掏成本 → 划算、不是过度治理**」。issue+PR 由 agent 开/标/流转/提/自动合，成本在 AI 不在人，故对单人不算过度。旧 §3.3 写在 §1.5 这层 nuance 被吃透之前。
- **代价（诚实记）**：agent 要为每个小改动付分支/PR 开销；owner 不付，换"概念统一 + 每改动都过新眼睛 review 的安全网"。owner 已接受。
- **连带必改 §5.1**：把「单人却装 issue 状态机」从"过度治理"清单**删掉**（否则 audit 会把新基线误报成过度治理）；**保留**「单人却背 coordination/scope label」（多人机器套在单人身上**仍**是过度治理）。

## 6. 反过度治理如何在"没有极简档"后仍守住

旧「极简块/直接做」是给一次性脚本的缩水档。删它后初心靠**范围边界**守：

- sop-init 的右尺寸校验从"选哪档"变**二值**：*这项目值不值得一份 SOP？* 值 → 给那条流程（按现实加结构）；**真跑一次就扔 → 告诉 owner 你不需要 init**（工具照样拒绝过度治理）。
- 上一轮发现的洞（极简块「直接做」无"别照猜需求"护栏，exp-004 同形）**随极简块一起消失**——人人走的流程里 §1.1+§1.2「人定要什么 / 别脑补需求」恒定在场。

## 7. 逐文件改动表（实现面 · 实现时逐条对 §4 护栏核一遍）

| 文件 | 改什么 | 守哪条护栏 |
|---|---|---|
| `STANDARD.md` §2 | 参数表删 `端数S/协作结构C` 两行；保留 ends/collaborators/risk/house_style；ends 多于 1 → 触发"多端结构"，collaborators 多于 1 → 触发"协作 doc" | §4#1-6,11 |
| `STANDARD.md` §3 | 整节重写：「两根档位轴 + 9 宫格表」→「一条流程 + 结构按现实长（两条正交触发：第2端 / 第2人）」 | §4 全 |
| `STANDARD.md` §3.5 | 「绝不为还没有的端预建」泛化为「端/人」；升级触发条件简化为两条触发 | §4#8,9,10 |
| `STANDARD.md` §3.3/C0 描述 | 删；并入"单人也走流程" | §5 反转 |
| `STANDARD.md` §4 | 指针更新：极简块删除，剩"标准块 + 多端附加块" | §4#2 |
| `STANDARD.md` §5.1 | 删「单人却装 issue 状态机」over-gov 例；留 coordination/scope 例 | §5 连带 |
| `STANDARD.md` §5.2 | 「档位错配（两轴各查）」重写为「多端错配（多端没契约 / 单端却建契约）+ issue/PR 基线缺位 + 多人没协作 doc」 | §4#1,2,3 |
| `templates/agent-constraints.md` | **删极简块**；标准块成唯一基线；多端追加块保留（契约+按端+worktree）；块头 `{{SC}}` 占位符改为不含档位编号的项目描述 | §4#1,2,4,5 |
| `skills/sop-init/SKILL.md` | description + 流程 step 2「定档 T0/T1/T2 + S/C」→「问两条：几个端 / 几个人（+risk/house_style）」；step 4 生成逻辑按两条触发；step 5「不超两轴」→「没有的端/人不预建」；删 S0·C0 极简分支 | §4#1,2,3,9 |
| `skills/sop-audit/SKILL.md` | description + step 2「测 (S,C)」→「测 几端/几人/风险」；severity 映射 P2「档位错配」改名「结构错配」 | §5.2 |
| `templates/issue-pr-workflow.md` | step 2 label 的 **C1/C2 分流删掉**，收成单一基线方案（默认沿用 media-ops 现行：阶段 label + 阻塞 flag + 状态标记；见判断点 J1） | §4#2 |
| `templates/collaboration.md` + `collaboration-c2.md` | 合并为一份协作 doc，按"多人/多端"伸缩；6+1 + 消息总线作为"多端多 agent"小节 | §4#3,6 |
| `README.md` | 档位表 + 两轴叙述改为新模型 | 文档一致 |
| `docs/superpowers/specs/2026-06-05-sop-better-design.md` | 不改正文（历史记录），但新 spec 取代其档位模型 | 留痕 |

**轻改(只换触发标签 / 重命名,实质结构不变)**：`end-role-claude.md`(S2→多端)/ `worktree-isolation.md`(C2→多端)/ `s2-contracts.md` **→ 重命名 `multiend-contracts.md`**(S2→多端)/ `contracts-README.md`(T2→多端)。〔实现期细化:为概念干净,把这几份多端文件里的死 token 也清了,超出 spec 初稿"不动"的保守范围——见 exp-012。〕

## 8. 判断点（实现时定，flag 给 owner）

- **J1 · label 方案**：旧 C1（评估中/开发中/已完成 状态机）vs C2（无 lifecycle label）合一选谁。**默认沿用 media-ops 现行方案**（owner 锚"参考 media-ops 一致"）；实现时读 media-ops 的 `issue-pr-workflow.md` 定基线。若 owner 想更轻可改"无 lifecycle label"。
- **J2 · 协作 doc 触发**：定为「≥2 个**不同的人**」才生成（单人 + 多 agent ≠ 多人）；角色变焦本身在 §1 公约恒定，不靠协作 doc。

## 9. 反向验收（可检验 · 因跳 writing-plans 验收必须硬）

1. **词汇清零**：`grep -rE 'S0|S1|S2|C0|C1|C2|T0|T1|T2|两轴|9 ?宫格|档位'` 在 `STANDARD.md`/`templates/`/`skills/` 命中数 = 0（`experiments/` 历史记录豁免）。
2. **护栏在场**：§4 表 12 条护栏，逐条在改后文本里 grep 得到对应规则（不靠记忆，靠命中）。
3. **净行数为负**：本是减法，STANDARD+模板+skill 总行数**应净减**（exp-005 净增上限的反向——这里要求净负，正增则说明没真删、只搬家）。
4. **真项目复验**：对 media-ops 重跑 `/sop-audit`，应判「刚好 / 无漂移」、且不再因"单人/单端有 issue 机器"报过度治理或档位错配。对 py-script（上一轮那个）重跑 sop-init 应给"单端流程"、无极简档、有"别照猜需求"护栏。
5. **owner 便宜验收**：扫改后 STANDARD §3 + 一份生成 CLAUDE.md，确认"一条流程 + 结构按现实长"一眼读得懂。

## 10. 风险 / 未验 / 信心

- **信心**：核心模型对得上 owner 要的效果 ~85%；护栏清单完整（从 STANDARD 唯一真相源盘出）~80%，未验点是模板里可能有 STANDARD 未显式声明的本地护栏——实现时 J1 读 media-ops、逐文件对 §4 核。
- **最大风险**：§3 重写是 STANDARD 承重段的大改，易在收口处漏带某条 carve-out。**缓解**：分文件 commit，每个 commit 自带"守了 §4 哪几条"，build 后派独立新眼睛 review 喂本 spec + diff 三查。
- **第二风险**：「单人也 issue+PR」若实跑下来 agent 的 git 开销烦到 owner，需回退一个"trivial 改动免分支、直 commit + 自动合"的轻通道（仍在一条流程内，不复活档位）。记为 exp-012 的观察项。

## 11. 实验登记

落 `experiments/2026-06-23-012-collapse-tiers-to-one-flow.md`（撒手实验）：撒手档 = owner 审 spec + 审最终 diff，实现 agent 改 + 自验 + 独立新眼睛 review。结晶若成立，沉 PLAYBOOK（须 exp-012 背书）。
