# ADR-0002: 并行端使用 per-scope worktree

- **日期**:{{initialized_on}}
- **状态**:已采纳

## 背景

{{project_name}} 已启用多端真并行 agent。多个窗口共用一个工作区时，切分支会互相改变 HEAD，并把未完成改动带到错误分支。

## 决策

- 每个端使用一个长期 worktree；主工作区供根协调角色使用。
- 端工作区都从 `origin/{{default_branch}}` 获取新鲜基线，再开任务分支。
- 不使用 per-feature 临时工作区，也不把机器绝对路径写进项目凭据。

## 影响与反转条件

隔离成本主要发生在一次性 setup 和每个工作区的本地依赖。若运行时原生接管隔离、共享对象库成为瓶颈或 onboarding 持续误用，重开 ADR 评估简化或多 clone。
