set -eu
assets=$(dirname "$(realpath "$0")")
test "$(id -u)" = 0
command -v nft >/dev/null
if ! getent passwd cage-native-ui >/dev/null; then
  useradd --system --no-create-home --shell /usr/sbin/nologin cage-native-ui
fi
install -d -m 0755 /etc/systemd/system/cage-browser.service.d
install -m 0644 "$assets/cage-native-ui.nft" /etc/claude-cage/cage-native-ui.nft
install -m 0644 "$assets/cage-native-ui-firewall.service" /etc/systemd/system/cage-native-ui-firewall.service
install -m 0644 "$assets/cage-popup-layout.socket" "$assets/cage-popup-layout@.service" /etc/systemd/system/
install -m 0644 "$assets/cage-popup-layout.conf" /etc/tmpfiles.d/
install -m 0644 "$assets/browser.conf" /etc/systemd/system/cage-browser.service.d/native-popup.conf
install -m 0644 "$assets/browser-popup-layout.mjs" /usr/local/lib/browser-popup-layout.mjs
if systemctl is-active --quiet cage-native-ui.service; then
  systemctl stop cage-native-ui.service
fi
systemd-tmpfiles --create /etc/tmpfiles.d/cage-popup-layout.conf
systemctl daemon-reload
systemctl start cage-native-ui-firewall.service
systemctl enable --now cage-popup-layout.socket
if ! ss -ltn 'sport = :19223' | grep -q 19223; then
  systemctl restart cage-browser.service
fi
