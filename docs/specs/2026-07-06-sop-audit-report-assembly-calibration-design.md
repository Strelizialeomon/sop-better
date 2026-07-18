# sop-audit 报告装配校准 · Design

## 1. 背景

`$sop-audit` 已有 6 行主线覆盖闸,并补过严格状态判定与 findings 对账。真跑 `media-ops` 后,大方向已经稳定,但报告合成仍有 3 类漏口:

- 同一主线项有多个高频入口冲突时,报告可能只列一个入口。例如 `Refs / Closes` 只报 PR 模板,漏根 `AGENTS.md` 速查。
- PR 风险区把“文档”列低风险时,可能没继续检查是否排除了治理 doc / `AGENTS.md` / SOP / 协作约定 / `docs/contracts/` / 跨端骨架。
- doc / issue / PR 三件套判覆盖时,证据可能只证明 issue/doc 或 PR 的局部动作,没有同时证明三者分工。

这些不是新主线,而是同一个覆盖闸的“报告装配校验”还不够硬。

## 2. 目标

让 `$sop-audit` 在产出 findings 前多一层很薄的装配自检:

1. 同一主线项发现多个高频入口冲突,同一个 finding 的 target / evidence 必须列全。
2. PR 风险区必须区分普通文档与治理 doc / 跨端契约骨架。
3. 三件套判覆盖时,证据必须直接证明 doc / issue / PR 三者分工;证明不全最多写部分覆盖。

## 3. 非目标

- 不新增第 7 个主线项。
- 不改 `STANDARD.md`。
- 不把 `$sop-audit` 变成逐文件背题清单。
- 不要求所有项目逐字贴合母本。
- 不把普通文档 PR 一律打成高风险。

## 4. 设计

### 4.1 同类冲突合并报全

在 `$sop-audit` 主线覆盖闸的“状态转 findings”后补一条:

- 同一主线项若多个高频入口都冲突,合并成一个 finding,但 target / evidence 必须列全。
- 典型例子: `Refs / Closes` 同时查 PR 模板、根 `AGENTS.md` 速查、workflow、collaboration PR 流程。若 PR 模板和根 `AGENTS.md` 都诱导 `Closes`,finding 不能只写 PR 模板。
- kind 用 `mismatch`;不要用 `nopushback` 这类和问题性质不匹配的 kind。

### 4.2 PR 风险区普通文档 / 治理文档分流

在“高风险治理项”必查 surface 后补一条:

- PR 模板若把“文档”列为低风险,必须继续查是否显式排除治理 doc。
- 治理 doc 包括 `AGENTS.md`、SOP、协作约定、issue/PR workflow、PR 模板自身、`docs/contracts/`、跨端契约 / 骨架。
- 若根 `AGENTS.md` 或 workflow 已要求治理 doc / 跨端契约回 owner 人审,但 PR 模板风险区没列,高风险治理项最多是部分覆盖,并进入 finding。
- 建议写法是“普通文档低风险;治理 doc / 跨端契约 / 骨架以根 AGENTS.md 完整清单为准并回人审”。

### 4.3 三件套覆盖证据必须三者齐全

在“doc / issue / PR 三件套”的必查 surface 后补一条:

- 判 `覆盖` 时,证据必须直接证明 doc、issue、PR 三者的分工都存在。
- 合格证据可以来自同一 workflow 的三件套定义,也可以来自 workflow + 根 `AGENTS.md` / collaboration 的互相补证。
- 只证明 issue/doc 协同,或只证明 PR “改了什么 / 验收怎么过”,都不能单独证明三件套覆盖。
- 证据不足但已有局部规则时写 `部分覆盖`,并按缺口位置决定是否进 P3 finding。

## 5. media-ops 压力断言

按新口径,同一份 `media-ops` audit 应满足:

- 压力样本行号以 `exp-033` 记录的 `media-ops` commit 为准;若实现时目标项目已变,用新 `file:line` 并说明变化,不要因旧行号漂移误判失败。
- `Refs / Closes` finding 同时指向 `.github/PULL_REQUEST_TEMPLATE.md:3` 和 `AGENTS.md:164`,kind 为 `mismatch`。
- PR 风险区 finding 同时覆盖“治理 doc”和“跨端契约 / 骨架”两个漏项。
- 三件套若判覆盖,证据必须同时指向 workflow 三件套定义或 workflow + `AGENTS.md` / collaboration 的直接分工;否则写部分覆盖。
- 结构触发仍保持 `缺失 / P2 mismatch`,不回退成部分覆盖。
- 总判仍可写“不是太重,主要偏轻 + 高频入口冲突”,不能为严判而 cry wolf。

## 6. 验收标准

- `skills/sop-audit/SKILL.md` 增加上述 3 条装配校验,且不新增主线项。
- 新增 `exp-033`,记录这次 media-ops 报告复核暴露的 3 个装配漏口。
- `PLAYBOOK.md` 增加一条 exp-033 背书的教训。
- 用只读 media-ops 压力检查能确认:
  - PR 模板默认 `Closes #` 与根 `AGENTS.md` 速查 `Closes #N` 同时被识别为 Refs/Closes 冲突入口。
  - PR 模板风险区没排除治理 doc / 跨端契约骨架。
  - 三件套覆盖证据能被明确区分为“证明三者齐全”或“不足只能部分覆盖”。

## 7. 风险与边界

- **变厚风险**:继续给每个主线项堆子清单。约束:只补 3 条装配校验,不新增主线项。
- **误报风险**:把普通文档 PR 都判高风险。约束:只针对治理 doc / 跨端契约骨架,普通需求 / 设计文档仍可低风险。
- **过拟合风险**:把 media-ops 写成背题。约束:skill 写通用口径,media-ops 只留在实验和压力断言里。
