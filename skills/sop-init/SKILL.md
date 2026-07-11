---
name: sop-init
description: Use when 用户要为项目初始化、增量升级或预览 Codex 开发 SOP，或需要根据端数、真人协作者数、并行 agent 与风险判断项目应有结构。
---

# sop-init

把项目事实和 owner 的决定写进 profile；把生成、差异和机械检查交给同版本 `sopctl`。

## 权威来源

- 规则语义从本 `SKILL.md` 目录解析 `../../rules/STANDARD.md`。
- profile 字段从本 `SKILL.md` 目录解析 `../../rules/schemas/profile.schema.json`。
- 路径从本 skill 所在 plugin 相对解析，不读取开发 checkout。
- 找不到兼容的 `sopctl`、规则快照或 schema 时，说明安装问题并停止；不得改用手工拼模板。

## 流程

1. 只读检查 Git 根、现有 `.sop/profile.json`、默认分支候选、目录和技术栈。一次性脚本不值得建 SOP 时，直接建议不初始化。
2. 分开事实与判断：
   - 从项目确认名称、端目录和可验证的默认分支；不猜 `master`。
   - 一次问清无法可靠推断的真人协作者数、是否真并行多 agent、风险和 house style。
   - 三个结构触发互不代替：第 2 位真人触发 handoff；第 2 个端触发 contracts 与端级 `AGENTS.md`；多端且真并行才触发 worktree 与协调规则。
3. 按 schema 在内存中形成候选 profile。只用仓库相对路径，不放密钥和机器绝对路径；已有 profile 保留未变的 owner 决定。预览阶段不得先覆盖项目里的 `.sop/profile.json`。
4. 把候选 profile 写到系统临时文件并运行只读预览；流程停止或完成时删除临时文件：

   ```text
   sopctl diff --project-root <repo> --profile <temporary-profile.json>
   ```

   用人话解释候选文件、触发原因和冲突。用户只要预览时停在这里。
5. 只有用户看过**本次实际 diff** 后明确接受，才运行事务式生成；不能把最初一句“初始化 / 升级”当成对随后具体差异的确认：

   ```text
   sopctl render --project-root <repo> --profile <temporary-profile.json>
   sopctl check --project-root <repo>
   ```

   `render` 会把 profile、托管产物和 lock 放进同一事务；任一步失败都不留下半升级项目。失败时报告“什么没完成、什么没变、下一步怎么做”，不得绕过机械错误。
6. 收尾只列实际生成/更新的文件、为何当前尺寸够用、未生成哪些未触发层。

## 边界

- 不覆盖同名未托管文件，不覆盖被人工修改的托管块。
- 不预建不存在的端、真人角色或并行流程。
- 不生成或保留 `CLAUDE.md`；发现旧格式时先展示迁移差异。
- 升级工具版本走 `sopctl release ...`，升级项目内容走本流程；两者不能混成一次确认。
