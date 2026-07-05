<!-- master/layer-collaborators/collaboration.md —— 触发:有第 2 个人(≥2 个不同的人 · 业务↔开发 handoff)。
     $sop-init 落成 docs/project/collaboration.md。若同时"真并行多 agent",再把 layer-parallel-agents/coordination.md 追加在后(单人多端多 agent 则只有那段、没有本段)。
     拆分自原 templates/collaboration.md「基线双角色段」——多 agent 协调骨架挂"并行 agent"触发、在 coordination.md,不挂"第 2 个人"。-->

# 协作约定（本项目）

> 本文件是"谁干什么、怎么不撞车"的真相源，**有协作才有**。单端单人 / 无并行 agent 没有这份（角色变焦在 agent 指令文件公约里恒定；issue+PR 归恒定流程）。**多端真并行多 agent 的协调骨架在 `coordination.md`（$sop-init 追加在本段之后）。** 跨端事实（仅多端）定义在 `docs/contracts/`，这里只管协作流。

## 协作主线(doc / issue / PR)

> 本文件不替代 `issue-pr-workflow.md`;这里只说人和 agent 怎么沿三件套交接。

- **业务 → 开发**:业务方只定"要什么" → req doc 写正文 → issue 只做索引 / 状态 / 验收入口 → 开发接 issue 前先验 doc 链接。缺 req doc / 链接打不开 / 形态没钉死 = 坏交接,反弹业务,不自补。
- **开发 → 业务**:开发发现范围 / 形态 / 验收要变,或需要业务裁决 → issue 评论写清"要拍什么 + 影响 + 推荐 / 选项",加 `待业务确认` / `待澄清`;不要把业务问题埋进 PR。
- **开发 → 开发**:方案变化、字段 / 契约调整、端间影响 → 写 issue 评论当消息总线;PR 评论只处理这次 diff。接手 agent 先看 issue 评论和 doc,再看 PR。
- **PR 收口**:中间交付 / doc PR 用 `Refs`;只有验收 / 关闭条件满足的最终交付才 `Closes`。

## 角色（基线 · 双角色小团队）

- **业务 / 需求方**：提需求 → agent 据此开 issue（一句话需求 + 验收 + 链到 req doc）。只管"要什么"，不碰技术架构。**业务会话有开工 / 收工仪式**：开工先扫 `待业务确认` 的 issue 逐条拍；收工把需求落 issue + req doc 按 `issue-pr-workflow.md` 推远端（文档 main,受保护才 PR;代码走 PR）。
- **开发**：认领 issue → 开分支 `<type>/issue-N-slug` → 实现 → PR（按 `issue-pr-workflow.md`:默认 `Refs #N`,最终收口才 `Closes #N`）→ 低风险自审自动合、高风险回人审。
- （单人 + 多 agent 时）主窗口 = 你；scope agent = 派出去的实现者。**派单 prompt 必须自包含**——subagent 看不到主窗口上下文。

## 业务端起需求（对话 → 收口 → issue + req doc）

> **开工先按 `issue-pr-workflow.md` 的起手 freshness**：`git fetch` sync + 扫 `待业务确认` 的 issue 一条条拍，再聊新需求。

1. **对话**：业务方跟 agent 聊需求，agent **主动建议补盲区**（可行性 / 怎么拆 · 照亮不代选），业务方拍——循环到收口。
2. **收口标准**（钉死才算，不是聊到完美）：**① 验收（怎么算对）· ② 主流程（哪几步）· ③ 范围（要啥 / 不要啥）· ④ 形态（交付长什么样：网页 / 按钮 / App / 定时任务…，用业务话定、别默认 CLI）**。
3. **落凭据**：**先把 req doc（`docs/superpowers/specs/` · brainstorming 默认落点）commit+push 到远端、再开需求 issue 指过去**（issue 轻量：idea + assignee + **指向 doc 的稳定链接 commit/PR**，别指会悬空的相对路径；"开 issue"和"推 doc"不许分两步漂着）。**req doc 只写"要什么"（语义级：做什么 / 验收 / 主流程 / 形态 / 跨端换什么数据），绝不碰"怎么做"**——技术留给 dev brainstorm。
4. **交棒**：收口后球交 agent 实施，业务方只在高风险闸 / 联调露面。

## 不撞车规则

- 每个任务独立工作；并行任务并行派，不串行。
- **永不阻塞**：卡住了主动汇报进度，不默默卡死别人。**但「永不阻塞」≠ 替别人补缺的交付物**：dev 起手接需求**先验 req doc 链接解析得开**；解析不到 / 别的角色该给而缺（req doc / 契约）= 坏交接 → **反弹回该角色 + ⏸️ 待澄清，不自己补**（自补 = 伪造需求）。

## 红线

- 不擅自 commit / push（需 owner 明确指令）。**owner 说"收工 / 结束"即此明确指令** → 把本会话收口的 doc 按 `issue-pr-workflow.md` 推远端（文档 main / 代码 PR）。
- 不跑 writing-plans；spec 验收要硬 + code review 必留（见 agent 指令文件）。
- 治理过重要主动喊停——右尺寸 > 全面。
