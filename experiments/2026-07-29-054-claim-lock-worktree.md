# exp-054 · 防撞车的锁必须建在"会硬失败"的原语上 → 取活抢坑压到 `issue-N` worktree,名字钉死

> ⚠️ **本条是修路 / 维护记录,不是撒手实验**——exp-045 起连续第 10 条非撒手记录。但与 050-053 不同,本条主体是**把仓里已存在却接不上的机制接上 + 用更硬的原语替换软约定**,且增量避开了最贵的地方(always-loaded 净 0 行),按 exp-048 北极星属"修路"那一侧。
>
> 公平归因:**设计主体是 owner 的**——两半方案(label 认领 + worktree 带 issue 号)、"名字不许带 slug / 不许换前缀"这条致命闸、三种撞法的分层、`open ≠ 没人做`、子代理沙箱、post-checkout 响铃,全部来自 owner 的实战积累。agent 侧的贡献是:实测出两半方案的硬度差、逮到"仓里早有 label 机制却从没接进取活动作"这个真病灶、定死分层(哪些进 always-loaded、哪些只进并行层)、以及**在 owner 指出前把命名写成了带 slug 的错版本**(见 §4.2 自曝)。

- **日期**:2026-07-29
- **触发**:owner 问「多 agent 时有没有防止取到同一个任务的机制」,自提方案后指出"之前就有 label 机制,现在执行得相当糟糕",随后给出完整的两层防撞实战设计。
- **性质**:公约层缺环补全 + **接线**(已有机制没接上取活动作)+ 一处降级(label 从"互斥状态机"降为"看板")。
- **体量口径(两个数)**:规则面 **净 +15 行**(36 insertions / 21 deletions,排除 `experiments/`;远低于 exp-005 的 ≤25 上限);**always-loaded 块(`master/base/CLAUDE.md` §2)12 条 → 12 条、净 0 行,2571 → 2644 字符(+73 / +2.8%)**——量法:`awk '/^## 2\./,/^## 3\./'` 抽整节,`grep -c "^- \*\*"` 计条数、`wc -m` 计字符。**+14 行里绝大部分落在 `parallel-agents.md`(on-demand,只有真并行项目才读)**;对比 exp-052 的 +13.2%、exp-053 的 +6.6%,本次 always-loaded 增量最小。
- **未在真项目复验**(见 §6);本仓无并行多 agent 场景,dogfood 覆盖不到疗效。

---

## 1. 病灶:不是"缺机制",是**已有的机制从来没接到取活动作上**

owner 的记忆是准的——`issue-pr-workflow.md:58` 白纸黑字写着「阶段 label 互斥:评估中 / 开发中 / 已完成」。它执行得糟糕是**结构断裂**,不是纪律松:

| 断点 | 现状 | 后果 |
|---|---|---|
| **取活动作不看 label** | 根 `CLAUDE.md`(agent 每次进 context 必读)写 `gh issue list --state open` 挑活,**不过滤任何 label** | agent 只看到"open 就能挑",压根不知道有 lifecycle label |
| **建 issue 默认光板** | 两个 issue 模板 `labels: ""` | 新 issue 一律无 label,过滤也过滤不出东西 |
| **定义在 on-demand 层,取活在 always-loaded 层,互不引用** | label 定义只在 `issue-pr-workflow.md`(触发才读) | 真相源与执行点断开——**§1.7 的反面形态**:不是重抄漂移,是**根本没抄、也没指针** |

**与 exp-052/053 同形但更基础**:那两条是判据不可达,这条是判据**压根没进执行路径**。再写多少"要照做"都无效。

## 2. 改法:互斥压到 git 原语上,label 降级为看板

### 2.1 关键实测:owner 提的两半,只有一半是锁

| 机制 | 是不是锁 | 凭据 |
|---|---|---|
| **worktree / 分支名带 issue 号** | ✅ **真互斥** | 本地实测:同名分支 `git worktree add -b feat/issue-5` 二次执行 **exit 255**;同名目录 **exit 128**;换新号 **exit 0**。创建失败是硬失败 |
| **label 标已认领 / 未认领** | ❌ **看板,不是锁** | read-modify-write 竞态:两个 agent 同时读到"未认领"、同时标上,GitHub **两边都收**。官方文档明写 add assignees 时 "Users already assigned to an issue are not replaced"(§3)——幂等 add,**不做 compare-and-swap** |

