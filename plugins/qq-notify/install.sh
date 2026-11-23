set -eu
: "${1:?请指定 Robin 源码目录或软件包路径}"
root="$HOME/.local/share/cage/qq-notify"
umask 077
mkdir -p "$root/build"
export TMPDIR="$root/build"
python3 -m venv "$root/venv"
"$root/venv/bin/pip" install "$1"
cd "$(dirname "$0")"
sudo install -D -m 0755 stop.sh /usr/local/libexec/cage-qq-stop.sh
sudo install -D -m 0644 managed.json /etc/claude-code/managed-settings.d/80-cage-qq-notify.json
