<!-- master/layer-parallel-agents/coordination.md —— 触发:真并行多 agent(= 上 worktree · 多端各端一个 scope agent + coordination)。挂"并行 agent"、不挂"第 2 个人"。
     $sop-init 把这段落进 docs/project/collaboration.md:若也有第 2 个人,接在 layer-collaborators 的双角色段之后;单人多端多 agent 则只有这段。占位符:{{ends}}
     拆分自原 templates/collaboration.md「多端多 agent 追加段」。agent 自决 / 永不阻塞 / 新眼睛 review 全在 STANDARD §1,此处不重复。worktree 机制见同层 worktree-isolation.md。-->

# 多端多 agent 协调（本项目）

> 单端小团队 / 无并行 agent **不用**这份。多端各端一个 scope agent + 一个 coordination（产品方向）+ owner 时才加（蒸馏自 taoxi-geo 829 行 SOP）。

## 角色（多端）

> 每端的**身份 + 边界**写在各端 `<端目录>/CLAUDE.md`（端级文档 · `$sop-init` 按 `ends[]` 生成 · Claude Code 自动加载）。此处只给跨端协作骨架，**不复述各端边界**（单一真相源 · STANDARD §1.7）。

| 角色 | scope | 干啥 |
|---|---|---|
| Coordination | docs | 出"在做什么" + 起跨端 req doc · **不出契约、不写端代码** |
| 各端 scope agent | {{ends}} | 端内代码 + 端内 spec/ADR + 自决实施 · 遇不合理自己解决 + 评论 · **边界见各端 agent 指令文件** |
| Owner | — | 提需求 + 审 req + 联调 + merge · **不当通讯人** |

**scope agent ≠ 执行 PM 派的细 task；= 高权限程序员，收到方向后自决实施。**

> **身份靠"进哪个端的目录"自动定**：Claude Code 自动加载 cwd 最近的 `CLAUDE.md`——在任务 worktree 里 cd 进 `<端目录>/`（如 `.worktrees/<issue>/<端目录>/`）就自动是该端 scope agent；**主仓根 = coordination；任务 worktree 根 = 端身份未定、cd 进端目录才定**（worktree 根上的是任务 agent，不是 coordination）。**端身份 ⊥ worktree**（worktree 按 issue/任务切、非按端）。不靠声明、不靠猜（细则见 `worktree-isolation.md`）。
>
> **错座位护栏**：端内活（端内 spec / 端代码）归 scope agent、在任务 worktree 的该端目录产出；coordination（主仓）只产**跨端 req doc**。**救场**——spec 已误产在主仓：别"释放分支"，按 req-doc 交接走 `push → doc PR(Refs)→ owner merge 进 master → scope agent 新建任务 worktree 从 origin/master 另切实施分支`。

## 6+1 流程骨架（轻 · 无硬 gate）

1. **起需求**：跨端 → coord 起 req doc；单端上下文明确 → 该 scope agent 自起（不必经 coord 中转）。req doc 写"做什么 + 怎么算对 + 主流程 + 形态 + 跨端换什么数据"，**不写实施层**（见 `multiend-contracts.md`）。
2. **取活**：scope label 有 open issue 即"待干"。
3. **细化**：scope agent 跟 owner 定端内方案 → 端内 spec doc → 自检 + 评论 announce（不再单独整体批）。
4. **开发**：切 `<type>/issue-N-slug` 分支（分支名自决）；实施 + 进展 / 变更写 issue 评论；push + PR（默认 `Refs #N`,最终收口才 `Closes #N`）；**过新眼睛 review**（STANDARD §1.2）再示意 merge。
5. **联调**：owner 跑；单端 bug → 该 agent；跨端 mismatch → 评论拍板改哪端。
6. **收口**：别把"做完了"手抄进多个 doc（单一真相源 · STANDARD §1.7）。收工书挡按根 `CLAUDE.md`「取活与书挡」条，不复述。
- **+1 回改 req doc**（可选 · 有重大 deviation 才值）。

## 多 agent 不撞车

- **消息总线**：需求 issue 保持 open 当容器；跨 agent 靠 issue 评论 sync（字段 / 决策 / 跨端影响），别端起手 `gh issue view N --comments` 自动 sync。**doc 写正文、issue 写状态和变更路由、PR 写交付验证**，别让任一件替代另外两件。
- **起手 freshness（不可跳）**：新会话先 `git fetch && 看 behind master`，落后先 sync 再读 SOP——否则读的是旧快照的旧规则。一句话起手报告：`我是 X agent · 在 Y 分支 · behind N · open issue M · 准备干 #K`。**owner 会话顺带扫 `待业务确认`，一条条拍**。
- **scope 隔离**：不动别端代码、不动别人起的 req doc / 契约（要改走 issue 评论提）。识别到要改 req doc / 跨端（含 owner 当面加的扩范围）→ **自己写评论上交 coord + 附设计草案 + 报 owner 一句，别把 escalate 做成 owner 选择题**（§1.9 carve-out）；只"做不做 / 优先级"留 owner 拍。
- **worktree（选项）**：多 agent 真频繁本地冲突才上（按 issue/任务建临时 worktree、用完即清）；否则别上（过度治理）。**上了 → 落 `worktree-isolation.md`（布局 / race trap / 创建+清理 / 起手按-ref-验 / 反转条件）+ 记一条 ADR。**

## 高价值坑（taoxi-geo 复发过的）

- **close-keyword 误关**：doc PR / 中间 PR 用 `Refs #N` 不用 `Closes`；commit 别让 `#N` 紧跟 `close/fix/resolve`（GitHub substring match，会误关 message-bus issue）。需求 issue 由最后一个实施 PR 关。
- **HEAD race**：多 worktree 共享 `.git`，主 worktree 下的端子目录只读，别在那 `git checkout`（会偷 coordination 的 HEAD）。
