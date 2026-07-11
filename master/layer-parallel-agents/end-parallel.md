## 并行执行

- 本端标签为 `scope:{{current_end_name}}`；取活可用 `gh issue list --label scope:{{current_end_name}} --state open`。
- 在本端专用 worktree 工作，起手确认 cwd、当前分支和未提交改动；布局与维护见 `docs/project/worktree-isolation.md`。
- 跨端 feature、契约或范围变化写需求 issue 评论，上交根协调角色；不得越界直接改另一端。
- 多 agent 的角色、6+1 流程和消息总线见 `docs/project/collaboration.md`。
