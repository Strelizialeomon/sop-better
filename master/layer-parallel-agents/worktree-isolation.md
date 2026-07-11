# Per-scope Worktree 物理隔离

worktree 为每个并行端提供独立 HEAD、index 和工作区。它解决多个 agent 窗口互相切走分支的问题，不是一套每日审批流程。

## 决策

1. 使用 `git worktree add`，共享一个 Git 对象库，各工作区独立 HEAD。
2. 每端一个长期 worktree，不按 feature 或窗口临时创建。
3. 主工作区只供根协调角色使用，保持在默认分支；端 agent 只在自己的工作区切任务分支。

## 布局

```text
<仓库父目录>/
├── {{project_name}}/       # 主工作区
├── wt-backend/             # 示例端工作区
└── wt-frontend/
```

真实端清单和路径读取 `.sop/profile.json`，不把机器绝对路径写进项目。

## 一次性 setup

```bash
git worktree add --detach ../wt-<端> origin/{{default_branch}}
git worktree list
```

每个工作区独立安装被忽略的依赖和本地配置；密钥仍不得提交。

## 每个 agent 起手

```bash
pwd
git branch --show-current
git status -sb
git fetch origin
git rev-list --count HEAD..origin/{{default_branch}}
```

落后时先保护 WIP，再从 `origin/{{default_branch}}` 建新任务分支；不要为了追新擅自覆盖、rebase 或移动别的工作区。

## 开任务分支

```bash
git fetch origin
git checkout -b <type>/issue-<N>-<slug> origin/{{default_branch}}
git push -u origin <type>/issue-<N>-<slug>
```

- 同一分支不能同时被两个工作区 checkout；遇到占用应回原工作区收尾或换明确的新分支。
- stash 在所有工作区之间共享；使用前标清来源，避免误取别端 WIP。
- 主工作区下的业务子目录不是端 agent 的替代工作区，不在其中切 feature 分支。

## 维护与反转

```bash
git worktree list
git worktree remove <path>
git worktree prune
git worktree repair
```

- 移动整个仓库父目录后运行 repair；有 WIP 时不 prune。
- 若 onboarding 反复进错目录，增加 cwd 启动检查。
- 若共享对象库造成明显性能问题，重新评估多 clone。
- 若目标运行时原生管理隔离目录，重新评估是否简化本约定。
