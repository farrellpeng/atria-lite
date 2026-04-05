# atria-lite 构建脚本设计

## 背景

当前仓库只有面向主程序 `atria` 的本地构建入口：

- `Makefile` 的 `build` 目标输出仓库根目录下的 `./atria`

但 `atria-lite` 目前缺少一个可直接运行的本地产物构建入口。用户希望增加一个「可直接运行的二进制构建脚本」，并明确约束：

- 只生成 `atria-lite`
- 产物路径固定为 `./dist/atria-lite`

## 目标

新增一个统一、直接、可重复运行的本地构建入口，使开发者在仓库根目录执行一次脚本后，就能得到可执行的 `atria-lite` 二进制文件。

目标具体为：

1. 提供脚本 `scripts/build-atria-lite.sh`
2. 默认生成 `./dist/atria-lite`
3. 自动创建 `dist/` 目录
4. 复用现有版本注入逻辑（`VERSION`、`COMMIT`、`DATE`、`ldflags`）
5. 优先使用 `/usr/local/go/bin/go`，避免本机默认 `go 1.13.8` 造成构建失败
6. 顺手提供 `make build-lite` 作为脚本别名入口

## 非目标

- 不同时构建主程序 `atria`
- 不引入交叉编译矩阵
- 不引入 Release 打包逻辑
- 不修改 `goreleaser` 工作流
- 不改变现有 `make build` 的行为

## 用户确认的方向

用户已确认：

- 采用独立脚本方案
- 输出路径固定为 `./dist/atria-lite`
- 脚本是主入口，`Makefile` 可以作为便捷别名补充

## 设计原则

### 1. 入口要直接

开发者应能直接执行：

```bash
./scripts/build-atria-lite.sh
```

然后得到可执行文件：

```bash
./dist/atria-lite
```

### 2. 本机 Go 环境要可控

当前环境里：

- `/usr/bin/go` 指向 Go 1.13.8
- `/usr/local/go/bin/go` 是 Go 1.25.0

仓库 `go.mod` 需要新版本 Go，因此脚本不能盲目依赖 PATH 里的默认 `go`。脚本应优先选用 `/usr/local/go/bin/go`，必要时再回退到 `go`。

### 3. 与现有 Makefile 版本注入保持一致

当前 `Makefile` 已定义：

- `VERSION`
- `COMMIT`
- `DATE`
- `LDFLAGS := -X main.version=... -X main.commit=... -X main.date=...`

新脚本应延续同样语义，避免出现主程序与 Lite 程序构建元数据不一致。

### 4. 输出路径固定、幂等

脚本每次执行都覆盖更新 `./dist/atria-lite`，不额外制造时间戳文件名或多层目录，保证使用者心智稳定。

## 目标行为

## 1. 脚本行为

执行 `./scripts/build-atria-lite.sh` 时，脚本应：

1. 切换到仓库根目录
2. 解析构建变量：
   - `VERSION`：默认 `dev`
   - `COMMIT`：默认 `git rev-parse --short HEAD`
   - `DATE`：默认 `date -u +%Y-%m-%dT%H:%M:%SZ`
3. 选择 Go 二进制：
   - 优先 `/usr/local/go/bin/go`
   - 否则回退到 `go`
4. 创建 `dist/`
5. 构建 `./cmd/atria-lite`
6. 输出到 `./dist/atria-lite`
7. 在成功时打印产物路径

## 2. Makefile 行为

新增：

- `build-lite`

行为：

- 内部调用 `./scripts/build-atria-lite.sh`
- 允许 `VERSION=... make build-lite` 这样的外部变量覆盖

## 3. README 行为

README 增加本地构建说明，至少告诉用户：

- 如何构建 Lite 二进制
- 产物在哪
- 如何直接运行

## 失败处理

脚本应在以下场景明确失败：

### 1. 找不到可用 Go

当：

- `/usr/local/go/bin/go` 不存在
- 且 `go` 不在 PATH 中

应给出明确错误信息，而不是继续执行。

### 2. 构建失败

若 `go build` 失败，脚本直接退出非 0，并保留错误输出，不吞日志。

### 3. Git 信息不可用

当仓库外部或 Git 不可用时：

- `COMMIT` 回退为 `unknown`
- `VERSION` 仍可默认为 `dev`

不要求脚本因为缺失 Git 元数据而失败。

## 文件级设计

### `scripts/build-atria-lite.sh`

职责：

- 作为 Lite 二进制的主构建入口
- 负责选择 Go、组装 `ldflags`、创建输出目录并执行构建

### `Makefile`

职责：

- 提供 `build-lite` 便捷入口
- 保持与现有 `build` 目标风格一致

### `README.md`

职责：

- 为用户提供最小必要的本地构建说明

## 测试策略

这次以验证脚本行为为主，不增加复杂 shell 单测框架。验收验证包括：

1. 运行脚本：

```bash
./scripts/build-atria-lite.sh
```

2. 确认产物存在：

```bash
test -x ./dist/atria-lite
```

3. 确认可执行：

```bash
./dist/atria-lite --help
```

至少要求命令成功启动并输出帮助信息或使用说明。

## 验收标准

满足以下条件即视为完成：

1. 仓库新增 `scripts/build-atria-lite.sh`
2. 执行后生成可执行文件 `./dist/atria-lite`
3. `Makefile` 提供 `build-lite` 入口
4. `README.md` 补充 Lite 本地构建说明
5. 实际运行脚本后，可验证产物存在且可执行
