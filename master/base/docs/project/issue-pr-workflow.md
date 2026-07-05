<!-- master/base/docs/project/issue-pr-workflow.md —— 所有项目(人人走 issue+PR,含单人)。agent 执行的 issue + PR 标准工作流。
     核心:issue/PR 是 agent 之间的共享内存 / 凭据,不是给人的 PM 表。agent 全程操作,人可读可不读,只在高风险闸出现。
     默认 GitHub 实现(gh CLI / GitHub PR);工作流是抽象,换平台时只换实现。
     $sop-init 把本文件落成 docs/ 下的工作流约定,并在 collaboration.md 引用。 -->

# Issue + PR 工作流(agent 执行 · 人可读可不读)

> issue/PR = agent 的共享内存与「**凭据**」:agent 开、标、流转、关、提 PR、按风险审合,**全自动**。
> 人只在**高风险闸**出现。为 agent 解析+操作而设计,但人想看随时看得懂。

## 三件套分工(协作主线)

- **doc = 正文 / 真相源**:需求、设计、契约、长期决策写在 doc / ADR 里,别塞进 issue 长文。
- **issue = 索引 + 状态 + 消息总线**:写现在做什么、等谁、指向哪个 doc / PR;方案 / 字段 / 范围变化回写 issue 评论,别只藏在 PR / commit。
- **PR = 交付凭据 + 验收 / 收口动作**:写改了什么、怎么验、风险是什么;只有满足验收 / 关闭条件才 `Closes #N`,否则 `Refs #N`。

**协作 flow**:发起方 / 上游角色(业务 / coord / 开发自起)先落 doc → 开 issue 指稳定 doc → 开发接 issue 先验 doc → 开发过程用 issue 评论同步变化 → PR 交付并写验证 → 验收 / 关闭条件满足才关 issue。

## 🧱 铁律:凭据保真(命门)

- 状态必须**真反映进展**——label / 状态标记 / 摘要不许跟实际不符。
- **不许烂尾**:开着没人管的 issue 是污染。做完就关;搁置标 ⏸️ 并写一句为什么。
- 一份会说谎的凭据**比没有更危险**(agent 会笃定照错的行动)。**合并 / 关闭前,agent 自检"写的 = 实际发生的"**。

## Issue 生命周期(agent 操作)

1. **开**:一个需求/缺陷一个 issue。**先把 doc commit+push 到远端、再开 issue**;正文 = 一句话需求 + 验收标准 + **指向 doc 的稳定链接(commit permalink / 已合 PR,别指会悬空的分支相对路径)**。**issue 薄、doc 厚**:细节进 doc,issue 只留索引 + 状态 + 验收。**"提 issue"和"推 doc"不许分两步漂着——doc 不在远端就别开 issue。**
2. **标**:阶段 label(互斥,一个 issue 同时只一个:评估中 / 开发中 / 已完成)+ **阻塞 flag**(叠加,如 待澄清 / `待业务确认`;`待业务确认` = 等业务方 / owner 拍板,开工要 surface 给人)+ 状态标记 ✅🚧⏸️⬜。**这是基线(media-ops 同款)**。(重协调的多端多 agent 项目若觉 lifecycle label 冗余,可在 `collaboration.md` 约定改用 open/closed + 状态标记 + 评论的更轻策略——非默认、不是档位。)
3. **跟进**:进度、决策、方案变化随时写进 issue 评论(留痕即凭据);要别的角色/agent 接手就 **tag**。
4. **关**:验收过 / 关闭条件满足 → 关。被 PR 收口的用 `Closes #N`;非收口 PR 只用 `Refs #N`,issue 保持 open。
   - **🚧 决策闸 carve-out(裁决 ≠ 收口)**:若 issue 是「裁决只是选了条路、这条路还要等下游 spike/数据验证才算数」的**决策闸**(典型:私信走 web 还是 App,取决于一个 spike 能不能跑通)——**别在裁决时关**。正文必带一行**关闭条件**(什么证据才收口 + 跑不过转哪条路);并给人一句「**只回复决策、别关本 issue**——关闭由开发按关闭条件统一做」;裁决后 label 不进终态、挂到那条 spike 上,闸合上才关。否则裁决一关 = 选路被当收口 = 会说谎的凭据(§1.8)。纯裁决(拍完即定,如选型)不吃这条,照旧"裁决即关"。

