# Atria Lite Monitor UI 对齐实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 让 `atria-lite monitor` 在视觉骨架上贴近 `atria` agents 视图，同时保留 `Slots` 作为次级信息区。

**架构：** 保持 Lite 的 `Model`、`Mode` 和 pane/slot 业务逻辑不变，只改渲染层。优先抽取或导出 `internal/tui` 中 Lite 真正需要的共享 title bar 与基础样式，Lite 自己负责 pane 列表、slots 摘要区和 mode-specific footer 的渲染。

**技术栈：** Go、Bubble Tea、Lip Gloss

---

## 文件结构

- 修改：`/home/farrell/project/atria/internal/tui/render.go`
  - 暴露或抽取 Lite 可复用的 title bar / 文本裁剪 helper。
- 修改：`/home/farrell/project/atria/internal/tui/styles.go`
  - 暴露 Lite 需要的最小样式集合，避免复制视觉 token。
- 修改：`/home/farrell/project/atria/internal/lite/view.go`
  - 将 Lite 视图改造成 Atria 风格标题栏 + 表格型主列表 + 次级 `Slots` 区 + 统一 footer。
- 可能修改：`/home/farrell/project/atria/internal/lite/model.go`
  - 如渲染需要，补充轻量 helper，不改变业务流转。
- 修改：`/home/farrell/project/atria/internal/lite/model_test.go`
  - 更新旧断言，补足新 UI 结构的测试。

### 任务 1：先写失败的 Lite UI 测试

**文件：**

- 修改：`/home/farrell/project/atria/internal/lite/model_test.go`

- [ ] **步骤 1：新增或改写默认视图断言**

为默认列表模式新增断言，要求 `View()` 至少包含：

- `agents`
- `atria`
- `slot1`

并继续保留现有的业务约束断言：

- agent 行可见
- normal pane 不混入 agent 主列表

- [ ] **步骤 2：新增 mode-specific footer 断言**

为以下模式补测试：

- list 模式包含 `enter`、`n`、`r`
- normal pane picker 包含 `enter`、`esc`、`r`
- replace prompt 包含 `1`、`2`、`3` 或允许替换的 slot 提示，以及 `esc`、`r`

- [ ] **步骤 3：运行 Lite 测试，确认失败**

运行：

```bash
go test ./internal/lite
```

预期：

- 因 UI 结构断言尚未满足而失败

### 任务 2：抽取 Lite 需要的共享 Atria 渲染能力

**文件：**

- 修改：`/home/farrell/project/atria/internal/tui/render.go`
- 修改：`/home/farrell/project/atria/internal/tui/styles.go`
- 测试：必要时补 `internal/tui` 现有测试

- [ ] **步骤 1：识别 Lite 需要复用的最小能力**

限制在：

- title bar
- 基础颜色样式（title、selected、dim、footer）
- 宽度裁剪 helper（如实现需要）

- [ ] **步骤 2：以最小暴露面导出 helper**

优先方案：

- 导出 title bar helper
- 为 Lite 暴露最少样式访问入口

避免：

- 让 Lite 直接依赖主应用 project list 的整套行渲染

- [ ] **步骤 3：运行相关测试**

运行：

```bash
go test ./internal/tui
```

预期：

- `internal/tui` 相关测试通过

### 任务 3：重写 Lite 主视图渲染

**文件：**

- 修改：`/home/farrell/project/atria/internal/lite/view.go`
- 可能修改：`/home/farrell/project/atria/internal/lite/model.go`

- [ ] **步骤 1：改造页面骨架**

将 `View()` 改成：

1. Atria 风格 title bar
2. mode-specific 主区域
3. 次级 `Slots` 区
4. Atria 风格 footer

- [ ] **步骤 2：实现列表模式表格化渲染**

列表模式渲染列：

- `pane`
- `type`
- `binding`
- `cwd`

要求：

- 只显示 agent panes
- 支持选中行高亮
- 允许基本裁剪，避免明显溢出

- [ ] **步骤 3：实现次级 `Slots` 摘要区**

要求：

- 一直显示在主区域下方
- `slot1/slot2/slot3` 都有明确占位
- 空槽位显示 `empty`
- 已绑定槽位显示 pane id 和 kind

- [ ] **步骤 4：统一 normal pane picker 与 replace prompt 的页面风格**

要求：

- 不再使用旧的裸文本 section heading
- 继续保留必要的说明文字
- footer 与默认模式保持同一视觉语言

- [ ] **步骤 5：运行 Lite 测试**

运行：

```bash
go test ./internal/lite
```

预期：

- 新旧业务测试全部通过

### 任务 4：做交叉验证并整理输出

**文件：**

- 修改：`/home/farrell/project/atria/internal/lite/view.go`
- 修改：`/home/farrell/project/atria/internal/lite/model_test.go`
- 可能修改：`/home/farrell/project/atria/internal/tui/render.go`
- 可能修改：`/home/farrell/project/atria/internal/tui/styles.go`

- [ ] **步骤 1：运行目标测试集**

运行：

```bash
go test ./internal/lite ./internal/tui
```

预期：

- 两个包均通过

- [ ] **步骤 2：运行全量测试做回归检查**

运行：

```bash
go test ./...
```

预期：

- 无新增回归

- [ ] **步骤 3：格式化改动文件**

运行：

```bash
gofmt -w internal/lite/view.go internal/lite/model.go internal/lite/model_test.go internal/tui/render.go internal/tui/styles.go
```

预期：

- 无格式问题

- [ ] **步骤 4：Commit**

```bash
git add internal/lite/view.go internal/lite/model.go internal/lite/model_test.go internal/tui/render.go internal/tui/styles.go docs/superpowers/specs/2026-04-05-atria-lite-monitor-ui-alignment-design.md docs/superpowers/plans/2026-04-05-atria-lite-monitor-ui-alignment.md
git commit -m "Align lite monitor UI"
```
