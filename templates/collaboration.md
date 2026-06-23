<!-- templates/collaboration.md —— 仅「有第 2 个人」时生成(单人不建)。按"几个人 / 几个端"伸缩:
     基线 = 双角色 / 小团队(业务↔开发,轻);有第 2 个端 + 多并行 scope agent 再加下方「多端多 agent 追加段」(蒸馏自 taoxi-geo)。
     生成为 docs/project/collaboration.md。占位符:{{ends}} {{collaborators}} -->

# 协作约定(本项目)

> 本文件是"谁干什么、怎么不撞车"的真相源,**仅当有第 2 个人时才有**——单人项目没有这份(角色变焦在 CLAUDE.md 公约里恒定、不靠它;issue+PR 也归恒定流程、不靠它)。跨端事实(仅多端)定义在 `docs/contracts/`,这里只管协作流。

## 角色(基线 · 双角色小团队)

- **业务 / 需求方**:提需求 → agent 据此开 issue(一句话需求 + 验收 + 链到 req doc)。只管"要什么",不碰技术架构。**业务会话有开工 / 收工仪式**:开工先扫 `待业务确认` 的 issue 逐条拍;收工把需求落 issue + req doc 走 PR 推远端(不直推受保护分支)。
- **开发**:认领 issue → 开分支 `<type>/issue-N-slug` → 实现 → PR(`Closes #N`)→ 低风险自审自动合、高风险回人审。
- (单人 + 多 agent 时)主窗口 = 你;scope agent = 派出去的实现者。**派单 prompt 必须自包含**——subagent 看不到主窗口上下文。

## 业务端起需求(对话 → 收口 → issue + req doc)

> **开工先按 `issue-pr-workflow.md` 的起手 freshness**:`git fetch` sync + 扫 `待业务确认` 的 issue 一条条拍,再聊新需求。

1. **对话**:业务方跟 agent 聊需求,agent **主动建议补盲区**(可行性 / 怎么拆 · 照亮不代选),业务方拍——循环到收口。
2. **收口标准**(钉死才算,不是聊到完美):**① 验收(怎么算对)· ② 主流程(哪几步)· ③ 范围(要啥 / 不要啥)· ④ 形态(交付长什么样:网页 / 按钮 / App / 定时任务…,用业务话定、别默认 CLI)**。
3. **落凭据**:**先把 req doc(`docs/superpowers/specs/` · brainstorming 默认落点)commit+push 到远端、再开需求 issue 指过去**(issue 轻量:idea + assignee + **指向 doc 的稳定链接 commit/PR**,别指会悬空的相对路径;"开 issue"和"推 doc"不许分两步漂着)。**req doc 只写"要什么"(语义级:做什么 / 验收 / 主流程 / 形态 / 跨端换什么数据),绝不碰"怎么做"**——技术留给 dev brainstorm。
4. **交棒**:收口后球交 agent 实施,业务方只在高风险闸 / 联调露面。

## 不撞车规则

- 每个任务独立工作;并行任务并行派,不串行。
- **永不阻塞**:卡住了主动汇报进度,不默默卡死别人。**但「永不阻塞」≠ 替别人补缺的交付物**:dev 起手接需求**先验 req doc 链接解析得开**;解析不到 / 别的角色该给而缺(req doc / 契约)= 坏交接 → **反弹回该角色 + ⏸️ 待澄清,不自己补**(自补 = 伪造需求)。

## 红线

- 不擅自 commit / push(需 owner 明确指令)。**owner 说"收工 / 结束"即此明确指令** → 把本会话收口的 doc 按 `issue-pr-workflow.md` 推远端(文档 main / 代码 PR)。
- 不跑 writing-plans;spec 验收要硬 + code review 必留(见 CLAUDE.md)。
- 治理过重要主动喊停——右尺寸 > 全面。

---

## === 多端多 agent 追加段(选用条件:有第 2 个端 + 多个并行 scope agent) ===

> 单端小团队**不用**这段。多端各端一个 scope agent + 一个 coordination(产品方向)+ owner 时才加(蒸馏自 taoxi-geo 829 行 SOP)。agent 自决 / 永不阻塞 / 新眼睛 review 全在 STANDARD §1,此处不重复。

### 角色(多端)

> 每端的**身份 + 边界**写在各端 `<端目录>/CLAUDE.md`(端级文档 · `/sop-init` 按 `ends[]` 生成 · harness 自动加载)。此处只给跨端协作骨架,**不复述各端边界**(单一真相源 · STANDARD §1.6)。

