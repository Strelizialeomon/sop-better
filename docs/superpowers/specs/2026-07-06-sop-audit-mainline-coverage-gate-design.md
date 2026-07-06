# sop-audit 主线覆盖闸 · Design

## 1. 背景

`$sop-audit` 已经写了模板版本差、doc/issue/PR 三件套、评论凭据塌缩、Refs/Closes 等审计规则。但在 `media-ops` 真跑时,报告只抓到了两个显眼问题:

- PR 模板默认 `Closes #`。
- PR 模板高风险勾选缺治理 doc / 跨端契约。

它漏了更关键的近期主线:

- `AGENTS.md` / workflow 的短流程仍诱导 `Closes #N`。
- `issue-pr-workflow.md` 缺当前母本的 doc/issue/PR 三件套。
- PR #271 的 issue 评论分层只落了 spec,没落执行 SOP。
- `Refs #N` 不应自动关 issue 的收口条件没有完整落地。

问题不是规则完全没写,而是规则藏在大段说明里,执行 agent 容易只抓显眼 finding 就收工。需要一个防漏机制,但不能把 SOP 写厚。

## 2. 外部调研结论

本轮调研目标是找"如何用清单防漏,但不让流程变重"的成熟做法。

- AHRQ PSNet 对 checklist 的总结指出:清单不是万能药;设计差或过长会拖累表现;好清单要理解任务和使用者。参考: https://psnet.ahrq.gov/perspective/what-makes-good-checklist
- AHRQ checklist primer 也强调:清单有用,但成功依赖合适目标和谨慎实施,不能到处硬套。参考: https://psnet.ahrq.gov/primer/checklists
- GitLab 的 description templates 把固定信息预填到 issue / merge request 中,价值是标准化和自动化入口,减少靠记忆补项。参考: https://docs.gitlab.com/user/project/description_templates/
- Google SRE postmortem culture 强调 action items 要具体、可追踪、可验证;写记录本身不够,要让后续行为改变。参考: https://sre.google/workbook/postmortem-culture/

抽象成适合本项目的原则:

1. 清单只放"容易漏且漏了代价大"的 killer items。
2. 清单必须在关键停顿点出现,不能只藏在正文里。
3. 清单项要短证据,不要变成长报告。
4. 清单不替代判断;合理本地偏离要允许。

## 3. 目标

给 `$sop-audit` 增加一个**很薄的主线覆盖闸**:

- 每次 audit 报告都必须出现一张 6 行覆盖表。
- 每行只写 `状态 + 一组短证据`。
- 只有状态异常时才展开成 finding。
- 让用户一眼看出 agent 是否查过最新主线。

## 4. 非目标

- 不把 `$sop-audit` 变成模板版本迁移工具。
- 不要求项目逐字贴合母本。
- 不把 README / STANDARD 再扩一大段。
- 不把所有项目都强制联网 / 全量爬 issue。
- 不把合理本地偏离报成问题。

## 5. 设计

### 5.1 新增报告停顿点:主线覆盖闸

`$sop-audit` 在输出 findings 前,必须先填这个表:

```md
## 主线覆盖闸

| 主线项 | 状态 | 证据 |
|---|---|---|
| Refs / Closes 收口 | 覆盖 / 缺失 / 偏离 | 母本 file:line -> 项目 file:line / 缺落点 |
| doc / issue / PR 三件套 | 覆盖 / 缺失 / 偏离 | 母本 file:line -> 项目 file:line / 缺落点 |
| issue 评论分层 | 覆盖 / 缺失 / 偏离 | 母本 file:line -> 项目 file:line / 缺落点 |
| 查证闭环 | 覆盖 / 缺失 / 过重 | STANDARD/SKILL file:line -> 项目 file:line / 缺落点 |
| 高风险治理项 | 覆盖 / 缺失 / 偏离 | 母本 file:line -> 项目 file:line / 缺落点 |
| 结构触发 | 匹配 / 缺失 / 预建 / 偏离 | STANDARD file:line -> 项目结构证据 |
```

状态解释:

- **覆盖 / 匹配**:项目已有对应护栏,不进 findings。
- **缺失 / 预建 / 过重**:进入 findings。
- **偏离**:必须用一句话判断是合理本地偏离还是不合理偏离;不合理才进 findings。
- **无权威锚点不得填覆盖**:每项都要先指向当前 `STANDARD.md` / `master/` 母本 / `skills/sop-audit/SKILL.md` 的对应规则,再指向目标项目落点。只在目标项目里搜到近似关键词,不能算覆盖。

### 5.2 六项为什么是这六项

这六项只覆盖近期最容易漏、且漏了会导致 agent 行为错误的主线:

1. **Refs / Closes 收口**:防止 PR 模板或短流程误关 message-bus issue。
2. **doc / issue / PR 三件套**:防止 issue 替代正文、PR 替代消息总线。
3. **issue 评论分层**:防止 spike / 收口 / 状态校正只写短评,后续 agent 无法判断。
4. **查证闭环**:防止"不瞎猜"只停留在口号,或反向变成所有小事都强制调研。
5. **高风险治理项**:防止治理 doc / AGENTS / 跨端契约被普通低风险 PR 自动合。
6. **结构触发**:防止多端 / 多人 / 并行 agent 结构错配,也防止预建过度治理。

它不是完整规则索引,只是审计时的防漏刹车。

### 5.3 如何避免变厚

硬限制:

