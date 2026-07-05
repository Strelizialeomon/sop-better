# exp-027 · doc/issue/PR 三件套协作主线

- **日期**:2026-07-05
- **真实任务**:用户指出本轮真正目标是补开发↔开发、开发↔业务协作,不是继续优化单次沟通闭环。需要把 media-ops / taoxi-geo 里 issue / PR / doc 的协作经验收进 sop-better 主线。
- **本次撒手档位**:L3(改 STANDARD / 母本 / sop-audit / README,影响后续生成与审计)

---

## 1. 选活 + 定档

上轮 agent 做偏:把用户的"无语义新增、瘦身重排"当成本轮目标,只整理了沟通闭环。用户真正要的是协作流:

业务要什么 → 文档沉淀 → issue 索引和流转 → 开发接活 → 开发间同步 → PR 交付验证 → issue/doc 收口。

爆炸半径:

- 写太轻:只在 README 说一句三件套,agent 实操时仍不知道业务↔开发、开发↔开发怎么交接。
- 写太重:把 media-ops 的 label 状态机或 taoxi-geo 的 829 行并行细则搬成所有项目默认,违背右尺寸。
- 写歪:让 issue 替代 doc,或让 PR 替代 issue 评论,三件套反而更乱。

## 2. 便宜验证方案(动手前必须答)

> 我怎么在 5 分钟内、**不逐行读**,就看出 AI 做对没?

- 验证手段:
  - `rg 'doc = 正文|issue = 索引|PR = 交付|协作主线|协作总线断节' STANDARD.md README.md master skills`
  - 看 diff 是否主要落在 `issue-pr-workflow.md` / `collaboration.md`,而不是继续扩 `查证 → 分流 → 调研 → 执行验证 → 收口`。
  - `git diff --check`
- 如果上面答不上来 → 先停,说明又把协作主线做成口号或协议堆。

## 3. 给 AI 的简报(只给目标+约束+验收,不给解法)

```
目标:
把 SOP 的协作主线补齐成 doc/issue/PR 三件套 flow,覆盖业务↔开发、开发↔开发交接。

约束:
- 不继续扩沟通闭环。
- 不把项目本地 label / 并行细节搬成所有项目默认。
- 不让 issue 代替 doc,也不让 PR 代替 issue 评论。
- STANDARD 只定原则,workflow / collaboration 放操作入口,sop-audit 只指向 §5。

验收标准:
- STANDARD 明确 doc/issue/PR 分工。
- issue-pr-workflow 有三件套分工和完整协作 flow。
- collaboration 有业务→开发、开发→业务、开发→开发、PR 收口路由。
- sop-init 对 issue-pr-workflow 的生成说明不漂移。
- sop-audit 能查三件套分工失真 / 协作总线断节。
```

## 4. AI 跑完,我来评

- **AI 做了哪些决策**(它替我想的部分):
  - 把三件套定义放进 STANDARD §1.8,把具体 flow 放进 `issue-pr-workflow.md`。
  - 在 `collaboration.md` 只写人/agent 交接路由,不复制 issue 生命周期。
  - 把多 agent 协调里的消息总线改成三件套口径,避免 issue 吞掉 doc/PR。
  - `$sop-init` 的生成说明补成「三件套分工 + 全生命周期 + 凭据保真」,只作指针不重抄。
  - `$sop-audit` 只更新 severity 映射,不重抄 STANDARD 查法。
- **超出我预期 / 我自己想不到的地方**:
  - “协作不是多一个文档,而是三件套各司其职”比单独强调 issue/PR 更准。
- **翻车 / 我得纠偏的地方**:
  - 新眼睛发现两处小漂移:base flow 写死"业务 / coord"不兼容开发自起;collaboration 的 req doc 推送口径和 workflow 默认口径冲突。已改成发起方 / 上游角色,并统一到 `issue-pr-workflow.md`。
  - `gh issue view` 用猜的仓库名失败,不能把 GitHub issue 实况当本轮证据;本轮证据只用本地 media-ops / taoxi-geo 文档和既有 exp 记录。

### 评分(10 分制)

| 维度 | 分 | 备注 |
|---|---|---|
| 设计质量(它做得好不好) | 8/10 | 主线回到协作,但还需生成样本复验 |
| 省力程度(比我自己干省了多少脑子) | 8/10 | 把散点收成三件套 flow |
| **爽感**(有没有"卧槽这样也行") | 7/10 | 从 issue/PR 规则升成协作凭据分工 |
| 验证成本(检查它的结果累不累) | 8/10 | rg + diff 可快速看 |

## 5. 抽一条教训 → 回填 PLAYBOOK

这次学到:在 L3 档,【协作 SOP】要先定 doc/issue/PR 的分工,再写业务↔开发、开发↔开发的 flow;否则规则会散成 issue、PR、doc 的孤立条款。

→ 已写入 `PLAYBOOK.md`?[x]