**判真假锁只需问一句:两个 agent 同时执行这个动作,会不会有一个报错?** 不报错 = 它是看板。

> **同轮纠错**:agent 上一轮曾建议"用 `gh issue edit --add-assignee @me` 占坑,assignee 是服务端状态天然当锁"——**错的**,已查官方文档推翻并当场纠正。

### 2.2 **名字钉死是这把锁的本体(owner 逮到的致命漏洞)**

agent 首版把命名写成 `.worktrees/issue-N-slug` + `<type>/issue-N-slug`(照抄仓里既有写法),**这个版本的锁是失效的**:

```
agent A: git worktree add .worktrees/issue-5-add-login  -b feat/issue-5-add-login
agent B: git worktree add .worktrees/issue-5-login-bug  -b fix/issue-5-login-bug
                                                            ↑ 两个名字撞不上 → 双双 exit 0 → 都在做 #5
```

**锁靠"名字必然相撞"生效。加 slug 撞不上,换前缀(`feat/` vs `fix/`)也撞不上——闸等于没装。** 落定:

- **目录 = `issue-N`,分支 = `feat/issue-N`**,不加 slug、不加后缀。
- **前缀全项目钉死一个**(默认 `feat/`,要换写进 ADR),**不许按任务类型变**。
- 原生 `EnterWorktree` 的 `<task>` 名同样必须是 `issue-N`(随手起 `fix-bug` 就把锁绕没了)。
- 连带清掉三处会绕开闸的既有措辞:`coordination-multiend.md` 的「分支名**自决**」、`worktree-isolation.md` 的「already checked out → **换分支名**」、以及两处 `<type>/issue-N-slug` 分支命名规则。

### 2.3 三种撞法,只有一种有硬闸(其余两种明写"在裸奔")

| 撞法 | 防法 | 硬闸? |
|---|---|---|
| **同一个 issue 被领两次** | 建 `issue-N` worktree 抢坑 | ✅ 有——第二个 agent 一行代码都写不了 |
| **不同 issue 改同一堆文件** | 动手前看有没有别人正在动那块 + issue 评论同步影响面 | ❌ 无闸,**只有纪律**——worktree 拦不住,各自改得好好的、合并照撞 |
| **同一任务内多个子代理写同一工作区** | 派之前**数写手**:≥2 个写手就各给一份 `cp -a` 沙箱或串行派;只读的随便并发 | ❌ 无闸,**只有纪律**——后写的盖掉先写的,两边都报成功 |

第三行是 agent 首版**完全漏掉**的场景(owner 补),而它恰恰是最常发生的——派并行子代理是日常动作。

### 2.4 配套三条纪律(owner 实战积累,本次收编)

- **`open` 不等于没人做**:issue 的 open 同时表示"待做"和"正在做",**worktree / 分支才是真信号**(`git worktree list` + `git branch -a | grep issue-N`)。**实战已发生过**:两个 agent 都看到 open、双双做完同一条 issue,白扔一整份工。
- **issue 当消息总线**(既有,保留):doc 写正文、issue 写状态和变更路由、PR 写交付验证;方案变更必须回写一句到 issue——**别人不会去翻你的 PR**。
- **卡住必须写进 issue 评论**(既有,保留):只留在给 owner 的报告里不算——另一个 agent 正照着旧假设往前修。

### 2.5 分层:抢坑只进并行层,不进 base 层

**最容易做错的一步是把抢坑写进 base 层**——那样单 agent / 串行项目会被迫建 worktree,正撞 §5.1「过度治理是头号查」。落定的分层:

