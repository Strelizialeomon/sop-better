# sop-audit 覆盖判定落地与 media-ops 校准 · Design

## 1. 背景

`$sop-audit` 已有主线覆盖闸,也已有严格覆盖判定设计:

- 主线覆盖闸: `2026-07-06-sop-audit-mainline-coverage-gate-design.md`
- 严格判定口径: `2026-07-06-sop-audit-strict-coverage-judgment-design.md`

但线上 `skills/sop-audit/SKILL.md` 仍停在旧状态集合: `覆盖 / 缺失 / 偏离`。实跑 `media-ops` 后仍出现:

- `Refs / Closes` 高频入口互相打架,却被写成「偏离」。
- `doc / issue / PR` 三件套只找到零散证据,却被写成「覆盖」。
- 多端缺端级 `AGENTS.md`,却被写成「偏离但合理」并不进 finding。

这说明问题已从「要不要有主线闸」推进到「线上 skill 是否真的按新口径判」。

## 2. 目标

本次只做两件事:

1. 把严格覆盖判定落进 `skills/sop-audit/SKILL.md`。
2. 用 `media-ops` 做一次只读校准报告,验证新口径是否生效。

## 3. 非目标

- 不改 `STANDARD.md`。
- 不新增第 7 个主线项。
- 不把 `media-ops` 的具体路径硬编码进 `$sop-audit` 主流程。
- 不修改 `media-ops` 文件。
- 不把校准报告变成每次 audit 都必须输出的长案例库。

## 4. 设计

### 4.1 skill 改动位置

只改 `skills/sop-audit/SKILL.md` 的「主线覆盖闸」附近:

- 表格状态说明。
- 覆盖最小口径。
- 状态处理 / findings 转换规则。

不改 `$sop-audit` 的总流程、默认只读原则、severity 总映射和收尾卡结构。

### 4.2 状态集合

主线覆盖闸仍固定 6 行,但每行状态允许:

| 状态 | 含义 |
|---|---|
| 覆盖 / 匹配 | 触发的必查 surface 都有等价规则,且没有高频入口冲突。 |
| 部分覆盖 | 主文档有规则,但入口不全 / 执行点缺失 / 低频提示缺引用。 |
| 冲突 | 一个高频入口写新规则,另一个高频入口仍写旧规则。 |
| 缺失 | 触发条件已满足,但没有执行落点;或只有 spec / 历史记录,没进执行 SOP。 |
| 偏离 | 有明确本地替代方案,且能说明为什么等价。 |
| 过重 / 预建 | 未触发却提前上结构,或低风险小事被流程拖重。 |
| n/a | surface 不适用,只写在证据里,不作为表格主状态。 |

### 4.3 判定顺序

每行先判 surface 是否适用:

- 已触发且缺文件 / 缺规则 = `缺失`。
- 未触发且不存在 = `n/a`,不算缺失。
- 未触发却存在治理骨架 = `预建` 或 `过重`。
- 文件存在则必须查内容;内容冲突不能被文件存在掩盖。

再按这个顺序判状态:

1. `冲突`
2. `缺失`
3. `部分覆盖`
4. `过重 / 预建`
5. `偏离`
6. `覆盖 / 匹配`

顺序的目的:先抓会误导 agent 行动的错误,最后才给「覆盖」。

### 4.4 必查 surface

每个主线项填状态前必须扫高频入口和执行落点,但不扩成全仓背题清单:

| 主线项 | 必查 surface |
|---|---|
| Refs / Closes 收口 | PR 模板;根 `AGENTS.md` 速查流程;`docs/project/issue-pr-workflow.md`;`collaboration.md` 中如有 PR 流程也要查。 |
| doc / issue / PR 三件套 | workflow 是否明确定义三件套;根 `AGENTS.md` / `collaboration.md` 是否漏掉 PR 或把 doc / issue / PR 互相替代。 |
| issue 评论分层 | workflow 是否有厚/短评论触发条件和最小字段;根 `AGENTS.md` 自检处是否有短引用。 |
| 查证闭环 | 根 `AGENTS.md` 沟通约束;workflow 起手凭据校验;是否把小事拖成联网 / 确认 / 长卡片。 |
| 高风险治理项 | 根 `AGENTS.md` 高风险闸;workflow PR 合并规则;PR 模板风险区。 |
| 结构触发 | STANDARD 推导出的端 / 人 / 并行 agent 三触发;实际目录;`docs/contracts/`;端级 `AGENTS.md`;协作 doc;worktree / coordination 是否按需存在。 |

