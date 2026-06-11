# exp-009 · escalate 是自决动作:跨端扩范围变更被做成 owner 选择题

- **日期**:2026-06-11
- **真实任务**:taoxi-geo admin 端 crawl 进度页(#206)。owner 在终端对 admin scope agent 说"进度应该可以展示爬到了哪些文章才对"——一个**冻结范围外 + 跨端**的新需求(req doc 明确把"文章级进度"砍到本期外、且要 backend/crawler 改 crawl-control 进度协议)。
- **本次撒手档位**:N/A —— 不是主动撒手实验,是**真用 SOP 时撞出的洞**(同 exp-002/004/007/008:真跑 > dry-run)。

---

## 1. 现象

- admin agent **方向判对了**:认出这需求"不能自己悄悄吞、要回 coordination 改 req doc + 拉 backend/crawler",有规则撑腰(§1.9「越界动他人 scope / 改跨端骨架 → 停下」+ collaboration-c2「要改 req doc/契约 走 issue 评论提」)。
- 但它把 **escalate 这个动作本身**也做成了 owner 选择题:列"选项 1 现在并进 / 选项 2 fast-follow……你选哪个",**逼 owner 回一句"1,你在 issue 留言并附设计,我让 coord 去读"才动手**。
- owner 反馈:下次这种情况**它自己就该把评论写了,或告知我一声它去写**——不用我来教它"去写评论"这步。

## 2. 根因:§1.9「不假民主」撞上 §1.1「owner 定要什么」,边界没划清

- "把变更上交 coord(写 issue 评论 + 附设计)"这个**动作**,按 SOP 本就是自决该做的(§1.9 不假民主 + collaboration-c2 scope隔离「要改走 issue 评论提」)。规则**已覆盖**,本不该请示。
- 但相邻的 §1.1(人定"要什么" + 否决权)+ §1.10(照亮不代选)把整件事"捕获"成了产品决策 → agent 把**真产品决策(做不做 / 优先级,归 owner)** 和**机械上交动作(归 agent)** 打包成一个 owner pick。**最强规则指向"让 owner 拍"——exp-004/007/008 同形:执行者忠实,不是违反。**
- 缺的是一句**拆包**:跨端/扩范围变更里,"做不做"是 owner 的,"怎么正确送到 coord 手里"是 agent 的;别把后者也塞进选择题。

## 3. 关键判断

### 为什么"一次"就改(顶 §3.5「复发 2+ 次才改规矩」,过两关)

- **设计缺陷**(非调参数据点):§1.9 不假民主 本应覆盖,但与 §1.1 的边界未划线 → agent 可证地把"机械上交动作"误并进"产品决策"做成 owner pick。不靠 N 成立。
- **代价不对称**:修 = 规则文件净增 ~4 行;复发 = **owner 得手把手驱动每一次跨端 escalate**——直接吃掉撒手杠杆(本工具的产品目标本身),不是一次性返工。
- **诚实标注(比 exp-004 弱的地方)**:这次 agent 方向判对了、只是**多问了一步**,不像 exp-004 真伪造了需求。所以"设计缺陷"更轻;升级靠的是**"多问一步直接defeat 撒手杠杆"+ 修复极便宜**,不是事故严重度。

### 拆包的两半(写死,防误读)

- **归 owner**:做不做 / 优先级 / 取舍(产品决策 · §1.1 人核)。
- **归 agent 自决**:把变更正确上交对的人(C2=coord / C1=业务方)——写 issue 评论 + 附设计草案 + 报一句。**owner 当面交办的扩范围/跨端需求同样走这条**:别因"老板亲口说"就吞进自己的活。

### 为什么不进 always-loaded 内核(守瘦内核 · exp-006)

- 这条只在"撞上跨端/扩范围变更"时才触发 = 情景规则,不是怼脸的天天约束。
- always-loaded 标准块(`agent-constraints.md:23`)已写"跨端契约…… → 到那场景再读 collaboration.md" → agent 进到这场景自然读到 collaboration-c2 的落地版。**靠现成的指针路由,不新增内核行。**

### 为什么不动 sop-audit

- 这是**运行时行为**的洞(escalate 被做成选择题),不是静态凭据 artifact 的洞——audit 难从 committed doc 看出"agent 多问了一步"。
- 传播靠现成机制:项目的 collaboration doc 若缺这条 carve-out = `/sop-audit` §3「模板版本差」(exp-007)自动逮的漂移 → 回灌。不新增 audit 查项(省得 §1.6 重抄)。

## 4. 修复(本次 · sop-better 母本)

- **`STANDARD.md §1.9`**:不假民主下加「🔀 escalate 是自决动作、不是选择题」carve-out——拆"做不做(owner)/ 怎么上交(agent 自决)";含"owner 当面交办 ≠ 授权吞"一句。真相源,端-agnostic(C2=coord / C1=业务方)。
- **`templates/collaboration-c2.md` scope隔离**:加一句 C2 落地——识别到要改 req doc/跨端 → 自己写评论上交 coord + 附设计 + 报一句,别做成选择题(指针引 §1.9,不复述论证)。
- **不动**:always-loaded 内核(靠现成指针路由 · exp-006 瘦内核)、sop-audit(运行时洞,传播靠模板版本差)、agent-constraints 标准块(已有跨端指针)。
- **过 exp-005 自审**:无新章节 / 新品牌概念(carve-out 并进 §1.9);collaboration-c2 指针不重抄;规则文件净增 ≤10 行(实测见 PR · 目标 ~4)。
- **新眼睛补回**(§1.3 · 同 exp-006 形态):独立审稿逮到 carve-out 有两处会被断章取义,补两条边界——① "附设计草案"焊上角色边界 carve-out(草案只到 how,别替业务伪造 req doc);② "上交 ≠ 获批施工"(递出后做不做/并进哪期等 owner+coord 回,别自决扩张成吞 owner 否决权)。

## 5. 抽一条教训 → 回填 PLAYBOOK

跨端/扩范围变更里,**"做不做 / 优先级"是 owner 的决策,但"把它正确上交 coord(写 issue 评论 + 附设计)"是 agent 自决该自己做的动作**——识别到就自己写、报一句,别把 escalate 也做成 owner 选择题(= 假民主真 ask + 逼 owner 手把手、吃撒手杠杆)。owner 当面交办的扩范围/跨端需求同样走这条:别因"老板亲口说"就吞进自己的活。

→ 已写入 `PLAYBOOK.md`?[x]

---

## 6. 复验(诚实标注 · 待补)

- **现状**:仅 taoxi-geo #206 这 1 例 + 规则可证指错(§1.9/§1.1 边界未划线,设计缺陷不靠 N)。
- **未做**:①跨项目复验——回灌时看 media-ops / geo-reverse 撞到跨端/扩范围变更时,agent 是否同样把 escalate 做成 owner pick,凑 2/2 才从"设计缺陷推断"升到"实测系统性缺口";②C1 落点未补——本次只落 collaboration-c2(C2),C1 的 `templates/collaboration.md` 同形场景(业务↔开发、escalate 回业务方改 req doc)待回灌时一并看要不要加。
- **回灌待办**:taoxi-geo collaboration / 端文档按需补这条 carve-out(它就是出洞的项目);media-ops / geo-reverse 跑 audit 时顺带核(回灌纪律 · exp-007 续:模板学会 ≠ 项目学会)。
