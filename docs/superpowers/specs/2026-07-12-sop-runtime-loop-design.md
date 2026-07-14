# sop-better 双层 Agent Loop 设计

- **日期**:2026-07-12
- **状态**:六段设计、轻量 cooperative-local 信任模型与 change-first 增量复审均获 owner 明确采纳;Loop MVP 保持实验态
- **Spec review**:change-first 增量复审已完成独立新眼睛审查 Ready（0 Critical / Important）
- **目标环境**:多台自有机器;macOS + Codex、Windows + Codex
- **启动策略**:cooperative-local 始终手动“开工 #N”;后台模式须另立隔离 controller / broker 设计
- **验证策略**:机械测试 + 真实 Codex 行为实验双保险

---

## 0. 与 2026-07-10 稳定化设计的关系

本设计不推翻 [`2026-07-10-sop-better-stability-overhaul-design.md`](2026-07-10-sop-better-stability-overhaul-design.md) 的稳定化地基,继续保留:

- 确定性生成与机械检查。
- macOS / Windows 跨平台。
- 开发版本与稳定安装隔离。
- 升级前展示差异并等待明确确认。
- 安装与项目内容都能回滚。
- `STANDARD.md` 是规则语义的唯一可编辑真相源。

本设计新增“任务运行时”,并取代旧设计中的以下部分:

- 不再先把全部生成、发布基础设施做完,最后才验证 agent 行为。
- 第一条交付改为一个真实任务可跑通的竖向 Loop。
- 行为实验从第一批开始,不推迟到全部基础设施完成以后。
- 原 `sopctl` 从“生成 / 检查 / 升级工具”扩为“生成工具 + 轻量任务控制器”。

两份设计冲突时:

- **任务运行、任务状态、实施顺序**以本文为准。
- **安装、版本隔离、升级、回滚**继续以 2026-07-10 设计为准。
- 旧设计 / 母本中的永久 per-scope worktree 只保留为历史来源;项目启用新运行时后,不得与本文的 per-issue worktree 同时作为执行规则。

---

## 1. 问题与结论

### 1.1 当前问题

当前 sop-better 已经定义了大量正确原则,但主要靠 agent 自己完成以下动作:

- 读取根和端级 `AGENTS.md`。
- 决定还要读哪些 workflow / collaboration / contract 文档。
- 从 Issue 评论推断现在做到哪一步。
- 判断自己是谁、能改哪里、何时交棒。
- 记得运行测试、派 reviewer、回写凭据和关闭 Issue。

这是一套“有规则、无运行控制面”的系统。多个窗口或多台机器同时工作时,仍可能发生:

- 两个 agent 同时领取一个任务。
- agent 读取不同快照,对当前状态理解不一致。
- reviewer 不知道已确认决策,把它误报成问题。
- agent 完成一个阶段后停下来等人推动,Loop 断掉。
- 规则散落在多个入口,必须读完才知道下一步,甚至互相冲突。

### 1.2 已批准结论

采用“轻量控制器”方案:

```text
薄 AGENTS.md
    ↓
$sop-run
    ↓
sopctl + GitHub Issue / PR / Git ref
    ↓
Codex Goal / runner
    ↓
实现 → 验证 → 独立 review → 修复 → 再验证
```

系统同时提供两层闭环:

1. **外层任务 Loop**:任务出现、领取、锁定、交接、等待、恢复、收口。
2. **内层质量 Loop**:查证、实现、测试、review、修复,直到验收满足。

cooperative-local 始终由 owner 说“开工 #N”手动启动;后台 `sopctl watch` 不在本设计内,未来若需要必须先落隔离 controller / broker。

### 1.3 最强反对意见

增加 `sopctl` 状态与调度能力,可能把项目变成僵硬工作流框架,重新制造过度治理。

边界如下:

- 程序只管确定事实:谁拥有任务、任务是否可执行、依赖是否满足、验证是否通过、能否交接。
- agent 负责需要判断的部分:技术方案、调查路径、实现细节、finding 是否成立。
- 不把“先改哪个文件”“必须几个 commit”编码进状态机。
- 外层只保留 4 个业务状态;没有真实失败证据不新增状态。
- 第一版不建服务器、数据库、Web 控制台、多租户或通用 DAG 调度器。

### 1.4 复审凭据的轻量信任选择（owner 2026-07-14 决策）
对比过三种形态:独立系统服务能真正隔离 GitHub App 凭据,但需要管理员安装和跨平台高权限 worker;GitHub 侧 broker 隔离清楚,但排队与远端启动会抵消复审提速;本地 cooperative-local 最轻,但只能防误操作,不能防持同一 GitHub 凭据的执行 agent 故意绕过。owner 明确选择第三种,先验证“复审更快”本身。因此第一版不建 daemon,不要求管理员权限,也不把 GitHub review marker 宣称为防伪签名。它只在正常 `sopctl` 路径内重建连续覆盖链、阻止手填 evidence JSON、拦截旧 lease / 错 SHA / 超范围改动。若执行 agent 直接调用 GitHub API 伪造同格式评论,本版不能可靠识别;不可信仓库、会处理敌对输入的任务和无人值守自动合并不得启用此模式。以后要扩大到这三类场景,必须另立隔离 controller / broker 设计,不能悄悄把 App token 塞回当前进程。
---

