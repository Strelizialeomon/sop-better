# exp-041 · 反转 exp-019:从 Codex-only 翻回 Claude 规范

- **日期**:2026-07-19
- **触发**:owner 判定 `AGENTS.md` 是"Codex 势力残留",要转回 Claude 规范。owner 本人日常就在用 Claude Code,工具却是给 Codex 的,本身拧巴。
- **决策**:反转 exp-019(codex-only-drop-claude)。工具的 agent 指令文件从 `AGENTS.md` 改回 `CLAUDE.md`,harness 从 Codex 改回 Claude Code;旧 `AGENTS.md` / `.codex/` 反过来成为 `$sop-audit` 该报的"待迁移残留"。

---

## 做法(照 exp-019 自己的教训:去/换兼容层要砍全链路)

exp-019 的 PLAYBOOK 结晶——"入口 + 生成器 + 审计 + 母本槽位一起改,只改门面会继续生成旧世界"——**方向无关,这次反过来用正好是检查清单**:

- **入口**:root `AGENTS.md` → `CLAUDE.md`;README / 本仓 CLAUDE.md 定位改回 Claude Code。
- **生成器**:`STANDARD.md`(术语 + §2 参数注 + §4 母本落点)、`skills/sop-init`(生成 `CLAUDE.md`、迁移方向翻转)。
- **审计**:`skills/sop-audit` 把旧 `AGENTS.md` / `.codex/` 报成 `stale` 残留、迁到 `CLAUDE.md`;STANDARD §5.2 删除残留同步。
- **母本槽位**:`master/base/AGENTS.md` → `master/base/CLAUDE.md`;`master/layer-multiend/end-role-agent.md` → `end-role-claude.md`(生成 `<端>/CLAUDE.md`);SLOTS / 约束块 / coordination / end-role / issue-pr-workflow 里所有落点与"进端即定身份靠 Claude Code 自动加载 cwd 的 CLAUDE.md"全翻。

**历史不改**:`experiments/` + `PLAYBOOK.md` 里 exp-017/019 等 Codex 时代记录忠实保留(它们记的是当时发生了什么,不是当前规范)。

## 未完成 / 风险

- **软链未挪**:两个 skill 当前实际软链在 `~/.codex/skills/`,不在 `~/.claude/skills/`——文档翻了但**运行位置没翻**,Claude Code 会话加载不到。需 owner 定:link 进 `~/.claude/skills/`,以及是否撤掉旧 `~/.codex/skills/`(若还偶尔用 Codex 则留)。这步动 home 目录、在 PR 之外,未擅自做。
- **静态自审**:同 exp-040,未真跑。翻转后没在真项目上跑 `$sop-init` / `$sop-audit` 验产物落成 `CLAUDE.md`、audit 把残留 `AGENTS.md` 报得出。
