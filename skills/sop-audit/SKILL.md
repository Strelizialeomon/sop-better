---
name: sop-audit
description: 只读审查一个项目的"开发 SOP"是否右尺寸——头号查过度治理(仪式吃掉 AI 杠杆),也查档位错配 / 顶嘴缺失 / 结构缺失。对照 sop-better/STANDARD.md,出"人读报告 + 可执行 findings"双轨,不改任何文件(改交 /sop-improve)。给老项目体检、怀疑 SOP 太重 / 不一致时用。
---

# sop-audit

只读给项目的**开发 SOP** 做体检。对照 STANDARD 挑"不合理",**头号挑"太重"**(过度治理)。**不改任何文件。**

**配套**:`SOP_HOME = /Users/sunchongsheng/code/sop-better/`;`STANDARD.md = $SOP_HOME/STANDARD.md`。**先读 STANDARD**,规则以它为准。

## 铁律

- **只读**,绝不改文件(改是 `/sop-improve` 的事)。
- **头号查"过度治理"——只算人掏的成本**:人手开 issue / 贴 label / 等每个 PR 才叫过度;**agent 自动维护、agent 消费的 issue/PR 不算**(那是撒手基础设施,冤枉它=cry wolf)。
- **每条 finding 必须带证据**(file:line / 行数 / 具体仪式名),空喊"太重了"=违规(顶嘴协议在审查上的落地)。
- **行数 ≠ 罪证**:体量只是**信号**,不直接判"错";说清"这是信号,该不该砍要看内容",别越权定罪。
- **不 cry wolf**:右尺寸的项目如实说"没大问题",别为显得有用硬挑。

## 流程

1. **读 `STANDARD.md`**(§3 两轴 + §5 查法 + §1 公约)。
2. **测目标项目的实际 (S, C, 风险)**:
   - `S` 端数:数后端/服务/子项目(单 backend=S1,多端前后端小程序爬虫=S2,纯脚本=S0)。
   - `C` 协作结构:扫 `.github/ISSUE_TEMPLATE`、`.claude/commands`(角色命令)、`docs/collaboration*`、worktree、scope label → 没有=C0,业务↔开发/小团队=C1,多端 scope agent=C2。
   - `风险`:碰生产库 / 付费 API 全量 / 改远端 = 高、不可逆。
3. **比"该有 vs 实际"**:STANDARD §3 由 (S,C) 推出"该有的结构";扫项目实际有的治理文件/仪式;两边相减。
4. **按 §5 四类出 finding,每条标 severity + 证据**:
   - **P1 过度治理(头号 · 只算人掏的成本)**:**人**手跑的仪式过重(人手开 issue / 贴 label / 等每个 PR、回溯 req doc、audit 补口、给单人建 worktree)。⚠️ **agent 自动维护的 issue/PR 不算过度**——是撒手基础设施。体量(collaboration 行数 / doc 子系统数)只当**信号**,问的是"人要不要为它掏成本"。
   - **P2 档位错配**:S 或 C 与实际不符(单端却建契约 / 单人却装 issue 状态机 / 反过来缺位)。
   - **P2 顶嘴缺失**:CLAUDE.md 无「顶嘴协议」或把 agent 设成顺从无异议 → 撒手不安全。
   - **P3 结构缺失**:该有的缺了(单一真相源 / ADR / 可检验验收 / 必跑 code review / 说人话)。
   - **P3 凭据失真**:issue/PR 烂尾(开着没人管)、状态与实际不符、issue 厚 doc 薄(细节堆 issue 难被 agent 复用)→ 凭据不可信(STANDARD §1.8)。
   - **P0 仅指针**:扫到硬编码密钥/凭据 → 只点一句"另走安全 track(STANDARD §7)",**不在本体检展开**。
5. **出双轨报告**:
   - **(a) 人读**:开头一句总判("太重 / 刚好 / 太轻" + 实测 S·C);然后按 severity 排,每条 = `现象 + 为什么不对(对照 STANDARD 哪条)+ 建议(降到哪 / 补什么)+ 证据`。
   - **(b) 可执行 findings**(给 `/sop-improve` 接):
     ```json
     [{"severity":"P1","kind":"over|mismatch|missing|nopushback","target":"file/dir","evidence":"...","suggest":"..."}]
     ```
6. **收尾**:一句"**最该先动的 1 条**"。

## 禁止

- 改任何文件(只读)。
- 无证据的 finding(凭印象喊"太重")——违反顶嘴。
- cry wolf:右尺寸项目硬挑毛病凑数。
- 把"体量大"直接等同于"错"——只能当信号。
