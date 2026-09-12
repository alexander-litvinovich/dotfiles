package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-telegram/bot"
	"github.com/muesli/termenv"
)

func plainRenderer() *lipgloss.Renderer {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.Ascii)
	return r
}

func styledRenderer() *lipgloss.Renderer {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.TrueColor)
	return r
}

func wizardForTest(t *testing.T, app application, cfg config, resolved settings) (wizard, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	w := newWizard(app, context.Background(), path, cfg, resolved)
	// Deliver the successful probe so the wizard reaches its entry state,
	// the same way the real program does at startup.
	next, _ := w.Update(apiProbeMsg{})
	w, ok := next.(wizard)
	if !ok {
		t.Fatalf("unexpected model type %T", next)
	}
	return w, path
}

func step(t *testing.T, m tea.Model, msg tea.Msg) (wizard, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	w, ok := next.(wizard)
	if !ok {
		t.Fatalf("unexpected model type %T", next)
	}
	return w, cmd
}

func transcriptOf(w wizard) string { return strings.Join(w.log, "\n\n") }

func TestRenderChatMessage(t *testing.T) {
	at := time.Date(2026, 9, 12, 23, 28, 0, 0, time.UTC)

	// Wide terminal: clock aligns to the 60-char text area, not the terminal edge.
	got := renderChatMessage(chatMsg{from: "TN", body: "Hello", at: at, status: statusOK}, 80, plainRenderer())
	want := "TN:" + strings.Repeat(" ", 50) + "23:28 ✓\nHello"
	if got != want {
		t.Fatalf("aligned header:\n got %q\nwant %q", got, want)
	}

	// Unmeasured width (first message before WindowSizeMsg): same 60-char area.
	got = renderChatMessage(chatMsg{from: "TN", body: "Hello", at: at, status: statusOK}, 0, plainRenderer())
	if got != want {
		t.Fatalf("unmeasured header:\n got %q\nwant %q", got, want)
	}

	// Very wide terminal: body still wraps at 60.
	long := "Setup is complete, but the test message didn't arrive because of a rather long reason text"
	got = renderChatMessage(chatMsg{from: "TN", body: long, at: at, status: statusNone}, 100, plainRenderer())
	if lines := strings.Split(got, "\n"); len(lines) < 3 {
		t.Fatalf("body did not wrap at 60: %q", got)
	} else {
		for i, line := range lines {
			if lipgloss.Width(line) > 60 {
				t.Fatalf("line %d exceeds 60 chars: %q", i, line)
			}
		}
	}

	// Narrow terminal: compact inline header.
	got = renderChatMessage(chatMsg{from: "You", body: "•••", at: at, status: statusOK}, 30, plainRenderer())
	if !strings.HasPrefix(got, "You: 23:28 ✓\n") {
		t.Fatalf("compact header %q", got)
	}

	// Narrow terminal: wrap at terminal width.
	wrapped := renderChatMessage(chatMsg{from: "TN", body: "https://t.me/someverylongbotname", at: at, status: statusWarn}, 20, plainRenderer())
	for i, line := range strings.Split(wrapped, "\n") {
		if lipgloss.Width(line) > 20 {
			t.Fatalf("line %d exceeds width: %q", i, line)
		}
	}
	if !strings.Contains(wrapped, "!") {
		t.Fatalf("missing warn glyph: %q", wrapped)
	}
}

func TestRenderChatMessageStyles(t *testing.T) {
	at := time.Date(2026, 9, 12, 23, 28, 0, 0, time.UTC)
	msg := chatMsg{from: "TN", body: "Hello", at: at, status: statusOK}

	if got := renderChatMessage(msg, 50, styledRenderer()); !strings.Contains(got, "\x1b[") {
		t.Fatalf("styled output has no ANSI: %q", got)
	}
	if got := renderChatMessage(msg, 50, plainRenderer()); strings.Contains(got, "\x1b") {
		t.Fatalf("styles-disabled output has ANSI: %q", got)
	}
}