| 角色 | scope | 干啥 |
|---|---|---|
| Coordination | docs | 出"在做什么" + 起跨端 req doc · **不出契约、不写端代码** |
| 各端 scope agent | {{ends}} | 端内代码 + 端内 spec/ADR + 自决实施 · 遇不合理自己解决 + 评论 · **边界见各端 `CLAUDE.md`** |
| Owner | — | 提需求 + 审 req + 联调 + merge · **不当通讯人** |

**scope agent ≠ 执行 PM 派的细 task;= 高权限程序员,收到方向后自决实施。**

> **身份靠"进哪个端的目录"自动定**:harness 自动加载 cwd 最近的 `CLAUDE.md` —— 在 `wt-<端>/<端目录>/` 就自动是该端 scope agent,在主仓就是 coordination。不靠声明、不靠猜(细则见 `worktree-isolation.md`)。
>
> **错座位护栏**:端内活(端内 spec / 端代码)归 scope agent、在对应端 worktree 产出;coordination(主仓)只产**跨端 req doc**。**救场**——spec 已误产在主仓:别"释放分支",按 req-doc 交接走 `push → doc PR(Refs)→ owner merge 进 master → scope agent 在端 worktree 从 origin/master 另切实施分支`。

### 6+1 流程骨架(轻 · 无硬 gate)

1. **起需求**:跨端 → coord 起 req doc;单端上下文明确 → 该 scope agent 自起(不必经 coord 中转)。req doc 写"做什么 + 怎么算对 + 主流程 + 形态 + 跨端换什么数据",**不写实施层**(见 `multiend-contracts.md`)。
2. **取活**:scope label 有 open issue 即"待干"。
3. **细化**:scope agent invoke brainstorming 跟 owner 定端内方案 → 端内 spec doc → 自检 + 评论 announce(不再单独整体批)。
4. **开发**:切 `<type>/issue-N-slug` 分支(分支名自决);实施 + 进展 / 变更写 issue 评论;push + PR(`Closes #N`);**过新眼睛 review**(STANDARD §1.3)再示意 merge。
5. **联调**:owner 跑;单端 bug → 该 agent;跨端 mismatch → 评论拍板改哪端。
6. **收口**:别把"做完了"手抄进多个 doc(单一真相源 · STANDARD §1.6)。**owner 说"收工"= 推收口 doc 远端的明确指令(文档 main / 代码 PR)**。
- **+1 回改 req doc**(可选 · 有重大 deviation 才值)。

### 多 agent 不撞车

- **消息总线**:需求 issue 保持 open 当容器;跨 agent 靠 issue 评论 sync(字段 / 决策 / 跨端影响),别端起手 `gh issue view N --comments` 自动 sync。**issue 薄 doc 厚**。
- **起手 freshness(不可跳)**:新会话先 `git fetch && 看 behind master`,落后先 sync 再读 SOP——否则读的是旧快照的旧规则。一句话起手报告:`我是 X agent · 在 Y 分支 · behind N · open issue M · 准备干 #K`。**owner 会话顺带扫 `待业务确认`,一条条拍**。
- **scope 隔离**:不动别端代码、不动别人起的 req doc / 契约(要改走 issue 评论提)。识别到要改 req doc / 跨端(含 owner 当面加的扩范围)→ **自己写评论上交 coord + 附设计草案 + 报 owner 一句,别把 escalate 做成 owner 选择题**(§1.9 carve-out);只"做不做 / 优先级"留 owner 拍。
- **worktree(选项)**:多 agent 真频繁本地冲突才上(每端一个 worktree 物理隔离);否则别上(过度治理)。**上了 → 落 `worktree-isolation.md`(布局 / race trap / setup+维护 / 起手按-ref-验 / 反转条件)+ 记一条 ADR。**

### 高价值坑(taoxi-geo 复发过的)

- **close-keyword 误关**:doc PR 用 `Refs #N` 不用 `Closes`;commit 别让 `#N` 紧跟 `close/fix/resolve`(GitHub substring match,会误关 message-bus issue)。需求 issue 由最后一个实施 PR 关。
- **HEAD race**:多 worktree 共享 `.git`,主 worktree 下的同名子目录只读,别在那 `git checkout`(会偷 coordination 的 HEAD)。
