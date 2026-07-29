<!-- master/layer-parallel-agents/parallel-agents.md —— 触发:真并行多 agent(与端数无关,单端也触发;串行 / 单 agent 不发)。
     $sop-init 落成 docs/project/parallel-agents.md,与 worktree-isolation.md 成对生成、两份互指。
     多端时另有 coordination-multiend.md(角色表 / 6+1 / 按端 scope 隔离)追加进 collaboration.md——本文不装端的事。
     蒸自 xhs-analysis(单端并行实测,exp-047)+ 原多端协调骨架里跟端数无关的部分(taoxi-geo)。占位符:{{base_branch}} -->

# 多 agent 并行不撞车(逻辑层)

> **和 `worktree-isolation.md` 的分工**:那份治**物理层**——防 A 窗口 `git checkout` 把 B 窗口的 HEAD 和未提交改动拽走。本份治**逻辑层**——两个 agent 各在自己 worktree 里改得好好的,改的却是同一堆文件,合并照撞。**worktree 拦不住这个。**
>
> **单端项目更要看这份**:多端项目里"你只动你端的代码"就是天然隔离;单端没有这个边界,所有 agent 都在同一堆目录里刨(xhs-analysis 实测:仓里从未有第 2 个端,照样撞出跨 PR 逻辑冲突)。

## 三种撞法 · 只有一种有硬闸(先认清哪层在裸奔)

| 撞法 | 怎么防 | 有没有硬闸 |
|---|---|---|
| **同一个 issue 被领两次** | 建 `issue-N` worktree 抢坑(约定 3) | ✅ **有**——同名分支 / 目录创建直接 fatal,第二个 agent 一行代码都写不了 |
| **不同 issue 改同一堆文件** | 动手前看有没有别人正在动那块 + issue 评论同步影响面(约定 2、4) | ❌ 无闸,**只有纪律**——worktree 拦不住:两个 agent 各自改得好好的,合并照撞 |
| **同一任务内多个子代理写同一工作区** | 派之前数写手,≥2 个就各给一份 `cp -a` 沙箱(约定 5) | ❌ 无闸,**只有纪律**——后写的直接盖掉先写的,两边都报成功 |

## 六条约定

