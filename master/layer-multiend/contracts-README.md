<!-- templates/contracts-README.md —— 仅多端(≥2 端)。生成为 docs/contracts/README.md。占位符:{{ends}} -->

# 跨端契约(单一真相源)

端之间共享的字段、接口、事件、错误码,**只在这里权威定义一次**。各端代码和文档**引用**本目录,不重复声明(STANDARD §1.6)。

## 规矩

- 一个跨端事实 = 一处定义。改它 → 先改本目录 + 通知相关端 + 两端对齐(handshake)后才落地。
- 契约 **freeze 需 owner 明确确认**(STANDARD §3 多端流程)。
- 端清单:{{ends}}。

## 目录

```
docs/contracts/
├── README.md          # 本文件
└── <主题>.md          # 每个跨端主题一份(如 article.md / dispatch.md)
```

## 契约文件模板(每个主题)

```markdown
# 契约:<主题>

- 涉及端:<A> ↔ <B>
- 版本:1

## 字段 / 接口
| 名 | 类型 | 说明 | 谁写 | 谁读 |
|---|---|---|---|---|

## 变更记录
- v1 (YYYY-MM-DD):初版
```
