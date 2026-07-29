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
   **为什么 label / 评论不能当锁**:两个 agent 会同时读到"没人占"、再同时标上,GitHub 两边都收(label / assignee 都是 add 型幂等,不做 compare-and-swap,[官方文档](https://docs.github.com/en/rest/issues/assignees)明写"Users already assigned to an issue are not replaced");**分支名 / 目录名是本地文件系统级互斥,创建失败是硬失败**(实测:同名分支 exit 255 / 同名目录 exit 128)。所以 label 只当看板给人看进度,互斥责任全部压在 worktree 上。**不带 issue 号的 worktree 名 = 名字撞不上 = 这把锁直接失效。**
   **跨机器不成立**:worktree 互斥只在同一个仓库克隆内有效;真出现多机并行,退回"认领评论 + owner 分派",别以为还锁着。
4. **escalate 是自决动作、不是给 owner 出选择题(§1.9 carve-out)**:识别到要改需求 / 扩范围 / 动别人正在改的地方(含 owner 当面加的扩范围)→ 自己写 issue 评论 + 附设计草案 + 报一句,上交对的人(多端给 coord,双角色给业务方,否则给 owner)。只有"做不做 / 优先级"留 owner 拍。
5. **同一任务内派多个子代理:先数写手,再给沙箱**(worktree 治不了这层——它按 issue 切,一个 issue 内的子代理全在同一个工作区里)。派之前先数**这批里有几个要写文件**:只有 1 个写手 → 直接派;**≥2 个写手 → 每人 `cp -a` 一份沙箱**(或串行派),写完由你合并;只读的(查代码 / 审 / 调研)不占沙箱、随便并发。**别让两个子代理同时写同一个工作区**——它们不共享 git 索引之外的任何互斥,后写的直接盖掉先写的,而且两边都报成功。
6. **卡住了别默默卡着(§1.9「遇阻处置」的并行侧)**:三档口径见 §1.9,此处只加并行特有的一条——**这条在并行下是双向的**:你卡着不吭声,另一个 agent 会照着旧假设继续往前修,它比你更晚发现、返工更贵。所以卡点**必须落到 issue 评论**(不能只留在你自己的报告里),让消息总线上的人看得见。

## 三个高价值坑

- **close-keyword 误关**:commit / PR 别让 `#N` 紧跟 `close` / `fix` / `resolve`(GitHub 子串匹配,`fix: #5` 会被读成 `fix #5` 直接关 issue #5)。要带号放末尾 `(#N)`;中间 PR 一律 `Refs #N`,真收口才 `Closes #N`。**当消息总线的 issue 被误关 = 并行同步当场断掉**,这是最贵的一种误关。
- **HEAD race**:多 worktree 共享同一个 `.git`,主仓 HEAD 永远停 {{base_branch}},实现分支的 `checkout` 只在任务 worktree 里跑。细则见 `worktree-isolation.md`,此处不重复。
- **抢坑动作本身别并发**:多个 agent 同时 `git worktree add` 会争主仓的 `.git/config.lock`,**报的是锁竞争错、不是"这坑被占了"**,两种失败长得像却要反着处理([claude-code #34645](https://github.com/anthropics/claude-code/issues/34645))。抢坑串行来;失败先看错误里是不是 `config.lock` / `File exists`——是就退避重试,别当成"别人在做"而放弃这条 issue。

## worktree(选项,不默认)

多 agent 真频繁本地冲突 / 明确隔离需求才上(按 issue/任务建临时 worktree、用完即清);否则别上(过度治理)。上了 → 落 `worktree-isolation.md` + 记一条 ADR。
