# exp-039 · 增量复审先过真实耗时与查漏门槛
- **日期**:2026-07-14
- **真实任务**:Loop MVP 已知租约竞态修复（`431d3a33a0807b79a677aa92d4b05dcfaa7581c1..d32b5edaed073d8e3adba6497891daadc64a64b3`）及本轮 review controller 候选
- **本次撒手档位**:L3（真实代码、隔离 worktree、无 push / merge）
- **凭据状态**:失败记录。租约修复区间可定位,但 controller 候选 tree、模型 / Codex 精确版本、完整命令、机器快照和原始运行日志未保存;下列数字只作本轮诊断,不得计入后续 3+3 汇总或稳定放行。
## 1. 选活 + 定档
复审反复重扫约 4k 行任务 diff，是真实耗时病灶；已知 F-001 / F-002 又有明确 closure，可同时量耗时和查漏。翻车只影响实验分支，但错放行会让后续任务绕过独立 review，按高安全标准验。
## 2. 便宜验证方案(动手前必须答)
> 我怎么在 20 分钟内、不逐行读，就看出 AI 做对没?
- **验证手段 / 放行阈值**:同模型 / high effort / read-only sandbox 对已知修复各跑增量与完整审查；记录 wall time、token、finding；全量测试 + race + 独立新眼睛审最终 diff。增量必须关闭已知 finding，最终完整审查不能补抓因增量漏掉的 blocking；3+3 次中位 wall time ≤ 完整审查 50%。**实现预算**:初始上限为本批相对 `db53a7a` 净增不超过 2150 行；反向验收确认 agent 不能手填 review/check 证据、覆盖链不能断、PR head 被 review 覆盖且实际 merged commit 仍跑全量 checks。独立审查发现 App 来源、merged commit 和 task diff scope 三个承重缺口后,首修上限固定为 2500 行。owner 随后明确把信任模型改为 cooperative-local,并要求补 exact-HEAD workspace、delta allowlist 和厚 doc 物化；这属于已确认的验收变更,实施前把第二版上限固定为 2850 行,不改性能放行阈值。设计落盘时净增 2494 行。
## 3. 给 AI 的简报(只给目标+约束+验收,不给解法)
`目标:`缩短修复后的复审，不牺牲独立审查和 done 证据闸。`约束:`reviewer 只读且独立；controller 生成 canonical event；边界变化回退 full。`验收:`连续覆盖链、稳定 finding ID、两级 checks，且真实对照过门槛。
## 4. AI 跑完,我来评
- **AI 决策 / 意外收获**:实现 full / delta 游标、稳定 finding ledger、changed-path check triggers、canonical GitHub review event、controller 续租与最终证据重建；reviewer 不加载项目 SOP，使用结构化输出。第一次真实跑暴露的主要浪费不是 diff 本身，而是 reviewer 自动加载长项目指令并倾倒大段工具输出；`project_doc_max_bytes=0` 后 token 明显下降。完整对照又抓到终态 machine fencing 和 review 自证两个旧阻塞点。**翻车 / 纠偏**:`codex exec review --base` 不能和自定义 prompt 同用，改为普通 `codex exec`。未隔离项目文档的增量轮在 183.75 秒、107,685 token 时中止；隔离后记录为 163.32 秒、62,486 token、F-001 / F-002 resolved。同设置完整审查记录为 250.73 秒、129,419 token，补抓 F-003（非 owner 机器可借 claim 关闭任务）和 F-004（执行 agent 可手填 review 自证）；记录比例 65.1%，未达 50%。但因本次没有保留完整运行凭据,这些数值不能独立复核,也不能充当性能基线。App Bot + provenance 字段挡住可信人评论,但同一 ambient `gh` 既不能同时完成 `/user` 与 installation-token Bot 写入,也没有把 App 写凭据隔离出执行 agent；独立代码复审仍判 Critical / Not Ready。PR head / merged commit、scope rename/type-change 已补,exact-HEAD review workspace、可靠失效识别和厚 doc 决策快照仍待设计。且仅有 1+1 个未固化样本，未达 3+3，只留实验态。
**评分(10 分制)**:设计质量 8；省力程度 7（token 约减半，wall time 只减 35%）；爽感 8（完整对照抓到两个安全洞）；验证成本 8。
## 5. 抽一条教训 → 暂不回填 PLAYBOOK
暂定假设:增量复审不能只缩 diff；还要隔离常驻指令、限制工具输出，并用同配置完整审查做查漏与耗时对照。因为缺少可复现运行凭据,该假设暂不结晶为全局经验；下一轮先固定候选 tree、模型 / Codex 版本、命令、sandbox、机器环境和原始日志 digest,再跑 3+3。
→ 已写入 `PLAYBOOK.md`?[ ]
**owner 取舍（2026-07-14）**:明确退回轻量 cooperative-local,不建 daemon / broker。它只防正常 `sopctl` 路径误填和错位,不防持同一 GitHub 凭据的 agent 故意伪造 marker；因此强制手动 start、关闭自动合并,不用于敌对输入或无人值守。
**未验证边界**:真实 GitHub 双机器 review event、真实 Windows shell与最终修正候选的 3+3 尚未跑。50% 阈值未过之前,`loop-v1-experimental` 不升级为稳定版,相关规则不进入 `STANDARD.md` / `PLAYBOOK.md`。
### 实现复审后的回退与再设计（2026-07-14）
- 独立代码复审确认原候选净增 3234 行,超过 2850 上限,并发现 terminal claim 泄漏和 delta 保护表漏新增并发文件;已把可失败的终态准备移到 claim 前,waiting 终态不再依赖原机器 workspace。未过 3+3 / 查漏 / 50% 门槛后曾撤下 delta 与 changed-path checks;owner 随后采纳不靠路径分类的 change-first 再设计,现仅重引入“首轮 full、后续全 change + 语义上下文”的 delta,checks 仍每轮全跑。3+3 未过前仍是实验态,性能结论仍为失败且不回填 PLAYBOOK。
### 修正前预备 3+3（不计最终放行）
固定 `baseline=db53a7a`、`candidate=65496ff8e85d71e495034db5dfd845a1dcb2904b`、`defect=c1ed82585d42d7da3f184ee08c00c69f9150a372`，Codex CLI 0.144.1 / high / read-only / Apple M5，交替跑 delta/full。T1（caller 错把 merged commit 当 coverage head）、T2（delta base 退回 merge-base）、T3（reset 后 ID 复用）均有失败测试，三次 delta 与 full 都抓全；delta wall time=`48.412/66.625/72.484s`，full=`92.865/94.386/134.927s`，中位比值 `66.625/94.386=70.6%`，未过 50%。原始 JSONL、schema、环境与 digest 保存在本机忽略目录 `.worktrees/exp039-artifacts/`；该 candidate 不含随后独立复审要求的 running→done 与显式 retired-ID 修复，所以这组只作失败诊断，不得挑结果放行。
### 最终修正候选 3+3（通过）
固定 `baseline=db53a7a`、`candidate=6384925d85034b29984e032f9f7bd5423da20cee`、`defect=6b31592a380a36510da65dc65b107db9b03f18f3`，同一 Codex / effort / sandbox / 机器配置按 delta/full 交替 3+3。六次均抓全 T1/T2/T3；delta wall time=`75.223/90.149/113.854s`，full=`316.336/316.873/205.106s`，中位数分别为 `90.149s` 与 `316.336s`，精确比值 `28.50%`，通过 `≤50%` 门槛。本机 `.worktrees/exp039-artifacts-final/` 保存原始 JSONL 与摘要；`summary.json` SHA-256=`52e5c837c807cdf315402797e7881313332bfb1a4bdbe42c1d9b507e39fd7791`，`metadata.json` SHA-256=`a86e578fb46b0a36914b438ee48797c0c4276323c9ae7fc456464ce6b56a3885`。
