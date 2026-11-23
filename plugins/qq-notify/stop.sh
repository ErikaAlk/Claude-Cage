set -eu
root="$HOME/.local/share/cage/qq-notify"
[ -f "$root/active.lock" ] || exit 0
# 插件运行期间持有独占锁。停用后直接退出。
if flock -n "$root/active.lock" true; then exit 0; fi
exec "$root/venv/bin/python" -I "$HOME/.local/share/plugins/qq-notify/notify.py" enqueue
