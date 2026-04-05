# Atria Lite Monitor UI 对齐设计

## 背景

`atria-lite monitor` 当前使用纯文本拼接输出：

- 顶部固定显示 `Atria Lite Monitor`
- 主体按 `Agents`、`Slots`、footer 三段直接输出字符串
- 选中态、标题栏、底栏帮助文案与主应用 `atria` 的 agents 视图不是同一套视觉语言

这导致用户在 `atria` 与 `atria-lite` 之间切换时，界面心智模型不连续。用户要求 Lite Monitor pane 的 UI 「做得跟 atria 一样」，同时明确要求：

- 保留 `Slots`
- 但把 `Slots` 降为更像 Atria 的次级信息区

## 目标

让 `atria-lite monitor` 的主视图尽量贴近 `atria` agents 页的视觉骨架，同时保留 Lite 特有的 pane/slot 语义。

目标具体化为：

1. 使用 Atria 风格的 title bar
2. 使用更接近 Atria agents 页的表格型主列表
3. 保留 `Slots`，但降为次级摘要区
4. 在 `ModeList`、`ModeNormalPanePicker`、`ModeReplacePrompt` 三种模式下维持一致的标题栏、选中态与 footer 风格

## 非目标

- 不将 Lite Monitor 改造成完整的 `atria` 主应用
- 不引入 store、stream panel、chat view 等完整 TUI 模块
- 不改变 Lite 的业务行为：pane 发现、slot 替换、normal pane 装载规则保持不变
- 不在本次改动中重构 WezTerm Lite 编排逻辑

## 用户确认的方向

用户已确认采用方案 A：

- Lite 主体尽量与 Atria agents 页一致
- `Slots` 保留，但作为列表下方的次级信息区
- 不保留当前 Lite 那种强分段、纯文本标题的主观感受

## 现状问题

### 1. 标题栏不一致

当前 Lite 直接显示 `Atria Lite Monitor`，没有 Atria agents 页的：

- 左侧 title
- 右侧 `atria` branding
- 下划线分隔

### 2. 主列表信息组织松散

当前 agent 列表只是：

- `>` 光标
- pane label
- 绑定状态
- 可选 `CWD`

缺少 Atria agents 页那种稳定列结构，导致扫描效率低。

### 3. `Slots` 视觉层级过高

当前 `Slots` 与 `Agents` 是并列主区块，抢占了与主列表相同的视觉权重，不符合「主列表为主、绑定为辅」的目标。

### 4. 模式切换后风格断裂

进入 normal pane picker 或 replace prompt 后，界面又退回裸文本描述，缺少主应用里跨模式的统一观感。

## 设计原则

### 1. 视觉一致优先于字面复刻

Lite 不需要复刻主应用所有列和交互，但必须复用：

- title bar 的视觉结构
- 选中行高亮语言
- dim/secondary 文本层级
- footer 帮助栏的表达方式

### 2. 业务模型继续由 Lite 持有

Lite 仍然使用自己的 `Model`、`CandidatePane`、`SlotBinding`、`Mode`。这次只调整渲染层组织，不把主应用状态模型直接引进来。

### 3. 共享渲染片段优先

凡是 Atria 已存在且 Lite 明确需要的渲染能力，应优先抽成共享函数或导出受限 helper，而不是在 Lite 里复制一份近似实现。

### 4. `Slots` 只做上下文，不做主舞台

`Slots` 仍然必须可见，但它的作用是告诉用户当前 3 个槽位绑定了什么，而不是作为主要交互焦点。

## 目标界面

## 1. `ModeList`

### 顶部

沿用 Atria 风格 title bar：

- 左侧标题建议使用 `agents`
- 右侧显示 `atria`
- 下方分隔线与 Atria 一致

不再显示 `Atria Lite Monitor` 这一整行裸文本标题。

### 主列表

主列表改成宽表格感布局，推荐列为：

- `pane`：pane label，基于现有 `paneLabel`
- `type`：agent / normal 的类型标签
- `binding`：当前绑定到哪个 slot，未绑定则显示 `unbound`
- `cwd`：若可解析，则显示路径；否则留空

其中：

- 默认只展示 agent panes
- normal panes 仍通过 `n` 进入 picker，不混入主列表
- 光标选中项使用 Atria 风格高亮
- 行级文本对齐以易扫读为优先，必要时做宽度裁剪

### 次级 `Slots` 区

位于主列表下方，使用 dim/secondary 风格，形式可以是：

- 3 行摘要；或
- 紧凑摘要标题 + 3 行内容

内容保留：

- `slot1` / `slot2` / `slot3`
- pane id
- occupant kind（`agent` / `normal`）
- 空槽位显示 `empty`

该区域不再使用和主列表同等级的 section heading 视觉。

### Footer

footer 使用 Atria 风格帮助栏，但文案保持 Lite 语义：

- `enter:load`
- `n:normal panes`
- `r:refresh`
- 需要时追加状态文本

