# Atria-lite — 设计文档

## 概述

`atria-lite` 是同仓库内新增的一个 WezTerm 专用入口，目标是在 **当前已有 agent 的 WezTerm 窗口** 中，将窗口重排成固定工作台布局：

- 上方一个监控 pane，运行精简监控版 atria
- 下方一个可按需扩展的原生 WezTerm 工作区，最多展开为同排 3 个 pane

它不是新的通用终端管理器，也不是现有 atria 的完整模式切换，而是一个更窄、更强约束的 WezTerm pane 编排器。

## 目标

- 在 WezTerm 中提供固定的“上监控、下工作台”布局
- 保留下方 pane 的 **100% WezTerm 原生终端体验**
- 复用现有 agent 识别、状态监控和 session 读取能力
- 将上方 TUI 收窄为“监控 + 槽位调度器”，不再承担完整 atria 交互

## 非目标

- 第一版不负责新建 agent
- 不跨 WezTerm window 或 workspace 管理 pane
- 不支持 tmux、Kitty、iTerm2 作为 lite 工作台的底部承载环境
- 不在上方 monitor 中保留聊天页、设置页、嵌入终端页
- 不做普通 pane 的全局统一管理，普通 pane 仅作为底部工作区最右侧活跃槽位的可选内容
- 第一版不提供 `atria-lite stop` 或自动恢复启动前窗口布局的机制

## 已确认的产品边界

- 形态：同仓库新入口，而不是独立仓库
- 架构方向：`Lite Orchestrator + Slim TUI`
- 上方 pane：精简监控版 atria，仅负责列表、状态、选中、槽位装载、替换提示
- 下方 pane：与 WezTerm 中完全一致的原生 pane
- 下方最多 3 个槽位，且 `Slot 1`、`Slot 2`、`Slot 3` **同排**
- 当 3 个槽位都活跃时：`Slot 1`、`Slot 2` 只能装载 agent pane，`Slot 3` 可装载普通 pane
- 当 2 个槽位活跃时：`Slot 1` 只能装载 agent pane，`Slot 2` 可装载普通 pane
- 当只活跃 1 个槽位时：若存在 agent，则 `Slot 1` 装载 agent；若没有 agent，则 `Slot 1` 可装载普通 pane
- agent 来源：只管理 **当前 WezTerm 窗口**
- 第一版只接管已有 pane，不负责新建 agent
- 普通 pane 不出现在上方 agent 列表中
- 普通 pane 通过独立动作选择，只能装载到“当前最右侧的活跃槽位”
- 启动方式：从 **当前已有 agent pane 的 WezTerm 窗口** 执行 `atria-lite start`
- 启动后重排当前窗口为 lite 布局，而不是新开一个空窗口
- 当 3 个槽位都被占用时，不自动替换，必须弹提示让用户选择替换哪个槽位或取消
- 当前窗口超过 3 个候选 pane 时：
  - 优先保留 agent pane
  - 普通 pane 只可能作为“当前最右侧的活跃槽位”的候选内容
  - 未接管的 pane 自动移出当前 lite 窗口

## 运行形态

建议新增两个子命令：

### `atria-lite start`

一次性入口，负责：

- 校验当前运行环境是否为 WezTerm
- 读取 `WEZTERM_PANE` 识别启动自身的 pane、tab、window
- 读取当前窗口中的 pane 列表
- 识别 agent pane 与普通 pane
- 按规则选择要保留在 lite 工作台中的 pane
- 将未接管 pane 移出当前窗口
- 将当前窗口重排为“上监控 + 下方按需展开的 1~3 槽位工作区”
- 启动顶部 monitor pane

### `atria-lite monitor`

长驻顶部 pane，负责：

- 渲染当前窗口内 agent 列表和状态
- 维护槽位绑定关系
- 响应 agent 选中、普通 pane 选择、满槽替换提示
- 周期性刷新当前窗口 pane 列表和 agent 状态
- 使用启动时传入的自身 pane 信息，将 monitor pane 自身排除在 agent 列表之外

## 固定布局

目标布局如下（示意的是 **3 槽全部展开时** 的形态）：

```text
┌──────────────────────────────────────────────┐
│                Top Monitor Pane             │
│   精简监控版 atria：agent 列表 + 状态 + 调度   │
├──────────────────────────────────────────────┤
│   Slot 1    │    Slot 2    │    Slot 3      │
│ agent only  │  agent only  │ agent / normal │
└──────────────────────────────────────────────┘
```

约束：

