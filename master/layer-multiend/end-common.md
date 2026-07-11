# {{current_end_name}} Agent · 端级操作台

你在 `{{current_end_path}}/` 工作，负责该端源码、配置、测试、migration 和端内文档。

## 范围与取活

- 负责范围：`{{current_end_path}}/`；不改其它端代码，不静默改已 freeze 契约或全局 ADR。
- 取活：先看 open issue 与最新评论，再判断它是否属于本端；链接、验收或上游契约缺失时评论反弹。
- 跨端事实读取 `docs/contracts/`；通用红线读取仓库根 `AGENTS.md`；完整流程读取 `docs/project/issue-pr-workflow.md`。

## 端内流程速查

1. **Freshness**：保护本地改动；任务依赖远端时 fetch，并比较当前 HEAD 与 `origin/{{default_branch}}`。
2. **Step 3 细化**：非简单修复先做端内 brainstorm，落可检验 spec；spec review 必须在收口后、交实现前完成，三查一手实证与信源、验收可机检、范围 / 形态 / 边界；只有琐碎 spec 可免。
3. **Step 4 实施**：从确认过的 `origin/{{default_branch}}` 创建 `<type>/issue-N-<slug>` 分支，自决实现边界细节。
4. **变化留痕**：范围、字段、endpoint、schema、依赖、算法或跨端影响改变时，立刻写需求 issue 评论。
5. **交付**：commit 前走 `$commit-msg`；PR 默认 `Refs #N`，最终收口才 `Closes #N`；代码新眼睛必须收到 spec / 验收、diff 和决策快照，再按风险合并。

## 本端 local

- 技术栈、常读文件、实施词汇、评论里程碑和端特有高风险项，以 `.sop/profile.json` 中当前端条目为准。
- 本端输出进入跨端契约前，必须完成相关端联调并留下验证证据。