1. **起手一句话报告(不可跳)**:新会话第一句报清坐标,让 owner 一眼看出两个 agent 是不是在啃同一块——`我是 <角色/端> agent · 在 <分支> · behind N · open issue M · 准备干 #K`(单端可省角色段)。`behind > 0` → 先 sync 到 `origin/{{base_branch}}` 再信本地 SOP / 代码,sync 后重读起手要读的 doc。owner 会话顺带扫 `待业务确认`,一条条拍。
2. **issue 当消息总线**:需求 issue 保持 open 当容器;跨 agent 靠 issue 评论 sync(字段 / 决策 / 影响面),各 agent 起手 `gh issue view N --comments` 自动同步。**doc 写正文、issue 写状态和变更路由、PR 写交付验证**,别让任一件替代另外两件。方案变更(换思路 / 改字段 / 调 schema)**必须回写一句到 issue**——另一个 agent 不会去翻你的 PR。
3. **取活靠建 worktree 抢坑(这是唯一的锁 · 别拿 label 当锁)**:取活的原子动作 = 建任务 worktree——`git worktree add .worktrees/issue-N -b feat/issue-N origin/{{base_branch}}`(细则见 `worktree-isolation.md`)。**exit 0 = 这活归你**;非 0(同名分支或同名目录已存在)= **别人正在做**,换一条 issue,**别删、别 `--force`、别改名绕开**。抢到后再标「开发中」label + issue 评论一句认领,并说清你要碰哪些文件。
   **名字就是 `issue-N`,不许加 slug、不许换前缀**:锁靠"名字撞上"生效——`feat/issue-5-add-login` 与 `fix/issue-5-login-bug` **撞不上**,两个 agent 双双开工,闸等于没有。**前缀全项目钉死一个**(默认 `feat/`;要换在 ADR 里改,但**不许按任务类型变**)。
   **`open` 不等于没人做**:issue 的 open 同时表示"待做"和"正在做",**worktree / 分支才是真信号**——取活前跑 `git worktree list` + `git branch -a | grep issue-N`。只看 issue 列表就取活,会出现两个 agent 都看到 open、双双做完、白扔一整份工(实战已发生过)。
   **为什么 label / 评论不能当锁**:两个 agent 会同时读到"没人占"、再同时标上,GitHub 两边都收(label / assignee 都是 add 型幂等,不做 compare-and-swap,[官方文档](https://docs.github.com/en/rest/issues/assignees)明写"Users already assigned to an issue are not replaced");**分支名 / 目录名是本地文件系统级互斥,创建失败是硬失败**(实测:同名分支 exit 255 / 同名目录 exit 128)。所以 label 只当看板给人看进度,互斥责任全部压在 worktree 上。**判据是"两个 agent 对同一 issue 必然生成同一个名字",不是"名字里带了 issue 号"**——`issue-6-add-login` 与 `fix/issue-6-login-bug` 都带号,实测照样双双 exit 0。
   **锁要能释放,不然 issue 会被永久锁死**:抢坑失败先判**活锁还是死锁**——`git worktree list` 看那个 worktree 在不在本机、issue 最后活动时间多久以前。**活的**(有人正在做)→ 换一条。**死的**(会话崩了 / 机器重启留下的孤儿,分支没合、也没人在推进)→ **直接进那个 worktree 续做**(它就是你的工作区,不算抢);确认整条任务废弃则报 owner 授权后 `git worktree remove` + 删本地分支,再重抢。**这不是"改名绕开"**——禁的是给同一条 issue 造第二个名字,不是禁清理孤儿。
   **跨机器不成立**:worktree 互斥只在同一个仓库克隆内有效;真出现多机并行,退回"认领评论 + owner 分派",别以为还锁着。
4. **escalate 是自决动作、不是给 owner 出选择题(§1.9 carve-out)**:识别到要改需求 / 扩范围 / 动别人正在改的地方(含 owner 当面加的扩范围)→ 自己写 issue 评论 + 附设计草案 + 报一句,上交对的人(多端给 coord,双角色给业务方,否则给 owner)。只有"做不做 / 优先级"留 owner 拍。
5. **同一任务内派多个子代理:先数写手,再给隔离**(约定 3 的锁按 issue 切,管不到一个 issue 内部)。派之前先数**这批里有几个要写文件**:只有 1 个写手 → 直接派;只读的(查代码 / 审 / 调研)随便并发。**≥2 个写手 → 每人一份独立工作区**——**优先用工具层自带的子代理 worktree 隔离**(Claude Code:`Agent(isolation: "worktree")`,用完自动清理);没这能力才退 `cp -a` 复制一份 + 写完手工合并(注意 `cp -a` 会把 `.git` 一起复制,且它**不是锁、不会硬失败**,合并纪律得自己定)。**别让两个子代理同时写同一个工作区**——后写的直接盖掉先写的,而且两边都报成功。
6. **卡住了别默默卡着(§1.9「遇阻处置」的并行侧)**:三档口径见 §1.9,此处只加并行特有的一条——**这条在并行下是双向的**:你卡着不吭声,另一个 agent 会照着旧假设继续往前修,它比你更晚发现、返工更贵。所以卡点**必须落到 issue 评论**(不能只留在你自己的报告里),让消息总线上的人看得见。

## 三个高价值坑

- **close-keyword 误关**:commit / PR 别让 `#N` 紧跟 `close` / `fix` / `resolve`(GitHub 子串匹配,`fix: #5` 会被读成 `fix #5` 直接关 issue #5)。要带号放末尾 `(#N)`;中间 PR 一律 `Refs #N`,真收口才 `Closes #N`。**当消息总线的 issue 被误关 = 并行同步当场断掉**,这是最贵的一种误关。
- **HEAD race**:多 worktree 共享同一个 `.git`,主仓 HEAD 永远停 {{base_branch}},实现分支的 `checkout` 只在任务 worktree 里跑。细则见 `worktree-isolation.md`,此处不重复。
- **抢坑失败有两种,别混为一谈**:错误含 `could not lock config file .git/config: File exists` = **争 `.git/config.lock` 的锁竞争**(多个 worktree 创建同时跑),**退避重试**,别当成"别人在做"而放弃这条 issue;只有 `a branch named ... already exists` / `'...' already exists` 才是坑真被占。**抢坑动作串行来**可绕开前者。(线索来源 [claude-code #34645](https://github.com/anthropics/claude-code/issues/34645),报的是 Windows + 运行时 `isolation: "worktree"` 并发、状态 closed as not planned,**未在本仓复现**;这里只取"两种失败要分开"这条判据。)

## worktree(选项,不默认)

多 agent 真频繁本地冲突 / 明确隔离需求才上(按 issue/任务建临时 worktree、用完即清);否则别上(过度治理)。上了 → 落 `worktree-isolation.md` + 记一条 ADR。
