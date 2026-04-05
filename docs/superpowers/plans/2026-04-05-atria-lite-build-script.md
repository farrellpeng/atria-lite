# atria-lite 构建脚本实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 增加一个可直接生成 `./dist/atria-lite` 的本地构建脚本，并补上 `Makefile` 入口与 README 说明。

**架构：** 以 `scripts/build-atria-lite.sh` 作为主入口，内部优先使用 `/usr/local/go/bin/go` 并复用当前 `Makefile` 的版本注入语义。`Makefile` 仅作为脚本别名入口，README 提供最小可用说明。

**技术栈：** Bash、Make、Go

---

## 文件结构

- 创建：`/home/farrell/project/atria/scripts/build-atria-lite.sh`
  - 负责生成 `./dist/atria-lite`
- 修改：`/home/farrell/project/atria/Makefile`
  - 新增 `build-lite` 目标
- 修改：`/home/farrell/project/atria/README.md`
  - 增加 Lite 本地构建说明

### 任务 1：先写失败验证

**文件：**

- 创建：`/home/farrell/project/atria/scripts/build-atria-lite.sh`

- [ ] **步骤 1：先确认脚本当前不存在**

运行：

```bash
test -e scripts/build-atria-lite.sh
```

预期：

- 失败，因为脚本尚未创建

- [ ] **步骤 2：先确认产物当前不存在**

运行：

```bash
test -e dist/atria-lite
```

预期：

- 失败，因为产物尚未生成

### 任务 2：实现构建脚本

**文件：**

- 创建：`/home/farrell/project/atria/scripts/build-atria-lite.sh`

- [ ] **步骤 1：创建脚本骨架**

要求：

- 使用 `#!/usr/bin/env bash`
- `set -euo pipefail`
- 自动切换到仓库根目录

- [ ] **步骤 2：实现 Go 选择逻辑**

优先：

- `/usr/local/go/bin/go`

回退：

- `go`

若都不可用则报错退出。

- [ ] **步骤 3：实现版本注入与构建**

要求：

- 计算 `VERSION`、`COMMIT`、`DATE`
- 组装 `LDFLAGS`
- 创建 `dist/`
- 构建 `./cmd/atria-lite` 到 `./dist/atria-lite`

- [ ] **步骤 4：赋予脚本可执行权限**

运行：

```bash
chmod +x scripts/build-atria-lite.sh
```

### 任务 3：补 Makefile 和 README

**文件：**

- 修改：`/home/farrell/project/atria/Makefile`
- 修改：`/home/farrell/project/atria/README.md`

- [ ] **步骤 1：增加 `build-lite`**

要求：

- 使用脚本作为唯一实现入口
- 保持 `VERSION` 等变量可透传

- [ ] **步骤 2：增加 README 说明**

至少说明：

- 执行 `./scripts/build-atria-lite.sh`
- 产物路径 `./dist/atria-lite`
- 如何直接运行产物

### 任务 4：验证产物

**文件：**

- 创建：`/home/farrell/project/atria/dist/atria-lite`（构建产物，不提交）

- [ ] **步骤 1：运行脚本构建**

运行：

```bash
./scripts/build-atria-lite.sh
```

预期：

- 成功生成 `./dist/atria-lite`

- [ ] **步骤 2：验证产物存在且可执行**

运行：

```bash
test -x ./dist/atria-lite
```

预期：

- 成功

- [ ] **步骤 3：验证帮助输出**

运行：

```bash
./dist/atria-lite --help
```

预期：

- 成功输出帮助或使用说明

- [ ] **步骤 4：Commit**

```bash
git add scripts/build-atria-lite.sh Makefile README.md docs/superpowers/specs/2026-04-05-atria-lite-build-script-design.md docs/superpowers/plans/2026-04-05-atria-lite-build-script.md
git commit -m "Add lite build script"
```