surface 规则:

- surface 是「存在则查、触发才必备」。
- PR 模板不存在时记 `n/a`,不自动报缺失。
- 第 2 个端触发 `docs/contracts/` 和端级 `AGENTS.md`。
- 串行只豁免 worktree / coordination,不豁免端级身份文档。

### 4.5 findings 转换

覆盖闸不是第六类 finding,但状态必须驱动 findings:

- `冲突`:通常进 finding。若冲突在 PR 模板 / 根 `AGENTS.md` / workflow 这类高频入口,优先级不低于 P2/P3。
- `缺失`:进入 finding。结构触发缺失按 STANDARD §5.2 优先判 P2 `mismatch`。
- `部分覆盖`:缺高频执行入口则进 finding;只缺低频提示引用可 P3 或收尾风险。
- `过重 / 预建`:按 STANDARD §5.1 / §5.2 判 severity。
- `偏离`:必须写等价理由。理由成立不进 finding;理由不成立改成 `缺失` 或 `冲突`。
- `覆盖 / 匹配`:不进 finding。

## 5. media-ops 校准报告

实现后对 `/Users/sunchongsheng/code/media-ops` 做一次只读校准报告。报告不改 media-ops 文件,也不在 `$sop-audit` 主流程硬编码 media-ops 案例。

合格报告必须满足:

| 主线项 | 期望判定 |
|---|---|
| Refs / Closes 收口 | `冲突`;workflow 默认 `Refs`,但 PR 模板 / 根速查诱导 `Closes`。 |
| doc / issue / PR 三件套 | `部分覆盖`;workflow 有分工,但 collaboration 仍只显式写 issue + doc。 |
| issue 评论分层 | `缺失`;spec 已有,执行 SOP 未落厚/短评论分层。 |
| 查证闭环 | `覆盖`;已有本地事实、外部事实、未验风险与 carve-out。 |
| 高风险治理项 | `部分覆盖 / 覆盖`;若 PR 模板风险区漏治理 doc / contracts,不得直接写干净覆盖。 |
| 结构触发 | `缺失` 并进入 P2 `mismatch`;多端已触发,缺端级 `AGENTS.md`;串行只豁免 worktree / coordination。 |

报告还必须保留总判:项目整体右尺寸,不是太重。不能因为严格判定而硬挑过度治理。

## 6. 验收标准

- `skills/sop-audit/SKILL.md` 明确新状态集合。
- skill 明确先判 surface 适用性,再按状态顺序判定。
- skill 明确「存在则查、触发才必备、缺省 n/a」。
- skill 明确高频入口冲突不能写覆盖。
- skill 明确第 2 个端缺端级 `AGENTS.md` 是 STANDARD §5.2 的 P2 `mismatch`;串行不豁免端级身份文档。
- media-ops 只读校准报告不再把:
  - `Refs / Closes` 写成普通偏离。
  - 三件套写成干净覆盖。
  - 端级身份文档缺失降成普通 P3 或合理偏离。

## 7. 风险与约束

- **变厚风险**:surface 继续膨胀。约束:只列高频入口和执行落点。
- **背题风险**:把 media-ops 变成硬编码案例。约束:media-ops 只在验收报告里出现,不进主流程规则。
- **误报风险**:没有 PR 模板的项目被误判缺失。约束:surface 先判适用性,缺省记 `n/a`。
- **过严风险**:本地合理替代被判缺失。约束:`偏离` 保留,但必须写等价理由。
