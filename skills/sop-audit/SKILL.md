---
name: sop-audit
description: Use when 用户要体检/审计/优化现有开发 SOP,怀疑 SOP 太重、结构错配、沟通闭环缺环/口号化/过度阻塞、结构缺失、凭据失真,或需要检查 Codex AGENTS.md 与旧 CLAUDE.md 残留。
---

# sop-audit

给项目的**开发 SOP** 做体检。对照 STANDARD 挑"不合理",**头号挑"太重"**(过度治理)。**默认只出报告、不动文件;owner 明确说"改 / go"才动手修。**

**配套**:`SOP_HOME = /Users/sunchongsheng/code/sop-better/`;`STANDARD.md = $SOP_HOME/STANDARD.md`。**先读 STANDARD**,规则以它为准。

## 铁律

- **默认只读**:只出报告、不动任何文件。**owner 明确说"改 / go"才动手修**——按 severity 走 issue/PR 工作流落地(见流程第 8 步)。没听到"改"就停在报告。
- **头号查"过度治理"——只算人掏的成本**:人手开 issue / 贴 label / 等每个 PR 才叫过度;**agent 自动维护、agent 消费的 issue/PR 不算**(那是撒手基础设施,冤枉它=cry wolf)。
- **每条 finding 必须带证据**(file:line / 行数 / 具体仪式名),空喊"太重了"=违规(反驳协议在审查上的落地)。
- **行数 ≠ 罪证**:体量只是**信号**,不直接判"错";说清"这是信号,该不该砍要看内容",别越权定罪。
- **不 cry wolf**:右尺寸的项目如实说"没大问题",别为显得有用硬挑。

## 流程

1. **读 `STANDARD.md`**(§3 一条流程+结构按现实长 + §5 查法 + §1 公约)。
2. **测目标项目的实际(几个端 / 几个人 / 风险)**:
   - **几个端**:数后端/服务/子项目(单 backend / 前端 = 单端;前后端小程序爬虫多个 = 多端;纯脚本 = 无端)。**≥2 端该有契约/按端文档**。
   - **几个人**:扫 `docs/collaboration*`、agent 指令文件(`AGENTS.md`)、旧残留(`CLAUDE.md` / `.claude/`)、scope label、gh 协作者 → 只有 owner = 单人;业务↔开发 / 小团队 / 多端 scope agent = 多人。**≥2 人该有协作 doc**。(issue+PR 人人都有、不据此判人数)
   - **风险**:碰生产库 / 付费 API 全量 / 改远端 = 高、不可逆。
3. **比"该有 vs 实际"**:STANDARD §3 由端 / 人 / 并行 agent 推出该有结构;扫项目实际治理文件 / 仪式;两边相减。
   - **结构相减**:第 2 个端 → contracts + 端级 `AGENTS.md`;真并行多 agent → worktree + 协调骨架;第 2 个人 → collaboration handoff。
   - **模板版本差**:治理 doc 多是 `$sop-init` 从 `$SOP_HOME/master/` 生成的快照。按目标项目触发命中的 layer,比项目 `issue-pr-workflow.md` / `collaboration*.md` / `AGENTS.md` 与当前 `master/` 对应文件。
   - **layer-gated**:只比项目触发命中的层;单端别拿 layer-multiend 比。
   - **slot-masked**:`{{槽}}` 是洞,不参与漂移判断。
   - **missing**:母本有、项目缺的新规则 / carve-out = 漂移根因;建议回灌。
   - **stale**:项目留着模板已删的块 / 档 / 文件,如极简块、旧档位编号、旧模板名、`CLAUDE.md` / `.claude/`;建议换成现行块或迁到 `AGENTS.md`。
   - **近似 ≠ 覆盖**:项目有近似版时逐条语义比。若母本有具体边界 / carve-out,项目只搬大原则、漏限定,仍报 `missing` 并标信心。普通措辞 / 详略差异按合理本地偏离宽放。
