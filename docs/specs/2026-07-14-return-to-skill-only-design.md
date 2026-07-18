# sop-better 回归 Skill-only 设计

- 日期：2026-07-14
- 状态：owner 已选择方案 A
- 目标：撤掉 PR #18 引入的客户端与本地 Agent 编排，只保留右尺寸 SOP Skill 和后来验证有效的文字约束。

## 1. 产品边界

`sop-better` 只负责两件事：

- `$sop-init`：按项目真实规模生成右尺寸 SOP。
- `$sop-audit`：检查 SOP 是否太重、缺环、错配或失真。

Codex 负责 Agent 调度，Git / GitHub 负责代码与协作凭据。`sop-better` 不启动、领取、续租、恢复或管理 Agent，也不提供本地客户端、安装器、版本管理器或发布运行时。

## 2. 删除范围

从 PR #18 的最终树删除：

- 全部 Go 程序、测试和 `go.mod`。
- `sopctl`、installer、manager、engine、release bundle、plugin 分发与 schema。
- `$sop-run`。
- Issue claim、lease、heartbeat、capsule、fencing、状态机和恢复逻辑。
- per-Issue worktree、exact-HEAD 临时运行器与任务 CLI。
- 仅为上述程序服务的 CI、fixture、manifest、runtime 设计和实验记录。
- README、Skill、模板和规则中的程序命令、安装升级、Agent Loop 与运行时引用。

最终仓库不得保留“可选高级编排模式”；这不是降级开关，而是删除错误产品方向。

## 3. 保留范围

保留 `origin/main` 已有且与客户端无关的全部文字护栏，包括：

- 人定目标 / 约束 / 验收，AI 负责方案与实现。
- 客观反驳、说人话、诚实查证、方案前调研、执行验证与收口。
- 单一真相源、凭据保真、高风险闸、缺交付物反弹不自补。
- 按端数、真人协作者数、真实并行和风险右尺寸生成结构。
- `$sop-init`、`$sop-audit`、`STANDARD.md`、`master/`、`PLAYBOOK.md` 和有效实验记录。

保留并结晶 PR #18 验证过的 change-first 复审规则：

1. 首轮审任务起点到当前 HEAD 的完整 change，不遍历无关仓库内容。
2. 后续只审上次已审核 HEAD 到当前 HEAD 的全部 change。
3. Reviewer 可顺着 change 查看相关函数、接口、调用方、契约、测试和不变量。
4. 审核基准缺失、范围不连续或影响无法界定时，退回本任务完整 change；不是默认全仓扫描。
5. 记录 reviewed HEAD 和未关闭 finding，保证修改轮次连续覆盖。

exp-039 改写成文字规则实验：保留固定候选、3+3 对照、缺陷命中和耗时结论，删除对任务控制器的实现依赖。`PLAYBOOK.md` 只沉淀该实验能证明的 change-first 结论。

## 4. 右尺寸流程恢复

撤销“所有项目、所有改动一律 Issue + PR”的统一流程，改回按实际需要触发：

- 一次性脚本、局部可逆小改：当前工作区直接完成；不强制 Issue、分支、PR 或 worktree；运行相关验证，代码 change 仍按 change-first 复审。
- 需要远端交付、协作审核或保护分支：使用分支和 PR；Issue 只在需要长期跟踪、跨会话恢复或角色交接时建立。
- 真并行多 Agent、当前工作区有无关 WIP 或明确需要物理隔离：才使用 worktree。
- 高风险、跨端契约或影响边界不清：扩大验证和复审范围，必要时回 owner 决策。

“Agent 自动做所以零成本”不再作为强制仪式的理由。等待时间、失败面、调试成本和 owner 被打断同样计入流程成本。

## 5. 文件落点

- `STANDARD.md`：定义右尺寸流程与 change-first 复审原则。
- `master/base/AGENTS.md`：给下游 Agent 的简短执行约束。
- `master/base/docs/project/issue-pr-workflow.md`：只描述触发 Issue / PR 后怎么做，不再声明人人必走。
- `skills/sop-init/SKILL.md`：继续从 `master/` 按触发增量生成，不依赖程序。
- `skills/sop-audit/SKILL.md`：把无条件 Issue / PR / worktree / 本地编排器列入过度治理检查。
- `README.md`：恢复 Codex-only Skill 仓定位。
- `PLAYBOOK.md` 与 exp-039：保存 change-first 实验证据和适用边界。

## 6. 验收

1. 相对 `origin/main` 的最终 PR diff 不含 `.go`、`go.mod`、`cmd/`、`internal/`、`acceptance/`、`integration/`、`plugin/`、`schemas/`、`manifest.json`、runtime fixture 或程序 CI。
2. 活跃 README、STANDARD、Skill 与 master 中不出现 `sopctl`、`sop-run`、task start / continue、claim、lease、heartbeat、capsule、fencing、per-Issue worktree 或 Agent Loop 执行入口。
3. `skills/` 只保留 `$sop-init` 和 `$sop-audit`。
4. 小脚本不再被强制进入 Issue、分支、PR 或 worktree。
5. worktree 只由真实并行 / 隔离需求触发。
6. change-first 五条在 STANDARD、Agent 母本和 audit 检查中语义一致，不重抄成多套漂移正文。
7. exp-039 的数据、结论和未证明边界保留；exp-037 / exp-038 与客户端运行时记录删除。
8. 删除后运行链接检查、残留关键词检查和 Skill 文本契约检查；没有 Go 程序时不伪造 `go test` 验证。

## 7. 风险与回退

- 大量删除可能误伤后期文字护栏：以 `origin/main` 为保留基线，再逐项移植 change-first，最后用承重墙矩阵反查。
- 旧提交仍包含已删除程序，但 PR 最终树不包含；本轮不改写已推送历史。如以后要清理提交历史，必须另获明确授权。
- PR #18 的标题和正文需改成“回归 Skill-only + change-first 复审”，避免远端凭据继续声称交付 Agent Loop。
