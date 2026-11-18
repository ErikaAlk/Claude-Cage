set -eu
assets=$1
controller=$2
test "$(id -u)" = 0
command -v nft >/dev/null
if ! getent passwd cage-native-ui >/dev/null; then
  useradd --system --no-create-home --shell /usr/sbin/nologin cage-native-ui
fi
install -d -m 0755 /etc/systemd/system/cage-browser.service.d
install -m 0644 "$assets/cage-native-ui.nft" /etc/claude-cage/cage-native-ui.nft
install -m 0644 "$assets/extension-anchor.json" /etc/claude-cage/extension-anchor.json
install -m 0644 "$assets/cage-native-ui-firewall.service" /etc/systemd/system/cage-native-ui-firewall.service
install -m 0644 "$assets/cage-native-ui.service" /etc/systemd/system/cage-native-ui.service
install -m 0644 "$assets/browser.conf" /etc/systemd/system/cage-browser.service.d/native-popup.conf
install -m 0644 "$controller" /usr/local/lib/browser-popup-layout.mjs
systemctl daemon-reload
systemctl restart cage-native-ui-firewall.service
systemctl restart cage-browser.service
