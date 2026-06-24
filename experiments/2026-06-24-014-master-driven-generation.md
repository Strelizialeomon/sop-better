# exp-014 · 把 /sop-init 改"母本驱动"——验证砍成"重排+骨架",真跑逮出静态照不出的洞

- **日期**:2026-06-24
- **真实任务**:turn-1 病根=`/sop-init` 生成的 CLAUDE.md 把「起手新鲜度」埋了(py-script 实测)。本次造一块:把 `/sop-init` 从"抽象 `templates/` 碎片拼装"改成"从具体母本 `master/` 按触发分层 copy"。
- **本次撒手档位**:**高**——owner 撒手让 agent 全程自驱(brainstorm → spec → 2 轮新眼睛自检 → 实现 → 4 镜头 review → dogfood 真跑),自己用工作流派子代理。owner 只在 **2 个 scope 岔路 informed override**,不碰 how。

---

## 1. 选活 + 定档

爆炸半径**大**:改的是线上 `/sop-init` + `/sop-audit`(软链即线上,影响所有项目)。原方案"以 media-ops 为纲、3 源合成母本"——agent 自己两轮就觉得偏大。

## 2. 便宜验证方案

不逐行读就看出对没的三道闸(plan 越轻 → 验收越硬 · §1.3):
- **spec 自检**:2 个 clean-context 新眼睛 verify-by-running,撞 overclaim。
- **实现 review**:4 镜头独立新眼睛(承重墙保真 text-equivalence / 挂对闸 exp-012 / bug 合规 / 右尺寸 exp-005)。
- **dogfood 真跑**:scratch 单人单端(远端+读生产 画像)真跑 `/sop-init` 的 copy+slot+prune,grep 验 freshness 戴皇冠 + 无残留槽 + 无档位 + 受众闸/承重墙行/6 反驳闸/3 说人话闸在 + 无别项目内脏。

## 3. 给 AI 的简报

= 收敛的 design spec(`docs/superpowers/specs/2026-06-24-母本驱动生成-design.md`)§11 分期 + §12 五条可检验验收。不另写 writing-plans(§1.3)。

## 4. 跑完,核结果

- **做对了**:`templates/` 8 文件搬进 `master/` 4 触发层(verbatim,git mv 保历史),`templates/` 整个退休=唯一真相源(§1.6);`base/CLAUDE.md` 骨架把 freshness 从"情景按需读"升到 §⛳ 第一节。dogfood 真跑生成物:freshness 戴皇冠、prod 风险串进"为什么排第一"、槽全填对、无档位/无别项目内脏、57 行右尺寸。

- **🟡 最大发现(scope · verify-by-running 砍大改)**:2 轮新眼睛实跑现状,逮出原方案 over-built——**`templates/` 早已是"当前母本"**(exp-011/012/013 修复都在、整份具体),**`/sop-init` 早已 copy-whole+slot**(无"现拼碎片"运行时步),**audit 双向 diff 早 ship**(exp-013)。"母本驱动大改"= **exp-005 自病(为小缺口立大概念)**。诚实残值 = 仅 1 件硬价值:base 骨架 crown freshness(authoring 改)。owner informed override 仍要 trigger-layer 目录(长期可维护性 taste),但右尺寸认清是"**重排+骨架**"级、非"3 源合成"级。**反驳协议 D**:agent 两轮荐小修、owner 两次 override,异议记进 spec §14/§15。

- **🟡 新眼睛逮到(实现)**:`multiend-constraints-block`「追加进根 CLAUDE.md §2 中段」**违反 spec 自己立的"绝不在 doc 中段动刀"不变量**——而那条不变量正是 freshness 不复埋的命根。改成 `{{multiend_constraints}}` 槽(单端整行删、多端填),回到不变量内。挂对闸(exp-012:三个真实项目类逐一 trace 分流对)、承重墙保真(text-equivalence、降级版=丢)全过。

- **🔴 真跑逮到(静态全绿、真跑才现形)**:4 镜头新眼睛 + grep 全绿,**dogfood 真跑 `/sop-init` 才逮出** master 文件顶部 `<!-- …占位符… -->` 元注释没被 strip → `{{proj}}` 连头注一起被填成乱码。SKILL 补「每个 copy 出来的文件先删顶部元注释、再填槽」。

### 评分(10 分制)

| 维度 | 分 | 备注 |
|---|---|---|
| 设计质量 | 8 | 右尺寸对了(砍掉 over-build);trigger-layer 分层清 |
| 省力程度 | 9 | agent 全程自驱、工作流并行自检,owner 只拍 2 次 scope |
| 爽感 | 9 | verify-by-running 当场砍掉一场 exp-005 自病;真跑逮出静态照不出的 bug |
| 验证成本 | 8 | 2 轮 spec 自检 + 4 镜头 review + dogfood,贵但值(改线上工具) |

## 5. 抽教训 → 回填 PLAYBOOK

- **教训 1(scope)**:想"以真项目为纲"改生成器前,**先 verify-by-running 现状**——现状可能已经就是你要造的东西(`templates/` 已是母本 / `/sop-init` 已 copy-whole),大改会塌成 exp-005 自病。✅ 已写入 PLAYBOOK。
- **教训 2(搬迁不变量)**:重排把护栏搬家时,**把"在 doc 中段插入"的机制改成"槽替换"**,守住"不中段动刀"不变量——否则载重位置(如 freshness crown)的埋点会复发。✅ 已写入 PLAYBOOK。
- **教训 3(真跑)**:**真跑 > dry-run 再次应验**(exp-002/013 同形)——4 镜头新眼睛 + grep 全绿,真跑 `/sop-init` 才逮出 meta-header 没 strip。生成器类改动必须真跑一遍生成。✅ 已写入 PLAYBOOK。
- **复验待补(诚实)**:dogfood **B**(taoxi-geo 增量跑 `/sop-init` 验三触发归位)+ **C**(py-script 跑 `/sop-audit` 验机械逮 stale)跨项目未跑;A(单端 crown freshness)已过。
