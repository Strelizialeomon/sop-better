# Spec:业务会话闭环(开工读凭据 / 收工写凭据)· 瘦身版

> 状态:待 owner review。
> 走本项目自己的工作流(STANDARD §1.3):**跳 writing-plans**,spec 必含**可检验验收**,build 后派**新眼睛子代理 code review**,记 `experiments/2026-06-08-005-*.md`。

---

## 1. 要解决什么(一句话)

业务人员打开 Claude Code 后,缺一套**人视角的"开工 / 收工"动作**:开工不知道"哪些 issue 在等我拍板",收工要自己 commit/push/开 issue。本 spec 用**最小改动**补齐,**不新立概念、不重抄已有规则**。

业务人员改完后的体验,核心三句:
- **开工**:agent 自动 sync + 把"等你拍板"的 issue 推到你面前,一条条过。
- **聊需求**:照旧(只梳"要什么",agent 提架构 + 补盲区 + 反驳)。
- **收工**:你说一句"收工",agent 自动把收口 doc 推远端 + 开/更 issue。

## 2. 为什么是瘦身版(对照初衷自审的结论)

原始设计违反了项目自己的两条铁律:**①重抄已有规则**(sync / 验链接 / doc 先推后开 issue / 自检保真 都已存在 → 犯 §1.6 单一真相源 + sop-init「重抄必漂移」);**②为小缺口立大概念**(新 §1.11 + "闭环"品牌 + 穿 7 文件各加一段 → 犯头号铁律「过度治理」)。

真正**新且该补**的只有两点,全程人零额外仪式成本:
1. **开工把"等业务拍板"的 issue 主动 surface 给人**(现模型是 agent 拉活,无机制把"人的待决策"推给人 —— 真空白)。
2. **文档=低风险凭据,审在对话里、默认推 main**(owner 已定的政策,值一行)。

其余一律**用指针接上已有规则**,不重写。

## 3. 改动清单(瘦身 · 全是"给已有行加半句 + 指针")

> 只在 **C≥C1**(有 issue 协作)生效;C0 无远端 issue,不加(过度治理)。

| # | 文件 | 改什么 | 形态 |
|---|---|---|---|
| 1 | `STANDARD.md` §1.8 | 加 1-2 句:凭据不只 agent 读 —— **标 `待业务确认` 的凭据,开工时主动 surface 给人,别让决策烂在 issue 里**;**收工 = 写凭据时点**,owner 说"收工"即红线那个"明确 push 指令";**文档(纯"要什么")审在 brainstorming 收口、不在 PR 再审 → 默认推 main,代码照旧走 PR** | 扩写已有原则 |
| 2 | `STANDARD.md` §5 | 凭据失真那条加半句:**C≥C1 项目 issue 堆着 `待业务确认` 没人 surface / 收工不推收口 doc = 交接断裂** | +1 查法 |
| 3 | `templates/issue-pr-workflow.md` 「起手 freshness」 | 那行尾巴加半句:**顺带 `gh issue list --label 待业务确认`,有就拉 owner 一条条过、没有一句带过** | 扩写已有 |
| 4 | `templates/issue-pr-workflow.md` Issue 生命周期·标 | 阻塞 flag 列加一个词 **`待业务确认`**(与 `待澄清` 并列,叠加) | +1 词 |
| 5 | `templates/issue-pr-workflow.md` PR 生命周期·低风险自动合 | 点明:**文档(req/design doc)内容的审 = brainstorming 收口,不在 PR 再审 → 默认直接推 main;main 受保护才开 PR 但 agent 自审自动合,人不等** | 澄清已有 |
| 6 | `templates/issue-pr-workflow.md` | 加 1 行指针:**收工(owner 说"收工/结束")= 红线"不擅自 push"的那个明确指令时点;按上面 Issue/PR 生命周期推本会话收口的 doc** | +1 指针(不重抄步骤) |
| 7 | `templates/agent-constraints.md` 标准块 | 加 1 行(自带门槛):**(C≥C1)开工先 sync + 扫 `待业务确认` 的 issue 一条条过;对完需求主动问一次"收工?",是 → 推收口 doc(文档 main / 代码 PR)。细则见 issue-pr-workflow** | +1 行(self-gating) |
| 8 | `templates/collaboration.md` (C1) 红线 | "不擅自 commit / push(需 owner 明确指令)"后加半句:**owner 说"收工/结束"即此指令 → 把收口 doc 按 issue-pr-workflow 推远端**;并在「业务端起需求」头加半句指针指向起手 freshness | 扩写已有 |
| 9 | `templates/collaboration-c2.md` (C2) | 起手 freshness 步加"(owner 会话顺带扫 `待业务确认`)";收口/owner 行加"收工推收口 doc" | 扩写已有 |
| 10 | `skills/sop-init/SKILL.md` 流程 | C1/C2 的 label 状态机注明含 **`待业务确认`** flag(标准块那行已随块自动带,无需另写) | +半句 |
| 11 | `skills/sop-audit/SKILL.md` 流程 P3 | 凭据失真查法加半句(镜像 STANDARD §5 第 2 条) | +1 查法 |