- monitor pane 固定在顶部
- monitor pane 在第一版通过 `split-pane --top --top-level --percent 35` 创建，整个会话期间高度保持不变
- 下方工作区最多展开为 3 个 pane，且一旦展开到 2 个或 3 个时，所有 pane 始终同排
- 不要求启动时就物理创建 3 个空 pane；槽位是逻辑概念，pane 按需拆分物化
- 普通 pane 永远只占“当前最右侧的活跃槽位”
- `--top-level` 只用于首次切出顶部 monitor pane；下方工作区后续只做水平拆分，不再对整窗使用 `--top-level`
- pane 一旦进入底部槽位，其输入、焦点、复制、滚动、全屏等行为全部保持 WezTerm 原生

## 组件边界

### 1. Lite Starter

一个面向 WezTerm CLI 的编排层，负责启动时的窗口重排。

职责：

- 直接持有 WezTerm 专用运行时能力，不经过 `terminal.Backend` / `CompositeBackend` / `CachedBackend`
- 获取当前 pane、tab、window 上下文
- 调用结构化的 WezTerm pane 列表能力获取当前窗口 pane，而不是只拿扁平 `terminal.Session`
- 执行 `split-pane` / `move-pane-to-new-tab` / `activate-pane` 等编排动作
- 生成 monitor 启动所需的上下文

它不负责长期状态监控。

### 2. Lite Monitor Model

从现有 atria TUI 中提炼出来的精简模型。

保留：

- agent 列表
- 状态刷新
- 选中态
- 满槽替换提示
- 当前窗口范围内的 session 更新
- 基于 `window_id` + `self_pane_id` 的当前窗口过滤

移除：

- chat view
- settings
- setup wizard
- embedded terminal
- batch send
- launch agent

Lite monitor 同样直接持有 WezTerm 专用运行时能力，不需要复用现有多后端组合抽象。

### 3. Pane Slot Registry

一个轻量状态层，用于维护槽位与实际 WezTerm pane 的绑定关系。

建议结构上表达的信息：

- `slot_id`
- `pane_id`
- `slot_kind`：`agent_only` / `mixed`
- `occupant_kind`：`agent` / `normal` / `empty`
- `session_id`（当槽位内容是 agent 时）
- `agent_type`
- `window_id`
- `tab_id`

该注册表是 **软状态**，可以丢失，可以通过当前窗口实际 pane 列表重建。

同时需要区分：

- 逻辑槽位：`Slot 1`、`Slot 2`、`Slot 3`
- 物理 pane：当前底部工作区中真实存在的 WezTerm pane

只有槽位被占用时，对应物理 pane 才需要存在。

### 4. 现有检测层复用

继续复用现有能力：

- `internal/terminal/wezterm/client.go`：WezTerm pane 读取与操作
- `internal/terminal/monitor.go`：agent 状态分类
- `internal/terminal/detect.go` 及相关逻辑：agent 类型识别
- `internal/model` 中现有 session/status 类型
- `InferAgentFromScreen` 作为标题不明确时的屏幕回退判定
- 现有 `discoverAgent` 流程中的 CWD 解析与屏幕回退逻辑，作为 lite 发现流程的复用来源

新逻辑只在“当前窗口过滤”和“槽位编排”上新增，不复制已有监控规则。

## Starter 到 Monitor 的上下文

`atria-lite start` 启动 `atria-lite monitor` 时，至少需要传递这些上下文：

- `self_pane_id`：monitor 自身 pane id
- `window_id`：当前 lite 工作台所属 window id
- `tab_id`：当前 lite 工作台所属 tab id
- `starter_pane_id`：启动命令最初所在 pane id
- `slot_bindings`：初始 `Slot 1/2/3 -> pane_id` 绑定关系
- `workspace_pane_ids`：当前底部工作区中仍受 lite 管理的 pane id 集合

这个上下文不需要持久化到磁盘，但必须足够让 monitor 在首次刷新前就知道：

- 哪个 pane 是自己
- 哪些 pane 属于当前 lite 工作区
- 当前 1/2/3 槽的初始占用情况

## Pane 分类与收敛规则

启动时先对当前窗口 pane 做分类：

- 排除 `WEZTERM_PANE` 指向的启动自身 pane，以及之后新建出来的 monitor pane
- `agent pane`：能通过现有标题/屏幕模式识别为 Claude、Codex、OpenCode、Copilot 的 pane
- `normal pane`：其余普通 shell、编辑器、日志 pane

当候选 pane 数量不超过 3 时：

- 最多保留 3 个 agent pane
- 普通 pane 只允许作为“当前最右侧的活跃槽位”的候选内容
- 因此“被自动保留的普通 pane”最多只能有 1 个

当候选 pane 数量超过 3 时：

