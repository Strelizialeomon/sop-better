<!-- master/layer-parallel-agents/parallel-agents.md —— 触发:真并行多 agent(与端数无关,单端也触发;串行 / 单 agent 不发)。
     $sop-init 落成 docs/project/parallel-agents.md,与 worktree-isolation.md 成对生成、两份互指。
     多端时另有 coordination-multiend.md(角色表 / 6+1 / 按端 scope 隔离)追加进 collaboration.md——本文不装端的事。
     蒸自 xhs-analysis(单端并行实测,exp-047)+ 原多端协调骨架里跟端数无关的部分(taoxi-geo)。占位符:{{base_branch}} -->

# 多 agent 并行不撞车(逻辑层)

> **和 `worktree-isolation.md` 的分工**:那份治**物理层**——防 A 窗口 `git checkout` 把 B 窗口的 HEAD 和未提交改动拽走。本份治**逻辑层**——两个 agent 各在自己 worktree 里改得好好的,改的却是同一堆文件,合并照撞。**worktree 拦不住这个。**
>
> **单端项目更要看这份**:多端项目里"你只动你端的代码"就是天然隔离;单端没有这个边界,所有 agent 都在同一堆目录里刨(xhs-analysis 实测:仓里从未有第 2 个端,照样撞出跨 PR 逻辑冲突)。

## 五条约定

1. **起手一句话报告(不可跳)**:新会话第一句报清坐标,让 owner 一眼看出两个 agent 是不是在啃同一块——`我是 <角色/端> agent · 在 <分支> · behind N · open issue M · 准备干 #K`(单端可省角色段)。`behind > 0` → 先 sync 到 `origin/{{base_branch}}` 再信本地 SOP / 代码,sync 后重读起手要读的 doc。owner 会话顺带扫 `待业务确认`,一条条拍。
2. **issue 当消息总线**:需求 issue 保持 open 当容器;跨 agent 靠 issue 评论 sync(字段 / 决策 / 影响面),各 agent 起手 `gh issue view N --comments` 自动同步。**doc 写正文、issue 写状态和变更路由、PR 写交付验证**,别让任一件替代另外两件。方案变更(换思路 / 改字段 / 调 schema)**必须回写一句到 issue**——另一个 agent 不会去翻你的 PR。
3. **取活先认领**:动手前先看这块文件有没有别人正在动的 open issue——有 → 先在那条 issue 评论说你要碰哪里,别另起炉灶开第二条线;没有 → 正常开工。
4. **escalate 是自决动作、不是给 owner 出选择题(§1.9 carve-out)**:识别到要改需求 / 扩范围 / 动别人正在改的地方(含 owner 当面加的扩范围)→ 自己写 issue 评论 + 附设计草案 + 报一句,上交对的人(多端给 coord,双角色给业务方,否则给 owner)。只有"做不做 / 优先级"留 owner 拍。
5. **卡住了别默默卡着(§1.9「遇阻处置」的并行侧)**:三档口径见 §1.9,此处只加并行特有的一条——**这条在并行下是双向的**:你卡着不吭声,另一个 agent 会照着旧假设继续往前修,它比你更晚发现、返工更贵。所以卡点**必须落到 issue 评论**(不能只留在你自己的报告里),让消息总线上的人看得见。

## 两个高价值坑

- **close-keyword 误关**:commit / PR 别让 `#N` 紧跟 `close` / `fix` / `resolve`(GitHub 子串匹配,`fix: #5` 会被读成 `fix #5` 直接关 issue #5)。要带号放末尾 `(#N)`;中间 PR 一律 `Refs #N`,真收口才 `Closes #N`。**当消息总线的 issue 被误关 = 并行同步当场断掉**,这是最贵的一种误关。
- **HEAD race**:多 worktree 共享同一个 `.git`,主仓 HEAD 永远停 {{base_branch}},实现分支的 `checkout` 只在任务 worktree 里跑。细则见 `worktree-isolation.md`,此处不重复。

## worktree(选项,不默认)

多 agent 真频繁本地冲突 / 明确隔离需求才上(按 issue/任务建临时 worktree、用完即清);否则别上(过度治理)。上了 → 落 `worktree-isolation.md` + 记一条 ADR。
