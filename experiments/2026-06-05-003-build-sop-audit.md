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

- **taoxi-geo**(已知过度治理:820 行 collaboration + 8 个 doc 子系统 / PoC)→ **必须**喊出 P1 过度治理。喊不出 = 没用。
- **geo-reverse**(刚被 sop-init 右尺寸过)→ **必须**基本干净、不 cry wolf。
- **校准成立 = taoxi-geo 喊得响 + geo-reverse 放得过。** owner 扫报告 top 几条即可判,不逐行读。

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

## 5. 验证结果 + 教训(待 taoxi-geo / geo-reverse 试金石跑完)

→ 写入 PLAYBOOK?[ ]
