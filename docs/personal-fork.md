# 个人改版维护

本仓库是 [Lee-xeo/Claude-Cage](https://github.com/Lee-xeo/Claude-Cage) 的个人 fork，保留原作者署名、提交历史和许可证。默认分支为 `personal`，个人仓库地址为 [ErikaAlk/Claude-Cage](https://github.com/ErikaAlk/Claude-Cage)。

## 当前修改

- 记住登录后恢复原有远程授权，支持关闭桌面窗口后继续访问；主动锁定保持认证要求。
- 修复 Windows 更新助手对子进程工作目录的处理。
- 新建终端等待周期网络检查释放操作锁，支持取消等待；Claude 启动确认匹配当前会话通道并检查当前终端画面。
- 终端选中文字后右键复制，成功后清除选区；没有选区时右键粘贴。程序接管鼠标时，Windows/Linux 使用 Shift 拖选，macOS 使用 Option 拖选。
- 浏览器网页的深浅色偏好跟随 Cage，Linux 浏览器原生界面同步主题；远程指针随画面比例调整。
- 扩展管理提供“打开扩展”和“固定到顶栏”，固定状态随 Cage 环境保存；地址栏可直接访问 `chrome-extension://扩展标识/路径`，保留查询参数和页面片段。支持打开 1Password 等导入的 Manifest V3 扩展弹窗。扩展从本机导入后，在 Cage 环境中登录并保存账户；宿主的 1Password 桌面应用连接仍受客机隔离边界约束。
- Windows 托盘左键单击打开 Cage；远程输入桥返回实际焦点状态，并向输入法提供原生候选位置。

已在 Windows 宿主、Linux 客机验证终端创建、真实输入输出、Claude 启动、右键操作与网络检查。其他宿主和客机组合尚未进行本地运行验收。

## 后续开发

在源码仓库修改功能。共享维护说明位于根目录 `CLAUDE.md`，Codex 从 `AGENTS.md` 进入；产品操作接口见 `AGENT-USE.md`。

`origin` 指向个人仓库，`upstream` 指向官方仓库。功能分支通过 PR 合入 `personal`；`main` 保留复制仓库时的官方版本。构建产物安装到共享工作区外，Windows 开发程序目录为 `%LOCALAPPDATA%\Programs\Cage-Local`。

本仓库提供个人修改源码，尚未发布个人安装包。README 中的官方主页、下载与发行链接继续指向上游项目。
