# exp-039 · 增量复审先过真实耗时与查漏门槛

- **日期**:2026-07-14
- **真实任务**:Loop MVP 已知租约竞态修复（`431d3a3..d32b5ed`）及本轮 canonical review controller
- **本次撒手档位**:L3（真实代码、隔离 worktree、无 push / merge）

## 1. 选活 + 定档

复审反复重扫约 4k 行任务 diff，是真实耗时病灶；已知 F-001 / F-002 又有明确 closure，可同时量耗时和查漏。翻车只影响实验分支，但错放行会让后续任务绕过独立 review，按高安全标准验。

## 2. 便宜验证方案(动手前必须答)

> 我怎么在 20 分钟内、不逐行读，就看出 AI 做对没?

- 验证手段:同模型 / high effort / read-only sandbox 对已知修复各跑增量与完整审查；记录 wall time、token、finding；全量测试 + race + 独立新眼睛审最终 diff。
- 放行阈值:增量必须关闭已知 finding，最终完整审查不能补抓因增量漏掉的 blocking；3+3 次中位 wall time ≤ 完整审查 50%。
- 实现预算:初始上限为本批相对 `db53a7a` 净增不超过 2150 行；反向验收确认 agent 不能手填 review/check 证据、覆盖链不能断、PR head 被 review 覆盖且实际 merged commit 仍跑全量 checks。独立审查发现 App 来源、merged commit 和 task diff scope 三个承重缺口后,首修上限固定为 2500 行。owner 随后明确把信任模型改为 cooperative-local,并要求补 exact-HEAD workspace、delta allowlist 和厚 doc 物化；这属于已确认的验收变更,实施前把第二版上限固定为 2850 行,不改性能放行阈值。设计落盘时净增 2494 行。

## 3. 给 AI 的简报(只给目标+约束+验收,不给解法)

`目标:`缩短修复后的复审，不牺牲独立审查和 done 证据闸。`约束:`reviewer 只读且独立；controller 生成 canonical event；边界变化回退 full。`验收:`连续覆盖链、稳定 finding ID、两级 checks，且真实对照过门槛。

## 4. AI 跑完,我来评

- **AI 做了哪些决策**:实现 full / delta 游标、稳定 finding ledger、changed-path check triggers、canonical GitHub review event、controller 续租与最终证据重建；reviewer 不加载项目 SOP，使用结构化输出。
- **超出我预期 / 我自己想不到的地方**:第一次真实跑暴露的主要浪费不是 diff 本身，而是 reviewer 自动加载长项目指令并倾倒大段工具输出；`project_doc_max_bytes=0` 后 token 明显下降。完整对照又抓到终态 machine fencing 和 review 自证两个旧阻塞点。
- **翻车 / 我得纠偏的地方**:`codex exec review --base` 不能和自定义 prompt 同用，改为普通 `codex exec`。未隔离项目文档的增量轮在 183.75 秒、107,685 token 时中止；隔离后为 163.32 秒、62,486 token、F-001 / F-002 resolved。同设置完整审查为 250.73 秒、129,419 token，补抓 F-003（非 owner 机器可借 claim 关闭任务）和 F-004（执行 agent 可手填 review 自证）；比例 65.1%，未达 50%。App Bot + provenance 字段挡住可信人评论,但同一 ambient `gh` 既不能同时完成 `/user` 与 installation-token Bot 写入,也没有把 App 写凭据隔离出执行 agent；独立代码复审仍判 Critical / Not Ready。PR head / merged commit、scope rename/type-change 已补,exact-HEAD review workspace、可靠失效识别和厚 doc 决策快照仍待设计。且仅有 1+1 个有效样本，未达 3+3，只留实验态。

### 评分(10 分制)

设计质量 8；省力程度 7（token 约减半，wall time 只减 35%）；爽感 8（完整对照抓到两个安全洞）；验证成本 8。

## 5. 抽一条教训 → 回填 PLAYBOOK

这次学到:增量复审不能只缩 diff；还要隔离常驻指令、限制工具输出，并用同配置完整审查做查漏与耗时对照。任一安全 / 50% / 3+3 门槛未过，就只能保留实验态。

→ 已写入 `PLAYBOOK.md`?[x]

**owner 取舍（2026-07-14）**:明确退回轻量 cooperative-local,不建 daemon / broker。它只防正常 `sopctl` 路径误填和错位,不防持同一 GitHub 凭据的 agent 故意伪造 marker；因此强制手动 start、关闭自动合并,不用于敌对输入或无人值守。

**未验证边界**:真实 GitHub 双机器 review event、真实 Windows shell、3+3 样本均未跑。macOS 只读 sandbox 的 `/tmp/xcrun_db` 警告会抬高两侧 wall time，未把它误算成 candidate 通过。这些边界和 50% 阈值未过之前,`loop-v1-experimental` 不升级为稳定版。
