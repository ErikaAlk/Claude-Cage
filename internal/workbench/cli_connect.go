package workbench

import (
	"context"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/x/vt"
)

var cliStartupEscape = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)
var cliOwnedChannel = regexp.MustCompile(`Channels:server:([A-Za-z0-9_-]+?)(?:[❯›>]?1\.|Iamusingthisforlocaldevelopment)`)

type cliStartupScreen struct {
	emulator *vt.Emulator
	drained  chan struct{}
}

func newCLIStartupScreen(columns, rows int) *cliStartupScreen {
	screen := &cliStartupScreen{emulator: vt.NewEmulator(columns, rows), drained: make(chan struct{})}
	go func() {
		defer close(screen.drained)
		if _, err := io.Copy(io.Discard, screen.emulator); err != nil {
			panic(err)
		}
	}()
	return screen
}

func (screen *cliStartupScreen) write(data []byte) {
	if _, err := screen.emulator.Write(data); err != nil {
		panic(err)
	}
}

func (screen *cliStartupScreen) close() {
	if err := screen.emulator.InputPipe().(io.Closer).Close(); err != nil {
		panic(err)
	}
	<-screen.drained
	if err := screen.emulator.Close(); err != nil {
		panic(err)
	}
}

func cliStartupText(data []byte) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, cliStartupEscape.ReplaceAllString(string(data), ""))
}

// Cage already owns this locally generated channel and the user has granted
// access to this project. Only their two startup dialogs can be completed here.
// Tool approvals, external MCP servers and login dialogs remain untouched.
func cliStartupDialog(text, workspace, channel string) string {
	if strings.Contains(text, "WARNING:Loadingdevelopmentchannels") &&
		strings.Contains(text, "Iamusingthisforlocaldevelopment") {
		if match := cliOwnedChannel.FindStringSubmatch(text); len(match) == 2 && match[1] == channel {
			return "channel"
		}
	}
	if workspace != "" && strings.Contains(strings.ToLower(text), "quicksafetycheck:isthisaprojectyoucreatedoroneyoutrust?") &&
		strings.Contains(text, cliStartupText([]byte(workspace))) &&
		strings.Contains(text, "Yes,Itrustthisfolder") {
		return "project"
	}
	return ""
}

func (s *Server) prepareCLIConnection(terminal *terminalSession, id string) {
	terminal.mu.Lock()
	terminal.cliStartupDone = make(chan struct{})
	screen := newCLIStartupScreen(terminal.columns, terminal.rows)
	screen.write(terminal.history)
	terminal.cliScreen = screen
	terminal.mu.Unlock()
	s.activities.Add(1)
	go func() {
		defer s.activities.Done()
		defer func() {
			terminal.mu.Lock()
			terminal.cliScreen = nil
			screen.close()
			terminal.mu.Unlock()
		}()
		defer terminal.finishCLIStartup()
		ctx, cancel := context.WithTimeout(s.lifetimeContext(), 45*time.Second)
		defer cancel()
		s.mobileCLI.mu.Lock()
		currentRecord, err := s.mobileCLI.record(s.profile, id)
		if err != nil {
			s.mobileCLI.mu.Unlock()
			return
		}
		record := *currentRecord
		cliID, config, connection, workspace := record.CLIID, record.GuestConfig, record.Connection, record.GuestWorkspace
		s.mobileCLI.mu.Unlock()
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		seenDialog := ""
		var seenAt time.Time
		completed := map[string]bool{}
		nextCheck := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case <-terminal.done:
				return
			case <-ticker.C:
			}
			if time.Now().After(nextCheck) {
				var current terminalConversation
				if err := s.runCLIChannel(ctx, "view", map[string]string{"cli_id": cliID, "config": config, "connection": connection}, &current); err != nil {
					nextCheck = time.Now().Add(time.Second)
					continue
				}
				if current.ChannelReady || current.CanSend {
					var response struct {
						OK bool `json:"ok"`
					}
					if err := s.runCLIChannel(ctx, "rpc", resumeChannelRequest(&record), &response); err != nil || !response.OK {
						continue
					}
					if !isClientMountPath(record.Workspace) {
						s.serveCodexChannel(terminal, &record, s.codexWindow())
					}
					return
				}
				if current.Active || current.ConnectionNotice != "" {
					return
				}
				nextCheck = time.Now().Add(time.Second)
			}
			terminal.mu.Lock()
			text := cliStartupText([]byte(screen.emulator.String()))
			terminal.mu.Unlock()
			dialog := cliStartupDialog(text, workspace, connection)
			if dialog != seenDialog {
				seenDialog, seenAt = dialog, time.Now()
				continue
			}
			if dialog == "" || completed[dialog] {
				continue
			}
			// The CLI ignores keys during a dialog's opening guard interval.
			if time.Since(seenAt) < time.Second {
				continue
			}
			input := "\r"
			if dialog == "project" && strings.Contains(text, "❯No,exit") {
				input = "\x1b[B\r"
			}
			terminal.mu.Lock()
			if cliStartupDialog(cliStartupText([]byte(screen.emulator.String())), workspace, connection) != dialog {
				terminal.mu.Unlock()
				continue
			}
			err := terminal.enqueueInputLocked(ctx, input)
			terminal.mu.Unlock()
			if err != nil {
				return
			}
			completed[dialog] = true
		}
	}()
}
