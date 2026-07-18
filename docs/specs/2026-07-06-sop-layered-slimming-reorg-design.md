# SOP 三层瘦身重排 · Design

## 1. 背景

sop-better 已经在短时间内连续吸收多轮真实项目反馈:

- 诚实无知与互联网调研。
- 查证 -> 分流 -> 调研 -> 执行验证 -> 收口。
- doc / issue / PR 三件套。
- issue 评论凭据分层。
- `$sop-audit` 主线覆盖闸与报告装配校验。

这些规则大多是对的,问题不在语义本身,而在组织方式开始横向叠加。现在的核心风险是:agent 读到的是一层层补丁,不是一条顺手执行的 flow。

本轮目标是做一次无语义新增的瘦身重排:把已有规则理顺,不再继续加规则。

## 2. 目标

把项目重新理成三层承载面:

> 这里的“三层”只指 sop-better 仓内文件职责分层,不替换 `STANDARD.md` 的两层 SOP 模型。`$sop-init = 固定公约 + 项目实例化` 仍是地基。

1. **权威层: `STANDARD.md`**
   - 定义原则、边界、触发条件和审计分类。
   - 保留为什么这条规则存在,但不承载过长事故复盘。
2. **执行层: `master/` 母本**
   - 定义 agent 到项目里实际怎么做。
   - 保留起手 freshness、沟通闭环、issue/PR flow、协作 handoff、端级身份等执行动作。
3. **审计层: `skills/sop-audit/SKILL.md`**
   - 定义怎么检查项目是否跑偏。
   - 保留主线覆盖闸,但让它更像顺序检查,而不是长段背题。

核心原则:同一条规则只在一层完整定义,其它层只短引用或渲染。

## 3. 非目标

- 不新增规则语义。
- 不新增第 7 个主线覆盖项。
- 不新增 skill。
- 不把 `PLAYBOOK.md` 变成规则入口。
- 不拆出一组新权威文档。
- 不改“跳 writing-plans”的项目主张。
- 不把普通文档 PR 误判为治理高风险,也不把治理 doc / 契约 / SOP / PR 模板自身误放进“文档低风险”。

## 4. 设计

### 4.1 `STANDARD.md`:权威骨架瘦身

`STANDARD.md` 继续是唯一真相源,但表达上从“事故解释 + 执行动作 + 审计口径混写”收成“原则 + 边界 + 指针”。

重点处理:

- **新眼睛 review**:保留 spec review + code review 的原则、触发和豁免;事故细节压短并指向 `PLAYBOOK.md` / experiment。
- **凭据保真**:保留 doc / issue / PR 分工、悬空链接、评论分层、开工/收工书挡;把执行细节交给 `issue-pr-workflow.md`。
- **运行时闭环**:保留五步 flow 和关键边界;不要把每一步扩成一套平行协议。
- **audit 查法**:保留五类 finding 和 severity 口径;细颗粒检查留给 `$sop-audit`。

### 4.2 `skills/sop-audit/SKILL.md`:检查表化

`$sop-audit` 不删主线覆盖闸,但把长段落拆成顺序动作:

1. 读 `STANDARD.md`。
2. 测目标项目形态:端、人、风险。
3. 比结构和模板漂移。
4. 填 6 行主线覆盖闸。
5. 异常项转 findings。
6. 输出人读报告 + 可执行 findings。
7. 收尾卡。

主线覆盖闸保留 6 行,但内部说明按短块组织:

- 状态词口径。
- surface 适用性。
- 必查 surface。
- 报告装配校验。
- 判定顺序。
- 状态转 findings。

目标是让 audit agent 顺着步骤做,不是在一大段里找条件。

### 4.3 `master/base/AGENTS.md`:入口降噪

根 agent 指令文件只保留 agent 必须开工就看到的东西:

- 起手 freshness。
- 人机分工。
- 工作约束短清单。
- 沟通闭环。
- 高风险闸。

对已经有专门文档承接的内容,只留短引用:

- issue/PR 细则 -> `docs/project/issue-pr-workflow.md`。
- 协作 handoff -> `docs/project/collaboration.md`。
- 多端契约 -> `docs/contracts/`。

不把根 `AGENTS.md` 改成小号 `STANDARD.md`。

### 4.4 `master/base/docs/project/issue-pr-workflow.md`:流程主线整理

