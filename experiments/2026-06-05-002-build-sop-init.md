# exp-002 · 用重瞄工作流造 /sop-init

- **日期**:2026-06-05
- **真实任务**:实现 sop-better 的第一个命令 `/sop-init`(技能 + 三档模板)
- **本次撒手档位**:**L3**(owner 给目标 + 验收;AI 拆解 + 设计 + 实现 + 定义"做完";owner code review 抽检)
- **默认是 L0**,本次挑到 L3。

---

## 1. 选活 + 定档

- 选这个:STANDARD(keystone)已 owner approve,该把它变成能跑的工具。
- 爆炸半径:低。产物是技能 md + 模板 md,可改可丢;真正风险在"右尺寸对不对",靠 §2 验证兜。

## 2. 便宜验证方案(动手前定)

> 怎么不逐行读就判 /sop-init 对不对?

- **试金石 = 拿 taoxi-geo(真 T2 项目)做 dry-run**:按它真实形态(ends=admin/backend/frontend/crawler,多 agent,部分线上)走一遍 /sop-init,看生成的 `CLAUDE.md` + 目录树:
  1. 是否右尺寸(T2 该有的角色/契约/collaboration 有,没多塞);
  2. 「Agent 工作约束」块是否含四铁律 + 顶嘴;
  3. 对比 taoxi-geo 现有 SOP,**能不能反过来指出它哪里过度治理**(这才是真价值)。
- owner 只扫 CLAUDE.md + tree 即可判,不逐行读模板。

## 3. 给 AI 的简报(owner 实际给的)

```
目标:做 /sop-init,三档都做;固定公约 + 按项目参数化(别死模板)。
约束:右尺寸,不过度;客观顶嘴写进去;遇到问题再迭代。
验收:见 STANDARD §6 + 上面 §2 的 taoxi-geo 试金石。
```

## 4. AI 跑完,owner 来评

- **AI 做的决策**:技能读 STANDARD 当单一真相源(不重复矩阵);参数化(tier/ends/collaborators/risk);三档模板分文件(agent-constraints / adr / collaboration / contracts);把"右尺寸校验 + 顶嘴"做成流程第 3 步;worktree 标为"仅真冲突才上"防过度。
- **超出预期 / 想不到的地方**(待 owner 填):
  > （你来写）
- **翻车 / 要纠偏**(待 owner 填,大概率在 taoxi-geo 验证时冒出来):
  > （你来写）

### 评分(待 owner 打)

| 维度 | 分 | 备注 |
|---|---|---|
| 设计质量 | /10 | |
| 省力 | /10 | |
| 爽感 | /10 | |
| 验证成本(越高=越便宜) | /10 | |

## 5. 教训 → PLAYBOOK(待 taoxi-geo 验证后定稿)

待填:L3 把"按既定 STANDARD 实现工具"交出去,行不行?前提是什么?

→ 写入 PLAYBOOK?[ ]
