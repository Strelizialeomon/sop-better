# exp-030 · sop-audit 主线覆盖闸防漏

- **日期**:2026-07-06
- **真实任务**:`media-ops` `$sop-audit` 漏掉近期协作主线后的 skill 修正
- **本次撒手档位**:L2(低风险文档 / skill 改动,但影响后续审计判断,需新眼睛 + 真项目只读复跑)

---

## 1. 选活 + 定档

`media-ops` 真审计里,旧 `$sop-audit` 只抓到两个显眼问题:

- PR 模板默认 `Closes #`。
- PR 模板高风险勾选缺治理 doc / 跨端契约。

但它漏了近期真正要回灌的主线:

- `Refs / Closes` 收口语义不完整。
- doc / issue / PR 三件套没有在执行 SOP 里成形。
- issue 评论分层只落了 spec,没进执行 SOP。

爆炸半径:改的是本仓 live skill,会立刻影响后续 `$sop-audit`。所以不能只靠看起来合理,必须用真实项目压一次。

## 2. 便宜验证方案(动手前必须答)

> 我怎么在 10 分钟内、**不逐行读**,就看出 AI 做对没?

- 验证手段:在 `/Users/sunchongsheng/code/media-ops` 做只读压力测试,填出 `$sop-audit` 新增的 6 行「主线覆盖闸」。
- 通过标准:表里必须用 `权威锚点 -> 项目落点/缺口`,且至少逼出 Refs/Closes、三件套、issue 评论分层三项;不能用"串行单 agent"豁免第 2 个端触发。
- 如果上面答不上来 → 说明覆盖闸仍是口号,不能进 skill。

## 3. 给 AI 的简报(只给目标+约束+验收,不给解法)

```
目标:
把 design spec 中的 sop-audit 主线覆盖闸落进 $sop-audit,避免 audit 只抓显眼 finding、漏近期协作主线。

约束:
- 不改 STANDARD.md,避免权威尺继续变厚。
- 不新增新协议名,统一叫"主线覆盖闸"。
- 表固定 6 行,不允许越写越长。
- 每行必须是权威锚点 -> 项目落点/缺口。
- 异常项才展开成现有 findings。

验收标准:
- skills/sop-audit/SKILL.md 在 findings 前强制输出主线覆盖闸。
- experiments/ 记录真实压力测试。
- PLAYBOOK.md 沉淀一条有 exp-030 背书的教训。
- media-ops 只读复跑能逼出之前漏掉的 Refs/Closes、三件套、issue 评论分层。
```

## 4. AI 跑完,我来评

- **AI 做了哪些决策**:
  - 把覆盖闸放在模板版本差之后、findings 之前,先对照母本和项目,再允许展开报告。
  - 不重抄 STANDARD,只在 skill 里放 6 行表 + 最小判定口径。
  - 把单个 `file:line` 修成 `权威锚点 -> 项目落点/缺口`,防止关键词式假覆盖。
  - 结构触发拆成端 / 人 / 并行 agent 三条,串行只豁免 worktree / 协调骨架,不豁免第 2 个端。

- **新眼睛复审**:
  - 第一轮 spec review verdict = `Needs changes`,指出 4 个问题:没强制比当前母本、单证据会假覆盖、结构触发可能误放行、验收没有要求真项目复跑。
  - 修订后第二轮 verdict = `Ready`;非阻断建议是压力测试样例也要改成证据对,已修。

- **media-ops 只读压力测试**:
  - commit:`806e44d61ee2cd18766a07c67b304cc16653ca11`
  - 工作区:`## main...origin/main`

| 主线项 | 状态 | 证据 |
|---|---|---|
| Refs / Closes 收口 | 缺失 | `master/base/docs/project/issue-pr-workflow.md:37 -> .github/PULL_REQUEST_TEMPLATE.md:3` 默认正文仍是 `Closes #`;`AGENTS.md:164` 短流程也写 `提 PR(Closes #N)` |
| doc / issue / PR 三件套 | 缺失 | `master/base/docs/project/issue-pr-workflow.md:11 -> docs/project/collaboration.md:13` 只写 issue + doc 协同,PR 角色没有进入三件套分工 |
| issue 评论分层 | 缺失 | `master/base/docs/project/issue-pr-workflow.md:51 -> docs/project/issue-pr-workflow.md:64` 只有 issue vs PR 评论和决策快照,没有厚/短评论分层 |
| 查证闭环 | 覆盖 | `STANDARD.md:67 -> AGENTS.md:140` 已覆盖方案前调研包、本地事实、外部事实、信源优先级和 carve-out |
| 高风险治理项 | 缺失 | `master/base/docs/project/issue-pr-workflow.md:40 -> .github/PULL_REQUEST_TEMPLATE.md:24` PR 模板风险项没有治理 doc / AGENTS / 协作约定 / 跨端契约或骨架 |
| 结构触发 | 缺失 | `STANDARD.md:103/104/108 -> AGENTS.md:47-53 + find . -maxdepth 3 -name AGENTS.md` 显示多端且有 `docs/contracts/`,但只找到根 `./AGENTS.md`,无端级身份文档 |

- **超出我预期 / 我自己想不到的地方**:
  - "短证据"不能等于"单项目证据";必须是证据对,否则样例本身会把实现 agent 带偏。
  - media-ops 已从早期单端变成多端,结构触发行不能沿用旧判断;这正好证明覆盖闸要先看现场。

- **翻车 / 我得纠偏的地方**:
  - 第一版 spec 自检不够,把单个 `file:line` 当成轻量,实际会诱导假覆盖。
  - 后续实现必须防止覆盖闸变成第六类 finding;它只是报告前的防漏停顿点。

### 评分(10 分制)

| 维度 | 分 | 备注 |
|---|---|---|
| 设计质量(它做得好不好) | 8/10 | 二轮新眼睛后收敛到薄而可验 |
| 省力程度(比我自己干省了多少脑子) | 8/10 | 6 行表能让漏项一眼暴露 |
| **爽感**(有没有"卧槽这样也行") | 7/10 | 把"更严"变成"证据对",没有明显加厚 |
| 验证成本(检查它的结果累不累) | 8/10 | media-ops 十分钟内能复跑出表 |

## 5. 抽一条教训 → 回填 PLAYBOOK

这次学到:在 L2 档,【审计类 skill 防漏】能交出去,前提是把大段规则收成一个固定薄闸,并强制 `权威锚点 -> 项目落点/缺口`;否则 agent 会用项目侧关键词填 `覆盖`。

→ 已写入 `PLAYBOOK.md`?[x]
