# exp-031 · sop-audit 严格覆盖判定校准

- **日期**:2026-07-06
- **真实任务**:`media-ops` `$sop-audit` 仍把冲突判偏离、把部分覆盖判覆盖后的 skill 修正
- **本次撒手档位**:L2(低风险文档 / skill 改动,但 live skill 会影响后续审计,需真项目只读校准)

---

## 1. 选活 + 定档

exp-030 加了 6 行主线覆盖闸,但实跑 `media-ops` 后仍出现判断偏软:

- `Refs / Closes` 高频入口冲突被写成「偏离」。
- doc / issue / PR 三件套只找到零散证据就写「覆盖」。
- 多端缺端级 `AGENTS.md` 被写成「偏离但合理」,没进 P2 结构错配。

爆炸半径:改的是 live `$sop-audit`,会影响之后所有 SOP 体检。动作仍是文档 / skill 层,可回滚,所以 L2 合适。

## 2. 便宜验证方案(动手前必须答)

> 我怎么在 10 分钟内、**不逐行读**,就看出 AI 做对没?

- 验证手段:用 `/Users/sunchongsheng/code/media-ops` 只读校准报告做压力测试。
- 通过标准:
  - `Refs / Closes` 判 `冲突`,不能写普通偏离。
  - doc / issue / PR 三件套判 `部分覆盖`,不能写干净覆盖。
  - issue 评论分层判 `缺失`。
  - 多端缺端级 `AGENTS.md` 按 STANDARD §5.2 进 P2 `mismatch`;串行只豁免 worktree / coordination。
  - 总判仍保留「整体右尺寸 / 不是太重」,不能为严判而 cry wolf。

## 3. 给 AI 的简报(只给目标+约束+验收,不给解法)

```
目标:
把严格覆盖判定落进 $sop-audit 线上 skill,并用 media-ops 只读校准验证。

约束:
- 不改 STANDARD.md。
- 不新增第 7 个主线项。
- 不把 media-ops 路径硬编码进 skill 主流程。
- 不改 media-ops 文件。
- surface 只列高频入口 + 执行落点,不扩成全仓背题清单。

验收标准:
- skills/sop-audit/SKILL.md 明确覆盖 / 部分覆盖 / 冲突 / 缺失 / 偏离 / 过重 / 预建 / n/a。
- skill 明确先判 surface 适用性,再按冲突 -> 缺失 -> 部分覆盖 -> 过重/预建 -> 偏离 -> 覆盖判定。
- skill 明确存在则查、触发才必备、缺省 n/a。
- media-ops 校准报告不再复现旧误判。
```

## 4. AI 跑完,我来评

- **AI 做了哪些决策**:
  - 不新增主线项,只把主线覆盖闸的状态集合和转换规则补齐。
  - 把 `n/a` 放在证据层,不让它成为表格主状态,避免报告状态膨胀。
  - 把「存在则查、触发才必备」写成 surface 适用性前置判断。
  - 明确第 2 个端触发 contracts 和端级 `AGENTS.md`,串行只豁免 worktree / coordination。

- **media-ops 只读校准报告**:
  - commit:`806e44d`
  - 工作区:`## main...origin/main`
  - open PR:`[]`
  - collaborators:`Strelizialeomon`, `Renan966`, `boph-guan`

| 主线项 | 新状态 | 证据 |
|---|---|---|
| Refs / Closes 收口 | 冲突 | 母本 `master/base/docs/project/issue-pr-workflow.md:37` 要 PR 默认 `Refs`;项目 workflow `docs/project/issue-pr-workflow.md:54` 也写 `Refs`;但 PR 模板 `.github/PULL_REQUEST_TEMPLATE.md:3` 默认 `Closes #`,根速查 `AGENTS.md:164` 也写 `Closes #N`。 |
| doc / issue / PR 三件套 | 部分覆盖 | 母本 `master/base/docs/project/issue-pr-workflow.md:13-15` 定三件套;项目 workflow 有 issue/PR 分工,但 `docs/project/collaboration.md:13` 仍只显式写 issue + doc 协同。 |
| issue 评论分层 | 缺失 | 母本 `master/base/docs/project/issue-pr-workflow.md:51-55` 有厚/短评论分层;项目 workflow `docs/project/issue-pr-workflow.md:66-67` 只有 issue vs PR 评论和决策快照;项目 spec `docs/superpowers/specs/2026-07-05-issue-comment-evidence-standard-design.md:180-201` 明确还要落 workflow + AGENTS。 |
| 查证闭环 | 覆盖 | STANDARD `§1` 闭环 -> 项目 `AGENTS.md:134-145` 已覆盖确认卡、方案前调研包、本地/外部事实和 carve-out。 |
| 高风险治理项 | 部分覆盖 | 项目 `AGENTS.md:107` / `docs/project/collaboration.md:41` 覆盖治理 doc 人审,但 PR 模板风险区 `.github/PULL_REQUEST_TEMPLATE.md:20-24` 未显式列治理 doc / contracts。 |
| 结构触发 | 缺失 / P2 mismatch | STANDARD `§3`/`§5.2` 第 2 个端触发 contracts + 端级身份文档;项目 `AGENTS.md:47-53` 是多端且有 `docs/contracts/`,但 `find . -maxdepth 3 -name AGENTS.md` 只找到根 `./AGENTS.md`。ADR-0014 只合理豁免 worktree / coordination,不豁免端级身份。 |

- **超出我预期 / 我自己想不到的地方**:
  - 真正防误判的不是再加主线项,而是给同一 6 行补「状态顺序」:先抓冲突 / 缺失,最后才允许覆盖。
  - `n/a` 必须只进证据,否则表格状态会继续膨胀,又回到背条款。

- **翻车 / 我得纠偏的地方**:
  - brainstorming 技能默认要求 writing-plans,但本仓 STANDARD 明确不走 writing-plans;本轮 owner 及时纠偏后直接实现。
  - 校准报告必须保留「整体右尺寸 / 不是太重」总判,否则严格判定会误变成硬挑毛病。

### 评分(10 分制)

| 维度 | 分 | 备注 |
|---|---|---|
| 设计质量(它做得好不好) | 8/10 | 保住 6 行闸,补判定而非加项 |
| 省力程度(比我自己干省了多少脑子) | 8/10 | media-ops 4 个校准点可快速验 |
| **爽感**(有没有"卧槽这样也行") | 7/10 | 用状态顺序解决误判,没有新协议名 |
| 验证成本(检查它的结果累不累) | 7/10 | 仍需真跑 media-ops 校准报告 |

## 5. 抽一条教训 → 回填 PLAYBOOK

这次学到:在 L2 档,【审计类 skill 误判修正】能交出去,前提是不要继续加主线项,而是把同一覆盖闸的状态、适用性和转换顺序钉死;否则 agent 仍会用单点证据把冲突写成覆盖 / 偏离。

→ 已写入 `PLAYBOOK.md`?[x]