4. **主线覆盖闸(防漏停顿点)**:在输出 findings 前,必须先填 6 行表;这是覆盖检查,不是第六类 finding。
   - **权威锚点**:每行先指向当前 `STANDARD.md` / `master/` / `skills/sop-audit/SKILL.md` 对应规则,再指向项目落点。只搜到项目侧近似关键词,不能算覆盖。
   - **证据强度**:证据行必须直接证明判断。相邻文件、泛化模板、只写"改了什么"这类弱证据不能证明三件套 / 风险 / 收口分工。
   - **入口冲突**:高频入口冲突不得写覆盖。
   ```md
   ## 主线覆盖闸

   | 主线项 | 状态 | 证据 |
   |---|---|---|
   | Refs / Closes 收口 | 覆盖 / 部分覆盖 / 冲突 / 缺失 / 偏离 | 母本 file:line -> 项目 file:line / 缺落点 |
   | doc / issue / PR 三件套 | 覆盖 / 部分覆盖 / 冲突 / 缺失 / 偏离 | 母本 file:line -> 项目 file:line / 缺落点 |
   | issue 评论分层 | 覆盖 / 部分覆盖 / 缺失 / 偏离 | 母本 file:line -> 项目 file:line / 缺落点 |
   | 查证闭环 | 覆盖 / 部分覆盖 / 缺失 / 过重 | STANDARD/SKILL file:line -> 项目 file:line / 缺落点 |
   | 高风险治理项 | 覆盖 / 部分覆盖 / 冲突 / 缺失 / 偏离 | 母本 file:line -> 项目 file:line / 缺落点 |
   | 结构触发 | 匹配 / 部分覆盖 / 缺失 / 预建 / 偏离 | STANDARD file:line -> 项目结构证据 |
   ```
   - **状态词口径**:
     - `覆盖/匹配`:触发的必查 surface 都有等价规则,且无高频入口冲突。
     - `部分覆盖`:主文档有规则,但入口不全 / 执行点缺失 / 低频提示缺引用。
     - `冲突`:一个高频入口写新规则,另一个仍写旧规则。
     - `缺失`:触发条件已满足但无执行落点,或只有 spec / 历史记录没进执行 SOP。
     - `偏离`:有明确本地替代方案且能说明等价。
     - `过重/预建`:未触发却提前上结构,或低风险小事被流程拖重。
   - **surface 适用性**:
     - surface 是“存在则查、触发才必备”。
     - 已触发且缺文件 / 缺规则 = `缺失`。
     - 未触发且不存在 = `n/a`,只写证据里,不做表格主状态。
     - 未触发却存在治理骨架 = `预建` 或 `过重`。
     - 文件存在必须查内容,内容冲突不能被文件存在掩盖。
     - PR 模板不存在时记 `n/a`,不自动报缺失。
     - 第 2 个端触发 `docs/contracts/` 和端级 `AGENTS.md`;串行只豁免 worktree / coordination。
   - **必查 surface(高频入口 + 执行落点)**:
     - Refs/Closes:PR 模板、根 `AGENTS.md` 速查、workflow、collaboration PR 流程。
     - 三件套:workflow 是否明确定义 doc / issue / PR;根 `AGENTS.md` / collaboration 是否漏 PR 或互相替代。
     - issue 评论分层:workflow 厚/短评论触发 + 最小字段;根 `AGENTS.md` 自检短引用。
     - 查证闭环:根 `AGENTS.md` 沟通约束、workflow 起手凭据校验、UI 风格来源闸(仅新增 / 大改 UI)、是否过度联网 / 确认 / 长卡片 / 样式确认。
     - 高风险治理项:根高风险闸、workflow PR 合并规则、PR 模板风险区。
     - 结构触发:端 / 人 / 并行 agent 三触发、实际目录、contracts、端级 `AGENTS.md`、协作 doc、worktree / coordination。
   - **报告装配校验**:
     - 同一主线项多个高频入口冲突,合并成一个 finding,但 target / evidence 必须列全;规则冲突 kind 用 `mismatch`。
     - PR 模板把“文档”列低风险时,继续查是否显式排除治理 doc(`AGENTS.md` / SOP / collaboration / workflow / PR 模板自身)和 `docs/contracts/` / 跨端契约骨架。
     - 三件套判 `覆盖` 时,证据必须直接证明 doc / issue / PR 三者分工;证明不全最多 `部分覆盖`。
   - **判定顺序**:`冲突` → `缺失` → `部分覆盖` → `过重/预建` → `偏离` → `覆盖/匹配`。
   - **状态转 findings**:
     - `覆盖 / 匹配` 不进 findings。
     - `冲突` 通常进 finding;高频入口优先级不低于 P2/P3。
     - `缺失` 进 finding;结构触发缺失按 STANDARD §5.2 优先判 P2 `mismatch`。
     - `部分覆盖` 看缺口位置;缺高频执行入口则进 finding。
     - `预建 / 过重` 按 STANDARD §5.1/§5.2 判 severity。
     - `偏离` 必须一句话判断等价理由;不成立就改成 `缺失` 或 `冲突`。
     - 主线表必须和 findings 对账;异常项未进入 findings 时,必须在收尾卡写豁免理由。
     - 表固定 6 行,新增主线必须替换旧项,不能追加。
