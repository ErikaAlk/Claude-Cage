# QQ 完成通知

Cage 客机中的 Claude Code 完成一个主会话回合后，Robin 会将项目名称、最终回复和会话编号发送到绑定的 QQ。长文按 Markdown 分条，网络中断时保存在客机私有队列，恢复连接后继续发送。插件停用期间完成的回合不会加入队列。

将本目录复制到宿主机 Cage 配置目录的 `plugins/qq-notify`。在 Debian 客机以运行 Claude 的用户执行 `sh install.sh /路径/Robin`，需要 Python 3.12 或更新版本、venv 和 sudo。安装程序为该用户安装 Robin 软件包，并将 Stop 回调放进 Claude Code 的系统设置目录。

在 Cage 插件页面重新加载，填写 Robin 的 HTTPS 服务地址和专用 notify 凭据，然后启用插件。设置中的密钥由 Cage 加密保存。队列位于客机的 `~/.local/share/cage/qq-notify/queue.sqlite3`。同一会话、同一完成记录只投递一次，QQ 返回结果未知的记录保留供核查。

在客机中运行以下命令检查队列。状态显示各类投递数量，暂停时还会显示 Robin 返回的 HTTP 状态码。修正服务地址、凭据或对应请求问题后，运行同一命令并将末尾的 `status` 改为 `resume`，即可继续投递待发记录。

```sh
~/.local/share/cage/qq-notify/venv/bin/python -I ~/.local/share/plugins/qq-notify/notify.py status
```

卸载时可移除客机内的 `/etc/claude-code/managed-settings.d/80-cage-qq-notify.json` 和 `/usr/local/libexec/cage-qq-stop.sh`。
