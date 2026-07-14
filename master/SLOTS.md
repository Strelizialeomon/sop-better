<!-- master/SLOTS.md —— 槽清单 + 清洗映射。$sop-init 起手读它,知道哪些是 {{槽}}(填项目值)、哪些是项目肉(剔除)。 -->

# master/ 槽清单 + 收编规则

`master/` = 现有 SOP 母本**按触发分层**的具体形态。`$sop-init` copy 对应层 + 填下表的槽 + 剪掉不触发的层。**STANDARD §1 公约仍是唯一上位权威，master/ 只是它的具体实例化快照。**

## 层与触发

| 层 | 触发 | 落到项目哪 |
|---|---|---|
| `base/` | 总是 | Codex agent 指令文件 `AGENTS.md` + `docs/decisions/`（adr-template + 你补 0001/升级触发 ADR）+ `docs/project/issue-pr-workflow.md` |
| `layer-collaborators/` | ≥2 个人 | `docs/project/collaboration.md`（双角色 handoff 段） |
| `layer-multiend/` | ≥2 个端 | `docs/contracts/`（README + multiend-contracts）+ 每端 `<端>/AGENTS.md`（end-role）+ 把 `multiend-constraints-block` 填进根 `AGENTS.md` 的 `{{multiend_constraints}}` 槽 |
| `layer-parallel-agents/` | 真并行多 agent（= 上 worktree · **前提：已多端**；单端不单独触发） | `docs/project/worktree-isolation.md` + 把 `coordination.md` 追加进 `docs/project/collaboration.md` |

> 三触发**正交**：端的事归端、人的事归人、并行的事归并行（口诀见 STANDARD §3）。`$sop-init` 必须按触发分流，**别按目录相邻**当门（否则复发 exp-012 挂错闸）。

## 槽（base/AGENTS.md 母本内）

| 槽 | 填什么 | 空值行为 |
|---|---|---|
| `{{proj}}` | 项目一句话描述（**别写档位编号 / "单人"等窄化词**） | 必填 |
| `{{house_style}}` | 立栈参考仓**本体**（按端）；无则写"无参照" | 必填（可"无参照"） |
| `{{default_altitude}}` | 默认撒手档（按 risk 定） | 必填 |
| `{{risk_gate_items}}` | 本项目高风险动作清单（逐行，如 写生产库 / 付费 API 全量 / 调 OA） | 可空 → 只留恒定的保护分支那行 |
| `{{prod_infra_note}}` | 读生产 / VPN / 远端协同 等风险旁注（串进 §⛳"为什么排第一"） | 可空 → 整句删，不留悬挂 |
| `{{multiend_constraints}}` | 多端时填 `layer-multiend/multiend-constraints-block` 的 3 条（端划分 / 独立 brainstorm / 状态标记） | 单端 → **整行删**（不在 doc 中段动刀，守 freshness 不复埋的不变量） |

> `role_identity` **不是 find/replace 槽**：base 恒写 §1「起手先确认业务还是开发」；2 人项目的 gh 身份自动判定是 `layer-collaborators` 整段 include、不是填 base 的槽。

## 层槽（`layer-*/` 内 · $sop-init 生成对应层时填）

| 槽 | 在哪层 | 填什么 |
|---|---|---|
| `{{ends}}` | multiend / parallel / coordination | 端清单（如 admin / backend / frontend / crawler） |
| `{{End}}` / `{{end}}` / `{{end_dir}}` | end-role（每端一份） | 端名首字大写 / scope 小写 / 端目录名 |
| `{{project}}` | end-role | 项目名 |
| `{{base_branch}}` | end-role | 工作基线分支（如 `master` / `main` / `dev`） |
| `{{stack}}` / `{{end_docs}}` / `{{impl_vocab}}` | end-role | 本端技术栈 / 本端独有常读 doc / 实施层词汇（Step 3 brainstorm 拍的） |
| `{{end_milestones}}` / `{{end_high_risk}}` | end-role | 本端特有评论里程碑 / 本端特有高风险项；没有就删整行，不留“无”占位 |

> 各 layer 文件头注里也自带槽说明（单一真相源，本表是索引，别两处都写细节）。

## 项目肉（媒体ops 等真项目自带、**不进**通用母本）

小红书 / 抖音等平台名、生产 IP（如 `47.109.x`）、北极星 / NOW.md / progress.yaml、gh 用户名、`S/C/T` 档位编号 / 两根轴 —— 这些是某项目特有，`$sop-init` 生成时不带；`$sop-audit` 见残留（如 `S1·C1`）报 `stale`。

## 收编纪律（重排时）

- `master/` 由原 `templates/`（已并入本目录、不再单独存在）**verbatim 重排**而来；后续改动直接改 `master/`。搬运时**半角→全角标点 / 斜杠加空格 / 标题层级**这类**格式归一不算改写**（audit 双向比别报成 drift），但**一个词都不许增删**。
- `base/AGENTS.md` 是根 agent 指令文件的母本——把 freshness 从原标准块的"情景按需读"里**升到顶上 §⛳**（治 turn-1 埋点），其余块逐字。公约文字取原 `agent-constraints.md`（媒体ops 那版渲染落后、不取）。
