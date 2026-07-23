# sop-better

`sop-better` 是一个 **Claude Code 开发 SOP 工具仓**。

它产出两个 skill:

- `$sop-init`: 给项目生成右尺寸的开发 SOP 骨架。
- `$sop-audit`: 给现有项目体检,查 SOP 太重、太轻、漂移或凭据失真。

这个仓的目标不是把规则越写越多,而是把 agent 的工作压成一条能执行的 flow:先查证,再分流,该调研就调研,做完要验证和收口。**给 agent 一条路,不是让它背题。**

> 唯一真相源是 [`STANDARD.md`](STANDARD.md)。README 只做入口说明,不承载具体规则。

---

## 当前主线

### 1. 两层模型

一份 SOP = **不变公约** + **按项目现实生成的结构**。

- 不变公约:人机分工、反驳、查证、review、高风险闸、触发后的凭据保真等。
- 可变结构:这个项目有几个端、几个人、风险多高,就生成多少结构。

换句话说:安全底线是同一组,具体流程动作和目录结构都按现实长。

### 2. 一条运行 flow

agent 干活默认走这条闭环:

```text
查证 -> 分流 -> 调研 -> 执行验证 -> 收口
```

- **查证**:本地事实查代码 / 配置 / 日志 / issue / PR;外部事实查可靠来源。查不到就说不知道。
- **分流**:低风险局部事直接做;缺目标 / 范围 / 验收、跨边界或高风险就确认。
- **调研**:给方案 / 设计 / 估算 / 选型前,先给调研结论、主流做法、建议方案、信源和风险。
- **执行验证**:改完主动跑能跑的验证;不能跑就明说。
- **收口**:结束时交代状态、验证、风险和推荐下一步。

### 3. 结构按现实长

旧版曾用档位枚举。现在已经收成更简单的触发规则:

| 现实触发 | 才生成什么 |
|---|---|
| 有第 2 个端 | 契约、按端身份文档 |
| 多端还要真并行多 agent | worktree 隔离、多 agent 协调骨架 |
| 有第 2 个人 | 业务↔开发协作 handoff |
| 要跨会话跟踪 / 角色交接 | Issue |
| 要远端交付 / 他人评审 / 保护分支收口 | 分支 + PR |

没有就不建。右尺寸的意思不是“少”,而是“该有的有,不该有的别预建”。

### 4. 协作用三件套

任务触发持久协作 / 交付时,业务↔开发、开发↔开发按这三件套协作:

| 凭据 | 职责 |
|---|---|
| doc | 正文 / 真相源:需求、设计、契约、长期决策 |
| issue | 索引 + 状态 + 消息总线:谁在做、等谁、变化去哪看 |
| PR | 交付凭据 + 验收 / 收口动作:改了什么、怎么验、是否关闭 issue |

口诀是: **doc 写正文,issue 跑状态和消息,PR 写交付验证**。

低风险、单会话、可逆的小改不为凑齐三件套强制开 Issue / PR。触发 Issue 后,评论再按凭据价值分层:影响后续判断的评论要写厚,只贴链接 / tag / 轻进度才短评。细则在 [`master/base/docs/project/issue-pr-workflow.md`](master/base/docs/project/issue-pr-workflow.md)。

### 5. 代码复审先审变化

第一次复审看“任务起点到当前 HEAD”的完整任务 change;后续复审只看“上次已审 HEAD 到当前 HEAD”的新增 change。reviewer 可以顺着变化查看相关函数、接口、调用方、契约和测试,但不会重新扫描无关仓库。基线失效或历史不连续时,回退到完整任务 change。

---

## 两个 Skill

| Skill | 用途 | 默认行为 |
|---|---|---|
| [`$sop-init`](skills/sop-init/SKILL.md) | 给新项目或已长大的项目生成 / 补齐 SOP | 按端数、协作人数、风险右尺寸生成 |
| [`$sop-audit`](skills/sop-audit/SKILL.md) | 审现有 SOP 是否不合理 | 默认只出报告;owner 明确说改 / go 才动文件 |

当前两个 skill 都已软链进 `~/.claude/skills/`。本仓工作树就是线上版本,改完即生效。

没有单独的 `$sop-improve`:audit 找到问题后,owner 点头就改。不为还没出现的需求养第三个 skill。

---

## 设计原则

### 头号敌人:过度治理

