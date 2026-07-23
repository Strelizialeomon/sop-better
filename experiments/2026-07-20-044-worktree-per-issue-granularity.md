# exp-044 · worktree 切分粒度:从「按端」回灌为「按 issue/任务」(mobile-os 实测背书)

- **日期**:2026-07-20
- **触发**:owner 指出"按端切 worktree 不灵活,能不能按 issue / 需求切",让参考 mobile-os。
- **决策**:确认 master 层 `worktree-isolation.md` 已漂离 STANDARD 自己的口径,回灌 mobile-os 实测的 per-issue 布局 + 清理纪律;STANDARD / base workflow / skills 不动(已对口径 / 只管触发时机)。

---

## 证据链:不是拍脑袋改口味,是母本漂了 + 活项目已淘汰旧法

1. **内部已不自洽**:`STANDARD §3`(line 135)、`master/base/docs/project/issue-pr-workflow.md`(line 16)**早就是"按任务"口径**——"真并行任务、明确隔离需求才用 worktree,不实行一任务一 worktree"。只有 `layer-parallel-agents/worktree-isolation.md` 还写死 per-scope(按端)/ 永久预建 / 主仓同级 `wt-<端>/`(蒸自更老的 taoxi-geo ADR-0007)。**是这层 doc 漂了,不是 STANDARD。**
2. **唯一"多端+真并行"活项目已换法**:mobile-os `docs/project/collaboration.md` 明文——"本项目当前采用**按 issue/任务建立临时 worktree,而不是永久 per-scope worktree**。原因是并行任务数量变化快,且一次任务常跨相邻目录。" 实测布局 `.worktrees/<issue-or-slug>/`(仓内、gitignore、用完即弃),端身份改由 **cwd 最近 `CLAUDE.md`** 定,与 worktree 正交。

→ 两条现实叠起来:per-端 是被真实并行形态顶翻的旧粒度。回灌 = 把跑偏的一层拉回 STANDARD 已有口径,顺带把 mobile-os 的解耦(端身份 ⊥ worktree)与清理纪律沉进母本。

## 新纹路:per-issue 更灵活是真的,但代价也真,且已在 mobile-os 上发生

按端切被否定的两个具体病(mobile-os 理由):① 并行任务数量变化快——永久预建要么建一堆空 worktree(为没有的形态预建 · §3.5),要么同端两任务并行时卡死;② 一任务常跨相邻端——按端 worktree 装不下跨端任务。

但 per-issue **不是纯赚**,反方两条,**改动必须自带解药、不能只搬好处**:
- **worktree 泛滥 / 清理欠债**:数了 mobile-os 当前 **26 个活 worktree**,一堆没清(两个 issue-38、两个 issue-42、两个 issue-55、两个 issue-75…)。按端切有端数上限,按 issue 切没有 → 回灌时把 mobile-os 已验的**"合并即清"清理 checklist 当一等公民**一起沉,并新增反转条件"数量长期居高 → 收紧为强制闸 / 加陈旧巡检"。
- **重环境各付一遍**:gitignored venv/node_modules/设备标定/local config,按端切一端配一次反复用,按 issue 切每个新 worktree 重配 → 母本明写这是代价,并给"重环境端保留一个长驻 worktree"作例外。**按任务切是默认粒度、不是铁律。**

## 改了什么

- `master/layer-parallel-agents/worktree-isolation.md`:主体重写。per-scope→per-task;永久预建→on-demand 用完即弃;主仓同级 `wt-<端>/`→仓内 `.worktrees/<issue>/`(gitignore,建前 `check-ignore`);新增"端身份 ⊥ worktree"解耦、"重环境各配 + 长驻例外"、"合并即清清理纪律"。**保留全部承重墙**:HEAD race trap、起手按-ref-验(§1.8)、stash 跨 wt 共享 caveat、反转条件。
- `master/layer-parallel-agents/coordination.md`:3 处解耦——身份判定改"cwd 最近 CLAUDE.md + 端身份⊥worktree"、错座位救场改"新建任务 worktree 另切"、worktree 选项行改"按 issue/任务建临时、用完清"。
- `master/layer-multiend/end-role-claude.md`:推荐 cwd 从 `wt-{{end}}/` 改 `.worktrees/<issue>/{{end_dir}}/`。
- `master/layer-multiend/multiend-contracts.md`:§二补一条「同 agent 跨端不豁免留痕」——per-issue 切法让一个 agent 可在同一 worktree 一把改完两端(旧按端切时物理结构逼着走 issue 握手,现在没这道物理墙),握手轮次自然蒸发可接受,但契约留痕(issue announce + 选择性 freeze)是给未来 feature 的、不随之豁免。mobile-os 同向背书:其 collaboration.md 明文「spec/contract 不得用未合并 commit 交接」+ 跨端任务强制 `scope:cross-end` label,活项目已把留痕当不可豁免项。

## 反向验收 / 自审(本仓家规:改 SOP 先用 audit 镜头审自己)

- **净增行数上限**(exp-005):worktree-isolation.md 重写后 **96 行(原 95,净 +1 行)**,基本持平(新增清理纪律用删掉的旧 per-端 setup/同级布局冗余抵掉)。
- **安全护栏没在减法/搬迁中丢**(exp-006 逐条核):HEAD race trap ✅、起手按-ref-验 ✅、清理前 ignored 产物盘点 ✅、禁 `--force` ✅、删远端分支另取授权 ✅、反转条件 ✅(且新增泛滥反转条件)。
- **无旧世界残字**:`grep per-scope|wt-<端>|每端一个|永久预建` 在 master 只剩"不是 per-scope / 不永久预建"的**故意对比**用法,无遗留(§5.2 删除残留)。

## 产出 / 未完成

- 本 PR(sop-better):上述 3 母本 + 本 exp + PLAYBOOK 一条 + README 近期主线。
- **复验待补**(诚实标坑):本次改的是母本,尚**未**在真项目重跑 `$sop-init` 增量验证生成物与 mobile-os 现状一致;也未回灌到 mobile-os 本体(它已自行采用该法,母本反而是追认它)。按 STANDARD §6,母本改动"验透"要等真项目扫一眼判对——留作下一步。
