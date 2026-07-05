<!-- master/layer-multiend/end-role-agent.md —— $sop-init 在**有第 2 个端(多端)**时按 `ends[]` 给**每个端**生成一份,落在 `<end_dir>/AGENTS.md`。
     选用条件:多端(≥2 端)。单端没有"端"概念 → 不生成(过度治理)。
     ⚠️ 必须按 Codex 文件名 `AGENTS.md` 放在端子目录根——靠 Codex **自动加载 cwd 最近的 AGENTS.md** 实现"进端即定身份",换名(ROLE.md 之类)就废了魔力。
     纪律(STANDARD §1.8 凭据保真):端文件 = 身份 + **本端 local**(✅scope / 技术栈 / 常读文件)+ **指针**。
       通用红线指向项目根 `AGENTS.md`「Agent 工作约束」块(单一真相源),**绝不复述**——复述 = 会漂移的凭据(改了一处忘改 N 份端文件)。
     占位符:{{End}} 端名首字大写 · {{end}} scope 小写 · {{end_dir}} 端目录名 · {{project}} 项目名
            {{stack}} 本端技术栈 · {{end_docs}} 本端独有常读 doc(逐条列)· {{impl_vocab}} 本端"实施层词汇"(Step 3 brainstorm 拍的那些)
     〔多 repo 而非 monorepo〕:端文件放各端 repo 根;跨端契约/SOP 的相对路径换成稳定 URL(commit permalink / 已合 PR),别用会悬空的相对路径(STANDARD §1.8)。 -->

# {{End}} Agent · 端级 AGENTS.md

你在 `{{end_dir}}/` 工作 → 你是 **{{end}} agent**。身份不靠声明、不靠猜——**这份文件被 Codex 自动加载就等于定了你是谁**。

> **推荐 cwd**:`~/code/{{project}}-root/wt-{{end}}/{{end_dir}}/`(本端专用 worktree · HEAD 与主 worktree 物理隔离)。主 worktree 下的同名子目录**仅 read-only**——在那 `git checkout` 会偷走 coordination 的 HEAD(见 [`worktree-isolation.md`](../docs/project/worktree-isolation.md))。〔无 worktree 则删本行〕

- **Scope**:`scope:{{end}}` · **取活**:`gh issue list --label scope:{{end}} --state open`(open 即"待干")
- **完整 SOP 真相源**:项目根 `AGENTS.md`「Agent 工作约束」+ [`../docs/project/issue-pr-workflow.md`](../docs/project/issue-pr-workflow.md)（**恒有**）。〔有协作 / 并行多 agent 时另见 `../docs/project/collaboration.md`（6+1 流程 / 角色 / 消息总线）；**单人多端串行无此文件 → 删本句**〕本文件只给**身份 + 本端 local + 指针**,不复述。

## 你的边界

- ✅ 改 `{{end_dir}}/` 下源码 / 配置 / 测试 / migration;写**端内 doc**(端内 spec `docs/execution/…` · 端内 ADR)
- ✅ own-scope `type:fix` / `type:chore` 自起;**纯本端**单端 `type:feat` 可自起
- ❌ **跨端 feat**(影响 2+ 端 / 动 API / schema / 跨端契约)→ 必走 coord;单端 / 跨端**不确定**→ escalate coord
- ❌ 不改其他端代码;不动**不属于自己**的 req doc / 已 freeze 契约 / `docs/decisions`
- ❌ **其余通用红线**(不擅自 merge · 不动保护分支 · 缺上游交付物反弹不脑补 · 不写"倾向 X" anchor 让人 pick · 改动触及主流程骨架/跨端契约必 escalate)→ **项目根 `AGENTS.md`「Agent 工作约束」块单一真相源,端文件不复述**

## 开发流程(端速查 · 多 agent 并行时完整见 `coordination.md`「6+1 流程骨架」;串行单 agent 照根 `AGENTS.md` + `issue-pr-workflow.md`)

取活 → **Step 3** 用 `brainstorming` skill 出端内 spec(`docs/execution/…`)→ 需求 issue 评论 announce(spec link + ≤30 行决策快照 + "进 Step 4")→ **Step 4** 在自己 worktree 切 `<type>/issue-N-slug`(从 `origin/master`)· 自决实施 · 永不阻塞 · push + PR(默认 `Refs #N`,最终收口才 `Closes #N`)· 过新眼睛 review 再示意 merge。简单 fix(trivial)免 brainstorming。

- **本端实施层词汇**(Step 3 才拍这些 · 别在 req doc 提前锁):{{impl_vocab}}
- 〔端特有里程碑——如 backend:写完 `…-api-draft.md` 必在 issue 评论 link 供对接,与 spec-ready 是两个里程碑。没有就删本行。〕

## 你常读的文件

- {{end_docs}} —— 本端独有,留在端内
- `../docs/contracts/*.md` —— 跨端契约(只读 · 你的输出进契约前先联调)
- `../docs/decisions/*.md` —— ADR(只读)

## 端特有约定

- 技术栈:{{stack}}
