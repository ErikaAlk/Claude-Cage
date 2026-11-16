package workbench

import "testing"

func TestCLIStartupOwnedConnectionDialog(t *testing.T) {
	text := "WARNING: Loading development channels Channels: server:c581fbbc-3d01-42f9-953c-585259509fe5 ❯ 1. I am using this for local development"
	if got := cliStartupDialog(cliStartupText([]byte(text)), "/work/project", "c581fbbc-3d01-42f9-953c-585259509fe5"); got != "channel" {
		t.Fatalf("当前会话的通道确认未被识别：%q", got)
	}
}

func TestCLIStartupDialogs(t *testing.T) {
	for _, tc := range []struct{ text, want string }{
		{"\x1b[31mWARNING: Loading development channels\x1b[0m\r\nChannels: server:c581fbbc-3d01-42f9-953c-585259509fe5\r\nI am using this for local development", "channel"},
		{"Quick safety check: is this a project you created or one you trust?\r\n/work/project\r\nYes, I trust this folder", "project"},
		{"Quick safety check: is this a project you created or one you trust? /other/project Yes, I trust this folder", ""},
		{"WARNING: Loading development channels Channels: server:other I am using this for local development", ""},
		{"WARNING: Loading development channels Channels: server:cage-extra 1. I am using this for local development", ""},
		{"WARNING: Loading development channels Channels: server:cage,server:other 1. I am using this for local development", ""},
		{"WARNING: Loading development channels Channels: server:c581fbbc-3d01-42f9-953c-585259509fe5 ❯ 1. I am using this for local development", "channel"},
		{"WARNING: Loading development channels Channels: server:00000000-0000-4000-8000-000000000000 ❯ 1. I am using this for local development", ""},
		{"WARNING: Loading development channels Channels: server:c581fbbc-3d01-42f9-953c-585259509fe5-extra ❯ 1. I am using this for local development", ""},
		{"WARNING: Loading development channels Channels: server:c581fbbc-3d01-42f9-953c-585259509fe5,server:other ❯ 1. I am using this for local development", ""},
		{"Do you want to proceed? 1. Yes 2. No", ""},
		{"New MCP server found in this project: other Use this MCP server", ""},
		{"Do you want to use this API key?", ""},
	} {
		if got := cliStartupDialog(cliStartupText([]byte(tc.text)), "/work/project", "c581fbbc-3d01-42f9-953c-585259509fe5"); got != tc.want {
			t.Errorf("dialog %q: %q, want %q", tc.text, got, tc.want)
		}
	}
}

func TestCLIStartupScreenDiscardsPriorDialogs(t *testing.T) {
	const connection = "c581fbbc-3d01-42f9-953c-585259509fe5"
	for _, prompt := range []string{
		"Do you want to proceed? 1. Yes 2. No",
		"New MCP server found in this project: other Use this MCP server",
		"Do you want to use this API key?",
	} {
		t.Run(prompt, func(t *testing.T) {
			screen := newCLIStartupScreen(100, 30)
			defer screen.close()
			screen.write([]byte("\x1b[2J\x1b[HWARNING: Loading development channels\r\nChannels: server:" + connection + "\r\n1. I am using this for local development"))
			if cliStartupDialog(cliStartupText([]byte(screen.emulator.String())), "/work/project", connection) != "channel" {
				t.Fatal("真实终端模拟器未识别当前通道确认")
			}
			screen.emulator.Resize(120, 35)
			screen.write([]byte("\x1b[H\x1b[J" + prompt))
			if got := cliStartupDialog(cliStartupText([]byte(screen.emulator.String())), "/work/project", connection); got != "" {
				t.Fatalf("已经清除的历史确认影响了当前窗口：%q", got)
			}
		})
	}
}
