# sop-better 稳定化改造设计

- **日期**:2026-07-10
- **状态**:设计已由 owner 分四段批准,尚未实施;其中实施顺序与运行时部分已被 [`2026-07-12-sop-runtime-loop-design.md`](2026-07-12-sop-runtime-loop-design.md) 取代
- **Spec review**:独立新眼睛已审;3 个 P1 契约缺口修订后复核通过
- **优先级**:稳定性第一
- **目标环境**:多台自有机器;macOS + Codex、Windows + Codex
- **升级方式**:先看差异,明确确认后升级
- **验证方式**:机械检查 + 真实 Codex 行为实验双保险

> **后续关系**:本设计关于确定性生成、跨平台安装、版本隔离、升级预览与回滚的结论继续有效。2026-07-12 新设计新增任务运行闭环,并取代本文的“两阶段顺序”、迁移顺序和缺少运行控制面的部分。两份文档冲突时,运行时与实施顺序以新设计为准。

---

## 1. 结论

采用已批准的 **方案 A:两阶段稳定化**。

1. **第一阶段先修确定性工程问题**:把生成、检查、跨平台安装、版本隔离做成可重复的程序行为。先消灭当前能直接证明的模板冲突、触发泄漏、硬编码分支、坏引用和“工作树即线上”。
2. **第二阶段再校准核心理念**:用 12～20 个真实 Codex 行为场景对比新旧规则。只有实际表现更稳,才改 `STANDARD.md` 中涉及自主权、凭据、计划、review 等非确定性原则。

本轮不靠继续堆提示词解决稳定性。核心变化是:

> 让程序负责确定事实,让 Codex 负责需要判断的部分,让版本系统负责升级和回滚。

owner 本轮戴“使用者帽子”:负责目标、约束、验收和否决。架构方案由 agent 提出并承担解释责任。owner 想介入技术决策时可随时切回开发角色。

---

## 2. 已确认需求

### 2.1 必须做到

- 首要优化目标是“更稳定”,不是“规则更多”或“功能更多”。
- 每次规则或模板变化都跑快速、无模型、无网络的机械检查。
- 核心原则变化还要跑真实 Codex 行为实验。
- 支持 macOS 与 Windows,不依赖项目作者机器上的绝对路径。
- 多台机器可停留在不同稳定版本,不自动追随仓库 `main`。
- 升级前展示差异;没有明确确认,不改安装版本和项目文件。
- 升级失败保留旧版本,并能退回上一个版本。
- 安装完成后,当前版本的生成和机械审计可离线运行。
- `STANDARD.md` 继续是规则语义的唯一真相源。

### 2.2 本设计不直接授权

- 不直接修改实现文件、skills、模板或 `STANDARD.md`。
- 不直接推送远端、发布 GitHub Release、设置分支保护或启用自动合并。
- 不在第一阶段提前修改“所有任务 issue + PR”“跳 writing-plans”等核心理念。
- 不一次性重写所有活跃 lab 项目。

---

## 3. 当前基线与已证实问题

### 3.1 仓库现状

本轮审计时的事实:

- 仓库几乎全是 Markdown,没有生成器源码、测试目录、CI workflow 或稳定的机械校验入口。
- `skills/sop-init`、`skills/sop-audit` 通过本机软链接直接进入 `~/.codex/skills/`。因此工作树里的半成品会立刻成为正在使用的版本。
- 当前本机 Codex CLI 为 `0.144.1`。
- 当前 CLI 已实测支持 `codex plugin add/remove`、`codex plugin marketplace add/upgrade` 和 `codex exec --json`。
- Git 默认分支是 `main`,但母本里仍多处假设 `master`。
- 仓库已有 36 次实验积累,但“文本改了”与“Codex 行为真的变稳了”还没有统一分级。

### 3.2 第一阶段必须修掉的确定性问题

