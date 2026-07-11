# 可读 fixture

每个目录有两份文件：

- `profile.json`：输入的项目事实与 owner 决定。
- `expect.json`：人可直接扫的期望文件树、端级必备内容和禁止内容；acceptance test 会读取同一份文件判定，避免“文档一套、测试另一套”。

四个故事分别覆盖单人单端、两位真人单端、单人多端串行、单人多端并行。其它合法布尔组合由 `acceptance/legal_matrix_test.go` 程序遍历。
