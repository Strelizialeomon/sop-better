# exp-037 · 用确定性引擎和本机可信凭据给 SOP 上双保险

- **日期**:2026-07-11
- **真实任务**:在隔离 worktree 中实现已批准的 phase 1 稳定性方案,把 sop-better 从工作树软链改造成可预览、确认、原子写入、可回退的本地发布候选
- **本次撒手档位**:L3(工程实现交给 agent + 多轮独立新眼睛;不迁移线上 skill,不 push,不宣称真实 Windows 已验证)

---

## 1. 选活 + 定档

owner 要在多台自己的 macOS / Windows 机器上使用 Codex,首要目标是“更稳定”,并选择“双保险 + 确认后升级”。旧形态的 `skills/` 直接软链工作树,开发中间态会立即影响日常使用;生成和升级又主要靠 Markdown 约定,无法证明预览、落盘和回退是同一笔变化。

本批只做 phase 1 的确定性工程,不借机改六条核心理念。爆炸半径高:工具会管理项目 `AGENTS.md`、`.sop/profile.json`、`.sop/lock.json` 和本机 plugin 版本。因此所有工作留在 `.worktrees/sop-stability-phase1` 的 `feat/sop-stability-phase1`,不碰当前线上软链、不迁移 lab、不 push。

## 2. 便宜验证方案

> 我怎么在 20 分钟内、不逐行读,就看出 AI 做对没?

- 四类 fixture + 合法组合矩阵证明同一 profile 确定生成,串行不混入并行规则,多端才生成端级操作台。
- acceptance / integration 用真实 CLI 跑 `diff -> render -> check -> checkpoint -> rollback` 和 `install -> upgrade -> rollback`,快照比较“失败前后完全没变”。
- transaction failure injection 验 profile、托管输出、lock 不会留下半套;下次 mutation 先恢复再读契约。
- 发布 gate 验 clean tagged exact commit、测试、vet、plugin validator、四个目标包和 bundle hash;非 gate 命令不能冒充正式发布。
- 冻结后复核的运行时 Markdown 行数从基线 `1117` 到 `874`,净减 `243`,满足净增不高于 0。

若这些答不上来,不准迁移线上 skill,也不准把“交叉编译成功”写成“Windows 实跑成功”。

## 3. 给 AI 的简报

```text
目标:
把 STANDARD / manifest+master / engine / skills / plugin+release 拆成五层;
项目变更可精确预览、原子提交、校验和回退;
工具升级必须确认,失败保留旧版,可指定健康旧版回退。

约束:
STANDARD 仍是唯一语义真相源;不重抄规则;
只支持 Codex;macOS + Windows 路径语义一致;
本批运行时 Markdown 净增 <= 0;
项目控制的 lock / journal 不能自证有权删除或恢复文件;
只在隔离 worktree 实现,不 push,不迁移线上。

验收标准:
确定性 matrix、负向用例、故障注入、项目回退、安装回退、plugin 校验和四目标构建通过;
每次失败都说明什么没完成、什么保持不变、下一步怎么做;
记录未验证边界和护栏迁移矩阵。
```

## 4. AI 跑完,我来评

### 已实现的决定

- 用 Go 做一个小型“编译器 + 包管理器”:profile 是输入,manifest 声明规则到产物的映射,engine 负责确定生成和检查,manager 负责安装版本切换。
- 项目写入顺序固定为 profile -> managed outputs -> lock,由 journal + 外部授权一次提交;`diff --profile` 只展示这笔候选事务,零项目写入。
- managed output 只支持带自校验 marker 的 block。phase 1 不支持无法证明所有权的 full-file 管理。
- 删除旧块、项目 rollback 和中断恢复不能只信项目内 lock / journal;还要匹配本机状态目录按物理项目身份保存的可信 provenance / authorization。
- checkpoint 保留精确旧 profile、lock 和托管内容,公开 `project checkpoints` 列 ID;同 schema 可跨 SOP 版本恢复。
- 工具发布包自带 bootstrap、installer、manager、engine;本地 / 同步目录作为 phase 1 发布源。升级展示文件与项目差异,TTY 输入完整版本号后才执行。
- 默认 rollback 目标损坏时扫描已安装版本并列健康候选;`release rollback --to VERSION` 仍走差异、确认、项目兼容和目标 engine 检查。

### 新眼睛实际抓到并纠正的翻车

- 第一版 `diff` 漏了 profile / lock,预览不是实际事务。
- release diff 只比 metadata,没有比真实文件和目标 engine 项目差异。
- checkpoint 在失败事务前先 prune,可能删掉旧退路却没造出新退路。
- 只有 upgrade、没有真正的首次 installer;中断恢复又排在读取半写 profile 之后。
- `.gitignore` 写错 transaction journal 路径;绝对路径、JSON `null` 和 macOS 大小写别名能穿过契约。
- 同一个已发布 output ID 能换 target;checkpoint ID 存在但用户无法发现。
- acceptance 一度只测被删除的开发 monolith,没测正式 `sopctl-engine`。
- 旧 `CLAUDE.md` 只扫当前端,删掉的端和 `.claude/` 残留会漏。
- 项目内 journal 可伪造 backup / target,还能经 symlink 碰 VCS;后来补成本机授权 + hash 绑定 + 先全验后写。
- 伪造 lock 一度能授权删用户文件;再加 provenance 后,伪造“当前输出 + lock”仍能被存进 checkpoint,又补了 active-lock 早拒。
- 瘦身时丢过反驳、spec 三查、决策快照、治理文档人审和 risk 执行差异;靠承重墙反向矩阵补回。
- 打包后的 skill 相对路径最初仍指开发 checkout;改为从 plugin 内 `../../rules/` 解析并实包验证。
- profile 单行事实最初可带换行变成隐形指令;manifest 现在必须声明 slot format,inline 值拒绝控制字符,默认分支按 Git ref 规则校验。

