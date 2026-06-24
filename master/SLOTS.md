<!-- master/SLOTS.md —— 槽清单 + 清洗映射。/sop-init 起手读它,知道哪些是 {{槽}}(填项目值)、哪些是项目肉(剔除)。 -->

# master/ 槽清单 + 收编规则

`master/` = 现有 SOP 母本**按触发分层**的具体形态。`/sop-init` copy 对应层 + 填下表的槽 + 剪掉不触发的层。**STANDARD §1 公约仍是唯一上位权威，master/ 只是它的具体实例化快照。**

## 层与触发

| 层 | 触发 | 落到项目哪 |
|---|---|---|
| `base/` | 总是 | `CLAUDE.md`（根）+ `docs/decisions/`（adr-template + 你补 0001/升级触发 ADR）+ `docs/project/issue-pr-workflow.md` |
| `layer-collaborators/` | ≥2 个人 | `docs/project/collaboration.md`（双角色 handoff 段） |
| `layer-multiend/` | ≥2 个端 | `docs/contracts/`（README + multiend-contracts）+ 每端 `<端>/CLAUDE.md`（end-role）+ 把 `multiend-constraints-block` 追加进根 CLAUDE.md §2 |
| `layer-parallel-agents/` | 真并行多 agent（= 上 worktree） | `docs/project/worktree-isolation.md` + 把 `coordination.md` 追加进 `docs/project/collaboration.md` |

> 三触发**正交**：端的事归端、人的事归人、并行的事归并行（口诀见 STANDARD §3）。`/sop-init` 必须按触发分流，**别按目录相邻**当门（否则复发 exp-012 挂错闸）。

## 槽（base/CLAUDE.md 内）

| 槽 | 填什么 | 空值行为 |
|---|---|---|
| `{{proj}}` | 项目一句话描述（**别写档位编号 / "单人"等窄化词**） | 必填 |
| `{{house_style}}` | 立栈参考仓**本体**（按端）；无则写"无参照" | 必填（可"无参照"） |
| `{{default_altitude}}` | 默认撒手档（按 risk 定） | 必填 |
| `{{risk_gate_items}}` | 本项目高风险动作清单（逐行，如 写生产库 / 付费 API 全量 / 调 OA） | 可空 → 只留恒定的保护分支那行 |
| `{{prod_infra_note}}` | 读生产 / VPN / 远端协同 等风险旁注（串进 §⛳"为什么排第一"） | 可空 → 整句删，不留悬挂 |

> `role_identity` **不是 find/replace 槽**：base 恒写 §1「起手先确认业务还是开发」；2 人项目的 gh 身份自动判定是 `layer-collaborators` 整段 include、不是填 base 的槽。

## 项目肉（媒体ops 等真项目自带、**不进**通用母本）

小红书 / 抖音等平台名、生产 IP（如 `47.109.x`）、北极星 / NOW.md / progress.yaml、gh 用户名、`S/C/T` 档位编号 / 两根轴 —— 这些是某项目特有，`/sop-init` 生成时不带；`/sop-audit` 见残留（如 `S1·C1`）报 `stale`。

## 收编纪律（重排时）

- `templates/` 内容是**当前的**，搬进 `master/` **基本 verbatim**、别改写（改写 = 引漂移）。
- `base/CLAUDE.md` 是唯一需 authoring 的——把 freshness 从原标准块的"情景按需读"里**升到顶上 §⛳**（治 turn-1 埋点），其余块逐字。公约文字取原 `agent-constraints.md`（媒体ops 那版渲染落后、不取）。