| 问题 | 当前证据 | 影响 |
|---|---|---|
| 规则自相矛盾 | `master/base/docs/decisions/adr-template.md` 仍示例“直推 master、不开分支”,与 `STANDARD.md` 当前 issue + PR 规则冲突 | agent 可能照最近模板执行错误流程 |
| 默认分支硬编码 | `master/base/AGENTS.md`、workflow、coordination、worktree 等多处写死 `master` / `origin/master`;`master/SLOTS.md` 只给端角色登记 `{{base_branch}}` | 在 `main` / `dev` 项目生成错误命令 |
| 串行形态泄漏并行规则 | `master/layer-multiend/end-role-agent.md` 无条件出现 scope label、coordination、专用 worktree | 单人多端串行项目背上不存在的流程 |
| 生成后的引用不存在 | 端角色指向 `coordination.md`,但 init 实际把它追加到 `docs/project/collaboration.md`;生成物还会引用目标项目里不存在的 `STANDARD.md` | agent 按链接找不到规则,形成“盲引用” |
| init 承诺与资产不一致 | `skills/sop-init/SKILL.md` 承诺 README、`.gitignore`、issue 模板、label 状态机、ADR 样例等,但 `master/base/` 没有完整可复制资产 | 每次生成依赖模型临场发挥,无法复现 |
| 安装路径绑死作者机器 | 两个 skill 都把 `SOP_HOME` 写成 `/Users/sunchongsheng/code/sop-better/` | 换一台 Mac 或 Windows 就找不到规则和模板 |
| 槽注册不完整 | `{{NNNN}}`、标题、日期等槽未完整登记;模板仍含“无则删本行”式人工清洗提示 | 容易漏占位符或把母本说明带进产物 |
| 只读任务被迫改 Git 状态 | 根母本要求分析前 fetch 后 `pull --rebase` | 只读审计也可能触发写操作;rebase 风险与全局规则冲突 |
| 术语和历史规则漂移 | “端身份文档/端操作台”、PLAYBOOK 中旧触发门槛等并存 | 同一概念被不同文本解释 |
| 开发版等于线上版 | skill 软链接直连工作树 | 半完成编辑会污染正常使用,无法稳定回滚 |
| 没有回归检查 | 模板、规则、skill 变化没有自动测试 | 同类错误只能靠下一次人工审计发现 |

这些问题不需要模型评判,也不需要先争论理念。它们都是可以机械复现的工程缺陷。

### 3.3 外部调研结论

外部资料支持“薄常驻指令 + 按需技能 + 程序校验 + 可安装版本”这个方向:

