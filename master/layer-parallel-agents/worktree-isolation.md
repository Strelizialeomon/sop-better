<!-- master/layer-parallel-agents/worktree-isolation.md —— 仅"真并行多 agent"(与端数无关,单端也触发)。worktree 是并行协作里的 (可选) 项,不是默认。
     选用条件:你真的同时开 ≥2 个 agent 窗口改同一个仓。串行干 / 单 agent → 别上(过度治理)。
     HEAD-race 机制蒸自 taoxi-geo(ADR-0007);per-issue 粒度 / 仓内布局 / 清理纪律蒸自 mobile-os 实测。
     $sop-init 在触发并行层时与 parallel-agents.md 成对落本文件(为 docs/project/worktree-isolation.md;文件先发、上不上 worktree 看头部门禁,动作可选),
     并在 parallel-agents.md 的 "worktree(选项)" 节指过来 + 用 adr-template.md 记一条 ADR(含下方反转条件)。
     占位符:{{project}} 项目名。-->

# Per-task Worktree 物理隔离（真并行 · 可选）

> **本质——靠物理隔离，不靠流程控制**：worktree 是"一次性把隔离做进物理结构"，建完**日常控制 ≈ 0**（git 自己保证各窗口 HEAD 不互扰，只要各待各的目录）。下面大半篇幅是"怎么摆台子 + 有哪几个坑 + 啥时该反悔"，**不是一套天天要跑的控制流程**——别被行数吓到。
>
> **要解的病（只有它成立才上）**：多个 agent 窗口**同时**开工时，git 的 **HEAD 是整仓共享的全局变量**——不随你 `cd` 进哪个子目录变。
> A 窗口一 `git checkout`，就把 B 窗口的 HEAD 偷走，B 的未提交改动（WIP）跟着飘到错的分支。
> worktree 给每个**并行**任务一份独立 HEAD / index / 工作区，这病就没了。
> **没有并行 agent = 没有这病 = 别上 worktree（那是为没有的形态预建 · STANDARD §3.5 最坏的过度治理）。**

## 决策（三条一起 · HEAD-race 机制蒸自 taoxi-geo ADR-0007，粒度/布局蒸自 mobile-os 实测）

1. **多 worktree 模型**：`git worktree add`，共享同一个 `.git/`（在主 worktree），各 worktree 独立 HEAD / index / 工作区。（不选多 clone：磁盘 ×N、history 重复；不选单 worktree：就是要解的那个 race。）
2. **per-task 粒度 + 名字必须带 issue 号**：真要开 worktree 时按 **issue / 需求 / task** 切（一任务一个），**不是** per-scope（按端）、**不是** per-window；**目录名就是 `.worktrees/issue-N`、分支名就是 `feat/issue-N`**，不加 slug、不加后缀、前缀全项目钉死一个——并行取活时**建 worktree 就是抢坑动作**，git 靠"同名分支 / 同名目录已存在"做互斥（实测：同名分支 exit 255、同名目录 exit 128、新号 exit 0）。**判据是"两个 agent 对同一 issue 必然生成同一个名字"，不是"名字里带了 issue 号"**——`issue-6-add-login` 与 `fix/issue-6-login-bug` 都带号，实测照样双双 exit 0（取活纪律见 `parallel-agents.md`）；但 worktree 本身可选、**不是每个 task 都建**（串行 / 单 agent 别上，见头部门禁）。理由（mobile-os 实测）：并行任务数量变化快（按端永久预建要么建一堆空的、要么同端两任务并行时卡死），且一个任务常跨相邻端（按端 worktree 装不下跨端任务）。
3. **on-demand 用完即弃**：开工临时建、合并即清，**不永久预建**（永久预建 = 为没有的形态预建 · STANDARD §3.5）。

## 布局（仓内 `.worktrees/`，gitignore）

```
{{project}}/                       ← 主 worktree（含 .git/）· 协调/只读窗口用（多端时即 coordination）· HEAD 永远停 master
└── .worktrees/
    ├── issue-17/                  ← 某任务一份完整 linked worktree
    └── issue-33/                  ← 每个 issue/需求/task 一份 · 名字就是 issue-N,不加 slug
```

