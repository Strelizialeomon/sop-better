# sop-better

`sop-better` 给 Codex 项目生成和审计开发 SOP。它的重点不是堆更多提示词，而是把规则、模板、程序、对话和安装拆开，让同一份项目配置能被重复生成、检查和回滚。

> 规则的唯一真相源是 [`STANDARD.md`](STANDARD.md)。README 只讲入口、命令和当前边界，不重抄规则正文。

## 当前状态

仓库正在做 phase 1（机械稳定性改造），目标是先让多台自有 macOS / Windows 机器稳定使用，再讨论公开分发。

- **稳定版**：从校验过的固定版本包和 Codex plugin 安装缓存运行，不跟随 Git 分支变化。
- **开发版**：在独立 worktree、独立 `CODEX_HOME` 和本地 marketplace 中测试。
- 仓库 checkout 只是源码，不再是稳定版运行入口。若某台机器仍有 `~/.codex/skills/sop-*` 指向工作树，那是待迁移的旧安装，不能当成新架构已经上线。

完整方案和迁移顺序见 [`phase-1 稳定性设计`](docs/superpowers/specs/2026-07-10-sop-better-stability-overhaul-design.md)。

## 五层架构

| 层 | 负责什么 | 主要入口 |
|---|---|---|
| 规则层 | 定义原则、触发条件和安全边界 | [`STANDARD.md`](STANDARD.md) |
| 生成契约层 | 把规则变成可重复渲染的组件和机器契约 | [`manifest.json`](manifest.json)、[`master/`](master/) |
| 执行层 | 生成、比较、检查、事务写入和回滚 | `sopctl` 的 bootstrap、manager、engine |
| 对话层 | 观察项目、向 owner 确认判断、解释结果 | [`$sop-init`](skills/sop-init/SKILL.md)、[`$sop-audit`](skills/sop-audit/SKILL.md) |
| 分发层 | 固定版本、校验、安装、升级和回退 | [`plugin/`](plugin/)、release bundle |

简单说：`STANDARD.md` 说“应该怎样”，manifest 和 master 说“要生成什么”，`sopctl` 保证“每次都按同一种方式做”，skills 负责和人沟通，plugin 负责把同一版本送到不同机器。

## 项目命令

项目先用 `.sop/profile.json` 记录事实和选择，再由当前版本的 engine 处理托管内容：

```text
sopctl diff
sopctl render
sopctl check
sopctl project checkpoints
sopctl project rollback --to <checkpoint>
```

- `diff` 只预览，不改文件。
- 新建或修改 profile 时，用 `diff --profile <候选文件>` 预览；确认后再用 `render --profile <同一候选文件>`，profile、托管产物和 lock 会一起成功或一起回退。
- owner 看过差异后，主动运行 `render` 才会写项目。
- `check` 核对 profile、lock 和生成物是否一致。
- `project checkpoints` 列出可直接用于回退的检查点 ID；`project rollback` 只恢复项目托管内容，不切换工具版本。

可用 `--project-root <path>` 指定别的项目目录。

## 首次安装：直接运行版本包里的安装器

首次安装不需要 checkout 本仓库，也不需要本机安装 Go。解压与系统匹配、已校验的 release bundle 后，在真实交互终端运行包内安装器：

```text
# macOS / Linux
./bin/sop-install

# Windows
.\bin\sop-install.exe
```

安装器会先展示版本、完整文件清单和本地发布源，再要求输入完整版本号。管道输入会被拒绝。它会校验整个 bundle，安装同版 manager / engine，激活该版本的 Codex plugin，安装固定 bootstrap，持久记录发布源，最后才提交 `current.json`；中途失败会补偿，进程中断可在下次运行时恢复。同一版本重跑安装器不是空操作：它会用已验证的同版 bundle 修复损坏的版本目录，并重新核对或补齐 runtime、plugin、固定 bootstrap 和发布源。

