# exp-026 · Refs/Closes 示例漂移修正

- **日期**:2026-07-05
- **真实任务**:复核 exp-025 后,发现多 agent / worktree 母本里仍有 `Closes #N` 示例,base workflow 也有「否则 agent 手动关」歧义,和「message-bus issue 由最后一个实施 PR 关」红线冲突。
- **本次撒手档位**:L2(只改母本文案的一致性,不改流程语义)

---

## 1. 选活 + 定档

这不是新增规则。`issue-pr-workflow.md` 已经写了 PR 正文默认 `Refs #N`,收口才 `Closes #N`;`coordination.md` 也已经写了 doc PR / 中间 PR 不能误关 message-bus issue。

风险在于:示例比原则更容易被 agent 照抄。前面一句 `PR(Closes #N)`、base workflow 一句「否则手动关」都会压过后面一段 close-keyword 红线,让中间 PR 把容器 issue 误关。

## 2. 便宜验证方案(动手前必须答)

> 我怎么在 3 分钟内、**不逐行读**,就看出 AI 做对没?

- 验证手段:`rg 'Closes #N|Refs #N|message-bus|最终收口|最后一个实施 PR|否则 agent 手动关' master`。
- 通过标准:层级文档里的短流程不再把 `Closes #N` 当默认动作;base workflow 不再把 `Refs` 合并后顺手关 issue 写成默认动作。

## 3. 给 AI 的简报(只给目标+约束+验收,不给解法)

```
目标:
修掉母本里会诱导误关 issue 的 Closes 示例漂移。

约束:
- 不新增 issue/PR 语义。
- 不把 parallel 层规则重抄到所有文件。
- 只改示例 / 收口文字和 close-keyword 提醒,不改流程语义。

验收标准:
- collaboration / end-role / coordination / worktree 示例默认写 Refs 或指向 workflow。
- base workflow 明确 `Refs` 不因合并而关 issue。
- 仍保留最终收口才 Closes 的信息。
- git diff --check 通过。
```

## 4. AI 跑完,我来评

- **AI 做了哪些决策**(它替我想的部分):
  - 把 `PR(Closes #N)` 改成「默认 `Refs #N`,最终收口才 `Closes #N`」。
  - worktree 命令示例只给 `Refs #N`,避免命令片段诱导误关。
  - 新眼睛发现 base workflow「否则 agent 手动关」仍会诱导误关后,把它收窄成「只有验收 / 关闭条件满足才手动关」。
  - close-keyword 红线补上「中间 PR」,对齐 message-bus issue 语义。
- **超出我预期 / 我自己想不到的地方**:
  - 这类漂移不是规则缺失,而是**示例覆盖规则**:agent 会照最短可复制片段执行。
- **翻车 / 我得纠偏的地方**:
  - 首轮只修了短流程示例,漏了 base workflow 的手动关歧义;靠新眼睛复审补上。
  - 暂未真跑 `$sop-init` 生成多端并行样本;本轮只做静态母本一致性修正。

### 评分(10 分制)

| 维度 | 分 | 备注 |
|---|---|---|
| 设计质量(它做得好不好) | 8/10 | 修的是真实冲突,没有另起制度 |
| 省力程度(比我自己干省了多少脑子) | 7/10 | rg 一把能验 |
| **爽感**(有没有"卧槽这样也行") | 6/10 | 小修,但避免一个高频误关坑 |
| 验证成本(检查它的结果累不累) | 9/10 | grep + diff 就能看明白 |

## 5. 抽一条教训 → 回填 PLAYBOOK

这次学到:在 L2 档,【母本短流程示例】可以交给 agent 修,前提是只修示例和既有红线的一致性,不借机新增语义。

→ 已写入 `PLAYBOOK.md`?[x]