1. 先为 `Slot 1`、`Slot 2`、`Slot 3` 选择可承载对象
2. agent pane 始终从左向右占据可用槽位
3. 普通 pane 最多保留 1 个，并且只能落在“当前最右侧的活跃槽位”
4. 若没有保留任何 agent，则最多只保留 1 个普通 pane，并放入 `Slot 1`
5. 若保留了 1 个 agent，则普通 pane 只能进入 `Slot 2`
6. 若保留了 2 个及以上 agent，则普通 pane 只能进入最右侧槽位；在 3 槽布局中即为 `Slot 3`
7. 其余未接管 pane 自动移出当前 lite 窗口

“移出当前 lite 窗口”要求：

- 不销毁 pane 中的进程
- 不强制关闭 pane
- 第一版统一通过 `move-pane-to-new-tab --window-id <current_window_id>` 将其移到同一 window 下的新 tab
- 第一版不依赖 `--new-window`

## 槽位模型

### `Slot 1`

- 当存在 2 个或 3 个活跃槽位时，只能绑定 agent pane
- 当只存在 1 个活跃槽位且当前没有 agent 时，可以绑定 1 个普通 pane
- 有 agent 时，优先承载第一个装入的 agent

### `Slot 2`

- 当存在 3 个活跃槽位时，只能绑定 agent pane
- 当只存在 2 个活跃槽位时，是当前最右侧活跃槽位，可绑定 agent pane 或普通 pane
- 有两个 agent 时，优先承载第二个 agent
- 有一个 agent 且需要显示普通 pane 时，可承载该普通 pane

### `Slot 3`

- 只在 3 槽布局中存在
- 是当前最右侧活跃槽位，可绑定 agent pane 或普通 pane
- 普通 pane 不能自动出现在 agent 列表，只能通过独立动作进入这里

### 物理展开规则

- 没有 agent 且保留了 1 个普通 pane 时，底部工作区只保留 `Slot 1`
- 只有 `Slot 1` 被 agent 占用时，底部工作区可以只有 1 个 pane
- 当需要展示“1 个 agent + 1 个普通 pane”时，底部工作区展开为 `Slot 1 + Slot 2`
- 当需要展示“2 个 agent + 1 个普通 pane”或“3 个 agent”时，底部工作区展开为 3 个 pane
- 一旦存在多个底部 pane，它们必须同排显示
- 不使用“空白占位 pane”来凑满三栏
- 当 agent 数量增加时，已保留的普通 pane 会右移到新的最右侧活跃槽位
- 当某个 pane 退出或被移出当前工作区时，底部工作区不保留空槽位；依赖 WezTerm 在 pane 消失后自然收缩
- 第一版不提供“主动合并 pane”的独立命令

## 装载规则

### 选中 agent 时

顶部 monitor 选择 agent，不是“打开终端”，而是“将该 pane 绑定到下方槽位”。

装载顺序：

1. `Slot 1`
2. `Slot 2`
3. `Slot 3`

前提：

- 槽位为空
- 槽位类型允许该 pane 进入
- 若当前最右侧活跃槽位已被普通 pane 占用，而新 agent 需要插入其左侧，则普通 pane 自动右移到新的最右侧活跃槽位

### 普通 pane 装载时

- 通过独立动作打开普通 pane 选择器
- 选择器仅列出当前窗口中的普通 pane
- 选中后装载到“当前最右侧的活跃槽位”
- 普通 pane 不参与上方主 agent 列表排序
- 若当前没有 agent，则普通 pane 进入 `Slot 1`
- 若当前有 1 个 agent，则普通 pane 进入 `Slot 2`
- 若当前有 2 个 agent，则普通 pane 进入 `Slot 3`
- 若当前已有 3 个 agent，则选择普通 pane 时只能替换 `Slot 3` 或取消

### 满槽时

若三个槽位都已有内容，则不自动替换，必须进入提示态：

- Replace Slot 1
- Replace Slot 2
- Replace Slot 3
- Cancel

要求：

- 替换 `Slot 3` 时，即使当前装的是普通 pane，也必须显式确认
- 替换动作只改变 lite 工作台的槽位绑定，不影响 agent 进程本身的生存
- 选择普通 pane 且当前为“3 个 agent 已满”时，只允许替换 `Slot 3`
- 被替换出的 pane 不留在当前 lite 工作区中，而是移动到同一 window 下的新 tab

## 交互流程

### 主流程

1. 用户在已有 agent pane 的 WezTerm 窗口中执行 `atria-lite start`
2. lite starter 读取当前窗口 pane 并重排布局
3. 顶部启动 `atria-lite monitor`
4. monitor 只列出当前窗口中的 agent pane
5. 用户从上方列表选中 agent
6. system 将该 agent 装载到空槽位；如无空槽则弹替换提示
7. 底部 pane 后续完全保持 WezTerm 原生使用方式