## PR 生命周期(agent 操作)

1. **分支**:`feat/issue-N-<slug>`(一需求一分支)。
2. **提交信息**:commit 前必须用 `$commit-msg` skill 读当前 diff 生成/校验 commit message;不许直接手写 `git commit -m ...` 绕过。若 skill 无法调用,先明说原因再提交。
3. **提 PR**:正文 `Refs #N`(收口用 `Closes #N`)+ 改了什么 + 验收怎么过 + 链接 issue/doc。
4. **审 + 合(按风险 · 撒手档)**:
   - **低风险 / 可逆**(纯函数、UI、文档、测试)→ agent 自审(过 code review 公约)+ **自动合**,人不等。**文档(req/design doc)内容的审 = brainstorming 收口那一步,不在 PR 再审一遍 → 默认直接推 main;main 受保护才开 PR(仍自动合)。代码不吃这条豁免,照走 PR + 新眼睛 review。**
   - **高风险 / 不可逆**(生产库 schema、付费 API 全量、改远端、删数据)→ **回人审**才合;审是**独立一道**,不是 AI 自己盖章。
5. **合后**:删分支;`Closes #N` 自动关 issue;只用了 `Refs #N` 则不因合并而关,只有验收 / 关闭条件已满足时才手动关 + 自检凭据保真。

## 与撒手档挂钩

- 默认撒手档越高,自动合的范围越大;但**"高风险闸回人"恒定不变**。
- 哪些算"高风险"写进项目 agent 指令文件的「升回我主导」清单。

## 凭据细则(吸自 taoxi-geo)

- **issue 评论 vs PR 评论**:为半年后 / 别的 agent / merge 后还能查 → 写 **issue**(留痕·总线);针对这次 diff 的代码评审来回 → 写 **PR**。方案变更(换思路 / 改字段 / 调 schema)必回写一句到 issue,别只留 PR / commit。
- **决策快照 ≤30 行**:spec ready / 方案变更时,在 issue 评论留"核心决策快照"(列拓扑 / 字段 / 关键选择,不展开理由)——让 review 子代理别把故意决策当 bug 报,也让回溯能复现设计。
- **gh 就绪(默认 gh 实现 · 业务侧一次性前置)**:整套 issue/PR 凭据靠 `gh`。`gh auth status` 不过(没装 / 没登录)→ **用大白话**带业务装(mac `brew install gh` / win `winget install GitHub.CLI`)+ 引导他敲 `! gh auth login` 跟着提示走;**一次性,装完不再问**。逐步引导由 agent 现场说人话(别展开成版本化图文教程,会过时)。
- **起手 freshness(多会话 / 多 agent)**:新会话先 `git fetch && 看 behind master`,落后先 sync 再信本地 SOP / 代码——本地常是旧快照、或停在别的分支。**业务会话顺带 `gh issue list --label 待业务确认`:有就一条条念给 owner 当场拍、没有一句带过**(别让等人决策的 issue 烂着没人 surface)。
- **接 issue 先验链接(消费侧凭据校验)**:起手照 issue 工作前,**先验它指的 doc 在远端解析得开**(fetch + 按工作 ref 找路径)。解析不到 = **坏交接 / 会说谎的凭据**(issue 说"详见此 doc"、doc 却不在)→ **反弹回开 issue 的角色**(评论 tag + ⏸️ 待澄清),**别自己补 doc**(自补 = 伪造需求 · STANDARD §1.8 + §1.9)。
- **取活先判细化(消费侧 · 别抓起就写)**:开发取活后 agent 先帮判一轮——够清楚且小 → 直接干;不清楚 / 太大 → 先端内 brainstorm 拆;**缺的是业务该给的"要什么"(含产品形态)→ 反弹业务(§1.9 carve-out)、不自补**(反例 geo-reverse #2:开发凭"永不阻塞"自己把需求补了)。
- **收工 = 写凭据时点(owner 说"收工 / 结束")**:这就是红线「不擅自 push」要的那个**明确指令**——把本会话收口的 doc 按上面 Issue / PR 生命周期推远端(**文档推 main、代码走 PR**),开 / 更 issue 指稳定链接、自检保真。**不重抄步骤,照上面那两套生命周期走即可。**
