# exp-018 · commit 前强制走 commit-msg

- **日期**:2026-07-02
- **真实任务**:用户追问上一条提交是否使用了 `commit-msg` skill;实际没有,需要把规则写进常驻约束和收工流程。
- **本次撒手档位**:L1(小范围文档护栏 + grep 验证)。

---

## 1. 选活 + 定档

缺口很窄:不是提交策略整体错,而是 commit message 这一步可以被 agent 手写绕过。只需要在本仓 `AGENTS.md` 加立即生效的常驻规则,并在 `master/base/docs/project/issue-pr-workflow.md` 加下游项目会继承的提交信息步骤。

## 2. 便宜验证方案

- 验证本机确实存在 `~/.codex/skills/commit-msg/SKILL.md`。
- grep 确认 `AGENTS.md` 和 workflow 母本都出现 `$commit-msg`。
- `git diff --check` 确认文档格式无空白错误。

## 3. 教训

只把“提交前要验证”写进流程不够,commit message 这种细小动作仍会被 agent 用 `git commit -m ...` 顺手绕过。要把它写成贴近动作的闸:凡要 commit,先用 `$commit-msg` 读 diff 生成/校验 message;无法调用必须明说。