### 普通 pane 流程

1. 用户触发“选择普通 pane”动作
2. monitor 打开普通 pane 选择器
3. 只展示当前窗口中非 agent pane
4. 选中后装载到当前最右侧活跃槽位；如果当前没有 agent，则落在 `Slot 1`

## 当前窗口过滤

`atria-lite` 只管理当前 WezTerm 窗口，不跨：

- workspace
- 其他 window
- 其他 tab（除非是启动时将多余 pane 挪出的目标位置）

因此所有：

- pane 发现
- agent 列表展示
- 状态刷新
- 槽位绑定

都必须先按 `window_id` 过滤。

## 状态刷新策略

顶部 monitor 周期性执行：

1. `wezterm cli list --format json`
2. 过滤当前 `window_id`
3. 将 pane 与槽位注册表对齐
4. 对 agent pane 调用现有屏幕读取与状态分类逻辑
5. 更新顶部列表展示

若某个已绑定 pane 在刷新时消失：

- 对应槽位自动置为空
- agent 列表移除或更新该 session
- monitor 给出轻量状态提示
- 如果物理 pane 已因退出或移出而消失，则依赖 WezTerm 的自然收缩结果重建槽位视图，而不是保留占位 pane

## 错误处理

### 不在 WezTerm 中启动

- `atria-lite start` 直接失败
- 不降级为普通 atria

### 当前窗口没有 agent pane

- 允许进入 lite 布局
- 顶部 monitor 显示空状态和引导文案
- 若当前窗口保留了 1 个普通 pane，则该 pane 可以落在 `Slot 1`
- 不自动新建 agent

### 窗口重排失败

- 任何 WezTerm CLI 编排步骤失败都立即终止
- 不尝试做复杂回滚
- 输出清晰错误信息，提示重新执行

### 启动重排竞态

- 读取 pane 列表与执行移动/拆分命令之间，用户可能新建、关闭或切换 pane
- 第一版采用“每次关键编排动作前重新读取并校验 pane id 仍然存在”的保守策略
- 若校验失败，则终止本次启动并提示用户重试，而不是猜测性继续重排

### 退出 lite 模式

- 第一版没有 `stop` 子命令
- 退出 monitor 只结束顶部监控进程，不自动恢复启动前布局
- 当前窗口中已经形成的 pane/tabs 保持在退出时的状态

### pane 消失 / 进程退出

- 下一轮刷新时解除槽位绑定
- 更新 monitor 列表和状态

## 测试策略

### 1. 槽位逻辑单元测试

覆盖：

- agent 装载顺序
- `Slot 1/2` 的 agent-only 限制
- `Slot 3` 的 mixed 限制
- 满槽替换提示逻辑
- 超过 3 个候选 pane 时的保留与移出决策

### 2. WezTerm JSON 解析测试

基于 fixture 验证：

- 当前窗口过滤
- pane 分类
- 当前 active pane 推断
- 同一窗口下 agent 与 normal pane 的区分

### 3. 精简 monitor 模型测试

验证：

- 不再依赖 chat/term/settings 分支
- 只保留列表、状态、提示、装载相关状态
- pane 消失、满槽、选择普通 pane 等分支处理正确

## 建议的实现拆分

为了保持边界清晰，建议实现时按以下方向收束：

- 新增 `cmd/atria-lite`
- 在 `internal/terminal/wezterm` 旁增加 lite 所需的窗口级编排辅助能力
- 为 WezTerm 运行时补充结构化 pane 列表、split、move 等编排辅助方法
- 在 `internal/tui` 中抽出一个精简 monitor 模型，而不是在现有主模型上继续堆条件分支
- 新增独立的 slot/runtime 状态模块，避免把槽位逻辑混入现有 project/session store
- slot registry 建议放在独立 lite 运行时模块中，而不是放进现有 `internal/model`

## 仍需坚持的 YAGNI 边界

第一版不要加入：

- agent 启动能力
- 跨窗口 pane 拖拽式管理
- 普通 pane 多槽位支持
- 顶部 monitor 直接输入消息给 agent
- 将 lite 与完整 atria 模式做统一状态切换

## 总结

`atria-lite` 的本质不是“另一个 atria UI”，而是“一个使用现有监控能力、以 WezTerm pane 为原生工作面的轻量工作台”。它的价值在于：

- 上方保留 atria 的状态感知能力
- 下方保留 WezTerm 的原生终端体验
- 通过强约束布局和槽位模型，把复杂度限制在当前窗口内部