- `.worktrees/` **必须被 git 忽略**（由仓库 `.gitignore` 统一忽略）；建前先 `git check-ignore -v .worktrees/` 确认（**必须带尾斜杠**——`xxx/` 型 pattern 对不存在的无斜杠路径不匹配，无尾斜杠时建前恒 exit 1 误报，实测复现）——**未被忽略则先 `echo '.worktrees/' >> .gitignore` 补上并 commit 再建**（否则主仓 `git status` 会把整个 worktree 当未跟踪、易被误 `git add`，新 clone 也拿不到忽略规则）。
- **〔多端〕端身份 ⊥ worktree（正交）**：端身份靠 **cwd 最近的 `CLAUDE.md`** 定（进哪个端子目录就是哪端 agent）；worktree 只管"这是哪个任务"。在 `.worktrees/issue-N/<端目录>/` 就自动是该端 scope agent——两件事各管各的，别把身份跟 worktree 名绑死。

## 头号铁律 —— HEAD race trap

- **绝不在主 worktree 跑 `git checkout <feature-branch>`**——会偷走主仓窗口（多端时即 coordination）的 HEAD。主仓 HEAD **永远停 master**。
- 〔多端〕主仓下的**端子目录只读**：在那 `git checkout` = 踩 race。
- 实现分支的 `git checkout` **只在任务 worktree（`.worktrees/<task>/`）里跑**。
- **主仓禁裸跑 `git clean -fdx` / `-fdX`**：`.worktrees/`（走原生路径则是 `.claude/worktrees/`）是 ignored 目录——**主仓眼里，所有并行 agent 的工作区都是"垃圾"**，一条 clean 把它们连同未提交的 WIP 一起删光，**不可逆**（仓外同级布局免疫此雷，仓内 / 原生布局必须补这道闸）。
- **可选加一层响铃（是警报，不是闸）**：`.githooks/post-checkout` 放个脚本（记得 `chmod +x`），主仓 HEAD 一离开 master 就喊一声（**每个 clone 各配一次**：`git config core.hooksPath .githooks`——它是 repo 级本地配置，不是机器级；且设了它会**整体停用** `.git/hooks/` 下的默认钩子）。**限度要说清**——git **没有** `pre-checkout` 钩子，它是**切完才跑**（事后报警，拦不住）；而且只在切换那一刻响，**会话开始时就已经跑偏的，它一辈子不响**。当补丁用，别当闸用，更别因为装了它就放松上面三条铁律。

## 创建一个任务 worktree（on-demand）

**并行取活必须走手动 `git worktree add`（见下），不走原生**——原生 `EnterWorktree` **没有分支名参数**（入参只有 `name` / `path`，分支由它自己起），而**分支名才是主锁**（实测：不同目录 + 同分支 → exit 255，先撞的是分支）；加上它建在 `.claude/worktrees/` 而手动建在 `.worktrees/`，**父目录不同、目录名永不相撞**（实测：`.worktrees/issue-6` 已存在时 `git worktree add .claude/worktrees/issue-6` → exit 0）。**A 走原生、B 走手动 → 双双成功、双双开工，锁是空的。**

**原生 `EnterWorktree` 仅用于非取活场景**（临时隔离、单 agent 探索）：会话内自动在 `.claude/worktrees/<task>/` 建目录 + 新分支并切会话；默认 baseRef=fresh 即从 `origin/<默认分支>` 新建，正合"不从主仓 HEAD 猜基线"；退出用 `ExitWorktree`（keep / remove，remove 对未提交改动有拒删闸）。原生路径同样建前 `git check-ignore -v .claude/worktrees/` 验忽略。**caveat**：原生退出清理只管**本会话原生建**的 worktree——手动建的、跨会话遗留的（含原生 keep 下来的），仍走下方手动创建 / "合并即清"清理。

**手动 fallback**（脚本化 / CI / 无原生支持时）：

```bash
cd {{project}}                                             # 主 worktree
git fetch origin --prune
git worktree add .worktrees/issue-N \
  -b feat/issue-N origin/master                    # 从 origin/master 新建,不从主仓当前 HEAD 猜基线
git -C .worktrees/issue-N status -sb                 # 建后验:分支 / 脏文件 / behind
```