这里继续承载 issue/PR 执行规则,不追求大幅变短。目标是顺序更接近真实动作:

1. 三件套分工。
2. 凭据保真。
3. Issue 生命周期。
4. PR 生命周期。
5. 评论分层。
6. 起手/收工等细则。

保持 `Refs #N` / `Closes #N`、doc 先落远端、接 issue 先验链接、厚/短评论边界等承重规则不丢。

### 4.5 `README.md`:原则上少动

README 当前已经比较接近正确入口:讲主线,不承载规则。最多同步一句本轮瘦身后的读法,不搬细则。

## 5. 验收标准

### 5.1 无语义新增

diff 不应出现:

- 新规则概念。
- 新流程名。
- 新主线覆盖项。
- 新 skill。
- 新权威入口文档。

允许:

- 改写。
- 拆段。
- 合并重复表达。
- 搬迁到更合适层。
- 用短指针替代长复述。

### 5.2 承重墙不丢

改完必须仍能确认这些护栏存在。不能只 grep 关键词;每项至少确认它在正确层仍有落点:

- `STANDARD.md`:权威原则或边界。
- `master/`:执行入口或渲染落点。
- `skills/sop-audit/SKILL.md`:必要时有审计检查点。

承重墙矩阵覆盖:

- 反驳。
- 诚实无知。
- 查证 -> 分流 -> 调研 -> 执行验证 -> 收口。
- doc / issue / PR 三件套。
- `Refs #N` / `Closes #N` 收口。
- issue 评论分层。
- 新眼睛 review。
- 高风险闸。
- 端级 `AGENTS.md`。
- worktree 只在真并行多 agent 时触发。
- 不跑 writing-plans。
- 凭据保真。

### 5.3 分层更清楚

每条主规则只有一个完整定义位置:

- 原则和边界在 `STANDARD.md`。
- 执行动作在 `master/`。
- 审计检查在 `skills/sop-audit/SKILL.md`。
- README 只做入口投影。
- PLAYBOOK 只收实验背书,不做运行时规则入口。

### 5.4 审计更像 flow

`$sop-audit` 读起来必须是顺序动作:

```text
读 STANDARD -> 测项目形态 -> 比结构/漂移 -> 填主线表 -> 转 findings -> 收尾卡
```

不能继续表现成多套横向协议名。

### 5.5 验证

实施后至少做:

- `git diff --check`。
- 静态 grep:确认承重墙关键词仍在。
- 承重墙矩阵:确认关键护栏有 `STANDARD` 权威点、`master/` 执行点,必要时有 `$sop-audit` 检查点。
- 行宽/长段检查:定位是否仍有明显过长段落,用于人工判断。
- `media-ops` 只读压力样本:确认 `$sop-audit` 仍能覆盖前几轮校准过的 Refs/Closes、三件套、issue 评论分层、高风险治理项、结构触发等主线,不能因瘦身漏掉。
- 风险分流检查:普通 req/design doc 仍按低风险;`AGENTS.md`、SOP、workflow、collaboration、PR 模板、`docs/contracts/` 仍按治理高风险口径检查。
- 独立新眼睛 review:检查无语义新增、承重墙未丢、执行性是否变好。

## 6. 风险与边界

- **瘦丢护栏**:压缩长段时最容易漏安全例外。用承重墙 grep + 新眼睛复审兜底。
- **只变短不变清**:如果只是删字,但层次仍混,不算成功。验收看分层边界是否更清楚。
- **过度重构**:拆太多新文档会制造新漂移源。本轮不新增权威入口。
- **审计弱化**:不能为了让 `$sop-audit` 变短而删掉主线覆盖闸。本轮只重排表达。
- **历史证据丢失**:事故细节可以从 `STANDARD.md` 压出,但必须仍能通过 `PLAYBOOK.md` / experiment 回溯。

## 7. 实施范围建议

优先改:

1. `STANDARD.md`
2. `skills/sop-audit/SKILL.md`
3. `master/base/AGENTS.md`
4. `master/base/docs/project/issue-pr-workflow.md`

视实际 diff 再决定是否轻触:

- `README.md`
- `PLAYBOOK.md`

必做:

- `experiments/`:本轮改动必须记录自举实验。若形成可复用教训,再沉到 `PLAYBOOK.md`。

不动:

- skill 名称。
- 主线覆盖闸 6 项数量。
- `master/` 分层目录结构。
- Codex-only 方向。