- 表固定 6 行,不允许继续加成 12 行。
- 每项证据只放一组最短证据对:`权威锚点 -> 项目落点/缺口`。
- 正常覆盖不解释;只有异常才进 findings。
- 不新增协议名,统一叫"主线覆盖闸"。
- 不改 `STANDARD.md`,先只改 `$sop-audit` 输出流程。若后续多次证明此闸承重,再考虑沉入 STANDARD。

### 5.4 覆盖判定口径

覆盖闸只防漏,不重抄 `STANDARD.md`。但每行填 `覆盖` 前必须确认下面的最小判定点,避免找一行关键词就放行:

| 主线项 | 覆盖必须同时确认 |
|---|---|
| Refs / Closes 收口 | 相关 PR 默认 `Refs`,只有真正收口且关闭条件满足才用 `Closes`。 |
| doc / issue / PR 三件套 | doc 是正文真相源,issue 是消息总线和状态,PR 是变更与验证,三者不互相替代。 |
| issue 评论分层 | 决策 / 实测 / 收口 / 状态校正 / 阻塞或高风险 / 残项迁移 / 路线变化不能只写短评,要有证据和下一步。 |
| 查证闭环 | 同时覆盖本地事实查证、外部事实联网或权威来源、未验证风险标明,且不把低风险小事拖成过度确认。 |
| 高风险治理项 | 治理 doc / AGENTS / 协作约定 / 跨端契约或骨架变更不能按普通低风险自动合,要回 owner 人审。 |
| 结构触发 | 端 / 人 / 并行 agent 三个触发分开判:第 2 个端触发 contracts/端级身份文档;真并行才触发 worktree/协调骨架;第 2 个人触发 handoff。 |

这张判定表不进入最终 audit 报告;它只是填覆盖闸时的内部口径。

### 5.5 与现有 findings 的关系

主线覆盖闸不是第六类 finding。

流程是:

1. 先填覆盖表。
2. 表中异常项转成现有 severity:
   - Refs/Closes、三件套、评论分层、高风险治理项 → 通常是 P3 凭据失真 / 交接断裂。
   - 查证闭环 → P2 沟通闭环缺环 / 口号化 / 过度阻塞。
   - 结构触发 → P2 结构错配或 P3 结构缺失。
3. findings 仍按原格式输出。

### 5.6 media-ops 压力测试

如果新闸用于本轮 `media-ops`,覆盖表应逼出这些状态:

| 主线项 | 预期状态 | 短证据 |
|---|---|---|
| Refs / Closes 收口 | 缺失 | `master/base/docs/project/issue-pr-workflow.md:37 -> .github/PULL_REQUEST_TEMPLATE.md:3` |
| doc / issue / PR 三件套 | 缺失 | `master/base/docs/project/issue-pr-workflow.md:11 -> docs/project/collaboration.md:13` |
| issue 评论分层 | 缺失 | `master/base/docs/project/issue-pr-workflow.md:51 -> docs/project/issue-pr-workflow.md:64` |
| 查证闭环 | 覆盖 | `STANDARD.md:67 -> AGENTS.md:140` |
| 高风险治理项 | 缺失 | `master/base/docs/project/issue-pr-workflow.md:40 -> .github/PULL_REQUEST_TEMPLATE.md:24` |
| 结构触发 | 匹配 / 缺失 | `STANDARD.md:103/104/108 -> 现场端数/人数 + docs/contracts/端级文档/handoff 落点` |

覆盖表只放短证据;进入 finding 后再展开其它证据,例如 `AGENTS.md:164` 和 `issue-pr-workflow.md:60`。

注意:串行 / 单 agent 只豁免 worktree 和多 agent 协调骨架,不能豁免"第 2 个端"触发的 contracts/端级身份文档。

## 6. 验收标准

- `$sop-audit` 的报告流程明确要求输出"主线覆盖闸"表。
- 表固定 6 行,每行要求 `权威锚点 -> 项目落点/缺口` 短证据。
- skill 明确异常项才展开成 findings。
- skill 明确合理本地偏离不报 finding。
- skill 不把主线覆盖闸写成新的长协议。
- 实现后必须在 `/Users/sunchongsheng/code/media-ops` 真实只读复跑一次,在实验记录里写明 media-ops 的 commit SHA、覆盖闸 6 行结果、以及至少这些断言:
  - 不会漏掉 Refs/Closes、三件套、评论分层三项。
  - 结构触发必须先记录实际端数/人数,不能用"串行单 agent"豁免第 2 个端规则。
  - 如果目标项目文件已变化,用新的 `file:line` 解释状态变化,不能写"思维复跑"。

## 7. 风险与约束

- **变厚风险**:覆盖表被继续加项。约束:固定 6 行,新增项必须先替换旧项,不能追加。
- **形式主义风险**:agent 随便填 `覆盖`。约束:每项必须给 `权威锚点 -> 项目落点/缺口`,无权威锚点不能填覆盖。
- **误报风险**:本地合理偏离被当缺失。约束:状态允许 `偏离`,并要求一句话判断是否合理。
- **重复风险**:findings 和覆盖表重复。约束:覆盖表只给短证据,解释进 finding。

## 8. 实施范围建议

下一步实现时只改:

1. `skills/sop-audit/SKILL.md`:流程第 4/5 步加入主线覆盖闸。
2. `experiments/`:新增实验记录。
3. `PLAYBOOK.md`:新增一条"审计用薄覆盖闸防漏,固定 6 行"教训。

暂不改 `STANDARD.md`,避免把权威尺继续变厚。
