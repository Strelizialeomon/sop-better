<!-- master/layer-parallel-agents/worktree-isolation.md —— 仅多端且"真并行多 agent"。worktree 是多端协作里的 (可选) 项,不是默认。
     选用条件:你真的同时开 ≥2 个 agent 窗口改同一个仓(每端一个)。串行干 / 单 agent → 别上(过度治理)。
     蒸馏自 taoxi-geo(ADR-0007 + collaboration §13 实战)。$sop-init 仅在 owner 选上 worktree 时落本文件,
     并在 collaboration.md 多端追加段的 "worktree(选项)" 行指过来 + 用 adr-template.md 记一条 ADR(含下方反转条件)。
     占位符:{{project}} 项目名 · {{ends}} 端列表(每端一个 wt-)。-->

# Per-scope Worktree 物理隔离（多端 · 可选）

> **本质——靠物理隔离，不靠流程控制**：worktree 是"一次性把隔离做进物理结构"，setup 完**日常控制 ≈ 0**（git 自己保证各窗口 HEAD 不互扰，只要各待各的目录）。下面大半篇幅是"怎么摆台子 + 有哪几个坑 + 啥时该反悔"，**不是一套天天要跑的控制流程**——别被行数吓到。
>
> **要解的病（只有它成立才上）**：多个 agent 窗口**同时**开工时，git 的 **HEAD 是整仓共享的全局变量**——不随你 `cd` 进哪个子目录变。
> A 窗口一 `git checkout`，就把 B 窗口的 HEAD 偷走，B 的未提交改动（WIP）跟着飘到错的分支。
> worktree 给每个**并行** agent 一份独立 HEAD / index / 工作区，这病就没了。
> **没有并行 agent = 没有这病 = 别上 worktree（那是为没有的形态预建 · STANDARD §3.5 最坏的过度治理）。**

## 决策（三条一起 · 蒸自 taoxi-geo ADR-0007）

1. **多 worktree 模型**：`git worktree add`，共享同一个 `.git/`（在主 worktree），各 worktree 独立 HEAD / index / 工作区。（不选多 clone：磁盘 ×N、history 重复；不选单 worktree：就是要解的那个 race。）
2. **per-scope 粒度**：每个端 `{{ends}}` 一个 worktree。**不是** per-feature、**不是** per-window。
3. **永久预建**：onboarding / 拉新机器时一次建好，不每次临时 on-demand。

## 布局（铁律：主仓 + 各 wt-* 必须同级）

```
~/code/{{project}}-root/
├── {{project}}/        ← 主 worktree（含 .git/）· coordination 用 · HEAD 永远停 master
├── wt-<scopeA>/        ← scopeA agent 用
├── wt-<scopeB>/
└── ...                 ← 每端一个 · 与主仓同级、不嵌套
```

scope agent 只在自己的 `wt-<scope>/` 里干活；**不在**主仓下的同名子目录工作（那是 race trap，见下）。

## 头号铁律 —— HEAD race trap

- **绝不在主 worktree 跑 `git checkout <feature-branch>`**——会偷走 coordination 窗口的 HEAD。主仓 HEAD **永远停 master**。
- 主仓下的**同名子目录只读**：在那 `git checkout` = 踩 race。
- scope agent 的 `git checkout` **只在自己 `wt-<scope>/` 里跑**。

## 一次性 setup（owner 拉新机器）

```bash
cd ~/code/{{project}}-root/{{project}}              # 主 worktree
git worktree add --detach ../wt-<scope> master      # 每个端一行（替 <scope>）
git worktree list                                   # 验证（含主 worktree）
```

- gitignored 的 `node_modules/` / Python venv / 各端 config **每个 worktree 各一份**，首次 setup 各自配。

## 每个 agent 起手（不可跳 · worktree 特有）

worktree 各自的 HEAD 是**独立的旧快照**——不 fetch 就读到旧 SOP / 旧代码（可能含已废除的红线）。

```bash
pwd                                                          # 我在哪个 worktree
git branch --show-current                                    # HEAD 在哪个分支
git status                                                   # 工作树干不干净
git fetch origin && git rev-list --count HEAD..origin/master # behind N?
```

`behind > 0` → **先 sync 到 origin/master 再信本地 SOP / 代码**（clean 树直接 `git checkout -b <type>/issue-<N>-<slug> origin/master` 开新分支），sync 后**重读起手要读的 doc**。

> 这就是"凭据要**按 ref 验**、别按本地旧快照信"的落地（STANDARD §1.8）：worktree 物理隔离 = 各端 HEAD 是各自的旧快照，不 fetch 就全程零信号读错规则。

## 开新 feature（scope agent）

```bash
cd ~/code/{{project}}-root/wt-<scope>
git fetch origin
git checkout -b <type>/issue-<N>-<slug> origin/master   # 不必 checkout master：主仓已 active，切不过去
# 干活 → git push -u origin <branch> → gh pr create ... Closes #N
```

- **`git stash` 跨 worktree 共享**：所有 wt 共用一个 `.git/`，`git stash list` 看到的是同一份——可拿它做临时跨端 hand-off（A 端 stash、B 端 pop）。
- **同一 feat 分支不能同时在两个 worktree checkout**（git 报 `already checked out`）——同端一时刻通常只一条 active feat，撞上别懵：换分支名或在原 worktree 收尾。

## 维护命令

```bash
git worktree list                  # 列 worktree
git worktree remove <path>         # 移除（要求 clean）
git worktree prune                 # 清理已删 worktree 的元数据
git worktree repair                # 修元数据指向旧路径（主仓 / 容器搬家后）
```

- **容器搬家**：整体 rename / `mv {{project}}-root/`（主仓 + 所有 wt-* 一起搬）后用 `git worktree repair` 修指针；**先冷备份、确认无 WIP 再 prune**，别急着删。

## 反转条件（任一发生 → 起新 ADR 重新决策）

| 触发 | 反转方向 |
|---|---|
| onboarding 老踩 race（误在主仓同名子目录 `git checkout`） | 加 cwd 检查 wrapper / shell alias，强制启动校验在不在 wt-* |
| 单 `.git/` 出现性能瓶颈（多 worktree 大 fetch / GC 慢） | 拆成多 clone 独立仓 |
| 单机协作变多机 / owner 加协作者 | 每个协作者各自 `git clone` + 各自 wt-* |
| 目标 agent 运行时原生支持 worktree（按子目录自动推 path） | 简化本 SOP |
