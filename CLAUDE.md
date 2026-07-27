# sop-better · 仓库家规(薄入口,只放指针,不重抄公约)

本仓 = `SOP_HOME`,产出 `$sop-init`、`$sop-audit` 两个 Claude Code 技能。**唯一真相源是 `STANDARD.md`**——改任何规则先读它;其 §1 公约(反驳协议、凭据保真等)对本仓会话同样生效。

- **软链即线上**:`skills/` 软链进 `~/.claude/skills/`,**工作树就是线上**——改完即生效,切分支 = 换线上版本;干完活回 main。
- **自举闭环**:每造/改一块 = 一次撒手实验 → 记 `experiments/`(用 `_TEMPLATE.md`)→ 结晶沉 `PLAYBOOK.md`。
- **PLAYBOOK 只收实验背书的条目**:没有 exp-NNN 编号的教训不进(规矩见其头部)。
- **回灌纪律**:STANDARD / master 长出新护栏后,对活着的 lab 项目(taoxi-geo / geo-reverse / media-ops)跑 audit 回灌,别等旧规则再咬(PLAYBOOK exp-007 续)。
- **不重抄**:README / SKILL / 模板引用 STANDARD 一律指针、不复述正文(重抄必漂移 · §1.7)。
- **改 SOP 先用 audit 镜头审自己的提案**:净增行数设上限 + 反向验收(exp-005);减法/搬迁后逐条核安全护栏没丢(exp-006)。
- **新眼睛已常设授权**:本仓改动的 review 闸按 STANDARD §1.2「新眼睛是授权、不是待批」——**直接派独立子代理审,不回头问 owner、不摆确认卡**;会话 / 工具层带"除非用户要求否则不派子代理"类默认约束(Claude Code 对 Opus 5 系内置)时以本行为准;子代理真不可用则停在闸前反弹说明,**不得跳过 review 直接合**(exp-050)。
- **出题税(exp-048)**:往 STANDARD / master / skills / 本仓家规 净增一条规则,同轮找一条可删或可降级为结构的;找不到就在 exp 里明说为什么必须净增(税挂 Codex 遗产清查偿还,清查删不出任何规则则本条自废)。**修路(结构 / 时刻路标 / 行为面验证)优先于出题(新规则)**——北极星见 README。
- **提交信息必须走 `$commit-msg`**:凡要 commit,先用 `$commit-msg` skill 读当前 diff 生成/校验 commit message;不许直接手写 `git commit -m ...` 绕过。若 skill 无法调用,先明说原因再提交。