需要把状态目录放到临时位置做验收时，可加 `--state-home <path>`。之后每次运行都必须带上同一个状态目录；安装完成时会打印可直接照抄的命令，例如：

```text
# macOS / Linux
SOP_STATE_HOME=<path> <path>/bin/sopctl release check

# Windows PowerShell
$env:SOP_STATE_HOME = '<path>'; & '<path>\bin\sopctl.exe' release check
```

phase 1 的发布源是**本地目录**：把各版本包放成 `<发布目录>/<版本号>/`，首装时运行其中的 `bin/sop-install`。安装器默认记录当前版本目录的父目录，也可明确指定共享盘或同步目录：

```text
./bin/sop-install --release-source <发布目录>
```

新开的 Codex session 会自动读取这份配置，不需要继续设置 `SOP_RELEASE_SOURCE`。环境变量仍可用于临时覆盖。HTTPS 下载、外部 SHA pin 和公开更新服务尚未实现；不要把本地通道说成公网供应链已经上线。

日常升级不再运行 `sop-install`，而是走下面的 `sopctl release` 命令。固定 bootstrap 只负责按 `current.json` 分发请求，升级和回退都不会覆盖它。

## 版本命令：确认后升级

```text
sopctl release check
sopctl release diff --to 0.2.0
sopctl release upgrade --to 0.2.0
sopctl release rollback
sopctl release rollback --to <已安装旧版本>
```

`release check` 会验证当前 bundle、manager / engine 握手、固定 bootstrap、Codex plugin 的精确来源；当前项目已有 `.sop` 状态时，还会在项目操作锁内用当前 engine 做完整 `check`，并打印 SOP / profile schema / compatibility。报错后的修复入口是重新运行当前精确版本包内的 `sop-install`。固定 bootstrap 缺失时安装器会补回；内容不同时安装器不会猜测所有权或直接覆盖，错误会给出准确路径，需确认它确实是损坏的受管文件、先人工移走留证，再重跑同版安装器。

`release diff` / `release upgrade` 会展示当前包与目标包的真实文件哈希差异。目标 release 的 bootstrap protocol 和 SHA 必须与已安装的固定 bootstrap 一致；phase 1 没有隐式 bootstrap 升级，差异会在 plugin 或 `current.json` 改动前被拒绝。若已有与目标版本匹配的候选 profile，可同时查看目标 engine 的项目差异：

```text
sopctl release diff --to 0.2.0 --project-root <项目目录> --profile <候选-profile.json>
```

这段项目预览使用真实项目操作锁，但不写项目文件。没有目标兼容的候选 profile 时，命令会明确写 `PROJECT_DIFF unavailable` 和下一步，不把元数据摘要冒充项目差异。同一个已发布 output ID 不允许换 target；这种包会标成 `INCOMPATIBLE`，升级在确认前拒绝。

`release upgrade` 会再次展示这些差异，并要求在真实交互终端输入完整目标版本；管道输入不会被当成确认。新版本校验、预检或 plugin 激活任一步失败，都不应切换 `current` 指针。

phase 1 发布版本只允许 `1.2.3` 这种 `X.Y.Z`；预发布和 `+build` 元数据等变体尚未开放，避免出现工具版本与项目 SOP 版本不能一一对应、或两个目录升级优先级相同的歧义。

工具升级不会顺手改项目。完整流程有两个确认点：

1. 先确认工具 / plugin 版本变化。
2. 再看项目级 `diff`，确认后单独执行 `render`。

同样，`release rollback` 只回退工具、engine 和 plugin；项目内容要用 `project rollback` 单独恢复。默认目标损坏或缺失时，命令会扫描并列出校验通过的已安装旧版本，可用 `--to` 明确选择；任意路径、未安装版本、当前版或更高版本都不会被当成回退目标。若项目已有 SOP 状态，回退前会在项目锁内运行目标版本 engine 的完整 `check`，托管文件缺失或被改动时不会切换 plugin / `current.json`。