如果有 `statusText`，则继续显示在帮助栏上方，但格式要与 Atria 的底部语言更接近，避免裸文本断层。

## 2. `ModeNormalPanePicker`

保留现有行为，但视觉统一：

- 复用同样的 title bar
- 主体列表只显示 normal panes
- 使用与 `ModeList` 相同的选中态
- footer 改为 Atria 风格短帮助栏，例如 `enter:load  esc:back  r:refresh`

该模式不再显示大标题 `Normal Panes` + 裸文本提示，而是让内容更像一张切换了数据源的 agents 列表。

## 3. `ModeReplacePrompt`

替换模式保留明确提示，但外观仍与 Atria 一致：

- 顶部仍是同一 title bar
- 中间区域展示一段清晰说明文字：目标 pane、当前槽位已满、允许替换哪些 slot
- `Slots` 次级区继续显示当前绑定，方便用户判断替换影响
- footer 改为 Atria 风格短帮助栏：`1/2/3:replace  esc:back  r:refresh`

替换模式可以保留 `Replace` 这个关键词，但不要再以纯文本 section 的方式占据整个页面骨架。

## 共享实现边界

建议优先复用或抽取以下能力：

- `internal/tui/render.go` 的 title bar 渲染逻辑
- `internal/tui/styles.go` 中与 title、selected、dim、footer 相关的基础样式

不建议直接依赖完整的 `projectlist` 渲染，因为：

- 主应用 rows 基于 `projectRow` / `AgentSession`
- Lite 的数据源是 `CandidatePane` / `SlotBinding`
- 直接复用整套主列表会带来不必要的状态映射和耦合

因此，更合理的方向是：

1. 将 title bar 与必要的基础样式维持为可复用能力
2. Lite 自己实现小而专注的 table row 渲染
3. Lite 继续自己控制 mode-specific 文案

## 文件级设计

### `internal/lite/view.go`

职责变化：

- 从纯字符串分段拼接，改为有样式的结构化渲染
- 负责三种模式的页面骨架与 footer
- 负责 `Slots` 次级摘要区

### `internal/lite/model.go`

职责变化较小：

- 保持当前状态流转
- 若渲染需要额外标题或 mode label，可在这里补轻量 helper

### `internal/tui/render.go`

可能调整：

- 导出或抽出 Lite 需要的 title bar helper
- 如果有必要，顺带抽出安全的文本裁剪辅助

### `internal/tui/styles.go`

可能调整：

- 导出 Lite 需要的少量样式或封装共享 getter
- 仅暴露必要样式，避免 Lite 直接依赖整个主应用内部实现细节

### `internal/lite/model_test.go`

需要更新与补充：

- 旧测试里针对裸文本标题、section heading 的断言需要改写
- 新增对 title bar、次级 `Slots` 区、不同 mode footer 的断言

## 测试策略

### 保留的测试意图

现有测试关注的业务意图应保留：

- 只列出当前 window 且排除 self pane
- normal pane picker 只列出 normal panes
- 替换模式只在槽位已满时出现
- slot summary 能反映 live panes 的重分类结果

### 需要替换的展示断言

由于 UI 文字会变化，下列断言要从「字面匹配旧标题」改成「匹配新结构」：

- `Atria Lite Monitor`
- `Agents`
- `Normal Panes`
- `Replace Prompt`
- `enter load agent | n normal panes | r refresh`

### 新增展示断言

建议新增：

1. 默认视图包含 Atria 风格 title bar（如 `agents` 与 `atria` branding）
2. `Slots` 区域仍存在，但文案和层级为次级信息
3. `ModeNormalPanePicker` footer 使用 Atria 风格帮助文案
4. `ModeReplacePrompt` footer 使用替换模式帮助文案
5. `CWD` 仍在主列表行里可见

## 风险与应对

### 风险 1：Lite 与主应用样式耦合过深

如果直接引用过多 `internal/tui` 私有实现，后续主应用改样式时，Lite 可能被动破坏。

应对：

- 只共享 title bar 和少量基础样式
- Lite 自己持有行渲染逻辑

### 风险 2：测试过度绑定具体文本

如果测试继续大量匹配整段 UI 字符串，后续微调文案会产生高噪音失败。

应对：

- 断言结构性关键词
- 对 footer 只校验当前 mode 的关键动作词

### 风险 3：窄宽度渲染退化

Lite 目前没有像主应用那样完整的宽度策略，直接引入表格后可能在窄 pane 中溢出。

应对：

- 首版先做基础裁剪
- 只保证不失真、不明显溢出
- 不在本次引入主应用那套完整窄屏布局策略

## 验收标准

满足以下条件即视为达成：

1. Lite Monitor 默认视图在视觉骨架上接近 Atria agents 页
2. `Slots` 仍然存在，但明显是次级信息区
3. 三种模式的标题栏、选中态、footer 风格统一
4. 业务行为不变，现有 slot 与 pane 逻辑保持正确
5. 测试覆盖新的 UI 结构，`go test ./internal/lite ./internal/tui` 通过