| 层 | 加了什么 | 谁吃 |
|---|---|---|
| **base**(所有项目 · always-loaded) | 取活条内一句:「挑中先看有没有被占——带「开发中」label / 有认领评论 / **已有同号任务分支** = 别人在做,换一条」。**一个字没提 worktree** | 所有用 Issue 的项目,零负担 |
| **base / issue-pr-workflow** | 分支命名加**条件例外**:真并行才钉死 `feat/issue-N`,串行带 slug 更可读;lifecycle label **降级**为"看板、不是锁" | 用 Issue 的项目 |
| **layer-parallel-agents** | 约定 3 改为"建 worktree 抢坑 = 唯一的锁"(命名钉死 / exit code 判据 / 失败处置 / 为什么 label 不行 / 跨机器不成立);**新增约定 5**(子代理沙箱);新增三种撞法总表;坑里加 `.git/config.lock` 竞争 | 仅真并行项目 |
| **layer-multiend** | 端级分支命名同加条件例外 | 仅多端项目 |
| **worktree-isolation** | per-task 粒度条升格为**命名钉死 + 理由**;原生路径同规;`git clean -fdx` 补 `.claude/worktrees/` 与"主仓眼里并行工作区都是垃圾"的说明;**新增 post-checkout 响铃**(明标是警报不是闸) | 仅上 worktree 项目 |
| **STANDARD §5.3** | audit 查法补 **"并行防撞写了却锁不住"**(取活只写"看一眼"、或命名不带号 / 带 slug → 互斥形同虚设);**反向同查**:无并行却被强制抢坑 | `$sop-audit` |

### 2.6 刻意留下的洞(写进规则、不藏着)