5. **按 §5 五类出 finding,每条标 severity + 证据**。**查法细则全在 STANDARD §5(第 1 步已读),本技能不重抄(重抄必漂移),这里只定 severity 映射**:
   - **P1 过度治理(头号)**= §5.1(人手跑的仪式过重 + 死规则笼子;「只算人掏的成本、agent 自动维护不算」见铁律)。
   - **P2 结构错配**= §5.2(含 ④删除残留 · kind `mismatch`/`stale`);**P2 沟通闭环缺环 / 口号化 / 过度阻塞**= §5.3(反驳+说人话、查证、分流、调研、执行验证、收口;含新增 / 大改 UI 漏确认风格来源,以及脚本 / 后端 / 既有样式小修被拖进样式确认)。
   - **P3 结构缺失**= §5.4;**P3 凭据失真 / 交接断裂**= §5.5(含 doc/issue/PR 三件套分工失真、悬空链接、开工/收工书挡〔所有项目〕、协作总线断节、评论凭据塌缩、决策闸误关)。
   - **P0 仅指针**:扫到硬编码密钥/凭据 → 只点一句"另走安全 track",**不在本体检展开**。
6. **出双轨报告**:
   - **(a) 人读**:开头一句总判("太重 / 刚好 / 太轻" + 实测 几端 / 几人 / 风险);然后按 severity 排,每条 = `现象 + 为什么不对(对照 STANDARD 哪条)+ 建议(降到哪 / 补什么)+ 证据`。
   - **(b) 可执行 findings**(owner 说"改"时据此动手;也可存盘供以后照单改):
     ```json
     [{"severity":"P1","kind":"over|mismatch|missing|stale|nopushback","target":"file/dir","evidence":"...","suggest":"..."}]
     ```
7. **收尾卡(任务收口卡在 audit 报告里的落地)**:报告末尾必须用轻量收口卡,别只写旧式一句话推荐。
   - **状态**:本次 audit 是「无阻断 / 可直接改 / 需 owner 先决策 / 需补现场复核」哪一种。
   - **验证**:列已查的关键凭据(如 STANDARD / 母本 / AGENTS / workflow / GitHub 实况);没查到或查不到的明说。
   - **风险 / 未验**:哪些 finding 依赖现场数据、权限、时间点或用户选择;别把未验当事实。
   - **推荐下一步**:给 1 条最建议动作。
   - **可选动作**:只有真有分叉才给 2-3 个选项并标推荐;没有分叉就不造选择题。
   - **高危边界**:推荐 push / 批量改 label / deploy / 删除 / 改 git 历史 等不等于授权,仍等 owner 明确确认具体动作。
8. **若 owner 看完说"改 / go"** → 按 `master/base/docs/project/issue-pr-workflow.md` 落地:开 issue 记 findings → 分支 → 改 → PR(`Refs`)→ 按风险审合(低风险自动合 / 高风险回 owner)。**没说"改"就停在第 7 步。**

## 禁止

- **owner 没说"改"就擅自动文件**(默认只读;动手前必须拿到明确 go)。
- 无证据的 finding(凭印象喊"太重")——违反反驳协议。
- cry wolf:右尺寸项目硬挑毛病凑数。
- 把"体量大"直接等同于"错"——只能当信号。