### 护栏迁移矩阵

| 旧位置 / 旧载体 | 新权威点 | 新执行面 | 反向失败用例 |
|---|---|---|---|
| review 补偿、spec 三查、决策快照散在旧长文 | `STANDARD.md` `SOP-REVIEW` / `SOP-EVIDENCE` | `master/base/AGENTS.md` + `master/base/docs/project/issue-pr-workflow.md` + `master/layer-multiend/end-common.md` | `TestGeneratedRootKeepsLoadBearingPushbackAndReviewGuardrails` 缺任一句即失败 |
| 反驳协议与承重墙例外 | `STANDARD.md` `SOP-PUSHBACK` | 根 `AGENTS.md` 的最强反对 / agreement source / 守一轮 / 高风险加倍 | 同一 guardrail acceptance 验“承重墙不豁免”等反向词 |
| 治理 doc / contracts owner review | `STANDARD.md` `SOP-GOVERNANCE-REVIEW` | `issue-pr-workflow.md` 风险分流 | acceptance 删除治理人审句即失败 |
| house-style 与不靠记忆填栈 | `STANDARD.md` `SOP-HOUSE-STYLE` | profile `house_style[]` + 根 `AGENTS.md` 立栈动作 | profile schema / portability 负测 + 生成物 guardrail 断言 |
| 主动建议但不代拍 | `STANDARD.md` `SOP-ACTIVE-ADVICE` | 根 `AGENTS.md` 主动列选项 / 权衡 / 盲区 | acceptance 删除主动建议句即失败 |
| doc / issue / PR、评论分层、Refs / Closes、待业务确认 | `STANDARD.md` `SOP-EVIDENCE` / `SOP-STRUCTURE-BASE` | `issue-pr-workflow.md` + collaborators handoff + coordination | acceptance 逐项断言三件套、开工展示、最终才 Closes |
| 只读 freshness | `STANDARD.md` 基础 flow | 根 `AGENTS.md` “纯解释、只读检查不修改工作树” | acceptance 删除只读句即失败;CLI diff/check 快照验零写入 |
| risk 过去只是 profile 标签 | `STANDARD.md` 风险分流 | manifest `risk_guidance` 派生槽 -> 根 `AGENTS.md` | `TestRiskChangesGeneratedReviewAndOwnerGate` 三档产物必须不同 |
| 多端“身份文档”太薄 | `STANDARD.md` `SOP-STRUCTURE-MULTIEND` | `master/layer-multiend/end-common.md` 端内操作台 | legal matrix + guardrail acceptance 验取活 / Step 3/4 / review |
| Claude 兼容入口继续复制旧运行时 | `STANDARD.md` Codex-only 边界 | init 不生成 + audit 报 stale + engine 递归拒绝 residue | `TestDeprecatedClaudeRuntimeResidueIsSurfacedWithoutSilentDeletion` 覆盖根、旧端、`.claude/` |

### 评分

owner 尚未对代码逐项单独打分;本批以便宜的客观验证代替“看起来不错”的主观完成判定。

| 维度 | 分 | 备注 |
|---|---|---|
| 设计质量 | 待 owner 真机验收 | 静态 / 本机路径已收紧,真实 Windows 仍未跑 |
| 省力程度 | 待 owner 真机验收 | 日常只需 profile + diff / confirm,但尚未迁移线上 |
| 爽感 | 不评分 | 这批目标是稳,不是追求惊喜 |
| 验证成本 | 9/10 | 绝大多数失败可用单条 CLI + 快照 / hash 复查 |

### 证据与成熟度

- **成熟度**:L3 static-verified。已在当前 macOS 本机跑真实 CLI、临时 `CODEX_HOME`、故障注入和交叉构建;没有跨真实 Windows / lab 项目 / 完整行为周期证据。
- 已通过的代表性命令:
  - `go test ./internal/config ./internal/engine ./acceptance -count=1`
  - `go test ./internal/... ./integration -count=1`(官方 plugin validator 实跑)
  - `git diff --check`、全部 JSON `jq empty`、STANDARD hash 对账
  - 官方 `quick_validate.py` 验两个 skill,官方 `validate_plugin.py` 验 source plugin
- 冻结前仍会重跑全量、race、vet、正式 release gate、首装 / 升级 / 回退和四目标包;只把实际通过结果写入最终交付,不提前冒充证据。

### 明确未验证边界

- 没在真实 Windows + Codex 上跑 plugin 发现、文件锁、中断恢复和 rollback;CI / 交叉编译不能替代真机。
- phase 1 只有本地 / 共享盘 / 同步目录发布源,没有 HTTPS downloader、外部固定 SHA pin 或公开更新服务。
- profile schema 固定为 1;没有 schema 2 的 MAJOR 迁移与跨 schema 回退。
- 本批没有运行第二阶段的 baseline / candidate 成对 Codex 行为实验,因此不声称六条核心理念已经被行为证据优化。
- 没迁移 taoxi-geo / geo-reverse / media-ops,也没改当前线上软链。

## 5. 抽一条教训 -> 回填 PLAYBOOK

这次学到:在 L3 档把【会改项目文件和自身版本的 SOP 工具】交出去,前提不是“项目里有 lock / journal”就够了。项目控制的 metadata 不能自证权限;删除、恢复和回退必须同时有本机可信 provenance、精确预览、原子事务、故障注入和失败后不变快照。否则“可回退”只是文案。

-> 已写入 `PLAYBOOK.md`?[x]
