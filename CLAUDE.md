# Cage 个人改版

## 仓库与分支

- 个人仓库：`ErikaAlk/Claude-Cage`，Git 远端 `origin`。
- 官方上游：`Lee-xeo/Claude-Cage`，Git 远端 `upstream`。
- 默认分支 `personal` 保存个人改版；`main` 保留复制仓库时的官方版本。
- 功能修改从 `personal` 创建分支，通过个人仓库的 PR 合入。同步官方更新前检查当前文件和提交，保留个人修改。
- 改动范围与验证情况见 `docs/personal-fork.md`。

## 源码结构

- `cmd/cage`：后台 CLI 与 Agent 接口入口。
- `internal/workbench`：控制服务、项目共享、终端、账户与桌面连接。
- `ui`：React 工作台；生产构建输出到 `internal/workbench/web`，由 Go 嵌入。
- `desktop/windows`：Windows 原生窗口、WebView2 与本机剪贴板桥。
- `scripts`、`packaging`：各平台构建、打包和环境准备。
- `third_party`：参与编译的依赖源码；保留许可证和来源说明。

## 构建与验证

先检查 `go.mod`、`global.json`、`ui/package.json` 与实际工具版本。构建命令从仓库根目录执行：

```text
npm --prefix ui ci --no-audit --no-fund
npm --prefix ui run build
pwsh -NoProfile -File scripts/build-windows.ps1
```

Windows 完整构建需要 Go、Node.js、.NET SDK 和 7-Zip，参数见 `scripts/build-windows.ps1`。其他平台沿用 Makefile 与对应脚本。

只运行覆盖修改的定向测试。终端启动与准入相关用例为 `TestCLIStartup*`、`TestTerminalAdmission*`；Windows Go 测试使用 `external_hcs,external_wsl2` 构建标签。修改界面后重新构建 UI，再构建后台；安装后验证实际程序。

## 运行约束

- 源码仓库、构建输出和安装目录分别管理。Windows 本地开发程序使用 `%LOCALAPPDATA%\Programs\Cage-Local`；安装目录必须位于准备共享的项目目录之外。
- 创建项目时保留程序目录、配置和环境存储的共享保护。
- 更新前检查正在执行的任务，保存文件并正常停止环境。账户配置、虚拟机磁盘与 Claude 历史继续保留。
- 终端、浏览器和 Claude Desktop 通过现有隔离环境访问项目。新增复制、粘贴和输入行为复用现有原生桥、附件通道和输入队列。
- `.project/` 与 `_output/` 存放本机计划、工具和构建产物，不提交。凭据、账户资料、代理配置、会话与虚拟机数据不得上传。
