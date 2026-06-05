<!-- templates/adr-template.md —— 复制为 docs/decisions/NNNN-<slug>.md。NNNN 四位递增。 -->

# ADR-{{NNNN}}: {{决策标题}}

- **日期**:{{YYYY-MM-DD}}
- **状态**:提议 / 已采纳 / 已废弃 / 被 ADR-NNNN 取代

## 背景
为什么要做这个决策?当时的约束、痛点。

## 候选
- A:……(优点 / 代价)
- B:……(优点 / 代价)

## 决策
选了哪个,一句话。

## 影响
带来什么、放弃什么、后续要注意什么。

---

<!-- /sop-init 同时生成的 0001 样例(让 owner 一看就懂格式): -->
<!--
# ADR-0001: 本项目直推 master,不开分支
- 日期:{{today}}
- 状态:已采纳
## 背景
单人项目,开 PR/分支是过度治理,吃迭代速度。
## 候选
- A 直推 master + /commit-msg 收口
- B feature 分支 + PR
## 决策
A。右尺寸,符合 STANDARD「宁可不足不要过度」。
## 影响
省流程;代价是无 PR 评审,故 code review 改为本地必跑(见 CLAUDE.md 约束)。
-->
