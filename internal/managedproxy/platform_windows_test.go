package managedproxy

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureRunningRestoresBundledWindowsCore(t *testing.T) {
	if os.Getenv("CAGE_PROXY_INTEGRATION") != "1" {
		t.Skip("requires the bundled Mihomo")
	}
	profile := t.TempDir()
	store, err := Load(profile)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	store.Port = listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Import("local", "", []byte("proxies: [{name: local, type: socks5, server: 127.0.0.1, port: 1080}]"), "local", 0, ""); err != nil {
		t.Fatal(err)
	}
	store.Selected = store.Sources[0].Nodes[0].ID
	t.Cleanup(func() {
		if err := Stop(profile); err != nil {
			t.Error(err)
		}
	})
	for attempt := 0; attempt < 2; attempt++ {
		if Running(t.Context(), profile, store) {
			t.Fatal("expected a stopped core before recovery")
		}
		if err := EnsureRunning(t.Context(), profile, store); err != nil {
			t.Fatal(err)
		}
		if err := checkRunning(t.Context(), profile, store); err != nil {
			t.Fatal(err)
		}
		if err := Stop(profile); err != nil {
			t.Fatal(err)
		}
	}
}

// A PID reused after a reboot must neither be terminated nor block startup.
func TestStopWindowsCoreForgetsReusedPID(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	core := filepath.Join(filepath.Dir(exe), "mihomo.exe")
	if err := os.WriteFile(core, []byte("placeholder"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(core) })
	for name, pid := range map[string]uint32{"same user": uint32(os.Getpid()), "system": 4} {
		t.Run(name, func(t *testing.T) {
			profile := t.TempDir()
			b, _ := json.Marshal(windowsCoreRecord{PID: pid, Executable: core, Created: 1})
			if err := Write(windowsCoreRecordPath(profile), b); err != nil {
				t.Fatal(err)
			}
			if err := stopWindowsCore(profile); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(windowsCoreRecordPath(profile)); !os.IsNotExist(err) {
				t.Fatal("stale core record was kept")
			}
		})
	}
}
