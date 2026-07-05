# 方案前证据优先设计

- **日期**:2026-07-05
- **状态**:已落地到 STANDARD / 母本 / sop-audit / exp-021
- **目标**:把“方案 / 设计 / 估算 / 选型前先摸真实情况”的工作习惯写入 sop-better,但避免变成“所有小事都强制联网”的新仪式。

---

## 1. 问题

用户长期偏好是:agent 给方案前先调研真实情况,不要凭训练记忆或旧经验拍脑袋。这个偏好已写进用户级全局指令,但项目级 SOP 只有后置检查:

- `STANDARD §1.3` 要求 spec 新眼睛审“实证 + 信源可信 vs 拍脑袋”。
- `exp-015` 记录过无脚手架项目自然长出“实证调研先行”。
- `exp-016` 把“多调研”作为 spec 审查项搭车下发,但那是 spec 收口后的检查,不是方案产出前的行为闸。

缺口:agent 仍可能先给方案,之后才在 review 中被指出“没查真环境 / 没查官方来源”。这会让用户在关键方案阶段承受拍脑袋风险。

---

## 2. 外部依据

调研结论支持“证据优先,但按任务分级”:

- [OpenAI Web Search 文档](https://developers.openai.com/api/docs/guides/tools-web-search):联网搜索适合获取最新信息并提供来源引用;复杂研究更慢,简单查询更快,说明不能把所有任务都推成深度调研。
- [Anthropic Building Effective Agents](https://www.anthropic.com/engineering/building-effective-agents):agent 的基础能力包括 retrieval / tools / memory,但复杂度、延迟、成本要权衡,应从够用的简单方案开始。
- [Anthropic Context Engineering](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents):系统提示要介于“太虚”和“硬编码 if-else”之间;本规则应写触发条件和 carve-out,不能写成死规矩。
- [Google Grounding 文档](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/grounding/overview):把输出连接到可验证数据源可以减少编造,并提供可审计来源。
- [RAG 论文](https://arxiv.org/abs/2005.11401):模型参数内知识有局限,更新世界知识和提供来源本身就是难点;检索增强能让输出更事实化。

结论:规则应叫“方案前证据优先”,不是“网络调研优先”。网络只是证据来源之一;项目代码、配置、日志、真实运行结果常常更优先。

---

## 3. 设计原则

1. **先本地真实情况,再外部网络**:项目内事实先看代码 / 配置 / 日志 / issue / PR / 运行结果;外部事实再上网。
2. **高变动 / 高代价才强触发**:涉及最新规则、第三方 API、依赖版本、价格、安全、部署、合规、外部最佳实践时,先调研。
3. **信源分档**:官方文档 / 源码 / 标准规范 / 真实环境实测 > release note / changelog > GitHub issue / StackOverflow / 博客。后者只当线索,关键结论要回一手源或实测复核。
4. **输出要标依据**:方案里至少说明“依据 / 未验证 / 风险”,让用户知道哪些查过、哪些没验。
5. **反过度治理 carve-out**:低风险局部补丁、纯解释、稳定常识、无真实环境可摸的小事,不为调研而调研。

---

## 4. SOP 落点

1. **`STANDARD.md`**
   - 在 §1.10 主动建议后追加“方案前证据优先”。
   - 明确它是建议来源的要求:照亮用户前,先让建议落在真实依据上。

2. **`master/base/AGENTS.md`**
   - 在 Agent 工作约束中加一句摘要。
   - 在沟通约束中加可检查小节,让生成项目立即继承。

3. **`skills/sop-audit/SKILL.md`**
   - 把“缺方案前证据优先”加入 audit 检查。
   - 审计时既查缺失,也查过度:如果项目把所有小事都强制网络调研,也算不合理。

4. **`experiments/2026-07-05-021-evidence-first-plan.md`**
   - 记录这次从 owner 全局偏好 + 外部依据 + exp-015/016 演进出来的规则。

5. **`PLAYBOOK.md`**
   - 只沉淀 exp-021 背书后的教训。

---

## 5. 验收标准

- `STANDARD.md` 有“方案前证据优先”规则,并包含触发条件、信源分档、carve-out。
- `master/base/AGENTS.md` 的生成物会要求 agent:
  - 给方案 / 设计 / 估算 / 选型前先摸真实情况。
  - 项目内事实优先本地实测,外部事实优先官方来源。
  - 输出方案时标“依据 / 未验证 / 风险”。
  - 小改 / 纯解释 / 稳定常识不强制调研。
- `$sop-audit` 能报“缺方案前证据优先”,也能避免把“没有所有任务强制联网”误报为缺失。
- 新增 exp-021,PLAYBOOK 条目引用 exp-021。
- 不新增独立 skill,不改外部 superpowers 插件缓存。

可检查场景:

1. “这个依赖怎么选?” → 查当前项目栈 + 官方文档 / release note 后再给方案。
2. “某平台 API 能不能这样接?” → 先查官方文档;博客 / issue 只能当线索。
3. “帮我改 README 一个错别字” → 不强制网络调研。
4. “优化架构”但没看代码 → 先读项目结构 / 关键配置,不能直接给泛泛方案。

---

## 6. 边界

- 这条不替代 spec 新眼睛 review;它是前置行为闸,spec review 是后置安全网。
- 这条不要求每次都写研究报告;小方案可只在回复里给一句依据。
- 如果环境无法访问网络或真实系统,agent 要明说“未联网 / 未实测”,再给保守建议。
- 如果用户明确要求“不要联网 / 只凭当前上下文”,按用户当场指令执行,但标记未验证风险。
