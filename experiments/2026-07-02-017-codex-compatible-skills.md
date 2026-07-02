# exp-017 · sop-better 技能兼容 Codex

- **日期**:2026-07-02
- **真实任务**:把原本偏 Claude Code 的 `$sop-init`、`$sop-audit` 改成 Codex-first。
- **本次撒手档位**:L2(agent 改 + 本地 grep 验证;owner 后续可扫 diff)。

---

## 1. 选活 + 定档

用户明确要从"专供 Claude Code"改成"兼容 Codex"。爆炸半径中等:改的是线上软链技能与 SOP 母本,会影响后续所有项目的 init/audit 输出。

## 2. 便宜验证方案(动手前必须答)

- 验证手段:grep 三类红灯——技能是否提到 `~/.codex/skills`;`$sop-init` 是否生成 `AGENTS.md`;`$sop-audit` 是否识别 Codex 项目指令文件。
- 如果上面答不上来 → 先补的功课是:确认 Codex 官方项目指令文件名。

## 3. 给 AI 的简报(只给目标+约束+验收,不给解法)

```
目标:让 sop-better 技能能在 Codex 下使用,不再只面向 Claude Code。
约束:不重写 SOP 核心模型;保持单一真相源;避免 CLAUDE.md 与 AGENTS.md 两份全文漂移。
验收标准:
- sop-init 明确支持 Claude Code / Codex / both runtime。
- Codex 项目生成 AGENTS.md,both 模式只有一份全文 + 一份桥接。
- sop-audit 能把缺 AGENTS.md 或双全文漂移报成 finding。
- 本仓 AGENTS/CLAUDE/README 说明双运行时入口与双软链安装状态。
```

## 4. AI 跑完,我来评

- **AI 做了哪些决策**:用"agent 指令文件"抽象运行时差异;Codex 走 `AGENTS.md`;both 模式以 `AGENTS.md` 为全文真相源、`CLAUDE.md` 只桥接;本仓自身也按此规则补 `AGENTS.md`。
- **超出我预期 / 我自己想不到的地方**:本机已经有 `sop-init` / `sop-audit` 的 `~/.codex/skills` 软链,真正缺口主要在生成物和 audit 逻辑。
- **翻车 / 我得纠偏的地方**:未真跑一次 `$sop-init` dogfood 生成 Codex 项目,当前只做了文本规则与 grep 验证。

### 评分(10 分制)

| 维度 | 分 | 备注 |
|---|---|---|
| 设计质量(它做得好不好) | 8/10 | 单一真相源没有被双全文破坏 |
| 省力程度(比我自己干省了多少脑子) | 8/10 | 主要是系统性扫残留 |
| **爽感**(有没有"卧槽这样也行") | 6/10 | 发现软链已在,少做安装层工作 |
| 验证成本(检查它的结果累不累) | 7/10 | grep 便宜,但缺真实生成复验 |

## 5. 抽一条教训 → 回填 PLAYBOOK

这次学到:兼容新 agent runtime 时,别只查 skill 是否装上;还要查它生成的 always-loaded 项目指令文件是否是该 runtime 会读的文件。

→ 已写入 `PLAYBOOK.md`?[ ] 不写。先等一次真实 Codex 项目 dogfood 后再沉淀。
