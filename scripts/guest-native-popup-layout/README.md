# Linux 扩展弹窗定位

Linux 客机使用 Chromium kiosk 模式。原生扩展弹窗通过独立服务保持在画面右侧，同时保留扩展 POPUP 上下文和原生输入行为；普通标签列表只接收具有真实 tabId 的页面。

安装前，将本目录文件和 `internal/cage/browser-popup-layout.mjs` 传入客机准备目录，安装 Debian/Ubuntu 的 `nftables`。使用 root 执行：

```text
sh install.sh <本目录文件所在目录> <browser-popup-layout.mjs 的路径>
```

安装过程会重启浏览器服务，网页与扩展的持久资料继续保留。`cage-native-ui-firewall.service` 在浏览器启动前配置防火墙；`19223` 仅监听 IPv4 回环，且只允许专用 `cage-native-ui` 用户主动连接。该用户仅运行布局服务，不运行代理或交互会话。网页和其他客机用户均不能访问原生控制接口。网页 CDP 继续通过继承 pipe 连接。

布局服务通过 Chromium UI DevTools 的原生 Widget 元数据更新弹窗位置，按 DOM 事件增量维护节点树，在画面尺寸改变后重新定位。`/run/cage-native-ui/extension-anchor.json` 保存按钮相对于画面的右侧位置，由浏览器控制器原子替换；不包含账户资料。未传入位置的调用将弹窗放置在画面右侧。

运行状态：`systemctl status cage-native-ui.service cage-native-ui-firewall.service`。所有运行资产安装在 `/usr/local/lib`、`/etc/claude-cage` 和 systemd 目录中。