func TestHyperlink(t *testing.T) {
	url := "https://t.me/BotFather"

	styled := wizard{out: styledRenderer()}
	got := styled.hyperlink(url)
	if !strings.Contains(got, "\x1b]8;;"+url+"\x1b\\") || !strings.Contains(got, url) {
		t.Fatalf("styled hyperlink %q", got)
	}
	if !strings.Contains(got, "\x1b[3") || !strings.Contains(got, "96m") {
		t.Fatalf("link is not italic bright cyan: %q", got)
	}

	plain := wizard{out: plainRenderer()}
	if got := plain.hyperlink(url); got != url {
		t.Fatalf("plain hyperlink %q", got)
	}
}

func TestFormatRemaining(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{d: 5*time.Minute - time.Second, want: "4:59"},
		{d: 90 * time.Second, want: "1:30"},
		{d: 0, want: "0:00"},
		{d: -time.Second, want: "0:00"},
	}
	for _, tt := range tests {
		if got := formatRemaining(tt.d); got != tt.want {
			t.Fatalf("formatRemaining(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestVersionBlock(t *testing.T) {
	app := testApplication(nil)
	w, _ := wizardForTest(t, app, config{}, settings{})

	got := w.versionBlock()
	if !strings.HasPrefix(got, "\n") {
		t.Fatalf("missing leading blank line: %q", got)
	}
	lines := strings.Split(got, "\n")[1:]
	if len(lines) != 3 {
		t.Fatalf("expected a boxed banner, got %q", got)
	}
	if lipgloss.Width(lines[0]) != maxBodyWidth {
		t.Fatalf("box width %d: %q", lipgloss.Width(lines[0]), lines[0])
	}
	want := strings.Repeat(" ", 18) + "Telegram Notifier v" + version + strings.Repeat(" ", 18)
	if lines[1] != "│"+want+"│" {
		t.Fatalf("banner not centered: %q", lines[1])
	}

	w.width = 40
	if narrow := strings.Split(w.versionBlock(), "\n")[1:]; lipgloss.Width(narrow[0]) != 40 {
		t.Fatalf("narrow box width %d", lipgloss.Width(narrow[0]))
	}
}

func enterToken(t *testing.T, w wizard, value string) wizard {
	t.Helper()
	w.input.SetValue(value)
	model, _ := w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next, ok := model.(wizard)
	if !ok {
		t.Fatalf("unexpected model type %T", model)
	}
	return next
}

func TestSubmitTokenMasksAndChecks(t *testing.T) {
	app := testApplication(nil)
	w, _ := wizardForTest(t, app, config{}, settings{})

	w = enterToken(t, w, "secret-token")
	if w.phase != phaseTokenCheck || w.token != "secret-token" || !w.persist {
		t.Fatalf("phase=%v token=%q persist=%v", w.phase, w.token, w.persist)
	}
	log := transcriptOf(w)
	if !strings.Contains(log, "••••••••••••") || !strings.Contains(log, "Got it. Checking...") {
		t.Fatalf("transcript %q", log)
	}
	if strings.Contains(log, "secret-token") {
		t.Fatalf("clear token in transcript %q", log)
	}
	if got := app.redact("x secret-token y"); got != "x [REDACTED] y" {
		t.Fatalf("token not remembered for redaction: %q", got)
	}
}

func TestSubmitTokenEmpty(t *testing.T) {
	app := testApplication(nil)
	w, _ := wizardForTest(t, app, config{}, settings{})

	w = enterToken(t, w, "  ")
	if w.phase != phaseTokenInput || !strings.Contains(transcriptOf(w), "Bot token cannot be empty.") {
		t.Fatalf("phase=%v transcript %q", w.phase, transcriptOf(w))
	}
}

func TestTokenCheckedInvalidReturnsToInput(t *testing.T) {
	app := testApplication(nil)
	w, _ := wizardForTest(t, app, config{}, settings{})
	w = enterToken(t, w, "secret-token")

	w, _ = step(t, w, tokenCheckedMsg{err: fmt.Errorf("%w: Telegram rejected secret-token", bot.ErrorUnauthorized)})
	if w.phase != phaseTokenInput || w.exit != 0 {
		t.Fatalf("phase=%v exit=%d", w.phase, w.exit)
	}
	log := transcriptOf(w)
	if !strings.Contains(log, "That token didn't work. Check if you copied it in full.") {
		t.Fatalf("transcript %q", log)
	}
	if strings.Contains(log, "secret-token") {
		t.Fatalf("clear token in transcript %q", log)
	}
}

func TestTokenCheckedNetworkErrorFails(t *testing.T) {
	app := testApplication(nil)
	w, _ := wizardForTest(t, app, config{}, settings{})
	w = enterToken(t, w, "secret-token")

	w, _ = step(t, w, tokenCheckedMsg{err: errors.New("dial tcp: no route to host")})
	if w.exit != 1 || w.phase != phaseDone {
		t.Fatalf("exit=%d phase=%v", w.exit, w.phase)
	}
	if !strings.Contains(transcriptOf(w), "Couldn't reach Telegram") {
		t.Fatalf("transcript %q", transcriptOf(w))
	}
}

func TestTokenCheckedSavesTokenAndUsername(t *testing.T) {
	startedAt := time.Date(2026, 9, 12, 23, 28, 0, 0, time.UTC)
	app := testApplication(nil)
	app.now = func() time.Time { return startedAt }
	w, path := wizardForTest(t, app, config{}, settings{})
	w = enterToken(t, w, "good-token")

	w, _ = step(t, w, tokenCheckedMsg{username: "mybot"})
	if w.phase != phaseWaitStart || !w.deadline.Equal(startedAt.Add(startWaitWindow)) {
		t.Fatalf("phase=%v deadline=%v", w.phase, w.deadline)
	}
	log := transcriptOf(w)
	if !strings.Contains(log, "Bot connected: @mybot") || !strings.Contains(log, "I'll wait for 5 minutes.") {
		t.Fatalf("transcript %q", log)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BotToken != "good-token" || cfg.BotUsername != "mybot" || cfg.ChatID != nil {
		t.Fatalf("saved config %#v", cfg)
	}
	if !strings.Contains(w.View(), "Waiting 5:00") {
		t.Fatalf("live view %q", w.View())
	}
}

func TestWizardWithSavedTokenLearnsMissingChat(t *testing.T) {
	app := testApplication(nil)
	cfg := config{BotToken: "saved-token"}
	w, path := wizardForTest(t, app, cfg, settings{token: "saved-token"})
	if w.phase != phaseTokenCheck || !w.persist {
		t.Fatalf("phase=%v persist=%v", w.phase, w.persist)
	}

	w, _ = step(t, w, tokenCheckedMsg{username: "savedbot"})
	w, _ = step(t, w, startReceivedMsg{chatID: 88})
	if w.phase != phaseFinishing {
		t.Fatalf("phase=%v", w.phase)
	}
	saved, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if saved.BotToken != "saved-token" || saved.BotUsername != "savedbot" || saved.ChatID == nil || saved.ChatID.value != int64(88) {
		t.Fatalf("saved config %#v", saved)
	}
}

func TestWizardDoesNotPersistEnvironmentToken(t *testing.T) {
	app := testApplication(nil)
	w, path := wizardForTest(t, app, config{}, settings{token: "env-token"})
	if w.persist {
		t.Fatal("env token must not persist")
	}

	w, _ = step(t, w, tokenCheckedMsg{username: "envbot"})
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("config written before /start")
	}
	w, _ = step(t, w, startReceivedMsg{chatID: 89})
	saved, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if saved.BotToken != "" || saved.ChatID == nil || saved.ChatID.value != int64(89) {
		t.Fatalf("saved config %#v", saved)
	}
}

func TestWizardWithSavedChatSkipsStart(t *testing.T) {
	app := testApplication(nil)
	cfg := config{ChatID: &chatTarget{value: int64(99)}}
	w, path := wizardForTest(t, app, cfg, settings{chatID: cfg.ChatID})
	w = enterToken(t, w, "entered-token")

	w, _ = step(t, w, tokenCheckedMsg{username: "mybot"})
	if w.phase != phaseFinishing {
		t.Fatalf("phase=%v", w.phase)
	}
	if !strings.Contains(transcriptOf(w), "Bot connected: @mybot.") {
		t.Fatalf("transcript %q", transcriptOf(w))
	}
	saved, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if saved.BotToken != "entered-token" || saved.BotUsername != "mybot" || saved.ChatID.value != int64(99) {
		t.Fatalf("saved config %#v", saved)
	}
}

func TestStartReceivedTimeoutRetryAndCtrlC(t *testing.T) {
	base := time.Date(2026, 9, 12, 21, 22, 0, 0, time.UTC)
	calls := 0
	app := testApplication(nil)
	app.now = func() time.Time {
		calls++
		return base.Add(time.Duration(calls) * time.Minute)
	}
	w, _ := wizardForTest(t, app, config{}, settings{})
	w = enterToken(t, w, "good-token")
	w, _ = step(t, w, tokenCheckedMsg{username: "mybot"})
	firstDeadline := w.deadline

	w, _ = step(t, w, startReceivedMsg{err: context.DeadlineExceeded})
	if w.phase != phaseWaitRetry || !strings.Contains(transcriptOf(w), "I didn't receive /start.") {
		t.Fatalf("phase=%v transcript %q", w.phase, transcriptOf(w))
	}

	w, _ = step(t, w, tea.KeyMsg{Type: tea.KeyEnter})
	if w.phase != phaseWaitStart || !w.deadline.After(firstDeadline) {
		t.Fatalf("phase=%v deadline=%v first=%v", w.phase, w.deadline, firstDeadline)
	}

	w, _ = step(t, w, tea.KeyMsg{Type: tea.KeyCtrlC})
	if w.exit != 130 {
		t.Fatalf("exit=%d", w.exit)
	}
}

func TestStartReceivedNetworkErrorKeepsToken(t *testing.T) {
	app := testApplication(nil)
	w, path := wizardForTest(t, app, config{}, settings{})
	w = enterToken(t, w, "good-token")
	w, _ = step(t, w, tokenCheckedMsg{username: "mybot"})

	w, _ = step(t, w, startReceivedMsg{err: errors.New("connection reset")})
	if w.exit != 1 || w.phase != phaseDone {
		t.Fatalf("exit=%d phase=%v", w.exit, w.phase)
	}
	saved, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if saved.BotToken != "good-token" || saved.ChatID != nil {
		t.Fatalf("saved config %#v", saved)
	}
}

func TestSetupFinishedTestFailureExitsNonZero(t *testing.T) {
	app := testApplication(nil)
	w, _ := wizardForTest(t, app, config{}, settings{})

	w, _ = step(t, w, setupFinishedMsg{testErr: errors.New("400: chat not found"), onPath: true})
	if w.exit != 1 || w.phase != phaseDone {
		t.Fatalf("exit=%d phase=%v", w.exit, w.phase)
	}
	log := transcriptOf(w)
	if !strings.Contains(log, "the test message didn't arrive") || !strings.Contains(log, "400: chat not found") {
		t.Fatalf("transcript %q", log)
	}
	if !strings.Contains(log, "telegram-notify is ready to use:") {
		t.Fatalf("PATH result missing: %q", log)
	}
}

func TestSetupFinishedPathMissingShowsSnippet(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{
			name: "zsh",
			env:  map[string]string{shellEnv: "/bin/zsh", binDirEnv: "/Users/x/dev/dotfiles/bin"},
			want: []string{"~/.zshrc", `export PATH="/Users/x/dev/dotfiles/bin:$PATH"`},
		},
		{
			name: "fish",
			env:  map[string]string{shellEnv: "/usr/bin/fish", binDirEnv: "/Users/x/dev/dotfiles/bin"},
			want: []string{"~/.config/fish/config.fish", "fish_add_path /Users/x/dev/dotfiles/bin"},
		},
		{
			name: "unknown shell",
			env:  map[string]string{shellEnv: "/bin/tcsh", binDirEnv: "/Users/x/dev/dotfiles/bin"},
			want: []string{"Add telegram-notify's directory to your PATH"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := testApplication(tt.env)
			w, _ := wizardForTest(t, app, config{}, settings{})

			w, _ = step(t, w, setupFinishedMsg{onPath: false})
			if w.exit != 0 {
				t.Fatalf("exit=%d", w.exit)
			}
			log := transcriptOf(w)
			for _, want := range append(tt.want, usageExamples) {
				if !strings.Contains(log, want) {
					t.Fatalf("transcript missing %q: %s", want, log)
				}
			}
		})
	}
}

func TestCtrlCWorksInEveryPhase(t *testing.T) {
	for _, phase := range []wizardPhase{phaseProbe, phaseTokenInput, phaseTokenCheck, phaseWaitStart, phaseWaitRetry, phaseFinishing, phaseDone} {
		app := testApplication(nil)
		w, _ := wizardForTest(t, app, config{}, settings{})
		w.phase = phase

		w, _ = step(t, w, tea.KeyMsg{Type: tea.KeyCtrlC})
		if w.exit != 130 {
			t.Fatalf("phase %v: exit=%d", phase, w.exit)
		}
	}
}

func TestWizardCmds(t *testing.T) {
	app := testApplication(nil)
	w, _ := wizardForTest(t, app, config{}, settings{})
	w.token = "cmd-token"

	if msg := w.checkTokenCmd()().(tokenCheckedMsg); msg.username != "testbot" || msg.err != nil {
		t.Fatalf("checkTokenCmd %#v", msg)
	}

	var gotStartedAt time.Time
	app.learn = func(_ context.Context, token string, startedAt time.Time) (int64, error) {
		if token != "cmd-token" {
			t.Fatalf("learn token %q", token)
		}
		gotStartedAt = startedAt
		return 55, nil
	}
	w.app = app
	if msg := w.waitStartCmd()().(startReceivedMsg); msg.chatID != 55 || msg.err != nil {
		t.Fatalf("waitStartCmd %#v", msg)
	}
	if gotStartedAt.IsZero() {
		t.Fatal("learn was not called")
	}

	var sent []any
	app.send = func(_ context.Context, token string, chatID any, message string) error {
		sent = []any{token, chatID, message}
		return nil
	}
	w.app = app
	if msg := w.finishCmd(int64(55))().(setupFinishedMsg); msg.testErr != nil || !msg.onPath {
		t.Fatalf("finishCmd %#v", msg)
	}
	wantSent := []any{"cmd-token", int64(55), testMessageText}
	if fmt.Sprint(sent) != fmt.Sprint(wantSent) {
		t.Fatalf("sent %#v, want %#v", sent, wantSent)
	}

	app.lookPath = func(string) (string, error) { return "", errors.New("not found") }
	w.app = app
	if msg := w.finishCmd(int64(55))().(setupFinishedMsg); msg.onPath {
		t.Fatal("lookPath failure must report missing PATH")
	}
}

func TestOfflineScreen(t *testing.T) {
	app := testApplication(nil)
	path := filepath.Join(t.TempDir(), "config.json")
	w := newWizard(app, context.Background(), path, config{}, settings{})

	next, _ := w.Update(apiProbeMsg{err: errors.New("dial tcp: no route to host")})
	w, ok := next.(wizard)
	if !ok {
		t.Fatalf("unexpected model type %T", next)
	}
	if w.exit != 1 || w.phase != phaseDone {
		t.Fatalf("exit=%d phase=%v", w.exit, w.phase)
	}
	log := transcriptOf(w)
	for _, want := range []string{
		"Telegram Notifier v" + version,
		"The Telegram API seems to be unreachable",
		"telegram-notify --set-token TOKEN",
		"telegram-notify --set-chat-id ID",
		"run telegram-notify again",
		"Usage:",
		"--prompt",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("offline transcript missing %q", want)
		}
	}
	if strings.Contains(log, "Hey! Let's set things up.") {
		t.Errorf("greeting shown despite offline: %s", log)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Error("config written while offline")
	}
}

