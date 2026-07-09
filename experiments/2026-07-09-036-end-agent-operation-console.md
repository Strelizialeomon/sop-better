# exp-036 · 端级 AGENTS 不能薄成桥接,要成为端内操作台

- **日期**:2026-07-09
- **真实任务**:检查本机活跃 worktree 项目(go_dispatch / media-ops / taoxi-geo / taoxi_mini)后,修正 sop-better 端级 AGENTS 母本
- **本次撒手档位**:L2(治理 doc 改动,先只改母本与审计口径,不回灌真实项目)

---

## 1. 选活 + 定档

owner 发现 media-ops 的 `server/AGENTS.md` 只有身份 / 边界 / 常读文件,担心端内文件太薄会让 agent 漏读根 SOP 或协作 SOP。这个问题不能只看单个项目,所以抽查所有活跃 `*-root/wt-*`:

- `go_dispatch_root`
- `media-ops-root`
- `taoxi-geo-root`
- `taoxi_mini_root`

爆炸半径:中。改的是 sop-better 母本和技能口径,会影响以后 `$sop-init` 生成的新项目 / 增量端文件;不直接改真实业务项目。

## 2. 便宜验证方案

> 我怎么在 10 分钟内、不逐行读,就看出 AI 做对没?

- 先用 `find` / `wc -l` 看根级与端级 `AGENTS.md` / `CLAUDE.md` 分布。
- 用关键词扫端文件是否覆盖:scope / 取活 / Step 3 / Step 4 / issue 评论 / 新眼睛 review / worktree cwd / Refs-Closes。
- 对照好样本:go_dispatch 和 taoxi_mini 的端内 `CLAUDE.md` 约 34-58 行,能直接指导端内开工。
- 对照坏信号:media-ops Codex 端 `AGENTS.md` 约 20 行,只剩身份 / 边界 / 常读文件;taoxi_mini 的 Codex `AGENTS.md` 多数只是桥接到 `CLAUDE.md`,不符合 Codex-only 方向。

如果上面答不上来,先补的功课是:不要改母本,先跑一次完整 `$sop-audit` 压力样本。

## 3. 给 AI 的简报

```
目标:
把端级 AGENTS 从“身份索引”升级为“端内操作台”,让 Codex 进端目录后能直接知道怎么取活、细化、评论、交付。

约束:
- 不把根 AGENTS 的通用红线整段搬进端文件。
- 不照抄 taoxi-geo / go_dispatch 私有细节。
- 保持右尺寸:端文件服务 agent 自动执行,不增加 owner 手工流程。
- 同步 STANDARD / master / sop-init / sop-audit,避免母本和审计口径漂移。

验收标准:
- end-role-agent.md 覆盖 scope、取活、Step 3、Step 4、评论留痕、review、本端里程碑槽。
- sop-init 说明生成端级“操作台”,不能只生成桥接或常读文件。
- sop-audit 能把“端文件存在但只剩身份/桥接”判为结构错配。
- PLAYBOOK 收一条带 exp-036 编号的教训。
```

## 4. AI 跑完,我来评

- **AI 做了哪些决策**:
  - 没采用“照抄 taoxi-geo 端文件”的方案,而是选 go_dispatch / taoxi_mini 端内 `CLAUDE.md` 的中等厚度形态。
  - 把结构名从“端级身份文档”修正为“端级操作台”,因为身份只是必要条件,不是足够条件。
  - 新增 `{{base_branch}}` / `{{end_milestones}}` / `{{end_high_risk}}` 槽,让不同项目能填自己的基线分支、端特有里程碑和高风险项。
- **超出我预期 / 我自己想不到的地方**:
  - 问题不是“规则缺失”,而是 Codex-only 后承载层缩水:旧项目靠 `CLAUDE.md` 承载端内操作台,新 Codex `AGENTS.md` 只桥接或太薄。
  - `end-role-agent.md` 原本关键词全在,但 Step 3/4 压成一行,属于“可搜到但不好执行”的薄入口。
- **翻车 / 我得纠偏的地方**:
  - 需要避免把审计标准从“端文件要能操作”扩成“端文件必须很长”。行数仍只是信号,最小操作面才是判断点。
  - 未回灌 media-ops / taoxi_mini 等真实项目;这次只修母本,后续需单独走项目 PR。

### 评分(10 分制)

| 维度 | 分 | 备注 |
|---|---|---|
| 设计质量(它做得好不好) | 8/10 | 承载层更准,但还没生成样本项目实跑 |
| 省力程度(比我自己干省了多少脑子) | 8/10 | 用 4 个项目把问题定位到 Codex-only 承载缩水 |
| **爽感**(有没有"卧槽这样也行") | 7/10 | 从“太薄”拆成“规则在别处,但自动加载入口不够” |
| 验证成本(检查它的结果累不累) | 8/10 | rg 关键词 + 母本槽位即可复查 |

## 5. 抽一条教训 → 回填 PLAYBOOK

这次学到:在 L2 档,多端项目的端级文件不能只是身份 + 常读文件;Codex 自动加载的最近 `AGENTS.md` 必须是端内操作台,至少覆盖取活、细化、实施、评论、review 和本端 local。

→ 已写入 `PLAYBOOK.md`?[x]
