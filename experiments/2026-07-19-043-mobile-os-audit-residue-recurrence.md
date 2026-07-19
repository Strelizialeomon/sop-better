# exp-043 · dogfood mobile-os audit:superpowers 残留复发 → §5.2 点名这类残留

- **日期**:2026-07-19
- **触发**:owner 让对 `~/code/mobile-os` 跑 `$sop-audit`,自查结果后判断修 mobile-os 还是 sop-better,改完两边分别 PR。
- **决策**:audit 判 mobile-os SOP 右尺寸健康;唯一实的 finding(`docs/superpowers/` 死工具目录名)是 exp-042 的同源复发 → mobile-os 回灌 rename;复发达 §3.5 阈值 → sop-better 补进工具。

---

## dogfood 结论:如实说"健康",不 cry-wolf

mobile-os = 7 端 / 多角色多并行 agent / 高风险(真机 + 生产 + 付费 API)。治理厚度与复杂度匹配:契约、端级操作台(非身份桥)、worktree 隔离、协作 doc、高风险闸、三件套 + Refs/Closes + 厚评论分层俱全;spec-first 是合理本地增强;workflow 主动写"不要求人手贴 label / 每天报进度 / 逐个低风险 PR 等待"。**按 §5 铁律如实判"没大问题"。**

**逐条排除假阳性**(不硬凑 finding):
- `mouse-control/CLAUDE.md` 的"执行计划"= 事故 / 执行 runbook,不是 writing-plans 残留。
- change-first「记录已审 HEAD」被 mobile-os 的 **PR-per-change 模型**天然覆盖(每个 PR 即一个审查单)——近似≠覆盖,但这里确实覆盖。
- `AGENTS.md` 已由其 ADR-0003 妥善迁移,残留提及都在 ADR 历史正文里,正确保留。

## 新纹路:删除残留会渗进代码,不止治理文件

mobile-os 的 `docs/superpowers/` 引用散在 **8 处**:2 doc 链接 + 5 research + **1 个 ios 测试硬编码路径**(`ios/MechInteractionKit/tests/test_signing_identity.py:149`,断言某 plan doc 存在且含特定内容)。exp-042 在 sop-better 只见于治理文档 + 目录;这次证明**同类残留能渗进测试代码**——去残留必须 `git grep` 扫全仓引用、改完跑受影响测试(本次 rename 后 ios 测试 9 passed),不能只扫治理文件。

## 回灌:让 auditor 点名这类残留(§3.5 复发阈值已到)

exp-042 时"外部工具卸载后的目录 / 技能名残留"是首次出现,按 §3.5「一次踩坑先留记录,复发成模式再改规矩」只记不硬化。现在 sop-better + mobile-os **复发** → 达阈值,正式补进工具:

- `STANDARD §5.2` 删除残留 + `sop-audit` SKILL stale 例:在 `AGENTS.md` / `.codex` 之外,点名"已卸载外部工具遗留的目录 / 技能名(如 `docs/superpowers/`)"。
- 这是 exp-013「审计漂移检查对删除残留是盲的」的再次应验:残留类型换了(Codex→superpowers、文档→测试路径),机制上仍是"删除残留要扫全、双向查"。**没新增闸,只把已有类别的例子点具体**——避免过度治理。

## 产出

- **mobile-os**(PR 到其远端):`docs/superpowers/{specs,plans}` → `docs/{specs,plans}`,更新 8 处引用,ADR 历史正文保留,ios 测试 9 passed。按其 SOP 走隔离 worktree + `$commit-msg`。
- **sop-better**(本 PR):§5.2 + SKILL 例;本 exp;README 近期主线。

## 未完成 / 风险

- mobile-os rename 会让引用了这些 spec 的在飞分支(~9 条)rebase 时冲突 → **PR 不自动合,交 owner 挑安静窗口**。
- sop-better 侧仍静态自审,未在第三个项目复跑验证新例真能逮到该类残留。
