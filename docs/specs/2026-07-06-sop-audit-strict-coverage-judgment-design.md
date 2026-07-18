# sop-audit 严格覆盖判定 · Design

## 1. 背景

`$sop-audit` 已落地「主线覆盖闸」,要求在 findings 前先填 6 行覆盖表,并用 `权威锚点 -> 项目落点/缺口` 防止漏查。

真跑 `media-ops` 后,覆盖闸已经逼出了主线项,但仍出现几个判断偏差:

- workflow 写了默认 `Refs`,但 PR 模板和根 `AGENTS.md` 仍诱导 `Closes`,报告只写「偏离」,没有明确这是冲突。
- doc / issue / PR 三件套有分散证据,但 `collaboration.md` 仍只写 issue + doc,报告直接写「覆盖」。
- 多端缺端级身份文档被报成 P3,但按 STANDARD §5.2 更接近 P2 结构错配。

问题不是 6 行闸不对,而是「覆盖」这个词太松。agent 仍可能找一处有利证据就写覆盖。

## 2. 目标

细化 `$sop-audit` 主线覆盖闸的判定口径:

- 有高频入口冲突时,不得写「覆盖」。
- 每个主线项有最小必查 surface。
- 状态词能稳定映射到 findings。
- 保持 6 行覆盖闸不变,不把 SOP 写厚。

## 3. 非目标

- 不新增第 7 个主线项。
- 不改 `STANDARD.md`。
- 不把覆盖闸变成完整迁移工具。
- 不要求项目逐字贴合母本。
- 不把所有「部分覆盖」都强制报成高严重度 finding。

## 4. 设计

### 4.1 覆盖状态定义

在 `$sop-audit` 主线覆盖闸下补一段状态判定:

| 状态 | 判定 |
|---|---|
| 覆盖 / 匹配 | 必查 surface 都有等价规则,且没有高频入口冲突。 |
| 部分覆盖 | 主文档有规则,但入口不全 / 执行点缺失 / 低频提示缺引用。 |
| 冲突 | 一个文件写新规则,另一个高频入口仍写旧规则。 |
| 缺失 | 没有执行落点,或只有 spec / 历史记录,没有进入执行 SOP。 |
| 偏离 | 有明确本地替代方案,且能说明为什么等价。写不出等价理由就不能叫偏离。 |
| 过重 / 预建 | 未触发却提前上结构,或低风险小事被流程拖重。 |

硬规则:

- 只要高频入口冲突,不能写「覆盖」。
- 只在目标项目里搜到近似关键词,不能写「覆盖」。
- `偏离` 必须带一句等价理由。
- 必查 surface 是「存在则查,缺省记 n/a」,文件不存在本身不自动等于缺失。

### 4.2 每项必查 surface

保持主线覆盖闸固定 6 行,但每行填状态前必须扫这些 surface:

| 主线项 | 必查 surface |
|---|---|
| Refs / Closes 收口 | `.github/PULL_REQUEST_TEMPLATE.md`;根 `AGENTS.md` 速查流程;`docs/project/issue-pr-workflow.md`;`docs/project/collaboration.md` 中如有 PR 流程也要查。 |
| doc / issue / PR 三件套 | `docs/project/issue-pr-workflow.md` 是否明确定义三件套;根 `AGENTS.md` / `collaboration.md` 是否漏掉 PR 或把 doc / issue / PR 互相替代。 |
| issue 评论分层 | `docs/project/issue-pr-workflow.md` 是否有厚/短评论触发条件和最小字段;根 `AGENTS.md` 自检处是否有短引用。 |
| 查证闭环 | 根 `AGENTS.md` 沟通约束;workflow 起手凭据校验;同时反查是否把所有小事都拖成联网 / 确认 / 长卡片。 |
| 高风险治理项 | 根 `AGENTS.md` 高风险闸;workflow PR 合并规则;PR 模板风险区。 |
| 结构触发 | `STANDARD.md` 推导出的端 / 人 / 并行 agent 三触发;实际目录;`docs/contracts/`;端级 `AGENTS.md`;协作 doc;worktree / coordination 是否按需存在。 |

### 4.3 findings 转换规则

覆盖闸不是第六类 finding,但状态要驱动后续 findings:

- `冲突`:通常进入 finding。若冲突在 PR 模板 / 根 `AGENTS.md` / workflow 这类高频入口,优先级不低于 P2/P3。
- `部分覆盖`:看缺口位置。缺高频执行入口则进 finding;只缺低频提示引用可 P3 或收尾风险。
- `缺失`:进入 finding。
- `偏离`:若等价理由成立,不进 finding;若理由不成立,改成 `缺失` 或 `冲突`。
- `覆盖 / 匹配`:不进 finding。
- `过重 / 预建`:按 STANDARD §5.1 / §5.2 判 severity。
- `结构触发`:第 2 个端已触发但缺 `docs/contracts/` 或端级 `AGENTS.md`,按 STANDARD §5.2 记 P2 `mismatch`;串行只豁免 worktree / coordination,不豁免端级身份文档。

### 4.4 media-ops 校准

按新口径,同一份 `media-ops` audit 应这样判断:

| 主线项 | 新状态 | 理由 |
|---|---|---|
| Refs / Closes 收口 | 冲突 | workflow 有默认 `Refs`,但 PR 模板默认 `Closes #`,根 `AGENTS.md` 速查也写 `Closes #N`。 |
| doc / issue / PR 三件套 | 部分覆盖 | workflow 有 issue / PR 凭据和 doc 链接,但 `collaboration.md` 仍写 issue + doc 协同,PR 角色不够显式。 |
| issue 评论分层 | 缺失 | PR #271 只落 spec,执行 SOP 没有厚/短评论分层。 |
| 查证闭环 | 覆盖 | 根 `AGENTS.md` 已有方案前调研包、本地事实、外部事实、未验风险和 carve-out。 |
| 高风险治理项 | 部分覆盖 / 覆盖 | 根 `AGENTS.md` 和 collaboration 已覆盖治理 doc / 跨端契约回人审;若 PR 模板风险区漏治理项,可判部分覆盖但不一定单独成 P2。 |
| 结构触发 | 缺失 | 项目是多端,有 contracts,但没有端级 `AGENTS.md`;串行只豁免 worktree / coordination,不豁免端级身份文档。 |

## 5. 验收标准

- `$sop-audit` skill 明确覆盖状态定义。
- skill 明确每个主线项的必查 surface。
- skill 明确「高频入口冲突不得写覆盖」。
- skill 明确 `部分覆盖` 和 `冲突` 如何转 finding。
- 用 `media-ops` 口径复核时,不会把 Refs/Closes 写成普通偏离,不会把三件套写成干净覆盖,不会把端级身份文档缺失降成普通 P3。

## 6. 风险与约束

- **变厚风险**:每项必查 surface 继续膨胀。约束:只列高频入口和执行落点,不列所有可能文件。
- **误报风险**:把所有局部缺口都报成冲突。约束:只有高频入口规则相反才叫冲突;低频引用缺失优先叫部分覆盖。
- **僵化风险**:本地合理替代被判缺失。约束:`偏离` 保留,但必须写等价理由。
