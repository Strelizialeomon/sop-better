# 跨端契约模式

## req doc 写到语义层

req doc 应写主流程、触发方、交换的语义数据、领域状态，以及会改变后端能力的交互选择。字段名、类型、endpoint、schema、组件和视觉细节留到实施层。

判据：决策会不会改变跨端交换的数据或领域状态机？会就进入 req doc；纯视觉或端内实现留给该端。

req doc 必须包含一条端到端走查。每一步标明触发方、推进的实体状态、交换的语义数据；只写“跑通 A → B → C”不算契约。

## 契约握手

- 任一端都可把 endpoint、schema 或消费者需要的数据形状作为 draft 发到需求 issue。
- 相关端在 issue 评论核对；一致后成为 agreed draft，各端可据此实施。
- draft 有异议时继续留痕并调整；握手用于尽早对齐，不是等对方全部做完。

## firmness 三级

1. **req 级**：锁会改变跨端能力的产品选择。
2. **agreed draft 级**：锁 endpoint 和字段 schema；改变时写 issue 评论。
3. **freeze 级**：联调后把可复用的平台边界搬进 `docs/contracts/<主题>.md`，成为只读硬契约。
