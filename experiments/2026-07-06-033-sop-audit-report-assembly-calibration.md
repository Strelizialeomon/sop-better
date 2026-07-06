# exp-033 · sop-audit 报告装配校准

- **日期**:2026-07-06
- **真实任务**:`media-ops` `$sop-audit` 报告已抓到主线异常,但 findings 装配仍漏同类入口 / 治理 doc 风险 / 三件套证据强度
- **本次撒手档位**:L2(低风险文档 / skill 改动,但 live skill 会影响后续审计,需真项目只读校准)

---

## 1. 选活 + 定档

exp-031/032 已把 `$sop-audit` 的覆盖状态、结构触发和 findings 对账钉住。owner 继续复核 `media-ops` 报告后,发现报告“查到了但装配不全”:

- `Refs / Closes` finding 只列 PR 模板,漏了根 `AGENTS.md` 速查也默认 `Closes #N`。
- PR 风险区只报跨端契约 / 骨架,漏了“文档低风险”与治理 doc 的冲突。
- 三件套判覆盖时,证据要直接证明 doc / issue / PR 三者分工,不能拿弱证据凑覆盖。

爆炸半径:改的是 live `$sop-audit` 的报告口径。动作仍是文档层,但会影响之后所有 audit,所以需要真项目压力断言。

## 2. 便宜验证方案(动手前必须答)

> 我怎么在 10 分钟内、**不逐行读**,就看出 AI 做对没?

- 验证手段:读 `$sop-audit` 主线覆盖闸,再用 `/Users/sunchongsheng/code/media-ops` 只读压力检查。
- 压力样本:`media-ops` commit `806e44d`。
- 通过标准:
  - `$sop-audit` 明确同一主线项多个高频入口冲突要合并报全,target / evidence 不能漏入口。
  - `$sop-audit` 明确 PR 模板“文档低风险”要继续查治理 doc 与跨端契约骨架 carve-out。
  - `$sop-audit` 明确三件套判覆盖必须直接证明 doc / issue / PR 三者分工。
  - media-ops 只读检查能看到 PR 模板默认 `Closes #`、根 `AGENTS.md` 速查 `Closes #N`、PR 风险区漏治理 doc / contracts、三件套证据可被区分强弱。

## 3. 给 AI 的简报(只给目标+约束+验收,不给解法)

```
目标:
把 media-ops audit 复核暴露的 3 个报告装配漏口,校准进 sop-audit skill。

约束:
- 不新增第 7 个主线项。
- 不改 STANDARD.md。
- 不把 media-ops 路径硬编码进 skill。
- 不走 writing-plans。
- 只补 3 条装配校验,不要扩成新协议名。

验收标准:
- 同类冲突 finding target/evidence 必须列全。
- PR 风险区能区分普通文档与治理 doc / docs/contracts / 跨端骨架。
- 三件套覆盖证据必须同时证明 doc / issue / PR。
- media-ops 压力样本按 commit 806e44d 可复核。
```

## 4. AI 跑完,我来评

- **AI 做了哪些决策**:
  - 不新增主线项,只在现有覆盖闸下补“报告装配校验”一条。
  - 把 `nopushback` 从 Refs/Closes 这类规则冲突中排除,统一归 `mismatch`。
  - 把 PR 模板“文档低风险”拆成普通文档 vs 治理 doc / contracts / 跨端骨架。
  - 把三件套覆盖证据收紧为必须直接证明 doc / issue / PR 三者分工。

- **超出我预期 / 我自己想不到的地方**:
  - 问题不在“没查到”,而在报告合成阶段把同类证据拆散、降弱或漏列。修装配比继续加扫描项更轻。

- **翻车 / 我得纠偏的地方**:
  - spec 新眼睛指出 media-ops 行号可能漂移;本实验记录 commit `806e44d`,后续行号以该样本或最新实况说明为准。

### 评分(10 分制)

| 维度 | 分 | 备注 |
|---|---|---|
| 设计质量(它做得好不好) | 8/10 | 三条装配校验,没有新主线 |
| 省力程度(比我自己干省了多少脑子) | 8/10 | 以后 audit 报告更少漏同类入口 |
| **爽感**(有没有"卧槽这样也行") | 7/10 | 修的是报告装配,不是再堆规则 |
| 验证成本(检查它的结果累不累) | 8/10 | grep + media-ops 只读断言即可验 |

## 5. 抽一条教训 → 回填 PLAYBOOK

这次学到:在 L2 档,【审计报告装配校准】能交出去,前提是只修“同类证据合并、风险分流、覆盖证据强度”这类报告装配问题,不继续加主线项;否则 `$sop-audit` 会查到异常却在 findings 里漏列 / 降弱。

→ 已写入 `PLAYBOOK.md`?[x]
