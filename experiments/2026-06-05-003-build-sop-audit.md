# exp-003 · 做 /sop-audit(只读 SOP 体检)

- **日期**:2026-06-05
- **真实任务**:实现 sop-better 第二个命令 `/sop-audit`(只读,头号查过度治理)
- **本次撒手档位**:**L3**(owner 给目标 + 验收;AI 设计 + 实现;owner review)

---

## 1. 选活 + 定档

- 接 exp-002:`/sop-init` 已验证可用,该做"反向"的审查命令了。
- 爆炸半径:低。只读技能,不改任何文件。

## 2. 便宜验证方案(动手前定)

> 怎么不逐行读就判 /sop-audit 对不对?**两个已知答案的项目当试金石,一喊一放:**

- **taoxi-geo**(实测 `S2·C2`:4 端 + 多 agent)→ ⚠️**纠偏**:两轴模型下它的 contracts/collaboration/worktree **大多正当**,审查**不该**粗暴喊"过度治理砍掉"(那才是 cry wolf)。正确表现 = **精细**:把 820 行 collaboration 当**冗长信号**(行数≠罪证)、并质疑"多 agent C2 是真并行还是单人摆架子",都带证据。
- **geo-reverse**(刚被 sop-init 右尺寸过 `S1·C1`)→ **应**基本干净、不 cry wolf。
- **校准成立 = 精细不粗暴**:对 taoxi-geo 拿证据问冗长/问 C2 真假、不乱砍正当结构;对 geo-reverse 放得过。
- **自省**:原始判据"taoxi-geo 必须喊 P1 过度治理"是错的——拿它跟单人模板比不公平(它本就该重)。这条纠偏本身就是 sop-audit 该有的「行数≠罪证」精神。

## 3. 给 AI 的简报(owner 实际给的)

```
目标:做 /sop-audit,只读,头号查过度治理,对照 STANDARD,出双轨报告。
约束:每条带证据(file:line/行数);行数≠罪证只当信号;不 cry wolf;密钥另走 track。
验收:见 §2 试金石。
```

## 4. AI 跑完,owner 来评

- **AI 做的决策**:只读硬约束;测实际 (S,C,风险) 的扫法(.github / .claude/commands / collaboration / worktree);"该有 vs 实际"相减;severity P0(仅指针)/P1过度/P2错配·顶嘴缺/P3结构缺;双轨输出(人读 + JSON findings 给 /sop-improve 接)。
- **超出预期 / 想不到**(待 owner 填):
  > （你来写）
- **翻车 / 要纠偏**(待 owner 填,大概率在试金石上冒出来):
  > （你来写）

### 评分(待 owner 打)

| 维度 | 分 | 备注 |
|---|---|---|
| 设计质量 | /10 | |
| 省力 | /10 | |
| 爽感 | /10 | |
| 验证成本(越高=越便宜) | /10 | |

## 5. 验证结果 + 教训

**试金石 1 · taoxi-geo(S2·C2)· owner 真跑 → PR #179 · 结论:过(精细不粗暴)**

- ✅ **没粗暴**:4 条 finding 无一是"砍 contracts/worktree",正确认 taoxi-geo 为正当 S2·C2。
- ✅ **冗长当信号**:820 行 collaboration 没判罪,精准点"局部补丁口诀重复 4 遍、可瘦 100-150 行",且因 #176 改同节而**主动 deferred**(判断力成熟)。
- ✅ **找到真问题**:顶嘴协议缺失(predates sop-better)、**principles.md 假凭据**(旧 subagent 模型 / 错的契约数 / 漏 crawler / commit 规则冲突)、plans 未标停用。
- ⭐ **§1.8 凭据保真第一次上真项目就逮到真烂凭据**(principles.md)——round 1 的重定义立刻见效。
- ✅ **懂单一真相源**:补顶嘴只落 collaboration.md §10 一处,明说"防 drift",把发现的教训用在自己修复上。
- ⚠️ **越界**:sop-audit 我写的只读,这次链进 improve + 开 PR(owner 拍"先动"授权)→ audit→improve→PR loop 跑通,但 /audit /improve 界限糊了 → 待定 UX。
- ⚠️ 判据来自 PR 正文,未逐行核 diff。

**试金石 2 · geo-reverse(应放得过、不 cry wolf)→ 待跑。**

→ 写入 PLAYBOOK?[x]