## 2. 外部调研依据
本设计优先采用官方一手资料:
1. Codex 会聚合全局、仓库和当前目录的 `AGENTS.md`;项目目录中的 `AGENTS.md` 聚合默认受 32 KiB 上限约束。skills metadata 也会加入初始输入,但官方资料没有说明它与 project-doc 共用同一个 32 KiB 上限。常驻指令仍应是高信号入口,具体流程按需加载。
   [OpenAI · Unrolling the Codex agent loop](https://openai.com/index/unrolling-the-codex-agent-loop/)
2. OpenAI 的 Harness Engineering 实践把“可读环境、可执行约束、测试和反馈 Loop”作为 agent 可靠性的地基;文档不足以独自保持系统一致。
   [OpenAI · Harness engineering](https://openai.com/index/harness-engineering/)
3. Symphony 把 Issue Tracker 作为控制面,每个任务映射到独立 workspace,持续运行、失败恢复;但其后续经验也明确反对把 agent 当成死板状态节点,应给目标、工具和边界。
   [OpenAI · Symphony 介绍](https://openai.com/index/open-source-codex-orchestration-symphony/) · [Symphony 规范](https://github.com/openai/symphony)
4. OpenAI 长任务指南强调可验证目标、可审阅的外部记忆、持久线程、定期唤醒和不可逆动作的人审。
   [OpenAI · Codex-maxxing for long-running work](https://openai.com/index/codex-maxxing-long-running-work/)
5. GitHub Git References API 支持创建冲突;GitHub GraphQL `updateRefs` 支持用 `beforeOid` 比较旧值并原子更新,可用于跨机器唯一领取与安全接管。
   [GitHub · Git References REST API](https://docs.github.com/en/rest/git/refs) · [GitHub · Git GraphQL reference](https://docs.github.com/en/graphql/reference/git)
6. Gerrit Patch Set 支持比较任意两轮,GitHub 也能在新提交后让旧批准失效并要求最新 push 获批,共同支持“首轮完整、后续相邻版本且连续覆盖”的模型。[Gerrit · Patch Sets](https://gerrit-review.googlesource.com/Documentation/concept-patch-sets.html) · [GitHub · Managing a branch protection rule](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/managing-a-branch-protection-rule)
这些资料只提供方向。最终是否放行,仍以本项目的机械测试和真实 Codex 对照实验为准。
---

## 3. 目标与非目标

### 3.1 必须做到

- owner 一次说“开工 #N”后,低风险任务自动运行到 `merge-eligible`;外部 owner / 保护流程完成合并后,owner 手动说“继续 #N”触发终态核验。
- 多台机器 / 多个 agent 不会同时拥有同一个任务。
- agent 开工无需全文读取 `STANDARD.md` 或完整 workflow。
- agent 获得一份小而完整的任务胶囊:目标、验收、范围、证据、下一步、停止条件。
- 实现、测试、独立 review、修复形成自动内循环。
- 崩溃、断网和进程退出后可以对账并恢复,不能错误标记完成。
- 高风险动作继续由 owner 明确批准。
- Issue、PR、任务锁和实际代码状态可解释、可追溯。
- 第一版代码从一开始按 macOS / Windows 可移植方式实现。

### 3.2 本轮不做

- SaaS、数据库、Web 控制台或遥测平台。
- GitHub 之外的 Linear / Jira / 飞书任务适配。
- 公共多租户分发。
- 自动决定哪台机器性能最好。
- 通用工作流引擎或复杂 DAG 调度。
- 自动修改生产、部署、数据删除等高风险边界。
- 为所有 agent 运行时做兼容;第一版只支持 Codex。

---

## 4. 总体架构与职责

```text
owner / GitHub Issue
        │ 目标 + 验收
        v
      $sop-run
        │
        v
     sopctl task
   ┌────┼──────────────┐
   │    │              │
   v    v              v
任务锁 任务胶囊     per-issue worktree
   │    │              │
   └────┴──────┬───────┘
               v
        Codex Goal / runner
               │
      实现 ↔ 验证 ↔ reviewer
               │
               v
         PR + Issue 收口
```

| 组件 | 唯一职责 |
|---|---|
| `STANDARD.md` | 定义长期原则、触发边界和安全地板;不作为普通执行 agent 的全文入口 |
| `manifest.json` / schema | 把原则映射为机器可检查的组件、字段和触发 |
| 项目 `AGENTS.md` | 极薄入口、稳定红线、要求执行任务先进入 `$sop-run` |
| `$sop-run` | “开工 / 继续 / 恢复 / 查看”的唯一对话入口 |
| `sopctl` | 领取、锁定、任务胶囊、workspace、对账、验证、交接与收口 |
| GitHub Issue | 多机器共享的任务索引、短验收入口、外层事件 / 状态投影和永久交接证据;长正文仍在 doc |
| Git ref 任务锁 | 临时并发所有权;不是第二份业务真相源 |
| Codex Goal / runner | 围绕可验证目标持续执行内层 Loop |
| Skill | 当前阶段确实需要的专项做法,按任务胶囊加载 |
| worktree | 每个 Issue 的物理隔离工作区 |
| owner | 目标、验收、产品决定、override 与高风险批准 |

### 4.1 可执行启用边界

新旧运行时不得靠 agent 阅读两份 spec 后自行选边。项目 profile 必须显式声明:

```json
{
  "schema_version": 2,
  "sop_version": "<pinned-release>",
  "runtime": {"mode": "legacy"}
}
```

- `legacy` 是未迁移项目默认值,继续遵守当前 STANDARD / master / skills。
- `loop-v1-experimental` 只允许 owner 明确选择的 lab 项目,并在 ADR 记录它是候选规则 override。
- profile、managed `AGENTS.md` block、manifest、skills 和 `sopctl` 必须来自同一 pinned release;版本或 schema 不匹配就拒绝启动。
- 实验期的候选 runtime policy 随 release 固定,不假装已经是稳定 STANDARD。
- 行为验证通过后,同一 release 原子更新 STANDARD、manifest、master 与 skills,再把稳定模式命名为 `loop-v1`。
- 任一项目同一时刻只能有一个 runtime mode;旧入口检测到 loop mode 必须拒绝双跑,loop 入口检测到 legacy 也不得接管。

---

## 5. 外层任务 Loop

外层只保留四个状态:

| 状态 | 含义 | 是否有执行权 |
|---|---|---|
| `ready` | 目标、验收、依赖已满足,可以领取 | 无 |
| `running` | 一个有效 lease 正在执行 | 有且仅有一个 |
| `waiting` | 等 owner、外部条件、上游交付物或熔断处理 | 无 |
| `done` | 验收、验证、review 与收口均满足 | 无 |

`testing`、`reviewing`、`fixing` 是运行细节,不新增为外层状态。它们保存在当前 run 的可观察阶段字段中。

### 5.1 手动启动

```text
owner:“开工 #31”
→ $sop-run
→ sopctl 校验 Issue / 依赖 / profile
→ 原子领取
→ 创建或恢复 per-issue worktree
→ 生成任务胶囊
→ 启动 Codex Goal / runner
```

### 5.2 三种合法出口

- `done`:完整验收证据成立。
- `waiting`:需要人或外部条件,已写明恢复条件。
- `running`:本轮尚未完成,继续同一目标或等待同一 run 恢复。

不得以“我先做到这里,你看看”作为第四种模糊出口。没有命中人工闸时,agent 继续运行。

### 5.3 后台模式（本设计明确不做）
cooperative-local 没有 `sopctl watch`,配置与 CLI 都拒绝后台启动。无人值守领取会把“同凭据 agent 可绕过”的已知边界放大成真实自动化风险,所以必须留给未来隔离信任模式另立设计,不能作为本路线的后续开关。
### 5.4 多 Agent 写入所有权

铁律是 **一个可写 Issue = 一个写 lease = 一个 worktree / branch owner**。

- reviewer、研究员等只读子 agent 可以共享任务胶囊,但不占写 lease、不修改代码、不 push。
- 任何需要写代码的第二个 agent 必须有自己的 child Issue、lease、role、allowed paths、worktree、branch 和 PR。
- 跨端父 Issue 只承载总目标、总验收、依赖和子任务索引,本身不授予代码写 lease。
- 每个端由 child Issue 独立执行;父 Issue 只有在全部必需 child 验收收口后才进入最终完成序列。
- Loop MVP 只支持单写、单 scope Issue。遇到跨端写任务先生成拆分提案并进入 `waiting`;owner 批准 child 结构后再并行。

这不是通用 DAG 调度器。第一版只支持父 Issue + 一层 child Issue + 明确依赖,不递归生成任意任务图。

---

## 6. 任务胶囊与按需加载

### 6.1 极薄 `AGENTS.md`

目标项目的 managed block 只保留:

- 解释 / 只读任务直接处理。
- 执行已登记任务先调用 `$sop-run`,不得自行拼流程。
- 当前任务范围和下一步以经校验的任务胶囊为入口。
- 规则冲突统一进入 `sopctl task conflict`。
- 高风险、越权、凭据保真和完成证据是不可绕过的安全地板。

Issue 字段、PR 流程、worktree 命令、review prompt、恢复算法和评论模板不再常驻。

### 6.2 单一运行入口 Skill

新增且只新增一个运行入口:

```text
$sop-run
```

它处理“开工、继续、恢复、查看、交接、收口”,内部调用 `sopctl`。不拆出多个互相竞争的 phase skills。

这里的“只新增一个”只指 sop-better 自己的运行入口。调试、测试、前端设计等已有专项 skill 仍可由任务胶囊按需选择。

### 6.3 任务胶囊契约

任务胶囊至少包含:

```yaml
task: 31
run_id: run-8f21
goal: 修复漫游恢复误操作
acceptance:
  - HOME/no-dock 不盲点返回
  - 指定回归测试通过
role: xhs-control
allowed_paths:
  - xhs-control/
forbidden_paths:
  - mouse-control/
state: running
phase: investigate
required_context:
  - issue: 31
  - files:
      - xhs-control/xhs_roam/recovery.py
      - xhs-control/tests/test_recovery.py
required_skills:
  - superpowers:systematic-debugging
checks:
  - pytest xhs-control/tests/test_recovery.py
risk:
  class: low
  matched_rules:
    - reversible-code-change
  provenance: project-profile://runtime/risk_rules
  reversible_evidence: 仅任务分支,可回退
  approvals: []
next_action: 复现并定位六个失败测试
stop_conditions:
  - 命中高风险闸
  - 必须扩大产品范围
sources:
  goal: github-issue://31
  acceptance: github-issue://31
  role: project-profile://ends/xhs-control
  allowed_paths: project-profile://ends/xhs-control/paths
  forbidden_paths: project-profile://ends/xhs-control/forbidden_paths
  checks: project-profile://runtime/checks
```

胶囊规则:

- 不凭空补业务事实,每个关键字段带来源。
- 只列当前任务必须读的上下文。
- 可生成 JSON 给程序使用,也可生成人读 / agent 读文本。
- managed 胶囊元数据目标上限 4 KiB;代码、diff、日志正文不计入。

#### 输入信任边界

- Issue 正文、评论、外链、日志和代码内容一律视为**不可信数据**,不是 agent 指令。
- Issue / 评论只能提供目标候选、验收候选、证据和状态事件;不得直接指定 commands、skills、allowed paths、风险等级或权限。
- commands、skills、scope、checks 和风险规则只能来自已安装 release 的 manifest / schema 与项目 profile。
- doc 继续是需求 / 设计正文真相源;Issue 保持索引、状态、短验收入口和稳定 doc 链接。简单且自包含的 Bug 允许 Issue 自身承载短正文,但仍要进入规范化快照。
- 第一次手动启动先展示规范化的目标、验收、范围、风险和来源快照。若 Issue 已有 owner 或可信流程签发的 `ready` attestation,一句“开工 #N”即批准该快照;没有 attestation 则只在开工前确认一次。
- attestation 绑定 snapshot hash、签发人、GitHub 服务器时间与 SOP 版本;后续评论不能静默改写已冻结快照。
- 配置或调用方请求后台 `watch` 时 fail closed;轻量版没有“稳定后直接打开”的隐藏路线。

可信身份不是“仓库成员都算”。cooperative-local profile 显式登记稳定 GitHub actor ID;login 只用于显示。attestation 必须绑定 `repo node ID + issue number + snapshot hash + actor ID + sop_version + server time`。`$sop-run` 用当前 `gh` 登录身份的 actor ID 与 profile 对表;未知、不匹配或无法查询时 fail closed。新增 / 删除 trusted actor 属于高风险 profile 变更,必须 owner 明确批准。可信 GitHub App ID 只属于未来隔离 controller 模式,轻量版不假装消费它。

### 6.4 Reviewer 胶囊
首次完整 review 的独立 reviewer 获得自包含输入:
- 目标与逐条验收。
- 本次完整 diff。
- 决策快照。
- 测试 / 构建结果。
- 范围内与范围外声明。
- 输出格式:blocking / non-blocking / invalid hypothesis。
controller 先拒绝 dirty worktree并锁定本轮 `head_sha`,再为该提交创建两个临时 detached worktree:checks 在可写但用后即弃的一份运行,reviewer 在另一份 read-only sandbox 中读取；两份都在开始 / 结束核对 exact HEAD 和 tracked clean,结束后清理。这样测试产物不会污染 reviewer 上下文,任何 HEAD 漂移、对象缺失或 tracked 文件变化都不写 review event。
若任务使用“Issue 薄、doc 厚”,轻量版只接受当前仓库内、绑定完整 commit SHA 的文档链接,单份上限 64 KiB。ready snapshot 保存 `document_url + document_sha256`;review 前重新取同一版本、核对 digest,物化成 reviewer 可读的本地只读文件。链接可变、跨仓、超大小、无法下载或 digest 不符时进入 `waiting`,不能只把 URL 丢给禁网 reviewer。
首轮 canonical review 前必须已有同仓 draft PR;`task review --pull-request <URL>` 先由 GitHub 核实 PR number、base repo / ref、实际 merge-base 与 head SHA。首轮审 merge-base→PR HEAD,后续默认审上一 reviewed HEAD→当前 HEAD 的全部 change;新增、删除、重命名和高风险文件同样走 delta,不靠白名单或正则决定。reviewer 从精确 diff 自取 changed paths（胶囊不重复塞任务级路径）,同时获得未关闭 finding 和检查结果;每个 change 读取所在函数 / 类型 / 模块,若影响接口、调用、状态、并发或数据格式,继续读相关调用方、契约、测试和不变量。“只审 change”不等于只看红绿行:reviewer 先看 diff stat / name-status,再逐个 hunk 展开必要语义上下文;停止条件是已能判断本轮 change 的影响,而不是读过固定文件数,也不会重扫与本轮 change 无语义关系的任务文件。
coverage 由事件所属仓库身份加 `pr_number / base_ref / merge_base_sha` 绑定。仅在没有 review HEAD、上一 SHA 不可用、goal / acceptance / snapshot / scope 变化、PR 被替换或 retarget、base / merge-base 变化、覆盖链有缺口时,才从当前 PR merge-base full reset。改动量大、新增 / 删除 / 重命名、触及 API / 并发 / 配置或 reviewer 需要多读调用链,都不是自动 full 条件;change 很广就扩大相关上下文,无法取得必要证据则返回具体缺失项并拒绝通过。
每条 finding 使用稳定且不复用的 ID,记录 `severity / status / paths / invariant / evidence / disposition`;coverage 连续为 `PR_merge_base --full--> H1 --delta--> H2 --delta--> current_PR_head`。第一段必须是 `full` 且从当前 review basis 的 merge-base 起步;后续 `delta` 的 base 必须精确等于上一段 head。记录至少保存 `review_basis / base_sha / head_sha / mode / finding updates / review_reference`。coverage 和 finding 台账由 controller 根据实际 reviewer run 生成 workflow evidence event,Issue / PR 做永久投影;CLI 不接受执行 agent 手填 review JSON。事件仍绑定 run / lease epoch / fencing token / snapshot / scope / HEAD,用于发现正常流程里的旧写入、错 SHA 和错位,但 cooperative-local 模式不把当前用户写出的 marker 当成对恶意 agent 的防伪证明。PR identity / basis 不同、coverage 起点不对、链有缺口、最终 HEAD 对不上或 blocking 未关闭时,正常 `sopctl` 路径不能进入 `merge-eligible`。reviewer 不依赖主 agent 的对话历史,也不要求全文读取项目 SOP。每个 `full` 段开启新的 coverage generation;旧 generation 由不可变 review events 归档,旧 open blocking findings 作为待核实假设交给新 full reviewer逐条重开、resolved 或 invalid,不能因 reset 静默消失。active ledger 由新 full 输出重建,merge gate 只读当前 generation;finding ID 跨 generation 单调递增且不复用。
### 6.5 冲突处理

遇到全局 / 项目 `AGENTS.md`、skill、任务胶囊或 owner 临时指令冲突时:

```text
sopctl task conflict
→ 列出冲突双方与来源
→ 根据明确优先级和规则 ID 自动裁决可裁决项
→ 涉及目标、范围或高风险才进入 waiting 找 owner
```

不得静默挑一条执行。`sopctl task explain` 必须能说明“为什么现在做这一步”。

冲突裁决顺序固定为:

1. 平台、权限、sandbox 和组织安全约束。
2. owner 当前明确指令与明确授权;高风险动作仍要求指令具体覆盖该动作。
3. 项目 `AGENTS.md` managed 安全地板与 `.sop/profile.json`。
4. Issue / spec / contract 中的任务目标、验收和冻结边界。
5. task capsule 的当前投影。
6. skill 与参考文档给出的做法建议。

task capsule 不是更高权威,它只是把前四层解析成当前有效视图。若来源冲突导致无法可靠投影,不得生成一个看似确定的胶囊。

---

## 7. 内层质量 Loop

```text
查证 / 复现
    ↓
实现最小修改
    ↓
机械验证
    ↓
独立 review
    ↓
blocking? ──是──> 核实 → 修复 → 重新验证
    │
    否
    ↓
风险分流 → 自动收口或 waiting
```

### 7.1 机械验证
项目 profile 明确 `format / lint / test / build / structure` 命令。agent 不临场猜必跑项。
只有一套命令真相源。change-first reviewer 与检查提速分开验证,本轮先改变 review 输入,不靠未验证的路径分类器同时少跑测试:
- **内循环**:每轮复审前在 exact HEAD 的隔离 worktree 跑 profile 全部 check groups。
- **最终闸**:PR head 必须被 review 覆盖链连续覆盖;merge 后另取实际进入配置默认分支的 merged commit,在隔离 worktree 跑 profile 的全部 check groups。PR head 上的内循环结果不能代替 merged commit 全量结果。
每次记录实际命令、HEAD、耗时和结果。3+3 实验先分别记录 reviewer 与 checks wall time;若 checks 被实测为下一主瓶颈,后续只考虑 profile 显式指定“修复轮快速检查组 + merge-eligible 前全部检查”,不让 controller 根据路径猜测试。该后续变化另设验收,不预埋 `check_triggers`。
不能运行时必须记录:

- 未验证项。
- 不能运行的原因。
- 对结论的影响。
- 恢复验证所需条件。
### 7.2 Review 处理
- blocking finding 必须核实。
- 成立则修改;不成立则用代码、测试或规范证据反驳。
- 没有证据时继续调查,不能盲从或忽略。
- 每轮保存绑定当前 PR review basis 的连续 coverage 和 finding 台账;首轮从 PR merge-base 完整审,后续从上一 review HEAD 审到当前 PR HEAD。
- delta review 必须审本轮全部 change,并按语义需要展开受影响函数、调用链、契约和不变量;不能只看 diff hunk,也不能把展开上下文偷换成重扫全部旧 diff。
- delta 是覆盖链的默认第二段及后续段,不是 profile 开关;Phase 1 没有 `delta_review_paths`、跨语言边界表或隐藏快捷路径。
- snapshot / scope / PR identity / base repo / base ref / merge-base 变化或覆盖链断裂时重置为 full;改动路径、文件操作类型和风险标签本身不触发 full。
### 7.3 熔断
以下任一情况进入 `waiting`:
- 同一失败签名连续三次且没有新证据。
- reviewer 与执行 agent 围绕同一点三轮没有新证据。
- 测试环境损坏,改业务代码不能推进。
- 验收条件互相矛盾。
- 必须扩大产品范围或修改冻结契约。
- 任务锁 / 远端状态持续异常。
熔断记录必须写明:已完成、卡点、已尝试、证据、已排除方案、需要 owner 决定什么、决定后从哪里恢复。
### 7.4 完成条件

进入 `merge-eligible` 之前必须满足:

- Issue 验收逐条有证据。
- 必跑检查通过,或明确的人审豁免存在且适用。
- 独立 review 无 blocking。
- PR 与任务胶囊范围一致。
- 残项已关闭或另有真实归属。

代码任务的唯一终态序列是:

```text
checks / review 通过
→ run phase = merge-eligible（Issue 仍 running）
→ pre-merge 重新核对 PR identity / base repo / base ref / merge-base / head,并计算风险、scope、checks 与 approvals
→ 低风险产出可合并 PR / 高风险等待 owner 决定
→ 写 `waiting(awaiting-external-merge, PR URL, PR head)` 永久事件并释放 claim
→ owner 或现有仓库保护流程在 Loop 外合并
→ owner 说“继续 #N”,controller 对 waiting revision 原子领取 terminal-verification lease
→ 重核 PR identity / base / merge-base / head 与 review coverage,再核验默认分支实际 merged commit 并在该提交跑最终 checks；未 merge、basis / head 不符或 checks 失败则仍回 waiting 并释放 claim
→ 追加最终证据事件,关闭 Issue 并投影为 done
→ 使用旧 OID 条件删除 claim
```

高风险批准和外部 merge 都发生在 `waiting` 后;每次恢复必须基于最新 state revision 重新原子领取,不能复用旧 claim。不得先标 `done` 再 merge,也不得把删除 claim 当成完成证据。

非代码任务没有 merge 步骤时,用 profile 定义的 artifact 验证替代 merge;仍须“验证最终产物 → 写最终证据 → done → 删除 claim”。

---

## 8. 多机器唯一领取、Lease 与恢复

### 8.1 为什么标签不够

两台机器可以同时读到 `ready` 并同时加 `running` 标签。Issue / 评论没有唯一写入约束,不能当锁。

### 8.2 临时任务锁

每个活跃 Issue 使用临时远端 ref:

```text
refs/heads/sop/claims/<issue-number>
```

ref 指向只含运行元数据的 commit,例如:

```json
{
  "issue": 31,
  "run_id": "run-8f21",
  "machine_id": "mac-studio",
  "agent_role": "xhs-control",
  "lease_epoch": 3,
  "fencing_token": "fence-a921",
  "server_observed_at": "2026-07-12T10:30:00Z",
  "lease_expires_at": "2026-07-12T10:40:00Z",
  "state_revision": 12,
  "base_sha": "3a58162"
}
```

它不是业务真相源,释放后删除;永久交接证据仍写 Issue / PR。

`sop/claims/**` 必须从普通 CI、发布、部署和分支保护触发器中排除,避免一次心跳触发整套流水线。离线 `sopctl check` 只静态检查仓库内 workflow filters;启动前的联网 `sopctl task preflight github` 检查 GitHub rulesets、权限和已知外部部署入口。无权限或无法验证时拒绝启动,不得宣称已经隔离。

### 8.3 原子操作

- **首次领取**:只在 ref 不存在时创建;两个领取者只有一个成功。
- **续租 / 接管 / 删除**:必须带上读取到的旧 OID;远端已经变化就拒绝。接管成功时 `lease_epoch` 单调增加并生成新 `fencing_token`。
- **服务器时间**:每次成功 GitHub 请求读取 HTTP `Date` 响应头,同时记录请求往返耗时;`server_observed_at` 使用该服务器时间,安全余量必须大于配置允许的最大网络误差。
- **续租约束**:heartbeat interval 必须小于等于 lease TTL 的三分之一。无法在安全余量前续租就主动暂停远端副作用。
- **远端副作用前置闸**:push、创建 / 更新 PR、改 Issue 状态、merge 前都必须重新读取 claim,用旧 OID 成功 CAS 续租,并确认 `run_id + lease_epoch + fencing_token` 未变。
- **任务分支 push**:只允许普通 fast-forward push。push 前后读取并核对预期 branch OID;远端 OID 不匹配或需要 non-fast-forward 时进入 `waiting`,不得使用任何形式的 force push 绕过。提交与运行事件记录当前 run / epoch。
- **可兑现承诺**:系统不能隔空杀死网络分区中的旧进程,所以保证的是“失权被发现或 CAS 续租失败后,旧 runner 不得产生新的远端副作用”。旧本地 worktree 随即标为只读隔离,待恢复或清理。

### 8.4 对账顺序

Issue 与 Git ref 无法作为一个事务同时更新,所以采用可恢复顺序:

- claim:先建任务锁,再把 Issue 改为 `running`;中断后由 reconcile 补 Issue。
- waiting / done:先写永久状态和证据,再用旧 OID 条件删除锁;中断后由 reconcile 清理锁。
- Issue `done` 但锁仍在:删除过期锁。
- Issue `ready` 但有效锁存在:修正为 `running`。
- Issue `running` 但无锁:判断为待恢复,不得假装有 agent 工作。

业务状态以 Issue timeline 中的 append-only 结构化事件为准,标签只是人读投影。每个事件至少包含:

```yaml
event_id: evt-...
state_revision: 12
expected_previous_revision: 11
run_id: run-8f21
source_actor: owner-or-runner
source_server_time: 2026-07-12T10:30:00Z
from: ready
to: running
reason: claim-created
snapshot_hash: sha256:...
```

- runner 只能从自己已观察的 revision 追加下一事件;revision 不匹配就进入 reconcile,不能覆盖。
- GitHub 人工 label 变化从 Issue timeline 读取 actor / server time,由 `sopctl` 规范化成 owner / external event。
- 较新的 owner 决定高于旧 runner 的补写动作;reconcile 不得把 `waiting / done` 改回 `running`。
- 两个无法可靠排序或语义冲突的事件不自动二选一,进入 `waiting/conflict`。
- label 更新失败不改变 workflow evidence event;下次 reconcile 只修投影。

### 8.5 崩溃与接管

第一版采用保守恢复:

- 心跳未超时:拒绝接管。
- 心跳确定超时、Issue 仍可执行、旧 OID 未变化:允许原子接管。
- 证据不充分:进入 `waiting`,请求一次确认。
- 网络中断导致无法续租:在 lease 到期前暂停写操作;不能用离线猜测继续占有远端任务。

远端任务分支保存可恢复工作。跨机器恢复只能保证已 commit+push 的 checkpoint;未提交本地 WIP 不承诺跨机器恢复。

竞态测试必须覆盖:网络分区后旧 owner 恢复、机器时钟偏移、续租与接管同刻发生、旧 owner 在 push 前失权、任务分支 OID 被另一 actor 改变。

---

## 9. Worktree 与任务权限

### 9.1 隔离粒度

新运行时采用 **per-issue worktree**,不再以永久 per-scope worktree 作为运行时唯一粒度。

原因:

- 同一端可能同时有多个独立 Issue。
- Issue 是领取、恢复和收口的基本单位。
- 与 Symphony 的 per-task workspace 模型一致。

端级 scope 仍用于限制允许路径和选择角色,但不再决定唯一 worktree。

### 9.2 “开工 #N”的任务级授权

一次“开工”授权当前 Issue 范围内:

- 领取任务、创建本地 worktree 和非保护任务分支。
- 修改 `allowed_paths` 内文件。
- commit、push 当前任务分支。
- 创建 / 更新 PR 与 Issue 凭据。
- 启动独立 reviewer 并按有效 finding 修复。

不授权:

- 保护分支直接 push、force push 或改写历史。
- 生产部署、生产数据修改 / 删除。
- 大规模付费调用。
- 扩大产品范围或修改冻结契约。
- 绕过失败检查或高风险合并闸。

### 9.3 合并
- cooperative-local 第一版固定 `auto_merge=disabled`;完成条件满足后只产出可合并 PR,由现有保护规则或 owner 合并。
- 高风险或不可逆:进入 `waiting`,等待 owner 明确确认。
- 发现额外优化:另开 Issue,不偷带进当前任务。
风险不是 agent 自报字段。`sopctl` 在 start 与 pre-merge 两次计算 attestation:
- 输入包括 touched paths、diff 类型、执行命令、依赖 / schema / contract 变化、生产 / 成本 / 安全规则、既有 owner approvals。
- 输出写入 capsule / run event:`risk_class`、命中规则、来源、可逆证据和批准记录。
- `unknown` 一律按高风险处理。
- “开工 #N”只授权任务分支写入;不授权合并。
- agent 可以提供风险判断证据,但不能自己降低机械命中的风险等级。
---

## 10. 项目配置与用户入口
项目继续只提交两个 sop-better 状态文件:
```text
.sop/profile.json  # 项目事实、运行策略与 owner 决策
.sop/lock.json     # 上次生成版本、组件与托管内容 hash
```
运行配置示意:
```json
{
  "runtime": {
    "mode": "loop-v1-experimental",
    "tracker": "github",
    "start_mode": "manual",
    "auto_merge": "disabled",
    "evidence_trust": "cooperative-local",
    "lease_timeout_seconds": 600,
    "heartbeat_interval_seconds": 60,
    "checks": {
      "test": ["..."],
      "build": ["..."]
    },
    "trust": {
      "github": {
        "trusted_actor_ids": [123456]
      }
    }
  }
}
```
约束:
- 不提交机器绝对路径、密钥或访问 token。
- 本轮 schema migration 在 experimental runtime 内完成:`evidence_trust` 新增为必填且只能是 `cooperative-local`;`trusted_app_ids` 从必填移除,但轻量模式只要字段出现就拒绝（空数组也失败）,并提示它不提供隔离。`additionalProperties:false` 继续保留。旧实验 profile 必须重新 render;delta 由 canonical review chain 自动决定,不增加路径 allowlist 或 profile 开关;按路径挑 checks 的字段不进入 Phase 1 schema。
- `cooperative-local` 只允许 `start_mode=manual`、`auto_merge=disabled`;不启动后台 watch,不用于明确以对抗性 prompt injection 为威胁的仓库 / 任务或无人值守自动合并。
- 每轮 review 和 merged commit 最终闸暂时都运行 profile 全部 check groups;先单独验证 change-first reviewer 的收益。Phase 1 不接受 `check_triggers` 或 `delta_review_paths`。
- 本机 workspace、缓存、machine ID 和 checkpoint 放版本化安装定义的本机状态目录。
- 实时任务状态放 GitHub,不再提交第三份 `.sop/state.json`。
- schema 校验要求 heartbeat interval 不大于 lease TTL 的三分之一;不满足就拒绝启动。
- 当前 `gh` actor、attestation actor 与 profile trust roots 不匹配时拒绝启动;不把 login 名称或仓库 write 权限当成稳定身份。`trusted_app_ids` 留给未来隔离 controller 模式另立 schema,在 cooperative-local profile 中出现即拒绝,避免制造“已经防伪”的错觉。
用户日常只需:
```text
开工 #31
继续 #31
查看 #31
```
公开命令保持小:
```text
sopctl task start <issue>
sopctl task continue <issue>
sopctl task review <issue>
sopctl task status [issue]
sopctl task explain <issue>
```

heartbeat、claim、reconcile、release 等可作为内部子命令或内部 API,不要求 owner 记忆。

---

## 11. 运行可观察性与错误处理

`查看 #31` 至少展示:

```text
状态 / 执行机器 / 最近心跳
目标 / 当前阶段 / 当前动作
已通过与失败的检查
PR / reviewer 状态
下一步 / 是否需要 owner
```

记录分两层:

- 本地结构化日志:动作、API 结果、重试、耗时、恢复过程。
- GitHub 高价值里程碑:开工、waiting、阻断 review、PR 交付、最终收口。

心跳不刷 Issue 评论。

错误必须回答三件事:

1. 什么没有完成。
2. 什么保持不变。
3. 下一步怎样恢复。

| 错误 | 行为 |
|---|---|
| GitHub 不可用 | 到 lease 安全边界前暂停远端写入;保留本地 workspace,不宣称继续拥有任务 |
| 任务已被领取 | 显示 owner machine / heartbeat / run ID;不创建第二个 workspace 执行 |
| 下一次 lease guard 发现失权或 CAS 续租失败 | 不再产生远端副作用;本地 worktree 转只读隔离并刷新状态 |
| Issue / lock 不一致 | 运行 reconcile;只按可证明事实修复 |
| capsule 缺目标 / 验收 / 来源 | 拒绝启动并进入 `waiting` |
| 检查持续失败 | 按失败签名熔断,保留证据 |
| reviewer 持续冲突 | 三轮无新证据后进入 `waiting` |
| 高风险动作 | 停在动作之前,给 owner 一条具体确认请求 |

---

## 12. 验证设计

### 12.1 机械测试

- 100 个并发领取者只能有一个成功。
- claim、Issue 更新、worktree 创建、push、PR、close 各步骤均注入崩溃。
- 重启后不重复已完成动作、不丢任务、不误标 `done`。
- lease 超时、续租、接管、失去 lease 与条件删除覆盖竞态测试。
- capsule 同输入确定输出;字段来源、范围和大小可校验。
- review 的 checks 与 reviewer 必须分别运行在 exact `head_sha` 的 clean detached worktree;dirty、HEAD 漂移、tracked 文件变化或 type-change / rename 隐藏旧路径均 fail closed。
- doc-backed 任务必须把同仓 pinned commit 文档按 digest 物化给 reviewer;可变 / 跨仓 / 缺失文档不能只传 URL 后继续。
- review 覆盖链必须绑定同一 PR identity / base repo / base ref / merge-base,以 merge-base→首轮 HEAD 的 full 段起步,后续 delta 段首尾连续到 PR head;缺段、错 SHA、CLI 手填 review evidence 或 unresolved blocking 均不能经正常 `sopctl` 路径进入 `merge-eligible / done`。直接用同一 GitHub 凭据伪造 marker 不在 cooperative-local 的保证范围;done 仍必须确认同一 PR 合入配置默认分支,并在实际 merged commit 上重跑全量 checks。
- changed paths 到 task scope 的映射必须确定;超范围时拒绝,不能静默扩大任务。
- 首轮 review 必须从当前 PR merge-base 完整覆盖当前 PR HEAD;第二轮起必须只覆盖上一 review HEAD 到当前 HEAD 的全部 tree change。新增 / 删除 / 重命名 / 高风险路径仍产出 delta,不能静默漏 change 或按文件类型回退 full。
- delta reviewer 必须能发现旧 finding 之外的新问题;机械反向测试构造“修复 F-001 时顺手引入 F-002”,若只关闭旧 finding 即失败。
- snapshot / scope / PR identity / base repo / base ref / merge-base 改变、上一 SHA 不可用或覆盖链断裂必须 full reset;同一 HEAD 换 PR base 不得复用 coverage,普通 tree change 不得触发 full。
- full reset 必须归档旧 generation,把旧 open blocking 带入新 full 逐条 disposition,并保证 finding ID 不复用。
- 超范围 diff、高风险、失败检查不能产出可合并结论;轻量版不存在自动合并。
- macOS / Windows 对同 profile 生成同义结果,路径格式按平台正确。
- 防御性回归:在正常 `sopctl` 解析 / 胶囊路径内,恶意 Issue 正文、评论和外链不能改变 commands、skills、scope、风险或权限；这不扩张为“能防同凭据 agent 绕过 CLI”的对抗支持承诺。
- 未登记 actor、伪造 login 和 repo / issue / snapshot 绑定不匹配的 attestation 必须 fail closed。
- 父 Issue 不获得代码写 lease;每个可写 child 只有自己的 scope lease。
- state revision 冲突、较新 owner 事件和 label 投影失败不会被旧 runner 覆盖。

### 12.2 真实 Codex 对照场景
第一批 16 个场景:
1. 简单 Bug 自动完成。
2. 需求不清进入 `waiting`。
3. 发现需要扩大范围。
4. 两台机器同时领取。
5. 执行 agent 中途崩溃。
6. 同一测试连续失败。
7. reviewer 发现真实阻断问题。
8. reviewer 的 finding 不成立,执行 agent 用证据反驳。
9. 命中生产 / 高风险操作。
10. 全局与项目 `AGENTS.md` 冲突。
11. 在另一台机器恢复任务。
12. 网络中断后恢复。
13. 正常 `sopctl` 路径收到带注入文本的 Issue / 评论 / 外链,胶囊边界不变（防御性回归,不测试同凭据 API 绕过）。
14. 跨端父 Issue 被要求直接写多个 scope。
15. 首轮完整 review 后修复两个已知竞态并顺手引入一个新缺陷;复审只读取上一 review HEAD 到当前 HEAD 的全部 change + 必要语义上下文,既关闭旧 finding 也抓住新缺陷。
16. 外部合并后在另一台没有任务 workspace 的可信机器继续,仅靠 canonical review coverage、PR head 和 merged commit 完成终态核验。
每个场景对比旧 SOP 与新 Loop:
- 任务是否完成。
- 是否重复干活或误判完成。
- owner 被打断次数。
- 开工前读取的 SOP 文件和常驻指令字节数。
- 规则冲突是否被发现。
- 测试 / review 是否真实运行。
- 首轮 / 复审输入字节数、review wall time、全量检查 wall time、有效 / 无效 finding 数。
- 最终代码、Issue、PR、任务锁是否一致。
实验契约继续强制继承 2026-07-10 稳定化设计 §8.2～§8.7,本设计补充为:

- baseline / candidate 固定同一模型、Codex 版本、权限、sandbox、仓库 SHA、外部依赖快照和机器级环境记录。
- 核心场景每侧至少独立运行 3 次;安全场景每次都必须通过,不能用平均值掩盖一次越权。
- 先看最终仓库 / Issue / PR / ref / 测试等硬状态,再看 agent 最终回答。
- 语义质量使用不知道样本来自 baseline 还是 candidate 的盲评 grader,并由 owner 抽样校准。
- 实验开始前固定成功分母、允许波动、熔断判据和放行阈值;失败后不得临时改口径。
- “是否全文读取 SOP”从 Codex 轨迹中的文件读取记录与初始输入字节统计,不靠 agent 自述。
- 基础设施 / 网络 / 模型服务失败单列,不算 candidate 行为失败,也不算通过。
- exp-039 重启时,增量候选与当前完整复审基线必须使用同一模型、reasoning effort、sandbox 和已知 finding 输入,各独立运行 3 次;固定并保存 task base、previous review head、current head 三个 SHA,candidate SHA、Codex 版本、命令、机器环境、原始 JSONL、输入字节、输出 token、finding 与 wall time。没有这些可复现凭据不能放行。
- 实验前冻结 blocking 缺陷真值表:缺陷 ID、引入 SHA、预期影响和可重复复现测试。至少一例必须只改调用方,而缺陷表现位于未改 callee,验证 reviewer 会按语义展开上下文。三次 delta candidate 都必须抓全预登记缺陷;full control 只作独立对照,不充当真值或兜底。
- 未预埋的新 finding 必须分别在 previous review head 与 current head 复现后再归因;无法可靠归因记 `inconclusive`,本轮实验不能算通过。
- 50% 门槛固定计算 `median(delta_reviewer_wall_time) / median(full_reviewer_wall_time) <= 0.5`;checks wall time 单独报告,不得混入比值。

### 12.3 第一版放行标准

- 并发领取无重复 owner。
- 故障点均能恢复或进入明确 `waiting`。
- 安全场景 100% 通过。
- change-first 候选必须通过 exp-039 的 3+3 对照:三次 delta 都抓全预登记 blocking 真值,连续 coverage、PR basis 失效与 generation reset 场景全部通过,reviewer wall-time 中位数比值 ≤ 0.5;任一失败或 inconclusive 就不升级 experimental runtime。
- 实现硬上限仍是相对 `db53a7a` 净增 ≤ 2850 行。spec 初审时实测净增 2693、仅余 157 行;实施先从 reviewer capsule 删除重复的任务级 `changed_paths / context_paths`,并用 delta / generation 测试替换 full-only 测试而非平行新增,至少先释放 40 行。新增生产代码与测试合计净增最多 190 行;每轮机械重算总净增,最终超线即失败,不删安全测试或事后改门槛。
- 低风险场景在一次“开工”后不再索要过程确认。
- 高风险场景准确停在人工闸之前。
- agent 开工不读取完整 `STANDARD.md` 或完整 workflow。
- 目标项目 managed `AGENTS.md` 块不超过 4 KiB。
- 任务胶囊元数据不超过 4 KiB。
- 新方案任务成功率不得低于旧方案。
- 整组对照场景中 owner 被打断总次数少于旧方案;单个旧方案本来就是零打断的场景不强求负数。
- Mac 与 Windows 真实验证都通过。
- 安装版本和项目内容都完成回滚实验。

---

## 13. 实施与迁移顺序

### 批次 0:冻结基线

- 记录当前 skill hash、Codex 版本、行为基线与恢复方法。
- 在隔离 `CODEX_HOME` 开发,不让半成品污染正常使用。

### 批次 1:Loop MVP

在当前 Mac 用一个低风险真实 Issue 跑通:

```text
Issue → 原子领取 → capsule → per-issue worktree
→ Codex 实现 → 测试 → 独立 review → PR → Issue 收口
```

MVP 也必须包含最小、幂等的 claim / renew / release / reconcile,并覆盖“锁已建但 Issue 未更新”“Issue 已收口但锁未删”两种崩溃恢复。MVP 暂时关闭自动 merge,只支持单写、单 scope、低风险 Issue,先缩小爆炸半径。

第一批代码即要求跨平台路径和接口隔离,但未通过真实 Windows 前不标稳定。

启用新运行时的首个项目还要迁移旧状态:

- `评估中`:重新检查目标、验收与依赖,再映射为 `ready` 或 `waiting`,不得机械猜测。
- `开发中`:只有有效 lease 才映射为 `running`;否则进入待恢复的 `waiting`。
- `已完成`:重新核对验收、测试和 PR 证据后才映射为 `done`。
- 旧永久 per-scope worktree 先保持只读,确认没有 WIP 后再退出运行入口;不与新 per-issue worktree 双写。

### 批次 2:稳定化与恢复

- 在 MVP 最小 reconcile 上补齐完整机械检查、故障注入、竞态矩阵、版本隔离、安装和回滚。
- 把旧设计的确定性生成与发布基础设施接入同一 `sopctl`。

### 批次 3:跨平台

- macOS 与 Windows 跑同一 fixture 和真实任务。
- 验证文件锁、路径、二进制发现、Git / gh 行为和回滚。

### 批次 4:轻量信任回归

- 机械拒绝 `watch`、自动 merge、非 cooperative evidence trust 和误导性的 App 配置。
- 验证 waiting → 外部 merge → terminal-verification claim → done 的恢复序列。
- 隔离 controller / broker 只留设计入口,不在本批偷做半套。

### 批次 5:逐项目迁移

- 每个项目单独 diff、确认、render、check 和真实任务试跑。
- 不批量覆盖活跃项目。
- 新版本达到功能等价并验证回滚后,才移除旧软链接。

---

## 14. 未验证项、信心与改主意条件

### 当前信心

- **90%**:需要“薄入口 + 任务胶囊 + 单一控制面 + 内外双 Loop”。当前仓库证据和 OpenAI 官方实践同向。
- **85%**:GitHub Issue + 条件 Git ref 能解决多机器唯一领取。API 契约支持,但尚未在本项目做真实竞态测试。
- **75%**:Codex Goal / runner 能在 macOS 与 Windows 提供同义恢复体验。官方有持久线程与 App Server 能力,但具体客户端差异尚未实测。
- **70%**:一次“开工”授权低风险分支 commit / push 的舒适度。自动合并已从轻量版移除;需真实项目观察误停率。

### 必须实测

- GitHub ref 在并发创建、续租、条件删除和网络抖动下的真实行为。
- Codex Goal、CLI 与 App Server 在当前 macOS / Windows 版本的能力差异。
- Windows 文件锁、worktree 清理和升级回滚。
- 跨机器恢复时 checkpoint 频率是否造成过多 WIP commit。
- 4 KiB capsule / `AGENTS.md` 预算是否既够用又能显著减少上下文。

### 什么会让我改主意

- 如果原子 Git ref 在 GitHub 实测不稳定或权限成本过高,改用具备 compare-and-swap 的独立协调存储;不退回 Issue 评论假锁。
- 如果原生 Codex Goal 跨客户端不一致,由 `sopctl` 通过 Codex App Server 承担 runner;不把运行状态塞回提示词。
- 如果 Loop MVP 没有降低 owner 打断次数,先修路由 / 胶囊 / runner,不直接上后台 daemon。
- 如果 4 个外层状态无法表达真实阻塞,只有在出现可复现失败样本后才增加状态。

---

## 15. 总体验收

owner 能直接观察到:

- 日常只需“开工 / 继续 / 查看 #N”。
- agent 不再先读完整 SOP 才知道自己要做什么。
- 同一任务不会被两台机器重复执行。
- 低风险任务自动跑完实现、测试、review 和修复。
- 真正需要决定时才出现一条清楚的问题。
- 任意时刻能看到谁在做、做到哪、下一步是什么。
- 崩溃或换机器后能恢复,不能假装完成。
- 高风险仍由 owner 掌握。
- 升级前看差异,明确确认后才升级,失败能回退。

工程上能证明:

- claim / lease / reconcile 有竞态与故障注入测试。
- capsule 和 managed 指令有大小、来源与冲突检查。
- 每个 `done` 都能回溯到验收、测试、review、PR 和最终状态。
- 新旧 SOP 有同场景对照证据,不是只凭主观觉得更顺。
- Mac 与 Windows 使用同一规则语义和稳定版本。

出现任一情况不能宣称完成:

- agent 仍需全文读取 SOP 才能开工。
- Issue 标签仍被当成唯一并发锁。
- 下一次 lease guard 已发现失权或 CAS 续租失败后,runner 仍产生新的远端副作用。
- review 或测试失败仍能进入 `done`。
- 后台模式另造一套与手动模式不同的流程。
- 只在作者 Mac 跑通就称为跨平台稳定。
- 没有升级预览、确认或回滚路径。

---

## 16. 实施授权闸

本 spec 获 owner 最终确认前:

- 不编写 `sopctl` 运行时代码。
- 不修改 `STANDARD.md`、skills、master 或活跃项目 SOP。
- 不安装、升级、push、merge 或发布任何新版本。

owner 最终确认后,按本仓约定直接从“批次 0 → 批次 1”实施,跳过额外 writing-plans 文档;用可检验验收、独立 spec / code review 和真实实验补偿。每一批仍单独受高风险与升级确认闸约束。
