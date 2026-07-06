# sop-audit Mainline Coverage Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the approved `$sop-audit` mainline coverage gate so audits cannot miss recent collaboration and evidence-flow rules.

**Architecture:** Keep `STANDARD.md` unchanged. Add one thin mandatory report checkpoint to `skills/sop-audit/SKILL.md`, then record the dogfood experiment and PLAYBOOK lesson. Verify by running a read-only pressure test against `/Users/sunchongsheng/code/media-ops`.

**Tech Stack:** Markdown skill docs, local git, `rg`, read-only shell inspection, optional `gh` for target project live context.

---

## Files

- Modify: `skills/sop-audit/SKILL.md`
- Create: `experiments/2026-07-06-030-sop-audit-mainline-coverage-gate.md`
- Modify: `PLAYBOOK.md`
- Reference: `docs/superpowers/specs/2026-07-06-sop-audit-mainline-coverage-gate-design.md`

## Task 1: Patch `$sop-audit` Flow

- [x] **Step 1: Insert the mainline coverage gate between template comparison and findings**

Edit `skills/sop-audit/SKILL.md` so the flow has a new step after template-version comparison and before severity findings. The step must require a 6-row table:

```md
## 主线覆盖闸

| 主线项 | 状态 | 证据 |
|---|---|---|
| Refs / Closes 收口 | 覆盖 / 缺失 / 偏离 | 母本 file:line -> 项目 file:line / 缺落点 |
| doc / issue / PR 三件套 | 覆盖 / 缺失 / 偏离 | 母本 file:line -> 项目 file:line / 缺落点 |
| issue 评论分层 | 覆盖 / 缺失 / 偏离 | 母本 file:line -> 项目 file:line / 缺落点 |
| 查证闭环 | 覆盖 / 缺失 / 过重 | STANDARD/SKILL file:line -> 项目 file:line / 缺落点 |
| 高风险治理项 | 覆盖 / 缺失 / 偏离 | 母本 file:line -> 项目 file:line / 缺落点 |
| 结构触发 | 匹配 / 缺失 / 预建 / 偏离 | STANDARD file:line -> 项目结构证据 |
```

- [x] **Step 2: Add the coverage judgment口径**

In the same step, require:

- no `覆盖` without an authority anchor from `STANDARD.md`, `master/`, or `skills/sop-audit/SKILL.md`;
- target-side keyword matches are not enough;
- abnormal rows become normal findings;
- local deviation must be judged in one sentence;
- the table stays fixed at 6 rows.

- [x] **Step 3: Keep the skill thin**

Do not copy the full design spec into the skill. Add only the gate, the minimum判定口径, and the relation to existing findings.

## Task 2: Record Experiment and PLAYBOOK Lesson

- [x] **Step 1: Create exp-030**

Create `experiments/2026-07-06-030-sop-audit-mainline-coverage-gate.md` from the experiment template. It must record:

- true task: `media-ops` audit missed recent mainline rules;
- cheap validation: read-only rerun against `/Users/sunchongsheng/code/media-ops`;
- review result: first spec-eye found 4 issues, second verdict `Ready`;
- expected verification: 6-row gate catches Refs/Closes, triad, comment layering, high-risk governance, and structure trigger.

- [x] **Step 2: Add PLAYBOOK entry**

Append one entry to `PLAYBOOK.md`:

- lesson: audit needs a thin mandatory coverage gate, not more prose;
- guardrail: fixed 6 rows, `authority anchor -> project landing/gap`, abnormal rows only expand into findings;
- evidence: `exp-030`.

## Task 3: Verify

- [x] **Step 1: Static checks**

Run:

```bash
git diff --check
rg -n "主线覆盖闸|权威锚点|Refs / Closes|issue 评论分层|结构触发" skills/sop-audit/SKILL.md
```

Expected: `git diff --check` exits 0; `rg` finds the new gate and all six row labels.

- [x] **Step 2: media-ops read-only pressure test**

In `/Users/sunchongsheng/code/media-ops`, collect:

```bash
git rev-parse HEAD
git status --short --branch
rg -n "Closes|Refs|三件套|issue 评论|高风险|AGENTS|contracts|待业务确认|查证" AGENTS.md docs .github
find . -maxdepth 3 -name AGENTS.md -print
```

Expected: enough evidence to fill the 6-row gate, including the previously missed Refs/Closes, triad, and comment layering rows.

- [x] **Step 3: Fresh review**

Ask an independent reviewer to inspect the implementation diff and the media-ops pressure-test result. Blocking issues must be fixed before commit.

- [x] **Step 4: Commit**

Use `$commit-msg` before committing. Expected commit type: `docs`.