对一个人指挥 agent 来说,最常见的问题不是“规则不够”,而是“规则吃掉杠杆”。

所以 `$sop-audit` 头号查:

- 人是不是被迫跑太多仪式。
- 单人项目是不是装了多人协作机器。
- 单端项目是不是预建了多端契约。
- 小任务是不是被强制开 Issue、分支、PR、worktree 或启动额外编排工具。
- SOP 是不是越写越像笼子。

agent 自动执行也有耗时、失败和调试成本。是否过度,看它有没有换来真实的跟踪、协作、隔离或交付价值。

### 撒手不是盲盒

本仓默认反对两种极端:

- 人永远 L0:什么都自己写,agent 只打字。
- agent 永远盲盒:人完全看不见判断和风险。

正确状态是:人定“要什么”,agent 负责“怎么做”;高风险、人类决策、业务边界仍然回到 owner。

### 凭据必须保真

issue / PR / doc / ADR 是 agent 的共享内存。

共享内存有用的前提是它是真的:

- issue 状态要反映现实。
- PR 要写验证和风险。
- doc 链接要能在远端打开。
- 决策 / 实测 / 收口 / 状态校正不能只写一句短评糊过去。

会说谎的凭据比没有凭据更危险。

---

## 自举方式

sop-better 用自己的方法造自己:

1. 真实项目里踩坑或验证。
2. 记一条 [`experiments/`](experiments/)。
3. 把可复用教训沉到 [`PLAYBOOK.md`](PLAYBOOK.md)。
4. 真成模式,才回灌到 [`STANDARD.md`](STANDARD.md) / [`master/`](master/) / skills。

近期主线可以从这些实验看:

- exp-024:诚实无知闸。
- exp-025:沟通规则瘦身成“查证 -> 分流 -> 调研 -> 执行验证 -> 收口”。
- exp-027:doc / issue / PR 三件套协作主线。
- exp-028:issue 评论凭据分层。
- exp-039:change-first review——先审本轮变化,语义上下文按需展开,不重扫无关仓库。
- exp-040:回归初衷——把 STANDARD §1 的双重叙述和 sop-audit 的覆盖闸机器砍薄,让工具能通过自己的 audit。
- exp-041:从 Codex-only 翻回 Claude 规范(`AGENTS.md` → `CLAUDE.md`)。
- exp-042:去耦 superpowers 残留——换工具时 de-name 到概念、不 name-swap;user-invoked 的 grill-me 不进 SOP。
- exp-043:dogfood mobile-os audit——SOP 健康不 cry-wolf;superpowers 残留复发(且渗进测试路径)→ §5.2 点名"已卸载外部工具残留"这一类。
- exp-044:worktree 粒度从按端回灌为按 issue/任务(mobile-os 实测)——on-demand 用完即弃、仓内 `.worktrees/`、端身份⊥worktree,并把"合并即清"清理纪律一起沉。
- exp-046:公约层三条(取活入口 / 两个书挡 / 不假民主+escalate)漏进触发层 → 回渲染进 base;audit 必查清单补"取活与书挡"(mobile-os 实测,#24)。

早期设计记录在 [`docs/specs/`](docs/specs/)。

---

## 仓库结构

```text
sop-better/
├── README.md                    # 项目入口说明
├── CLAUDE.md                    # 本仓 Claude Code 工作约定
├── STANDARD.md                  # 唯一真相源:生成与审计的权威尺
├── skills/
│   ├── sop-init/                # 生成右尺寸 SOP
│   └── sop-audit/               # 审计 / 优化现有 SOP
├── master/                      # 母本:base + 按触发追加的 layer
├── docs/specs/                  # 历史设计 spec
├── experiments/                 # 每次 dogfood / 回灌实验
└── PLAYBOOK.md                  # 从实验沉淀出的护栏
```

---

## 读文件顺序

如果你是第一次看这个仓:

1. 先读本 README,知道项目在干什么。
2. 再读 [`STANDARD.md`](STANDARD.md),看当前权威模型。
3. 要看生成产物,读 [`master/base/`](master/base/) 和对应 layer。
4. 要理解规则怎么长出来,读 [`PLAYBOOK.md`](PLAYBOOK.md) 和最近的 `experiments/`。

改规则时反过来:先查 `STANDARD.md`,再动 `master/` 或 skills,最后补 experiment / PLAYBOOK。
