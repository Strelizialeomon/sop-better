# exp-032 · sop-audit 覆盖闸对账校准

- **日期**:2026-07-06
- **真实任务**:`media-ops` `$sop-audit` 报告已比上一版进步,但仍把结构触发写成部分覆盖、证据行偏弱、主线表异常未进入 findings
- **本次撒手档位**:L2(低风险文档 / skill 改动,但 live skill 会影响后续审计,需真项目只读校准)

---

## 1. 选活 + 定档

exp-031 把 `$sop-audit` 的状态判定补严,但 owner 复核 `media-ops` 报告后发现 4 个细误差:

- 多端缺端级 `AGENTS.md` 仍被写成「部分覆盖」,应是 `缺失 / P2 mismatch`。
- doc / issue / PR 三件套证据引用了弱证据,没有指向真正描述协作分工的行。
- Refs / Closes 证据行号没指到危险默认值本身。
- 主线表写了「部分覆盖」,findings 却没有对应处理。

爆炸半径:改的是 live `$sop-audit` 和权威 `STANDARD.md` 的触发口径。动作仍是文档层,但会影响后续所有 audit,所以需要真项目压力校准。

## 2. 便宜验证方案(动手前必须答)

> 我怎么在 10 分钟内、**不逐行读**,就看出 AI 做对没?

- 验证手段:读 `STANDARD.md` / `$sop-init` / `$sop-audit` 关键段,再用 `media-ops` 这份报告做压力断言。
- 通过标准:
  - STANDARD 和 `$sop-init` 不再写「多端 = 默认 worktree」。
  - `$sop-audit` 明确多端缺 `docs/contracts/` 或端级 `AGENTS.md` 任一必备 surface 时,结构触发写 `缺失` 并进 P2 `mismatch`。
  - `$sop-audit` 明确证据行必须直接证明判断,弱证据不能证明三件套 / 风险 / 收口分工。
  - `$sop-audit` 明确主线表异常必须进 findings,不进则收尾卡写豁免理由。
  - 删除旧 `docs/superpowers/plans/` implementation plan 残留,避免继续诱导 executing-plans。

## 3. 给 AI 的简报(只给目标+约束+验收,不给解法)

```
目标:
把 media-ops audit 复核暴露的 4 个误差,校准进 sop-better 的权威口径和 sop-audit skill。

约束:
- 不新增第 7 个主线项。
- 不把 media-ops 路径硬编码进 skill。
- 不走 writing-plans。
- 保留"整体右尺寸"的判断能力,不能为严判而 cry wolf。
- 只做小补丁,不要扩成新协议名。

验收标准:
- 多端 / worktree 触发口径在 STANDARD 与 sop-init 中一致。
- sop-audit 对结构触发缺必备 surface 的状态不再允许写部分覆盖。
- sop-audit 的主线表和 findings 必须对账。
- sop-audit 的证据要求能防止拿弱证据证明强判断。
```

## 4. AI 跑完,我来评

- **AI 做了哪些决策**:
  - 把 `STANDARD.md` 参数表和演进 ADR 文案里的 worktree 触发拆出来,只挂到真并行多 agent。
  - 把具名项目例子改成抽象形态例子,避免项目演进后 STANDARD 自己变成 stale 凭据。
  - `$sop-audit` 不新增主线项,只补证据强度、结构触发 hard rule、主线表 findings 对账。
  - 删除旧 implementation plan 文件,保留 spec / experiment 作为历史依据。

- **超出我预期 / 我自己想不到的地方**:
  - 这次真正的根因不是报告写得不细,而是权威口径里还有「多端 = worktree」的旧影子。先修权威口径,再修 audit 判定,比只训 audit 更稳。

- **翻车 / 我得纠偏的地方**:
  - `superpowers:writing-skills` 推荐完整 TDD / subagent 压测,但本仓当前活是小型 live 文档补丁;本轮用真项目断言 + diff 校验替代重流程,避免把 SOP 校准做厚。

### 评分(10 分制)

| 维度 | 分 | 备注 |
|---|---|---|
| 设计质量(它做得好不好) | 8/10 | 修口径而不是加协议 |
| 省力程度(比我自己干省了多少脑子) | 8/10 | owner 给出的 4 个校准点都能对应验 |
| **爽感**(有没有"卧槽这样也行") | 7/10 | 发现 STANDARD 自己的 stale 例子和 worktree 旧影子 |
| 验证成本(检查它的结果累不累) | 8/10 | grep + media-ops 报告断言即可验 |

## 5. 抽一条教训 → 回填 PLAYBOOK

这次学到:在 L2 档,【审计 skill 的覆盖闸校准】能交出去,前提是主线表异常必须和 findings 对账,且证据行要直接证明判断;否则 agent 会查到异常但在报告收口时把它漏掉。

→ 已写入 `PLAYBOOK.md`?[x]
