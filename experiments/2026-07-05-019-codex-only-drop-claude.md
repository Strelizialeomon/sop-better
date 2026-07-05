# exp-019 · sop-better 去掉 Claude 兼容层

- **日期**:2026-07-05
- **真实任务**:sop-better 自身从 Codex-first + Claude Code 桥接,改成 Codex-only。
- **本次撒手档位**:L3(改线上技能和母本,但不改核心 SOP 模型)

---

## 1. 选活 + 定档

用户明确要求「全部转 Codex,不再兼容 Claude」。爆炸半径中等:改的是 `STANDARD.md`、`skills/`、`master/` 母本和仓库入口,会影响后续所有 `$sop-init` / `$sop-audit` 输出。

风险不是代码运行坏掉,而是文档继续制造两套入口:新项目还生成 `CLAUDE.md`,或 audit 把旧桥接当成正常。

## 2. 便宜验证方案(动手前必须答)

> 我怎么在 3 分钟内、**不逐行读**,就看出 AI 做对没?

- 验证手段:`rg` 扫活文档和母本,确认 `Claude Code / both / ~/.claude / {{agent_instruction_file}}` 这类兼容分叉不再出现;`CLAUDE.md` 根文件被删除;`sop-init` 只落 `AGENTS.md`;`sop-audit` 把 `CLAUDE.md` 当 `stale`;`~/.claude/skills/sop-*` 软链删除且 `~/.codex/skills/sop-*` 仍在。
- 如果上面答不上来 → 先补的功课是:分清活文档(会影响生成)与历史实验记录(只保留事实)。

## 3. 给 AI 的简报(只给目标+约束+验收,不给解法)

```
目标:
把 sop-better 从 Claude Code 兼容状态改成 Codex-only。

约束:
不重写 SOP 核心模型;保留单一真相源;不要改写历史实验事实。

验收标准:
- 根目录不再有 CLAUDE.md。
- README / AGENTS / STANDARD 不再宣称支持 Claude Code 或 both runtime。
- sop-init 只生成 AGENTS.md。
- sop-audit 把 CLAUDE.md / .claude 识别为旧残留。
- master 母本不再有 agent_instruction_file 运行时槽。
- `~/.claude/skills/` 不再暴露 sop-better 两个技能。
```

## 4. AI 跑完,我来评

- **AI 做了哪些决策**:去掉 `runtime` 参数,不保留「固定 Codex」的假参数;`CLAUDE.md` 从可接受桥接降级为 `stale` 删除残留;历史 `experiments/` 和旧 spec 不批量改写事实。
- **超出我预期 / 我自己想不到的地方**:把 `{{agent_instruction_file}}` 槽也删掉,否则文案改了但生成器母本仍留着运行时分叉。
- **翻车 / 我得纠偏的地方**:需要明确区分活文档与历史记录,否则 `rg claude` 永远会被历史实验打红,但那不是当前行为残留。

### 评分(10 分制)

| 维度 | 分 | 备注 |
|---|---|---|
| 设计质量(它做得好不好) | /10 | |
| 省力程度(比我自己干省了多少脑子) | /10 | |
| **爽感**(有没有"卧槽这样也行") | /10 | |
| 验证成本(检查它的结果累不累) | /10 | 越高=越便宜 |

## 5. 抽一条教训 → 回填 PLAYBOOK

这次学到:在 L3 档,【去兼容层】能交出去,前提是把「生成入口、审计规则、母本槽位、线上软链说明」一起砍掉;只改 README 这种门面不够。

→ 已写入 `PLAYBOOK.md`?[x]