- gitignored 的 `node_modules/` / Python venv / 设备标定 / 各端 local config **每个 worktree 各一份**——这是 per-task 切法的代价，首次进各自配。**重环境端**（要重装 venv / 重跑标定的）可保留**一个长驻 worktree** 当例外，不必每任务重付。

## 每个 agent 起手（不可跳 · worktree 特有）

worktree 各自的 HEAD 是**独立的旧快照**——不 fetch 就读到旧 SOP / 旧代码（可能含已废除的红线）。

```bash
pwd                                                          # 我在哪个 worktree / 哪个端子目录
git branch --show-current                                    # HEAD 在哪个分支
git status                                                   # 工作树干不干净
git fetch origin && git rev-list --count HEAD..origin/master # behind N?
```

`behind > 0` → **先 sync 到 origin/master 再信本地 SOP / 代码**，sync 后**重读起手要读的 doc**。

> 这就是"凭据要**按 ref 验**、别按本地旧快照信"的落地（STANDARD §1.8）：worktree 物理隔离 = 各 HEAD 是各自的旧快照，不 fetch 就全程零信号读错规则。

## 清理（合并即清 · 压 worktree 泛滥）

per-task 没有数量上限，**不清就堆成坟场**。本会话原生建的 → `ExitWorktree`（remove 自带拒删未提交改动的闸，与本节同向）；手动建的 / 跨会话遗留的（含原生 keep 下来的——路径换 `.claude/worktrees/<task>` 同理），PR 合并 / 任务明确废弃后：

```bash
git -C .worktrees/issue-N status --short             # 有无普通 WIP
git -C .worktrees/issue-N status --short --ignored   # ignored 产物(.venv / runs/ / 标定 / 设备 config)
```

- 先读回 PR/issue 确认**分支确已合并**（任务废弃则要有明确凭据）。
- `status --short` 为空只证明没普通 WIP，**不证明 ignored 产物可丢**——重点盘点 venv、运行产物、标定、本地设备配置；有价值先迁到明确位置。
- 以上全过才 `git worktree remove .worktrees/issue-N && git worktree prune`。**禁 `git worktree remove --force`**，不强删未知 WIP。
- **删远端分支是另一项动作**，需 owner 明确授权（项目宜将其列入根高风险闸的 `{{risk_gate_items}}`），不随本地 remove 自动执行。

## 其它须知

- **`git stash` 跨 worktree 共享**：所有 wt 共用一个 `.git/`，`git stash list` 看到的是同一份——名字必须带 issue/任务信息；优先 commit 到任务分支而非长期 stash。
- **同一分支不能同时在两个 worktree checkout**（git 报 `already checked out`）——**这正是抢坑闸生效的样子，不是要你绕开**：换一条 issue 去做，或在原 worktree 收尾。**别改名重建**（`issue-5b` / `feat2/issue-5` 之类）——那等于把锁拆了。
- **维护 + 搬家**：`git worktree list`（列）/ `remove <path>`（移除，要 clean）/ `prune`（清元数据）/ `repair`（修指针）。整体 `mv {{project}}/`（主仓带 `.worktrees/` 一起搬）后用 `repair` 修指针，**先冷备份、确认无 WIP 再 prune**。

## 反转条件（任一发生 → 起新 ADR 重新决策）

| 触发 | 反转方向 |
|---|---|
| onboarding 老踩 race（误在主仓端子目录 `git checkout`） | 加 cwd 检查 wrapper / shell alias，强制启动校验在不在 `.worktrees/` |
| 单 `.git/` 出现性能瓶颈（多 worktree 大 fetch / GC 慢） | 拆成多 clone 独立仓 |
| 单机协作变多机 / owner 加协作者 | 每个协作者各自 `git clone` + 各自 `.worktrees/` |
| worktree 数量长期居高、清理欠债 | 收紧"合并即清"为强制闸 / 加陈旧 worktree 巡检 |
| ~~目标 agent 运行时原生支持 worktree~~ | **已执行（exp-047）后被部分反转（exp-054）**：原生 `EnterWorktree` 曾收编为首选；但它**没有分支名参数**、且建在另一个父目录下，用于并行取活时锁是空的 → **取活退回手动 `git worktree add`**，原生只留非取活隔离。**下次原生支持指定分支名时可重新收编** |