## 4. 明确不做(反过度治理 · 防未来漂移)

- ❌ 不新建 STANDARD §1.11 / 不立"业务会话闭环"作为新公约或新 section 标题。
- ❌ 不在 `agent-constraints.md` 新建「C≥C1 追加块」/ 不在 workflow 新建「收工结账」小节。
- ❌ 不重抄已有规则:`git fetch` 同步、验 doc 链接、doc 先推后开 issue、自检凭据保真 —— **仍只在原处定义一次**,本 spec 一律指针引用,不复制正文。
- ❌ C0 不加任何东西(无远端 issue)。

## 5. 验收标准(可检验 · 跳 plan 的补偿)

### A. 正向 —— 加了该加的(grep 可核)
1. `STANDARD.md` §1.8 能搜到 `待业务确认`、`surface`、`收工`、`推 main`(或等义)四个点;§5 能搜到"`待业务确认` 没人 surface"查法。
2. `issue-pr-workflow.md`:「起手 freshness」段含 `待业务确认` 扫描;阻塞 flag 列含 `待业务确认`;PR 低风险段含"文档…推 main…不在 PR 再审";含收工指针 1 行。
3. `agent-constraints.md` 标准块含 **1 行**且仅 1 行 `(C≥C1)…开工…收工…文档 main / 代码 PR`。
4. `collaboration.md` 红线含"收工…明确指令…推远端"。
5. `sop-init/SKILL.md` 流程 C1/C2 label 提到 `待业务确认`;`sop-audit/SKILL.md` P3 含对应查法。

### B. 反向 —— 没加不该加的(防漂移)
6. 全仓 grep **无** `§1.11`、**无** "业务会话闭环" 作为标题、**无** 新增"收工结账"小节、**无** 新「C≥C1 追加块」标题。
7. `git fetch`/同步、验链接、自检保真的**正文定义仍各只一处**(本次改动只新增指针,不新增第二份正文)。
8. 全部文件净增行数合计 **≤ 25 行**(瘦身硬指标;超了即说明又在膨胀,回炉)。

### C. 端到端语义(build 后验)
9. 对一个 **C1 样例**(可用 geo-reverse 或临时造的最小 C1 目录)跑 `/sop-init`(增量模式),生成/更新的 `CLAUDE.md` 能搜到那 1 行开工/收工约束,且 label 集合含 `待业务确认`。
10. 对一个**"issue 挂着 `待业务确认` 却无任何 surface 约定"**的假项目跑 `/sop-audit`,能报出该 P3 finding(带证据)。

### D. 安全网(本项目工作流)
11. build 完成后,派**独立新眼睛子代理** code review:喂本 spec + diff + 决策快照,三查(合规 / bug / 体量是否又超重)。非阻断直接改,阻断回 owner。
12. 记一条 `experiments/2026-06-08-005-business-session-loop.md`:这次改动是不是又差点过度治理(自审→瘦身的过程本身是高价值狗粮)。

## 6. 范围(三样钉死)

- **验收**:见 §5(grep 正向 + 反向 + 端到端 + review + exp)。
- **主流程**:改 §3 那 11 处 → grep 核 §5A/B → build 后跑 §5C → 新眼睛 review → 记 exp-005。
- **要啥 / 不要啥**:要"补两点真缺口 + 其余指针接上"(§2);**不要**新概念 / 新 section / 重抄(§4)。

---

## 7. 增补(同次 go · owner 追加)

两条业务"起手"前置,延续瘦身:

1. **起手先确认角色(补 §1.2 真空白)**:§1.2 讲了业务/开发变焦 + owner 陷阱,**独缺"agent 怎么定角色"** → 全靠猜。补:起手问一句「业务还是开发?」(CLAUDE.md 钉死单一角色则不问),**默认业务**。
   - 落点:`STANDARD.md` §1.2 +1 句;`agent-constraints.md` 标准块 +1 行(凡非 S0·C0,因它管 brainstorming 变焦)。
2. **gh 就绪(业务侧一次性前置)**:开工扫 issue / 整个凭据系统靠 `gh`,业务人员可能没装/没登录 → 第一条命令就跑不起来。补:开工前置——`gh auth status` 不过则 agent **用大白话**带装(mac brew / win winget)+ 引导 `! gh auth login`,**一次性**。
   - 落点:`issue-pr-workflow.md`「起手 freshness」前置 +1 条;`agent-constraints.md` 加一条 `(C≥C1·一次性前置)gh 就绪` bullet(**与每次开工的节奏分开** · 采纳 exp-005 增量 review 的一条)。
   - **瘦身硬约束**:SOP 只写"带他装登"指令 + 命令一行,**不嵌完整教程**(gh 版本会变);逐步引导 agent 运行时现生成(说人话是公约)。

**不做**:不嵌 gh 安装教程;collaboration*.md 经已有「起手 freshness」指针自动获得 gh 前置,不另写;sop-audit 不为这两条加新查法(模板已带即足,避免过度治理)。**净增 ≤ 4 行**。