| 洞 | 处置 |
|---|---|
| **跨机器不互斥** | 规则明写"真出现多机并行,退回认领评论 + owner 分派,别以为还锁着"。**不为想象中的多机场景预建分布式锁**(§3.5) |
| **抢坑失败有两种,长得像却要反着处理** | `.git/config.lock` 竞争 ≠ 坑被占。抢坑串行来;错误里是 `config.lock` / `File exists` → 退避重试,别当"别人在做"放弃这条 issue([claude-code #34645](https://github.com/anthropics/claude-code/issues/34645)) |
| **post-checkout 钩子拦不住** | git **没有** `pre-checkout`,钩子切完才跑(事后报警);且只在切换那一刻响,**会话开始就已跑偏的一辈子不响**。规则里原样写出这两条限度,防止"装了就以为安全" |

### 2.7 出题税账目

**净增**:always-loaded +73 字符(净 0 行、无新 bullet)、并行层约定 3 扩写 + 新增约定 5 与撞法表、STANDARD §5.3 一句查法。
**同轮抵扣**:`issue-pr-workflow.md` 的 lifecycle label **从"互斥状态机"降级为"看板"**(家规要求的"降级为结构");并行层约定 3 是**替换**软认领、不是叠加;三处会绕开闸的旧措辞(「分支名自决」「换分支名」「`<type>/issue-N-slug`」)**被删或收窄**。
**净增成立的理由**:主体是接线 + 换更硬原语,规则面**变窄**(取活从"自由挑"收成"抢到才算",分支名从"自决"收成"钉死");文本增量避开了最贵的 always-loaded。按 exp-048 北极星算修路,不算出题。

## 3. 调研依据(§1.4 信源分档 · 一手 / 二手分开)

**一手源**:

- **本地实测(最硬的一条,自己跑的)**:临时仓复现 `git worktree add` 互斥——同名分支 exit 255 / 同名目录 exit 128 / 新号 exit 0。**"创建成功即抢到坑"由此成立**,不是推理。
- **label / assignee 不是 CAS**:GitHub REST API 官方文档「Add assignees to an issue」原文——"Adds up to 10 assignees to an issue. **Users already assigned to an issue are not replaced**";并注 "Only users with push access can add assignees to an issue. **Assignees are silently ignored otherwise**"(后半句更坏:无权限时**静默丢弃、不报错**,当锁用会假成功)。已 WebFetch 核对。<https://docs.github.com/en/rest/issues/assignees>
- **owner 的实战凭据**:同一条 issue 被两个 agent 同时领走、双双做完的真实事故;以及 post-checkout 钩子的实装限度。属**本仓一手实测**,但**未留可回溯的 issue 链接**(见 §6)。

**二手源(只当线索 · 不为定义背书)**:

- **`.git/config.lock` 竞争**:anthropics/claude-code issue #34645。虽在官方仓,issue 仍属二手(§1.4);**本次未本地复现**,据此只加了"串行抢坑 + 分辨两种失败",没加更重机制。<https://github.com/anthropics/claude-code/issues/34645>
- **"One task → one branch → one worktree → one agent"**:2026 年一批讲并行 AI agent + worktree 的博客(Upsun developer / appxlab / raine.dev / MindStudio)口径一致。**全是博客 / 厂商内容**,只用来确认业界共识与本次方向一致。

## 4. dogfood(本仓自己吃)

- **反向验收(exp-005 纪律)** 跑了两轮,第二轮逮到两处漏网:`end-role-claude.md` 与 `issue-pr-workflow.md` 里还留着 `<type>/issue-N-slug` 分支命名。**处置是加条件例外而非一刀钉死**——串行 / 单 agent 项目带 slug 更可读,不该为并行场景牺牲(§5.1)。
- **分层验证**:`grep` 确认 base 层仍零 worktree 强制提及(仅存两处是既有的"真并行才加"按需升级条款)。
- **体量核算**:见头部。
- **新眼睛复审**:见 §4.1(按**回滚后**的口径跑——findings 是待核实假设,交作者 / owner 解;exp-052/053 的两档判据已于本日回滚,不适用)。

### 4.1 复审结果

<!-- 待新眼睛子代理返回后填 -->

### 4.2 自曝:agent 首版把锁写成了失效版本

首版照抄仓里既有的 `issue-N-slug` 写法,**没意识到 slug 一加锁就失效**——而整条规则的卖点正是"这是唯一的硬闸"。是 owner 指出才改。教训归档:**写"这是锁"的规则时,必须当场演一遍"两个 agent 各跑一次会怎样"**,别停在"名字带了 issue 号"这种看起来对的表述上。同形提醒已写进 §5。

## 5. 抽一条教训 → 回填 PLAYBOOK

**锁要建在"会硬失败"的原语上,别建在"大家说好"的标记上;而且要验的是"名字必然相撞",不是"名字带了标识"。** 判一个防撞机制真假,问两句:① **两个 agent 同时执行这个动作,会不会有一个报错?**(不报错 = 看板不是锁);② **这两个 agent 生成的名字,可能不一样吗?**(可能 = 撞不上 = 闸失效)。看板有价值(给人看进度),但把互斥责任压在看板上**比没有互斥更坏**——因为大家以为有。

→ 已写入 `PLAYBOOK.md`?**[ ] 暂不回填**——本条有**机制侧**硬证据(worktree 互斥实测 + 官方文档证 label 非 CAS),且有一例 owner 报的真实撞车事故;但**疗效侧仍为零**(新规则没在任何并行场景跑过)。照 048-053 同一纪律(PLAYBOOK:7 准入规矩),等真项目实跑后与那批一起集中回填。

---

## 6. 未验清单(诚实标坑)

| 未验项 | 为什么重要 |
|---|---|
| **新规则没在任何并行场景跑过** | 本仓无并行多 agent,dogfood 只验了规则面一致性 |
| **原生 `EnterWorktree` 路径的同名冲突未实测** | 实测跑的是手动 `git worktree add`;原生是**首选路径**,它遇到同名 task 是硬失败还是静默复用、错误码是什么,全没验——**若它不硬失败,首选路径上的锁就是空的** |
| **`.git/config.lock` 竞争未本地复现** | 只引了 #34645(二手)。"抢坑串行来"的必要性与充分性都没实测 |
| **owner 报的撞车事故未留可回溯凭据** | 口述"两个 agent 双双做完同一条 issue",但没有 issue 链接 / 时间。按 §1.8 凭据保真,这是**弱凭据**;疗效对比(改前 N 次 → 改后 0 次)缺基线 |
| **`cp -a` 沙箱的合并纪律没写** | 只写了"各给一份沙箱",没写写完怎么合、冲突怎么办。首次真用会暴露 |
| **label 降级后会不会干脆没人标** | 去掉互斥责任的同时可能去掉了标它的动力,进度看板会失真 |
| **post-checkout 钩子本身没在本仓装过** | 收编的是 owner 在别处的实践,本仓没跑过,脚本内容也未落盘(只写了机制与限度) |
| **与回滚同轮** | 本轮同时回滚了 exp-052/053 的复审两档;两件事的交互未验 |
