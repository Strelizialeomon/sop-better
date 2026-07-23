# exp-046 · 公约层三条漏进触发层:base 补取活入口 / 两个书挡 / 不假民主+escalate(mobile-os 实测回灌 · #24)

- **日期**:2026-07-23
- **触发**:issue #24——`$sop-audit` 在 mobile-os(7 端 · 单人 · 真并行 · 高风险真机)实测,owner 原话「为什么我感觉有时候 agent 不知道自己下一步要做什么」。体检结论项目侧右尺寸,根因在母本:STANDARD §1 公约层的三组规则漏渲染进了触发层。
- **决策**:按 #24 的 F1–F3 补 `master/base/CLAUDE.md`,`$sop-audit` SKILL step 4 补「取活与书挡」检查项,coordination.md 收工书挡降为指针。

---

## 病灶:公约层内容落错了层(#24 已给可复现 grep 证据)

STANDARD §1 开宗明义「跟项目规模无关」,base/CLAUDE.md 头注也写明「所有项目都有」。但 `书挡 / 待业务确认 / 不假民主 / 选择题 / escalate / 取活 / 永不阻塞` 七个关键词在 base 全为 **0**——只活在 `coordination.md` / `end-role-claude.md` 这些触发层里:

- 不触发 parallel 层的项目永久拿不到 §1.8 两个书挡、§1.9 不假民主 / escalate 不是选择题;
- 不触发 multiend 层的单端项目永久拿不到任何取活入口;
- `不假民主` 连 coordination.md 都没有——任何生成物都拿不到。

**表现出来是「agent 不知道下一步」,实际是「不敢自己走下一步」**(缺 §1.9 限定,遇岔路就把问题甩回给人)。

## 改了什么

- `master/base/CLAUDE.md` §2「Agent 工作约束」:
  - **F1+F2 合一条**:新增「取活与书挡(仅项目用 Issue 时)」——`gh issue list --state open` 取活(多端被端文件 scope 版覆盖);开工书挡(标 `待业务确认` 主动 surface);收工书挡(owner 说「收工」= 推收口 doc 的明确指令,未约定远端交付不补造)。
  - **F3**:「自决不阻塞」条追加两句——已被 SOP / 已定方案 / owner 指令覆盖的事直接执行不摆回 pick(不假民主);确需升级自己写影响 + 草案 + 建议再报一句,owner 只拍做不做和优先级(escalate 是自决动作、不是选择题)。
- `master/layer-parallel-agents/coordination.md`:收口步的收工书挡句降为指向根 CLAUDE.md 的指针(§1.7 不复述);「owner 会话顺带扫待业务确认」保留——那是 owner 侧动作,与 agent 侧 surface 互补、非复述。
- `skills/sop-audit/SKILL.md` step 4 补一条「取活与书挡」——#24 是逐行 diff 母本才逮到的,不进必查清单换个不做全量 diff 的 audit 就漏。

## 反向验收 / 自审

- **净增行数**:base 净 +1 行(+2/-1:一条新 bullet + 一条追加长句),coordination.md 净 0(替换),SKILL +1 行。三条都是 STANDARD §1 已承诺「任何项目照搬」的欠账,属补欠、不是加新规矩(#24 边界节自判同此)。
- **单一真相源**:base 渲染 §1 条目是「母本→生成物」的渲染关系,不算重抄;coordination.md 收工句降指针,避免两处漂。
- **归层判据结晶**:规则归哪层,看它**自己的管辖声明**(§1 说「任何项目」就必须进 base),不看它最初在哪个项目 / 哪层被发现——三条全是先在多端并行项目被踩出来、就近沉在触发层,从此单端项目永久缺席。

## 下游参照 / 未验证

- mobile-os 已按同三条自行补齐(mobile-os#133,scope 标签 + 20 个 open issue 回填,取活查询实测可用)——母本这次是追认 + 普及到所有生成物。
- 只在 mobile-os 一个项目实测;但 F1–F3 是母本静态缺失,与项目无关,不需更多样本即成立(#24 边界节)。
- 补进 base 后未重跑 `$sop-init` 验证新生成物;留给 #15 说的集中 sweep 一并做。
- 「取活入口」对不用 issue 的项目是 n/a——条目带「仅项目用 Issue 时」限定,audit 不应对无 issue 项目硬报缺失。
