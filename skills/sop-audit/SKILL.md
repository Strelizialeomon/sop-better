---
name: sop-audit
description: Use when 用户要只读体检、审计或优化现有 Codex 开发 SOP，或怀疑治理太重、结构触发错配、沟通闭环缺失、凭据失真、AGENTS.md 与 CLAUDE.md 漂移。
---

# sop-audit

先让程序查确定事实，再由 Codex 判断“这套 SOP 对这个项目是否合适”。默认只读；没有 owner 明确的修改授权就停在报告。

## 权威来源

- 规则语义从本 `SKILL.md` 目录解析 `../../rules/STANDARD.md`。
- 机械契约由同版本 `sopctl`、manifest 和 schema 执行。
- 路径从本 skill 所在 plugin 相对解析，不读取开发 checkout。
- 找不到兼容工具或规则快照时，报告安装问题并停止；不得靠人工比模板冒充机械审计。

## 流程

1. 读取项目 `.sop/profile.json`、根与端级 `AGENTS.md`、实际目录和相关治理文档。区分 profile 中 owner 已确认的决定与只能现场推断的事实。
2. 先运行：

   ```text
   sopctl check --project-root <repo>
   ```

   完整保留错误类别和文件路径。机械错误不能被语义解释成“没关系”，`check` 失败也不自动修文件。
3. 按 `../../rules/STANDARD.md` §5 做语义审计：头号查人实际承担的过度治理；再查端数、真人数、并行 agent 三个正交触发，以及沟通闭环、结构缺失、凭据真实性和旧 `CLAUDE.md` 残留。行数只能当信号，不能单独定罪。
4. findings 前填写固定的主线覆盖闸：

   | 主线项 | 状态 | 直接证据 |
   |---|---|---|
   | Refs / Closes 收口 |  |  |
   | doc / issue / PR 三件套 |  |  |
   | issue 评论分层 |  |  |
   | 查证闭环 |  |  |
   | 高风险治理项 |  |  |
   | 结构触发 |  |  |

   状态只用“覆盖、部分覆盖、冲突、缺失、偏离、过重、预建”；未触发且不存在的 surface 在证据中记 `n/a`。异常状态必须进入 finding，或在收尾卡写明豁免理由。
   - 每行证据必须完成 `STANDARD / master 权威锚点 → 项目实际落点` 对账；没有权威锚点，不得填“覆盖”。
   - 项目只有近似关键词或泛化大原则、却缺权威规则的场景限定 / carve-out 时，最多算“部分覆盖”并标信心；近似不等于覆盖。
5. 每条 finding 都给 severity、kind、目标、直接 `file:line` 证据、对应规则和最小建议。没有直接证据就不报；项目右尺寸时如实说没有大问题。
   - P1：人实际承担的过度治理；P2：结构错配，或反驳 / 查证 / 沟通闭环失效；P3：结构缺失、陈旧残留、凭据失真或交接断裂。
   - 硬编码密钥或凭据只标 P0 并指向单独安全处理，不在本 SOP 报告里展开或复述秘密。
   - 高频入口要对账：根/端 `AGENTS.md`、workflow、PR 模板、collaboration、contracts；同一规则一处写新、一处留旧时判冲突，不能算覆盖。
6. 输出顺序固定为：一句总判 → 主线覆盖闸 → 按严重度的人读 findings → 可执行 JSON → 收尾卡。JSON 形状：

   ```json
   [{"severity":"P1","kind":"over|mismatch|missing|stale|nopushback","target":"file/dir","evidence":"file:line","suggest":"最小动作"}]
   ```

   收尾卡写状态、已验证凭据、风险/未验、一个推荐下一步；没有真实分叉就不造选择题。

## 修改边界

- owner 只说“检查/审计”时不改任何文件。
- owner 明确要求修复时，先运行 `sopctl diff --project-root <repo>`；确认候选后才运行 `sopctl render` 与 `sopctl check`。
- 修复若要改 profile，复用 `$sop-init` 的临时候选流程：`diff --profile <temporary-profile.json>`，owner 看过本次 diff 后才 `render --profile <同一文件>`；不得先改项目 profile。
- push、merge、deploy、删除或 Git 历史改写仍需对应的单独授权。
