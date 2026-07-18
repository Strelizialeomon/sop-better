# exp-042 · 去耦 superpowers:换工具时 de-name 到概念,别 name-swap

- **日期**:2026-07-19
- **触发**:owner 卸载 superpowers、日常改用手动触发的 `/grill-me`。本仓生きた文档仍残留 superpowers 两个技能名——`master/` 生成的 `CLAUDE.md` 甚至叫下游"用 `brainstorming` skill / 跳 writing-plans",指向已卸载技能的死引用(sop-audit §5.2「删除残留」)。
- **决策**:去耦到工具无关,而不是把 superpowers 的名字换成 grill-me。

---

## 做法(直接复用 exp-019 的"去兼容层扫全链路"清单——方向无关,又一次当检查表)

- **入口**:README 两处引用 + 目录树。
- **生成器**:`STANDARD.md §1.2`、`skills/sop-init`、`master/`(base/CLAUDE.md + 4 个 layer)——`writing-plans`→「重型实施计划」、`brainstorming`→「定方案 / spec 收口」。
- **审计**:无需动——STANDARD §5.2「删除残留」本就覆盖旧技能名 / 旧模板名。
- **目录**:`docs/superpowers/` → `docs/specs/`(17 spec `git mv`,blame 保留)+ 清空残留 `docs/superpowers/plans/`。

净 16 增 16 删,纯换词 + 改路径,承重墙原样。

## 关键判断:de-name,不是 name-swap(本次新纹路,先记不硬化)

owner 第一反应是"superpowers 换成 grill-me,你没体现出来,万一 agent 不知道调谁"。但正确动作不是换名,是**去名到概念**,三条理由叠加:

1. **下游未必装** grill-me——写进生成的 SOP 就是新的死引用(和 superpowers 一个坑)。
2. **重犯「指技能名→快照会过时」**(STANDARD §2 已立、exp-041 刚踩过)。
3. **grill-me 是 user-invoked**:SKILL.md `disable-model-invocation: true`(+ openai.yaml `allow_implicit_invocation: false`),agent 压根调不了——它是 owner 敲 `/grill-me` 去拷问 agent 的方案,不是 agent 的一个可调步骤。写进 agent SOP 是范畴错误。

→ owner 拍板:grill-me 维持纯手动,SOP 一个字不提。**"user-invoked 替代品不能进 agent SOP"是首次出现,按 §3.5「一次踩坑先留记录,复发成模式再改规矩」——只记 exp,不新增 PLAYBOOK / STANDARD 条目**(已有 §2「不指技能名」+ §5.2「删除残留」够用,再加=过度治理,撞本仓头号敌人)。

## 历史不改(凭据保真)

`experiments/` + 2 个 spec 里指向老路径 `docs/superpowers/specs/...` 的引用:目标文件确实迁到了 `docs/specs/` 的 **4 处更新**到新路径;指向"从未入仓 / 已删"文档的 **4 处保留**原样(exp-004 那句原文就是"req doc 在远端不存在"、exp-032 是"删除 plans 残留")——改了会假装文件搬过去,制造会说谎的凭据。

## 未完成 / 风险

- **静态自审**:同 exp-040 / 041,未真跑。改动纯 doc 层,未在真项目复跑 `$sop-init` 验生成物、`$sop-audit` 验残留清零。owner 可补 dogfood。
