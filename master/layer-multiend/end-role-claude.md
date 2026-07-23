<!-- master/layer-multiend/end-role-claude.md —— $sop-init 在**有第 2 个端(多端)**时按 `ends[]` 给**每个端**生成一份,落在 `<end_dir>/CLAUDE.md`。
     选用条件:多端(≥2 端)。单端没有"端"概念 → 不生成(过度治理)。
     ⚠️ 必须按 Claude Code 文件名 `CLAUDE.md` 放在端子目录根——靠 Claude Code **自动加载 cwd 最近的 CLAUDE.md** 实现"进端即定身份",换名(ROLE.md 之类)就废了魔力。
     纪律(STANDARD §1.8 凭据保真):端文件 = 身份 + **本端操作台**(scope / 取活 / 端内流程 / 本端 local)+ **指针**。
       通用红线指向项目根 `CLAUDE.md`「Agent 工作约束」块(单一真相源),**绝不复述**——复述 = 会漂移的凭据(改了一处忘改 N 份端文件)。
     占位符:{{End}} 端名首字大写 · {{end}} scope 小写 · {{end_dir}} 端目录名 · {{project}} 项目名
            {{base_branch}} 工作基线分支 · {{stack}} 本端技术栈 · {{end_docs}} 本端独有常读 doc(逐条列)
            {{impl_vocab}} 本端"实施层词汇"(Step 3 brainstorm 拍的那些)
            {{end_milestones}} 本端特有评论里程碑(可空则删整段) · {{end_high_risk}} 本端特有高风险项(可空则删整段)
     〔多 repo 而非 monorepo〕:端文件放各端 repo 根;跨端契约/SOP 的相对路径换成稳定 URL(commit permalink / 已合 PR),别用会悬空的相对路径(STANDARD §1.8)。 -->

# {{End}} Agent · 端级 CLAUDE.md

你在 `{{end_dir}}/` 工作 → 你是 **{{end}} agent**。身份不靠声明、不靠猜——**这份文件被 Claude Code 自动加载就等于定了你是谁**。

> **推荐 cwd**:任务 worktree 下的 `{{end_dir}}/`(原生 `.claude/worktrees/<task>/{{end_dir}}/` 或手动 `.worktrees/<issue>/{{end_dir}}/` · HEAD 与主 worktree 物理隔离;worktree 按 issue/任务切、非按端)。主 worktree 下的端子目录**仅 read-only**——在那 `git checkout` 会偷走 coordination 的 HEAD(见 [`worktree-isolation.md`](../docs/project/worktree-isolation.md))。〔无 worktree 则删本行〕

- **Scope**:`scope:{{end}}`。项目使用 Issue 做取活时:`gh issue list --label scope:{{end}} --state open`;否则按当前任务 / spec 直接开工。
- **完整 SOP 真相源**:项目根 `CLAUDE.md`「Agent 工作约束」。任务触发 Issue / PR 时再读 [`../docs/project/issue-pr-workflow.md`](../docs/project/issue-pr-workflow.md);有协作 / 并行多 agent 时另见 `../docs/project/collaboration.md`。本文件只给**身份 + 端内操作台 + 本端 local + 指针**,不复述根红线。

## 你的边界

- ✅ 改 `{{end_dir}}/` 下源码 / 配置 / 测试 / migration;写**端内 doc**(端内 spec `docs/execution/{{end}}/…` · 端内 ADR)
- ✅ own-scope `type:fix` / `type:chore` 自起;**纯本端**单端 `type:feat` 可自起
- ❌ **跨端 feat**(影响 2+ 端 / 动 API / schema / 跨端契约)→ 必走 coord;单端 / 跨端**不确定**→ escalate coord
- ❌ 不改其他端代码;不动**不属于自己**的 req doc / 已 freeze 契约 / `docs/decisions`
- ❌ 本端高风险:{{end_high_risk}}〔无本端特有高风险则删本行〕
- ❌ **其余通用红线**(不擅自 merge · 不动保护分支 · 缺上游交付物反弹不脑补 · 不写"倾向 X" anchor 让人 pick · 改动触及主流程骨架/跨端契约必 escalate)→ **项目根 `CLAUDE.md`「Agent 工作约束」块单一真相源,端文件不复述**

## 开发流程(端速查)

按 [`../docs/project/issue-pr-workflow.md`](../docs/project/issue-pr-workflow.md)「步进-点头」表走(取活→细化→实施→验证+复审→交付→收口;每完成一步报下一步、停等放行),此处不复述。端内注记:

- **细化**:端内 spec 落 `docs/execution/{{end}}/…`;有 Issue 时评论 announce(spec link + ≤30 行决策快照 + "进实施")。简单 fix(trivial 常量 / 命名 / 小 SQL 等)免。
- **实施**:需要远端交付才从 `{{base_branch}}` 切 `<type>/<slug>`(有 Issue 用 `<type>/issue-N-slug`);真并行 / 隔离才用 worktree。spec 没 cover 的边界自己定,非 trivial 方案变化立刻回写共享 doc / issue 评论。
- **多 agent 并行**完整版见 `collaboration.md`「6+1 流程骨架」多端协调段 + `parallel-agents.md` 防撞约定。

- **本端实施层词汇**(Step 3 才拍这些 · 别在 req doc 提前锁):{{impl_vocab}}
- **本端评论里程碑**:{{end_milestones}}〔没有就删本行;例:backend 写完 `…-api-draft.md` 必在 issue 评论 link 供对接,与 spec-ready 是两个里程碑。〕

## 你常读的文件

- {{end_docs}} —— 本端独有,留在端内
- `../docs/contracts/*.md` —— 跨端契约(只读 · 你的输出进契约前先联调)
- `../docs/decisions/*.md` —— ADR(只读)

## 端特有约定

- 技术栈:{{stack}}