func TestProbeCmd(t *testing.T) {
	called := false
	app := testApplication(nil)
	app.checkAPI = func(context.Context) error {
		called = true
		return errors.New("offline")
	}
	w := newWizard(app, context.Background(), filepath.Join(t.TempDir(), "config.json"), config{}, settings{})

	if msg := w.probeCmd()().(apiProbeMsg); msg.err == nil || !called {
		t.Fatalf("probeCmd %#v called=%v", msg, called)
	}
}

func TestWizardSmokeOffline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	var out bytes.Buffer
	app := testApplication(nil)
	app.stdout = &out
	app.checkAPI = func(context.Context) error { return errors.New("dial tcp: no route") }

	w := newWizard(app, context.Background(), path, config{}, settings{})
	program := tea.NewProgram(w, tea.WithInput(strings.NewReader("")), tea.WithOutput(&out))
	final, err := program.Run()
	if err != nil {
		t.Fatal(err)
	}
	if fw, ok := final.(wizard); !ok || fw.exit != 1 {
		t.Fatalf("final model exit=%d", fw.exit)
	}

	outStr := out.String()
	for _, want := range []string{"The Telegram API seems to be unreachable", "telegram-notify --set-token TOKEN", "Usage:"} {
		if !strings.Contains(outStr, want) {
			t.Errorf("output missing %q", want)
		}
	}
	if strings.Contains(outStr, "Hey! Let's set things up.") {
		t.Errorf("greeting shown despite offline: %q", outStr)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Error("config written while offline")
	}
}