正式打包的唯一入口是 `sop-release gate`。它要求 source 是 clean 的最终 tag / commit，显式重跑 `git diff-tree --check`、`go vet ./...`、`go test ./...`，并对源码 plugin 与生成后的 plugin 各跑一次官方 validator；随后从这份 checkout 自己交叉编译 bootstrap、`sop-install`、manager、engine，最后组装并复核 bundle。调用前把官方校验脚本路径交给它：

```text
export SOP_PLUGIN_VALIDATOR="$HOME/.codex/skills/.system/plugin-creator/scripts/validate_plugin.py"
sop-release gate --source . --plugin-root plugin --output <仓库外目录> \
  --version <版本> --tag v<版本> --commit <完整-HEAD-SHA> \
  --release-notes <发布说明> --upgrade-impact <升级影响> \
  --target-os <darwin|windows> --target-arch <amd64|arm64>
```

`sop-release verify --bundle <release-dir>` 只复核 bundle 的内部结构和哈希一致性，不能单独证明它来自哪个 Git commit。`assemble-unverified` 只是测试内部组装器，不是发布入口；公开的 `sop-release build` 会直接拒绝绕过 gate。普通项目不需要运行这些维护命令。

tagged GitHub Actions gate 明确运行在预配置标签为 `self-hosted, linux, sop-release` 的发布机上；该机器必须预装 Codex、`uv`，并通过仓库变量 `SOP_PLUGIN_VALIDATOR` 指向本机官方 validator 文件。tagged gate 还会显式开启临时 `CODEX_HOME` 的真实 plugin 安装 / 升级 / 回退测试。普通 hosted runner 没有这些前提，不能冒充正式发布环境。

## macOS / Windows 边界

- 状态目录按系统解析：macOS 使用用户级配置目录，Windows 使用 `%LOCALAPPDATA%`；项目文件只保存相对路径。
- 每个版本放在独立目录。固定 bootstrap 读取 `current.json` 后启动对应 manager；升级新增目录，不覆盖正在运行的 `.exe`。
- 至少保留当前版和上一版，工具回退与项目回退互不混在一起。
- phase 1 只支持带自校验标记的托管块，不支持无法证明文件所有权的全文件托管；删除旧托管块还必须匹配本机状态目录记录的可信 lock，换新机器后先用当前 profile 和匹配版本执行一次 `render` 建立本机凭据。
- phase 1 的 profile schema 固定为 1；同 schema 的 SOP 版本可回退，未来 schema 2 等 MAJOR 迁移尚未实现跨 schema 回滚编排，会明确拒绝而不是假装成功。
- CI 配置包含 macOS / Windows 原生测试和 macOS / Windows amd64、arm64 交叉构建；交叉构建成功不等于目标机器已经实跑。

phase 1 目前仍缺一轮**真实自有 Windows + Codex**端到端验收，不能把下面这些当成已经证明：

- plugin 内二进制在 Windows amd64 / arm64 的真实发现与执行路径。
- Codex 对同名不同版本 plugin 的实际缓存、重装和激活行为。
- Windows 文件锁、进程中断下的 upgrade、自动恢复和 rollback。

这些项目通过前，不会把开发版冒充稳定版，也不会自动迁移其它机器。

## 从哪里继续读

- 规则语义：[`STANDARD.md`](STANDARD.md)
- 机器契约：[`manifest.json`](manifest.json) 和 [`schemas/`](schemas/)
- 设计与未验证边界：[`phase-1 稳定性设计`](docs/superpowers/specs/2026-07-10-sop-better-stability-overhaul-design.md)
- 实验记录：[`experiments/`](experiments/)
- 只有实验背书的长期教训：[`PLAYBOOK.md`](PLAYBOOK.md)

改规则时先改 `STANDARD.md`，再同步 manifest / master；每次改动继续走“实验 → 结晶”的自举闭环。