- OpenAI 官方把 `AGENTS.md` 定位为每次都加载的持久规则,明确建议保持小;可复用流程放 skill,需要分发时打成 plugin,并建议用 hooks、lint、类型检查等基础设施执行硬规则。[OpenAI Customization](https://learn.chatgpt.com/docs/customization/overview)
- Codex 会从全局到当前目录合并 `AGENTS.md`,越近的文件优先;合并上限默认 32 KiB。这说明嵌套文件适合真正的局部差异,不适合复制通用规则。[OpenAI AGENTS.md](https://learn.chatgpt.com/docs/agent-configuration/agents-md)
- Codex 官方插件机制支持版本 manifest、Git ref / SHA 来源和安装缓存。安装副本来自 cache,无需继续用“仓库软链接即线上”。[OpenAI Build plugins](https://learn.chatgpt.com/docs/build-plugins)
- Anthropic 把上下文视为有限注意力预算,建议保留最小高信号内容,其余按需加载。[Effective context engineering](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents)
- Agent 行为评测不宜只看最终一句话;应结合运行轨迹、最终环境状态、确定性检查和人工校准。[Demystifying evals for AI agents](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents)

研究证据并不完全同向:

- 一项 2026 年预印本发现,泛化或自动生成的仓库上下文常增加超过 20% 的推理成本,且未稳定提高成功率,结论偏向“只保留最小必要规则”。[Evaluating AGENTS.md](https://arxiv.org/abs/2602.11988)
- 另一项基于 10 个仓库、124 个 PR 的预印本则观察到带 `AGENTS.md` 时中位运行时间和输出 token 更低,任务完成表现相近。[Impact on efficiency](https://arxiv.org/abs/2601.20404)
- 配置气味研究列出的 context bloat、skill leakage、blind reference、conflicting instructions 等,与本仓实测问题高度相似。[Configuration smells](https://arxiv.org/abs/2606.15828)

因此本设计不把任何一篇研究当成普遍真理。它们只支持一个结论:必须在自己的 Codex、自己的项目和自己的任务上做成对实验。

---

## 4. 方案选择

### 4.1 采用方案 A:两阶段稳定化

先建立确定性生成和检查,再做真实行为实验。优点是先止住当前明确故障,又不会凭感觉重写核心理念。

### 4.2 未采用方案 B:只修模板并加 CI

它改动最小,但继续保留“模型临场拼装生成物”和“工作树即线上”。当前最重要的版本隔离、跨平台复现和升级回滚仍然没有解决。

### 4.3 未采用方案 C:直接平台化

服务器、数据库、后台 UI、遥测平台会显著扩大维护面。当前使用者只有 owner 的多台机器,没有证据证明需要平台。

### 4.4 对方案 A 的最强反对意见

评测系统和发布系统本身可能长成新的过度治理。

范围护栏:

- 只做一个小 CLI。
- 第一批只保留 4 个可读 fixture、合法组合生成测试和 12～20 个行为场景。
- 不建服务器、数据库、Web UI、账号体系或遥测平台。
- 没有真实失败样本支持,不新增检查项和场景。
- 先让 owner 的 Mac / Windows 跑稳,不提前为公众发行建设大型基础设施。

---

## 5. 总体架构

```text
项目事实 + owner 判断
          |
          v
  .sop/profile.json  ----->  sopctl  <-----  manifest.json + master/
                                  |
                    render / diff / check
                                  |
                                  v
                       项目 SOP + .sop/lock.json

STANDARD.md --定义规则语义--> manifest / master --由程序执行-->

sop-init / sop-audit 只负责对话判断和解释
plugin + 固定版本负责安装、升级、回滚
行为 eval 负责判断核心理念是否真的更稳
```

拆成五层,每层只承担一种责任。

### 5.1 规则层:`STANDARD.md`

只定义:

- 原则是什么。
- 何时触发。
- 哪些边界不能越过。
- 需要留下什么证据。

不再承载:

- macOS / Windows 路径。
- 具体复制、替换和删除算法。
- 写死的 `master` 命令。
- “无则删本行”之类生成器操作说明。

第一阶段只修确定性冲突,不改待实验的核心理念。

### 5.2 生成契约层:`manifest.json + master/`

`manifest.json` 是机器可读的生成契约,负责:

- 每个文件属于哪个触发层。
- 每个槽的类型、来源、必填性和清洗规则。
- 文件是全托管、块托管还是只读参考。
- 产物路径和合法引用。
- 模板、profile schema 与 SOP 版本兼容关系。

manifest 不是第二份政策真相源。每个触发和组件必须引用 `STANDARD.md` 的稳定规则 ID;规则变化先改 STANDARD,再同步 manifest / master,由一致性检查拒绝孤立规则、孤立组件和未同步版本。

`master/` 只保留最终可渲染内容。条件内容拆成组件,由 manifest 选用。模板里不再保留要求模型手工删除的注释。

例如端角色模板拆成:

- 多端串行共有部分。
- 仅并行时追加的 scope / worktree / coordination 部分。

这样“串行项目没有 worktree”由程序保证,不靠模型记得删一行。

### 5.3 执行层:`sopctl` bootstrap + manager + engine

用户只看到一个 `sopctl` 命令,内部明确拆成三件东西:

- **固定 bootstrap shim**:安装在版本目录之外,只读取当前指针并启动对应 manager。它不包含 SOP 规则和升级逻辑,日常 release 不覆盖它。
- **版本 manager**:是版本切换的唯一 owner。它负责 release check / diff / upgrade / rollback、版本目录、当前指针和 Codex plugin 激活;升级时只新增目标版本,不覆盖正在运行的 manager。
- **版本 engine**:随每个 release 安装。它负责项目 diff / render / check / project rollback,读取同版本 STANDARD 快照、manifest、master 和 schema。manager 根据明确版本调用它。

plugin 和项目 engine 都不得自行切换版本,避免多方争夺 current 指针。只有 manager 可切换。若未来必须改变 bootstrap 协议,视为需明确确认的安装器级 MAJOR 升级,不做静默自更新。

两者使用同一份小型 Go 代码库构建。选择 Go 的原因:

- 同一份代码可生成 macOS arm64 / amd64、Windows amd64 / arm64 可执行文件。
- 运行时不要求用户预装 Python、Node 或 Go。
- 文件、Git、JSON、校验和事务式替换都是标准库擅长的工作。

公开命令契约:

```text
sopctl diff       # 只展示如果生成/升级会变什么
sopctl render     # 按 profile 生成或升级托管内容
sopctl check      # 机械检查 profile、模板和生成物
sopctl project rollback --to <checkpoint>

sopctl release check
sopctl release diff --to <version>
sopctl release upgrade --to <version>
sopctl release rollback
```

命令名是用户可依赖的接口。内部目录和函数不是公开契约。

### 5.4 对话层:`sop-init` 与 `sop-audit`

skills 保持薄:

- `$sop-init` 负责观察项目、区分事实与判断、只在真正含糊时询问 owner,然后写 profile 并调用 `sopctl`。
- `$sop-audit` 先调用 `sopctl check` 得到机械报告,再用 Codex 判断语义问题,最后把两者合并成人读报告。
- 两个 skill 从自身 plugin 读取同版本规则快照,通过安装器登记的 bootstrap 进入当前 manager,再调用对应 engine;不得包含作者机器绝对路径。
- skill 只使用当前 plugin,不负责安装、升级或回滚 plugin。版本动作全部交给 manager。
- 机械错误不能被 LLM 解释成“没关系”。
- 找不到兼容 `sopctl` 时直接说明安装问题,不得退回“让模型手工照模板生成”。手工 fallback 会重新引入不可复现性。

### 5.5 分发层:Codex plugin + 固定版本

plugin 包含:

- `.codex-plugin/plugin.json` 与语义版本。
- `sop-init`、`sop-audit`。
- 该 release 对应的只读 `STANDARD.md` 快照、checksum 和规则版本。
- `manifest.json`、`master/` 和 schema;manifest 只引用同包 STANDARD 快照的规则 ID。
- 当前版本说明。

release bundle 另外包含各平台 bootstrap shim、版本 manager、项目 engine 和校验值。plugin 是 Codex 的薄入口;release bundle 是完整安装单元。

仓库根 `STANDARD.md` 仍是唯一可编辑真相源。plugin 内副本只是由 release 构建产生的不可编辑快照;构建时 checksum / 规则版本不一致就拒绝发布。

稳定安装使用固定 tag 和最终 commit SHA。开发 checkout 只供开发和隔离测试,不再软链接到正常使用环境。

---

## 6. 项目 profile 与生成流程

### 6.1 两个项目内状态文件

`.sop/profile.json` 是人可读、可提交的项目事实与决定。它不得包含密钥或机器绝对路径。

示意:

```json
{
  "schema_version": 1,
  "sop_version": "1.0.0",
  "project": {
    "name": "example",
    "default_branch": "main"
  },
  "ends": [
    {"name": "backend", "path": "backend"}
  ],
  "humans": [
    {"id": "owner", "roles": ["product", "developer"]}
  ],
  "parallel_agents": false,
  "risk": "reversible",
  "house_style": []
}
```

`.sop/lock.json` 由工具维护并提交,记录:

- 实际渲染版本和 schema 版本。
- 使用的模板组件 ID。
- 每个托管块的规范化 hash。
- 生成器版本。

`humans[].id` 使用项目内稳定称呼即可,不要求真实姓名。profile 说明“这个项目是什么”;lock 说明“上次准确生成了什么”。两者不能混成机器本地状态。

### 6.2 事实发现

`$sop-init` 与 `sopctl` 的分工:

- Git 根目录、已有文件、远端、当前分支由程序读取。
- 默认分支优先读取 `refs/remotes/origin/HEAD`;无法可靠判断时列出候选并让 owner 确认。绝不默认 `master`。
- 端目录可由 agent 提候选,owner 只确认会影响结构的歧义。
- “有几位真人协作者”“是否真并行多 agent”“风险级别”不能从文件名可靠推断,由 owner 确认后写入 profile。
- 路径全部存仓库相对路径。macOS 与 Windows 不共享绝对路径。

### 6.3 第一次生成

```text
sop-init 检查项目
-> 形成/确认 profile
-> sopctl diff 展示计划
-> owner 已授权生成时 sopctl render
-> sopctl check
-> skill 用人话解释生成了什么、为什么触发
```

如果用户只要求预览,流程停在 `diff`,不写文件。

### 6.4 已有项目升级

文件分三类:

1. **全托管文件**:完全由 sop-better 生成。内容没被本地改过时可替换;改过则拒绝覆盖并展示 diff。
2. **块托管文件**:例如已有项目的 `AGENTS.md`。只更新隐藏标记之间的标准块,标记外是项目本地规则。
3. **用户文件**:工具永不修改。

托管块使用稳定标记和 hash,例如:

```md
<!-- sop-better:begin id=core-guidance version=1.0.0 hash=... -->
...
<!-- sop-better:end id=core-guidance -->
```

升级规则:

- 托管块未改过:展示新旧 diff,确认后更新。
- 托管块被人工改过:拒绝覆盖,展示本地内容与候选内容的差异,并标明 lock 中记录的上次 hash。只有旧版本内容仍可用时才补充完整三方差异。
- 同名文件存在但没有托管标记:视为用户文件冲突,拒绝接管。
- 非托管内容始终保留。

### 6.5 事务式写入

多文件修改不能假装成操作系统级原子操作。实现采用可恢复事务:

1. 先在临时目录生成全部目标内容。
2. 对临时结果跑完整机械检查。
3. 备份所有将受影响的旧文件、`.sop/lock.json` 并写入事务日志。
4. 用同目录临时文件逐个 rename 替换。
5. 任一步失败,按日志恢复旧文件。
6. 下次启动发现未完成事务,先恢复或明确提示,不继续叠加修改。

只有所有文件替换成功后才更新 `.sop/lock.json`。事务备份解决“写到一半”;成功 render 前的旧托管内容和旧 lock 还要形成持久 project checkpoint,解决“成功升级后想降级”。checkpoint 存在本机 `SOP_STATE_HOME`、不提交,至少保留最近两个成功版本。

`sopctl project rollback` 只恢复项目托管块和 lock,不切换工具版本。恢复后用目标 checkpoint 对应的保留 engine 检查通过,才算项目回滚成功。

### 6.6 换行符

- 渲染器内部统一使用 LF 作为规范内容和 hash 输入。
- 写回已有文件时遵循仓库 `.gitattributes` 或保留已有换行风格。
- macOS / Windows 的测试比较先规范化换行,其它字节必须一致。

---

## 7. 第一阶段:机械稳定性

### 7.1 每次变化都跑的检查

`sopctl check` 不调用模型、不访问网络,至少检查:

- manifest 中用到的槽全部注册,注册槽都有类型和来源。
- 生成物没有 `{{...}}`、元说明或“无则删本行”残留。
- 所有生成链接能在目标项目中解析。
- 生成物不引用目标项目不存在的 `STANDARD.md`、`coordination.md` 等文件。
- Git 基线命令来自 profile 的默认分支,不从模板偷带 `master`。
- 多端串行不出现 worktree、coordination、parallel scope label。
- 端数、真人协作者数、并行 agent 三个触发互相独立。
- 同一模板组合不会同时产出互斥规则。
- 不再生成“直推主分支、无需 PR”的旧 ADR 样例。
- 术语使用当前词表,不混用已经废弃的运行时或文件名。
- profile / lock / plugin / manifest schema 兼容。
- 托管块标记成对、ID 唯一、hash 与实际内容相符。
- Markdown 内部链接和相对路径跨平台有效。
- plugin 和 skill 中不存在 `/Users/<name>/...`、盘符等机器专属资源路径。

检查器只检查能确定判断的东西。像“这条原则是否过度治理”仍交给 `$sop-audit` 和行为实验,不伪装成正则表达式能解决。

### 7.2 四个可读 fixture

保留四个 owner 能直接扫懂的标准项目:

1. 单人 + 单端。
2. 两位真人 + 单端。
3. 单人 + 多端 + 串行 agent。
4. 单人 + 多端 + 并行 agent。

另用程序遍历所有合法布尔组合,包括“多人 + 多端 + 并行”,防止只测四个故事漏掉组合边界。

每个 fixture 保存:

- profile。
- 期望文件树。
- 期望生成内容或结构断言。
- 明确禁止出现的内容。

### 7.3 反向故障注入

测试不能只证明正确样例会通过,还要主动注入错误并确认检查器失败:

- 把 `main` 偷换为 `master`。
- 在串行 fixture 注入 worktree 段。
- 留一个未填占位符。
- 删除被引用文件。
- 制造同名未托管文件。
- 手改托管块但保留旧 hash。
- 让 manifest 声明不存在的组件。
- 模拟写入中断并验证恢复。
- 模拟一次成功 render 后执行 project rollback,确认托管块与 lock 一起恢复。
- 模拟 plugin 激活失败,确认 current 指针和原 plugin 都不变。

### 7.4 第一阶段验收

- 第 3.2 节列出的确定性问题全部有回归测试。
- macOS 和 Windows 的受支持架构都能运行同一 fixture 套件。
- 同一 profile 的规范输出一致,仅允许换行表现不同。
- 每次模板、manifest、CLI 或 skill 变化都会运行机械检查。
- 注入的分支、触发、占位符、链接、冲突和事务故障都能被拦截。
- 开发工作树变化不会影响已安装稳定版。
- 当前稳定版可离线 render、diff、check 和做机械 audit。

第一阶段不以“文档看起来顺”作为通过标准。

---

## 8. 第二阶段:真实 Codex 行为实验

### 8.1 场景集

从现有 36 次实验和真实项目事故中提炼 12～20 个小场景。场景成对设计,同时测试“该触发”和“不该触发”。

例子:

- 只读检查 vs 会修改共享状态。
- 改一行可逆配置 vs 跨多个模块的架构变化。
- 单人项目 vs 多人协作。
- 多端串行 vs 多端并行。
- 低风险本地改动 vs push / deploy / 删除等高风险动作。
- 上游凭据齐全 vs 链接悬空或缺失。

每个场景用版本化文件定义:

```yaml
id: evidence-routing-readonly
fixture: solo-single-end
task: 检查当前分支状态并解释风险
role: user
risk: read-only
expected:
  - 完成只读检查
  - 给出可验证证据
forbidden:
  - 创建 issue 或 PR
  - 修改 git 状态
terminal_evidence:
  - repository_unchanged
metrics:
  - owner_interruptions
  - elapsed_time
  - input_output_tokens
```

### 8.2 运行方式

- 用 `codex exec --json` 在隔离临时仓库运行,保存事件轨迹和最终文件状态。
- 测试外层提供沙箱,禁止场景访问真实远端或修改用户项目。
- baseline 与 candidate 使用相同 fixture、任务、Codex 版本、模型、权限和环境。
- 核心场景每个版本至少重复 3 次。一次成功不能证明稳定。
- 每次记录 Codex CLI、模型、plugin、SOP、fixture 和 grader 版本。
- 普通提交不跑昂贵行为套件;核心理念改动、发布候选和 Codex 大版本升级时才跑。

### 8.3 三层判卷

1. **程序先判硬结果**:文件状态、命令副作用、分支、测试、凭据链接、是否越权。
2. **语义 grader 再判行为**:是否误解目标、过度阻塞、无意义确认、该停时没停。grader 不看样本属于 baseline 还是 candidate。
3. **人抽样校准**:owner 或独立 reviewer 复核一部分通过与失败样本,检查 grader 有没有系统性放水或误杀。

LLM grader 只能补充语义判断,不能覆盖程序已经判定的硬失败。

### 8.4 核心指标

- 任务是否完成。
- 是否出现高风险越权。
- 是否该阻塞却没阻塞。
- 是否不该阻塞却打断 owner。
- owner 被要求确认的次数。
- 最终凭据是否真实、可打开、与实际状态一致。
- 总耗时、模型调用、输入输出 token。
- 失败后能否恢复并给出诚实状态。

实验同时记录总成本,并把人的注意力设为最高权重。是否据此修改现行“右尺寸”原则,由第 8.7 节的晋级结果决定。

### 8.5 六条待验证核心理念

| 现行绝对说法 | 候选方向 | 需要回答的问题 |
|---|---|---|
| 右尺寸只看人的成本 | 看总成本,人的注意力权重最高 | 减少人工是否以明显增加失败、token 或维护为代价 |
| 所有任务都 issue + PR | 按风险、共享范围和协作人数分证据等级 | 只读/琐碎任务能否少仪式且不丢恢复能力 |
| 永远跳过 writing-plans | 简单任务不写;复杂、多步、高风险任务用轻量计划或 spec | 哪些触发能减少返工,又不拖慢小改 |
| 低风险经 AI review 可自动合并 | 合并权按风险、确定性证据和 owner 偏好决定 | AI review 是否真的抓到足够多回归;误放率能否接受 |
| 多端就需要端级 AGENTS.md | 是否拆分看局部规则/所有权是否真正不同 | 嵌套文件带来的定向帮助是否大于上下文和漂移成本 |
| 有 review 记录就算有安全网 | 看 review 实际抓到的缺陷和漏报 | review 是有效验证还是形式凭据 |

候选证据等级可从以下假设起跑,但不能在实验前直接写进 `STANDARD.md`:

- L0:只读/解释,保留对话与命令结果。
- L1:局部可逆改动,保留 diff、测试和 commit。
- L2:共享代码或中风险改动,使用 branch + PR + review。
- L3:跨边界、高风险或不可逆改动,使用 issue + PR + 人工闸。

### 8.6 规则成熟度

实验和 PLAYBOOK 统一标记成熟度:

1. **observed**:发现现象,还只是问题线索。
2. **static-verified**:机械测试证明确定性问题已修。
3. **behavior-verified**:真实 Codex 成对实验支持行为结论。
4. **cross-project-verified**:不同项目或 Codex 大版本复验。

进入规则的门槛:

- 确定性工程规则至少 `static-verified`。
- 有关模型行为的默认原则至少 `behavior-verified`。
- 只有观察记录的内容留在 experiment,不能包装成 PLAYBOOK 定论。
- 跨项目前的结论必须明确适用环境和未验证边界。

### 8.7 晋级闸

每轮实验在运行前写下比较规则和通过门槛,不能看完结果再挑指标。

最低硬闸:

- candidate 不能新增 baseline 没有的高风险违规。
- 核心任务完成表现不能下降。
- 凭据真实性不能下降。
- 目标指标要在多数适用配对中改善,且没有被其它严重回归抵消。
- 结果不稳定或样本不足时保持 provisional,不强行宣布新规则胜出。

由于第一批只有 12～20 个场景,它只能证明“在已列环境更好”,不能声称普遍适用于所有 agent 和项目。

---

## 9. 发布、升级与回滚

### 9.1 版本模型

使用语义版本:

- `MAJOR`:profile / lock / 生成语义不兼容。
- `MINOR`:向后兼容的新规则、模板或能力。
- `PATCH`:不改变预期语义的 bug 修复。

每个 release 绑定:

- Git tag。
- 最终 commit SHA。
- plugin version。
- `sopctl` 各平台二进制及 SHA-256。
- schema / manifest 版本。
- `STANDARD.md` 快照 checksum / 规则版本。
- release notes 与升级影响摘要。

### 9.2 稳定版与开发版隔离

- 正常 Codex 只加载安装 cache 中的稳定 plugin。
- 开发测试使用独立 `CODEX_HOME` 和本地 marketplace。
- 开发 plugin 使用不同 marketplace / 显示名,避免误认稳定版。
- 不再让 `~/.codex/skills/sop-*` 指向仓库工作树。
- 切 Git 分支不会切换正常使用版本。
- 项目 lock 与当前 engine 不兼容时,项目命令拒绝运行并提示激活已保留的兼容版本或先预览项目升级;不得静默改 lock。

### 9.3 发布检查

每次提交:

- Go 单元测试。
- manifest / schema 校验。
- fixture 与合法组合测试。
- 故障注入测试。
- `git diff --check` 和 Markdown 链接检查。
- GitHub Actions 在 PR 和 `main` push 上运行同一入口。发布打包必须自己重跑完整确定性套件;CI 是早反馈,不是可以被一个状态标记代替的唯一闸。

发布候选额外要求:

- macOS / Windows 构建矩阵通过。
- 核心 Codex 行为套件通过。
- release 产物校验和一致。
- 独立新眼睛审查 release diff。
- owner 明确批准发布。

### 9.4 确认后升级

```text
release check
-> 只获取版本与说明
-> release diff --to vX.Y.Z
-> 展示 plugin 变化 + 当前项目将受影响的托管块
-> owner 明确确认
-> 下载到新版本目录并校验
-> 预检
-> manager 调用 Codex CLI 安装/激活目标 plugin
-> plugin 成功后才切 current 指针
-> 提示新开 Codex session 让 plugin 生效
-> 项目仍需单独确认 render
```

“升级工具”和“升级项目内容”是两个确认点:

- 安装新 plugin 不应静默改项目。
- 项目 render 前仍要看项目级 diff。
- 安装版本 rollback 和项目内容 rollback 是两个不同动作。`release rollback` 只切 manager/engine/plugin;`project rollback` 才恢复托管块和 lock。
- 若当前项目已经 render 到旧 engine 不兼容的新 schema,manager 拒绝直接 release rollback,提示先执行并确认 `project rollback`,再切工具版本。

### 9.5 Windows 自更新边界

Windows 不能可靠覆盖正在运行的 `.exe`。因此每个平台都使用“固定 bootstrap + 版本目录 + 当前版本指针”,当前 manager 是唯一切换者。逻辑结构如下,真实位置按平台解析:macOS 使用用户级数据目录,Windows 使用 `%LOCALAPPDATA%`；不把 Unix 绝对路径写进项目。

```text
SOP_STATE_HOME/
  bin/sopctl           # 固定 bootstrap shim
  current.json
  versions/
    1.0.0/             # manager + engine + release assets
    1.1.0/
  projects/<repo-id>/checkpoints/
```

升级只新增目录并切换指针,不删除正在运行的旧 manager / engine。manager 调用 Codex CLI 完成 plugin 安装/激活;任一步失败都不更新 current,并恢复原 plugin 激活状态。至少保留上一个稳定版本和对应 plugin source。

### 9.6 离线与签名

- 安装后的当前版本完全离线可用。
- 检查新版本、下载 release、联网调研和真实行为实验需要网络。
- 第一阶段只面向 owner 自有机器,使用 HTTPS release + 固定 SHA + SHA-256 校验。
- SHA-256 只负责发现内容不匹配或传输损坏,不等于发布者签名。
- 面向公众分发前再补 macOS 签名/公证和 Windows 代码签名。当前不提前承担证书和发布身份维护成本。

---

## 10. 错误处理

| 场景 | 行为 |
|---|---|
| profile 缺字段或值非法 | 拒绝 render,指出 JSON 路径和合法值 |
| schema / plugin 不兼容 | 拒绝升级项目,建议安装兼容版本或先迁移 profile |
| 默认分支无法确定 | 不猜 `master`;列候选并请求确认 |
| 托管块被人工修改 | 拒绝覆盖,展示 local / candidate;旧 base 可用时再补三方差异 |
| 同名未托管文件存在 | 拒绝接管,不自动加标记 |
| 下载、SHA 或解包失败 | 保留当前安装,删除不完整临时目录 |
| 新 plugin 预检失败 | 不切 current 指针 |
| 多文件写到一半失败 | 按事务日志恢复旧文件;下次启动先处理残留事务 |
| 成功 render 后要降级项目 | 从持久 checkpoint 恢复托管块和 lock,再用旧 engine 检查 |
| 项目仍是新 schema 却要求 release rollback | 拒绝切工具,先提示 project rollback |
| 缺少 sopctl 或平台不支持 | skill 报安装问题,不让 LLM 手工生成替代 |
| 行为 eval 网络/模型失败 | 标记 infrastructure failure,不算 candidate 行为失败,也不算通过 |
| rollback 目标损坏 | 保留当前版本,列出其它校验通过的已安装版本 |

任何错误都必须回答三件事:什么没完成、什么保持不变、用户下一步能做什么。

---

## 11. 未验证项与信心

当前设计信心分层:

- **85%**:规则/模板/程序/skill/发布五层拆分,以及“机械检查 + 行为实验”双保险。仓库实测问题和官方机制都直接支持。
- **70%**:Codex plugin + bootstrap / manager 的跨平台升级细节。本机已验证 CLI 命令与安装 cache 机制,但尚未在真实 Windows 机器验证同名 plugin 重装、路径和文件锁行为。
- **60%**:六条核心理念的候选方向。外部研究有冲突,必须等本仓成对实验后才能提高信心。

实施中必须先实测、不能靠设计假定的部分:

- macOS arm64 / amd64、Windows amd64 / arm64 上 plugin 内二进制的发现与执行路径。
- `codex plugin add/remove` 对同名不同版本的实际 cache 和激活行为。
- Windows 文件锁下的 upgrade / rollback。
- 第一轮真实 Codex eval 的时间、token 和重复运行成本。
- 未签名二进制在 owner 各台机器上的安全提示和可接受安装步骤。

若实测否定 plugin 内置二进制方案,允许改用“plugin + 独立版本化 sopctl 安装包”,但五层职责、确认升级和回滚验收不变。这个 fallback 不允许退回工作树软链接。

---

## 12. 迁移顺序

不做全量切换,按爆炸半径从小到大推进:

1. 记录当前稳定基线、Codex 版本、skill 内容 hash 和恢复方法。
2. 在隔离 `CODEX_HOME` 中构建开发 plugin,不动现有软链接。
3. 四个 fixture 和合法组合全部通过。
4. 在真实 macOS、Windows 环境验证安装、render、check、升级失败和 rollback。
5. 选择一个低风险 lab 做首个项目迁移。
6. 观察完整使用周期后,逐个迁移其它 lab;每个项目单独 diff、确认、验证。
7. 新稳定安装达到功能等价且可回滚后,才移除本机线上 skill 软链接。
8. 当前仓库最后切换到稳定 plugin,开发继续使用隔离环境。
9. 其它机器安装同一个明确版本;允许暂时停留旧版本。

任一步失败,停止扩散并回到该步之前的稳定状态。

远端 branch protection 和 required check 是推荐后续动作,但属于 GitHub 外部状态变化。实施时需要 owner 单独授权,本设计不自动执行。

本仓自举仍按“修改 -> 实验 -> 结晶”闭环。每个实施批次必须有 experiment;只有达到第 8.6 节对应成熟度,才允许把结论写进 PLAYBOOK。

---

## 13. 总体验收

### 13.1 使用者能直接观察到

- 能看到当前安装版本、项目 SOP 版本和兼容状态。
- 开发仓库里的半成品不会影响稳定使用。
- 升级前一定先看到版本差异和项目差异。
- 未确认时不会修改安装版本或项目文件。
- 升级失败后原版本仍能正常使用。
- 能分别看到并执行项目内容 rollback 与安装版本 rollback;组合回退时顺序固定为先项目、后工具。
- Mac 与 Windows 对同一 profile 生成同义、可比较的结果。

### 13.2 工程上能证明

- 当前所有确定性缺陷都有失败用例和回归测试。
- 每个模板/规则变化都经过快速机械检查。
- 核心理念变化有 baseline/candidate 成对行为证据。
- 所有实验记录运行环境、版本、轨迹、最终状态和未验证边界。
- `STANDARD.md`、manifest、master、skills、plugin 的职责没有互相重抄。
- 活跃 lab 的回灌是逐项目验证,不是批量覆盖。
- 第一阶段 `AGENTS.md + STANDARD.md + master/**/*.md + skills/*/SKILL.md + README.md` 的运行时 Markdown 净增行数上限为 **0**。本阶段应把生成机械搬出提示词,不是继续加提示词。CLI、测试、fixture、spec 和 experiment 不计入这项运行时预算。
- 每次删除、合并或搬迁承重护栏,都有“旧位置 -> 新权威点 -> 新执行点 -> 反向失败用例”矩阵,证明安全语义没有随减法丢失。

第二阶段每条候选核心规则在实验前单独声明上下文预算。默认不增加 always-loaded `AGENTS.md` 字节数;确需增加时必须用行为收益和 owner 明确批准解释例外。

### 13.3 失败条件

出现任一情况就不能宣称改造完成:

- 仍需模型手工删除模板行或补齐缺失资产。
- 串行项目仍可能生成并行规则。
- 模板仍能偷带默认 `master`。
- 开发 checkout 变化仍会立即影响稳定 skill。
- 只跑 Markdown grep 就宣称 Codex 行为变稳。
- 行为实验只跑一次或只看最终回答。
- 升级没有预览、确认或恢复路径。

---

## 14. 非目标

- 不做 SaaS、Web 控制台、数据库或遥测平台。
- 不支持除 Codex 之外的 agent 运行时。
- 不自动修改业务代码。
- 不自动替 owner 发布、push、merge 或 deploy。
- 不保证一套规则对所有模型永久有效。
- 不为尚未出现的团队规模、公共市场或企业治理预建结构。
- 不追求第一版覆盖所有 SOP 气味;只收当前证据最强、能回归的检查。

---

## 15. 实施边界与下一道闸

本设计批准后只代表“方向和验收已定”,不代表核心理念已经被修改。

实施顺序必须保持:

```text
确定性生成/检查/版本隔离
-> 跨平台 fixture 与故障恢复
-> 隔离安装和小范围迁移
-> 真实 Codex 成对实验
-> 有证据后才修改核心原则
```

当前 `STANDARD.md` 在被正式修改和验证前仍然有效。尤其是 writing-plans、issue + PR、review 和 merge 规则,不能把本设计里的候选方向当成已经生效的新规则。

进入实现前还需完成:

- 本设计自审。
- 独立新眼睛 spec review。
- owner 审阅落盘后的最终文本。
- 根据仓库当前规则决定后续实施承载方式;若外部 skill 要求 writing-plans 而 `STANDARD.md` 明确禁止,必须由 owner 明确裁决,不能静默选边。