// delayedInput feeds keys to a wizard program after the startup probe has
// completed, so nothing is consumed by the probe phase.
func delayedInput(keys string) io.Reader {
	reader, writer := io.Pipe()
	go func() {
		time.Sleep(500 * time.Millisecond)
		writer.Write([]byte(keys))
		writer.Close()
	}()
	return reader
}

func TestCountdownTicksDuringWait(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	var out bytes.Buffer
	app := testApplication(nil)
	app.stdout = &out
	app.validateToken = func(context.Context, string) (string, error) { return "mybot", nil }
	app.learn = func(context.Context, string, time.Time) (int64, error) {
		time.Sleep(2500 * time.Millisecond)
		return 77, nil
	}
	app.send = func(context.Context, string, any, string) error { return nil }

	w := newWizard(app, context.Background(), path, config{}, settings{})
	program := tea.NewProgram(w, tea.WithInput(delayedInput("secret-token\r")), tea.WithOutput(&out))
	if _, err := program.Run(); err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	for _, match := range regexp.MustCompile(`Waiting \d:\d\d`).FindAllString(out.String(), -1) {
		seen[match] = true
	}
	if len(seen) < 2 {
		t.Fatalf("countdown did not tick, values seen: %v", seen)
	}
}

func TestWizardSmokeRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	var out bytes.Buffer
	app := testApplication(nil)
	app.stdout = &out
	app.validateToken = func(context.Context, string) (string, error) { return "mybot", nil }
	app.learn = func(context.Context, string, time.Time) (int64, error) { return 77, nil }
	app.send = func(context.Context, string, any, string) error { return nil }

	w := newWizard(app, context.Background(), path, config{}, settings{})
	program := tea.NewProgram(w, tea.WithInput(delayedInput("secret-token\r")), tea.WithOutput(&out))
	final, err := program.Run()
	if err != nil {
		t.Fatal(err)
	}
	if fw, ok := final.(wizard); !ok || fw.exit != 0 {
		t.Fatalf("final model exit=%d", fw.exit)
	}

	outStr := out.String()
	for _, want := range []string{
		"Telegram Notifier v0.1",
		"─",
		"Hey! Let's set things up.",
		"Got it. Checking...",
		"Bot connected: @mybot",
		"I'll wait for 5 minutes.",
		"Chat connected.",
		"Setup complete. I sent a test message to your chat.",
		"telegram-notify is ready to use:",
		`telegram-notify --text "Hello from terminal"`,
	} {
		if !strings.Contains(outStr, want) {
			t.Errorf("output missing %q", want)
		}
	}
	if strings.Contains(outStr, "secret-token") {
		t.Errorf("token leaked into output: %q", outStr)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BotToken != "secret-token" || cfg.BotUsername != "mybot" || cfg.ChatID == nil || cfg.ChatID.value != int64(77) {
		t.Fatalf("saved config %#v", cfg)
	}
}
