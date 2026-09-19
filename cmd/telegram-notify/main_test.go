package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
)

func TestRunSendsMessageToEnvironmentChat(t *testing.T) {
	var gotToken, gotMessage string
	var gotChatID any
	app := testApplication(map[string]string{
		botTokenEnv: "secret",
		chatIDEnv:   "-123",
	})
	app.send = func(_ context.Context, token string, chatID any, message outgoing) (int, error) {
		gotToken, gotChatID, gotMessage = token, chatID, message.text
		return 1, nil
	}

	if err := app.run(context.Background(), []string{"--text", "job finished"}); err != nil {
		t.Fatal(err)
	}
	if gotToken != "secret" || gotChatID != int64(-123) || gotMessage != "job finished" {
		t.Fatalf("unexpected send: token=%q chat=%v message=%q", gotToken, gotChatID, gotMessage)
	}
}

func TestRunUsesSavedChat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := saveConfig(path, config{ChatID: &chatTarget{value: int64(456)}}); err != nil {
		t.Fatal(err)
	}

	var gotChatID any
	app := testApplication(map[string]string{botTokenEnv: "secret"})
	app.configPath = func() (string, error) { return path, nil }
	app.send = func(_ context.Context, _ string, chatID any, _ outgoing) (int, error) {
		gotChatID = chatID
		return 1, nil
	}

	if err := app.run(context.Background(), []string{"--text", "done"}); err != nil {
		t.Fatal(err)
	}
	if gotChatID != int64(456) {
		t.Fatalf("got chat ID %v", gotChatID)
	}
}

func TestRunAllowsChannelUsername(t *testing.T) {
	app := testApplication(map[string]string{botTokenEnv: "secret", chatIDEnv: "@alerts"})
	app.send = func(_ context.Context, _ string, chatID any, _ outgoing) (int, error) {
		if chatID != "@alerts" {
			t.Fatalf("got chat ID %v", chatID)
		}
		return 1, nil
	}
	if err := app.run(context.Background(), []string{"--text", "done"}); err != nil {
		t.Fatal(err)
	}
}

func TestRunValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "legacy positional send", args: []string{"done"}, want: "unexpected positional arguments"},
		{name: "legacy prompt", args: []string{"--prompt", "question"}, want: "unexpected positional arguments"},
		{name: "prompt without text", args: []string{"--prompt"}, want: "requires --text"},
		{name: "button without prompt", args: []string{"--text", "question", "--button", "Yes"}, want: "requires --prompt"},
		{name: "empty text", args: []string{"--text", ""}, want: "non-empty"},
		{name: "learn chat override", args: []string{"--learn", "--chat-id", "42"}, want: "cannot be combined"},
		{name: "invalid chat", args: []string{"--text", "done", "--token", "secret", "--chat-id", "somewhere"}, want: "integer or an @channel username"},
		{name: "empty set token", args: []string{"--set-token", ""}, want: "non-empty"},
		{name: "zero set chat", args: []string{"--set-chat-id", "0"}, want: "must not be zero"},
		{name: "set with send", args: []string{"--set-token", "secret", "--text", "done"}, want: "cannot be combined"},
		{name: "set with override", args: []string{"--set-chat-id", "42", "--token", "secret"}, want: "cannot be combined"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := testApplication(nil)
			err := app.run(context.Background(), tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want error containing %q", err, tt.want)
			}
		})
	}
}

func TestRunLearnSavesChatAndConfirms(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	startedAt := time.Unix(100, 0)
	var sent []any
	app := testApplication(map[string]string{botTokenEnv: "secret"})
	app.now = func() time.Time { return startedAt }
	app.configPath = func() (string, error) { return path, nil }
	app.validateToken = func(_ context.Context, token string) (string, error) {
		if token != "secret" {
			t.Fatalf("unexpected token %q", token)
		}
		return "learnbot", nil
	}
	app.learn = func(_ context.Context, token string, gotStartedAt time.Time) (int64, error) {
		if token != "secret" || !gotStartedAt.Equal(startedAt) {
			t.Fatal("unexpected learn arguments")
		}
		return 789, nil
	}
	app.send = func(_ context.Context, token string, chatID any, message outgoing) (int, error) {
		sent = []any{token, chatID, message.text}
		return 1, nil
	}

	if err := app.run(context.Background(), []string{"--learn"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ChatID == nil || cfg.ChatID.value != int64(789) {
		t.Fatalf("got saved chat ID %#v", cfg.ChatID)
	}
	wantSent := []any{"secret", int64(789), testMessageText}
	if !reflect.DeepEqual(sent, wantSent) {
		t.Fatalf("got send %#v, want %#v", sent, wantSent)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("got config permissions %o", info.Mode().Perm())
	}
}

func TestSendNotConfiguredReportsMissing(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		args []string
		want string
	}{
		{
			name: "nothing set",
			args: []string{"--text", "hi"},
			want: "not configured: missing bot token and chat ID",
		},
		{
			name: "token only",
			env:  map[string]string{botTokenEnv: "secret"},
			args: []string{"--text", "hi"},
			want: "not configured: missing chat ID",
		},
		{
			name: "chat only",
			env:  map[string]string{chatIDEnv: "42"},
			args: []string{"--text", "hi", "--prompt"},
			want: "not configured: missing bot token",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout strings.Builder
			app := testApplication(tt.env)
			app.stdout = &stdout

			err := app.run(context.Background(), tt.args)
			var silent silentExitError
			if !errors.As(err, &silent) || silent.code != 1 {
				t.Fatalf("got %v, want silent exit 1", err)
			}
			if !strings.Contains(stdout.String(), tt.want) {
				t.Fatalf("stdout %q, want %q", stdout.String(), tt.want)
			}
		})
	}
}

func TestLearnNotConfiguredReportsMissingToken(t *testing.T) {
	var stdout strings.Builder
	app := testApplication(map[string]string{chatIDEnv: "42"})
	app.stdout = &stdout

	err := app.run(context.Background(), []string{"--learn"})
	var silent silentExitError
	if !errors.As(err, &silent) || silent.code != 1 {
		t.Fatalf("got %v, want silent exit 1", err)
	}
	if !strings.Contains(stdout.String(), "not configured: missing bot token") {
		t.Fatalf("stdout %q", stdout.String())
	}
}

func TestSettingsPrecedence(t *testing.T) {
	app := testApplication(map[string]string{botTokenEnv: "env-token", chatIDEnv: "11"})
	cfg := config{BotToken: "config-token", ChatID: &chatTarget{value: int64(22)}}

	resolved, err := app.resolveSettings(cfg, cliOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.token != "config-token" || resolved.chatID.value != int64(22) {
		t.Fatalf("config did not override environment: %#v", resolved)
	}

	opts, err := parseCLI([]string{"--token", "cli-token", "--chat-id", "33"})
	if err != nil {
		t.Fatal(err)
	}
	resolved, err = app.resolveSettings(cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.token != "cli-token" || resolved.chatID.value != int64(33) {
		t.Fatalf("CLI did not override config: %#v", resolved)
	}
}

func TestSetFlagsUpdateConfigWithoutNetwork(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := saveConfig(path, config{BotToken: "old-token", ChatID: &chatTarget{value: int64(1)}}); err != nil {
		t.Fatal(err)
	}
	app := testApplication(nil)
	app.configPath = func() (string, error) { return path, nil }

	if err := app.run(context.Background(), []string{"--set-token", "new-token"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BotToken != "new-token" || cfg.ChatID == nil || cfg.ChatID.value != int64(1) {
		t.Fatalf("config after token update %#v", cfg)
	}

	if err := app.run(context.Background(), []string{"--set-token", "final-token", "--set-chat-id", "@alerts"}); err != nil {
		t.Fatal(err)
	}
	cfg, err = loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BotToken != "final-token" || cfg.ChatID == nil || cfg.ChatID.value != "@alerts" {
		t.Fatalf("config after combined update %#v", cfg)
	}
}

func TestInvalidSetDoesNotChangeConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := saveConfig(path, config{BotToken: "old-token", ChatID: &chatTarget{value: int64(1)}}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	app := testApplication(nil)
	app.configPath = func() (string, error) { return path, nil }
	if err := app.run(context.Background(), []string{"--set-token", "new-token", "--set-chat-id", "bad"}); err == nil {
		t.Fatal("expected invalid chat ID")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("config changed: before=%q after=%q", before, after)
	}
}

func TestConfiguredRunWithoutActionShowsHelp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := saveConfig(path, config{BotToken: "token", ChatID: &chatTarget{value: int64(1)}}); err != nil {
		t.Fatal(err)
	}
	var stdout strings.Builder
	app := testApplication(nil)
	app.stdout = &stdout
	app.configPath = func() (string, error) { return path, nil }
	if err := app.run(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Usage:") || !strings.Contains(stdout.String(), "--set-token") {
		t.Fatalf("help output %q", stdout.String())
	}
}

func TestHelpDoesNotReadConfig(t *testing.T) {
	var stdout strings.Builder
	app := testApplication(nil)
	app.stdout = &stdout
	app.configPath = func() (string, error) { return "", errors.New("should not read config") }
	if err := app.run(context.Background(), []string{"--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("help output %q", stdout.String())
	}
}

func TestLoadLegacyAndUsernameConfig(t *testing.T) {
	legacyPath := filepath.Join(t.TempDir(), "legacy.json")
	if err := os.WriteFile(legacyPath, []byte("{\"chat_id\":123}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	legacy, err := loadConfig(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.BotToken != "" || legacy.ChatID == nil || legacy.ChatID.value != int64(123) {
		t.Fatalf("legacy config %#v", legacy)
	}

	usernamePath := filepath.Join(t.TempDir(), "username.json")
	if err := saveConfig(usernamePath, config{ChatID: &chatTarget{value: "@alerts"}}); err != nil {
		t.Fatal(err)
	}
	username, err := loadConfig(usernamePath)
	if err != nil {
		t.Fatal(err)
	}
	if username.ChatID == nil || username.ChatID.value != "@alerts" {
		t.Fatalf("username config %#v", username)
	}
}

func TestFreshPrivateStart(t *testing.T) {
	startedAt := time.Unix(100, 0)
	start := &models.Update{Message: &models.Message{
		Date: 100,
		Text: "/start payload",
		Chat: models.Chat{ID: 1, Type: models.ChatTypePrivate},
	}}
	if !isFreshPrivateStart(start, startedAt) {
		t.Fatal("expected fresh private /start to match")
	}

	old := *start.Message
	old.Date = 99
	group := *start.Message
	group.Chat.Type = models.ChatTypeGroup
	other := *start.Message
	other.Text = "hello"
	for name, message := range map[string]*models.Message{"old": &old, "group": &group, "other": &other} {
		t.Run(name, func(t *testing.T) {
			if isFreshPrivateStart(&models.Update{Message: message}, startedAt) {
				t.Fatal("unexpected match")
			}
		})
	}
}

func TestRedact(t *testing.T) {
	got := redact(`Post "https://api.telegram.org/bot123:secret/sendMessage": failed`, "123:secret")
	if strings.Contains(got, "123:secret") || !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("secret was not redacted: %s", got)
	}
}

func testApplication(env map[string]string) application {
	redactions := []string{env[botTokenEnv]}
	return application{
		getenv: func(key string) string { return env[key] },
		stdout: io.Discard,
		stderr: io.Discard,
		now:    time.Now,
		configPath: func() (string, error) {
			return filepath.Join(os.TempDir(), "missing-telegram-notify-config"), nil
		},
		validateToken: func(context.Context, string) (string, error) {
			return "testbot", nil
		},
		checkAPI: func(context.Context) error { return nil },
		lookPath: func(string) (string, error) {
			return "/usr/local/bin/telegram-notify", nil
		},
		learn: func(context.Context, string, time.Time) (int64, error) {
			return 0, errors.New("unexpected learn")
		},
		send: func(context.Context, string, any, outgoing) (int, error) {
			return 0, errors.New("unexpected send")
		},
		awaitAnswer: func(context.Context, string, int64, int, []string, time.Time) (promptAnswer, error) {
			return promptAnswer{}, errors.New("unexpected awaitAnswer")
		},
		answerCallback: func(context.Context, string, string) error {
			return errors.New("unexpected answerCallback")
		},
		removeKeyboard: func(context.Context, string, int64, int) error {
			return errors.New("unexpected removeKeyboard")
		},
		appendAnswer: func(context.Context, string, int64, int, string, bool) error {
			return errors.New("unexpected appendAnswer")
		},
		react: func(context.Context, string, int64, int, string) error {
			return errors.New("unexpected react")
		},
		redactions: &redactions,
	}
}

func TestParseCLI(t *testing.T) {
	opts, err := parseCLI([]string{"--text", "Proceed?", "--prompt", "--button", "Yes", "--button", "No"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.text.value != "Proceed?" || !opts.prompt || !reflect.DeepEqual([]string(opts.buttons), []string{"Yes", "No"}) {
		t.Fatalf("got text=%q prompt=%v buttons=%v", opts.text.value, opts.prompt, opts.buttons)
	}

	_, err = parseCLI([]string{"--text", "hello", "--button"})
	if err == nil || !strings.Contains(err.Error(), "needs an argument") {
		t.Fatalf("got %v", err)
	}

	for _, args := range [][]string{
		{"--image", "a", "--file", "b"},
		{"--filename", "report.pdf"},
		{"--learn", "--file", "report.pdf"},
	} {
		if _, err := parseCLI(args); err == nil {
			t.Fatalf("expected validation error for %v", args)
		}
	}
}

func TestResolveAttachment(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n")
	path := filepath.Join(t.TempDir(), "chart.png")
	if err := os.WriteFile(path, png, 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, source, filename string
		stdin                  string
		wantName, wantType     string
	}{
		{"URL", "https://example.com/chart.png", "", "", "chart.png", ""},
		{"data URI", "data:image/png;base64,iVBORw0KGgo=", "", "", "image.png", "image/png"},
		{"path", path, "", "", "chart.png", "image/png"},
		{"stdin", "-", "stdin.png", "iVBORw0KGgo=", "stdin.png", "image/png"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveAttachmentFrom(tt.source, tt.filename, strings.NewReader(tt.stdin))
			if err != nil {
				t.Fatal(err)
			}
			if got.filename != tt.wantName || got.contentType != tt.wantType {
				t.Fatalf("got filename=%q contentType=%q", got.filename, got.contentType)
			}
		})
	}
	raw := base64.StdEncoding.EncodeToString(bytes.Repeat(png, 10))
	if got, err := resolveAttachment(raw, "upload.png"); err != nil || got.filename != "upload.png" || got.contentType != "image/png" {
		t.Fatalf("raw base64 attachment: %#v, %v", got, err)
	}
	if _, err := resolveAttachment("not a source", ""); err == nil {
		t.Fatal("expected invalid source error")
	}
}

func TestRunSendsAttachment(t *testing.T) {
	var got outgoing
	var stderr strings.Builder
	app := testApplication(map[string]string{botTokenEnv: "secret", chatIDEnv: "42"})
	app.stderr = &stderr
	app.send = func(_ context.Context, _ string, _ any, message outgoing) (int, error) {
		got = message
		return 1, nil
	}
	if err := app.run(context.Background(), []string{"--text", "chart", "--image", "data:image/png;base64,iVBORw0KGgo="}); err != nil {
		t.Fatal(err)
	}
	if got.attachment == nil || got.asDocument || got.text != "chart" {
		t.Fatalf("unexpected image outgoing: %#v", got)
	}
	if err := app.run(context.Background(), []string{"--file", "data:application/pdf;base64,JVBERi0="}); err != nil {
		t.Fatal(err)
	}
	if got.attachment == nil || !got.asDocument {
		t.Fatalf("unexpected file outgoing: %#v", got)
	}
	if err := app.run(context.Background(), []string{"--image", "data:application/pdf;base64,JVBERi0="}); err != nil {
		t.Fatal(err)
	}
	if !got.asDocument || !strings.Contains(stderr.String(), "sent as a file") {
		t.Fatalf("fallback document=%v warning=%q", got.asDocument, stderr.String())
	}
}

func TestRunPromptSendsQuestionAndReturnsReply(t *testing.T) {
	var promptQuestion string
	var promptButtons []string
	var reactedChat int64
	var reactedMsg int
	var stdout strings.Builder
	app := testApplication(map[string]string{botTokenEnv: "secret", chatIDEnv: "42"})
	app.stdout = &stdout
	app.send = func(_ context.Context, _ string, chatID any, message outgoing) (int, error) {
		if chatID != int64(42) {
			t.Fatalf("unexpected chat %v", chatID)
		}
		promptQuestion, promptButtons = message.text, message.buttons
		return 10, nil
	}
	app.awaitAnswer = func(_ context.Context, _ string, chatID int64, questionMsgID int, buttons []string, _ time.Time) (promptAnswer, error) {
		if chatID != 42 || questionMsgID != 10 || len(buttons) != 0 {
			t.Fatalf("unexpected await chat=%d msg=%d buttons=%v", chatID, questionMsgID, buttons)
		}
		return promptAnswer{text: "user answer", replyMsgID: 99}, nil
	}
	var reactedEmoji string
	app.react = func(_ context.Context, _ string, chatID int64, messageID int, emoji string) error {
		reactedChat, reactedMsg, reactedEmoji = chatID, messageID, emoji
		return nil
	}

	if err := app.run(context.Background(), []string{"--text", "pick one", "--prompt"}); err != nil {
		t.Fatal(err)
	}
	if promptQuestion != "pick one" || len(promptButtons) != 0 {
		t.Fatalf("unexpected prompt q=%q buttons=%v", promptQuestion, promptButtons)
	}
	if reactedChat != 42 || reactedMsg != 99 || reactedEmoji != eyesEmoji {
		t.Fatalf("unexpected react chat=%d msg=%d emoji=%q", reactedChat, reactedMsg, reactedEmoji)
	}
	if stdout.String() != "user answer\n" {
		t.Fatalf("stdout %q", stdout.String())
	}
}

func TestRunPromptWithButtonTap(t *testing.T) {
	var gotButtons []string
	var removeCalled bool
	var callbackID string
	var appendedText string
	var reactedMsg int
	var reactedEmoji string
	var stdout strings.Builder
	app := testApplication(map[string]string{botTokenEnv: "secret", chatIDEnv: "42"})
	app.stdout = &stdout
	app.send = func(_ context.Context, _ string, _ any, message outgoing) (int, error) {
		gotButtons = message.buttons
		return 11, nil
	}
	app.awaitAnswer = func(context.Context, string, int64, int, []string, time.Time) (promptAnswer, error) {
		return promptAnswer{text: "Yes", callbackID: "cb-1"}, nil
	}
	app.removeKeyboard = func(context.Context, string, int64, int) error {
		removeCalled = true
		return nil
	}
	app.answerCallback = func(_ context.Context, _ string, id string) error {
		callbackID = id
		return nil
	}
	app.appendAnswer = func(_ context.Context, _ string, chatID int64, messageID int, text string, _ bool) error {
		if chatID != 42 || messageID != 11 {
			t.Fatalf("unexpected appendAnswer chat=%d msg=%d", chatID, messageID)
		}
		appendedText = text
		return nil
	}
	app.react = func(_ context.Context, _ string, _ int64, messageID int, emoji string) error {
		reactedMsg, reactedEmoji = messageID, emoji
		return nil
	}

	err := app.run(context.Background(), []string{"--text", "Proceed?", "--prompt", "--button", "Yes", "--button", "No"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotButtons, []string{"Yes", "No"}) {
		t.Fatalf("buttons %v", gotButtons)
	}
	// removeKeyboard is not called on a button tap: appendAnswer already clears the markup.
	if removeCalled {
		t.Fatal("removeKeyboard should not be called on a button tap")
	}
	if callbackID != "cb-1" {
		t.Fatalf("callback %q", callbackID)
	}
	if appendedText != "Proceed?\n\nAnswer: Yes" {
		t.Fatalf("appended text %q", appendedText)
	}
	if reactedMsg != 11 || reactedEmoji != checkmarkEmoji {
		t.Fatalf("reacted msg=%d emoji=%q", reactedMsg, reactedEmoji)
	}
	if stdout.String() != "Yes\n" {
		t.Fatalf("stdout %q", stdout.String())
	}
}

func TestRunPromptWithAttachmentEditsCaption(t *testing.T) {
	var got outgoing
	var hasAttachment bool
	app := testApplication(map[string]string{botTokenEnv: "secret", chatIDEnv: "42"})
	app.send = func(_ context.Context, _ string, _ any, message outgoing) (int, error) {
		got = message
		return 11, nil
	}
	app.awaitAnswer = func(context.Context, string, int64, int, []string, time.Time) (promptAnswer, error) {
		return promptAnswer{text: "Yes", callbackID: "cb-1"}, nil
	}
	app.answerCallback = func(context.Context, string, string) error { return nil }
	app.appendAnswer = func(_ context.Context, _ string, _ int64, _ int, _ string, attachment bool) error {
		hasAttachment = attachment
		return nil
	}
	app.react = func(context.Context, string, int64, int, string) error { return nil }

	if err := app.run(context.Background(), []string{
		"--text", "Looks good?", "--image", "data:image/png;base64,iVBORw0KGgo=", "--prompt", "--button", "Yes",
	}); err != nil {
		t.Fatal(err)
	}
	if got.attachment == nil || !hasAttachment {
		t.Fatalf("attachment=%#v hasAttachment=%v", got.attachment, hasAttachment)
	}
}

func TestRunPromptTextReplyWithButtonsRemovesKeyboard(t *testing.T) {
	var removeCalled bool
	var reactedMsg int
	var reactedEmoji string
	var callbackCalled bool
	var appendCalled bool
	app := testApplication(map[string]string{botTokenEnv: "secret", chatIDEnv: "42"})
	app.send = func(context.Context, string, any, outgoing) (int, error) { return 5, nil }
	app.awaitAnswer = func(context.Context, string, int64, int, []string, time.Time) (promptAnswer, error) {
		return promptAnswer{text: "custom", replyMsgID: 7}, nil
	}
	app.removeKeyboard = func(context.Context, string, int64, int) error {
		removeCalled = true
		return nil
	}
	app.react = func(_ context.Context, _ string, _ int64, messageID int, emoji string) error {
		reactedMsg, reactedEmoji = messageID, emoji
		return nil
	}
	app.answerCallback = func(context.Context, string, string) error {
		callbackCalled = true
		return nil
	}
	app.appendAnswer = func(context.Context, string, int64, int, string, bool) error {
		appendCalled = true
		return nil
	}

	if err := app.run(context.Background(), []string{"--text", "q", "--prompt", "--button", "A"}); err != nil {
		t.Fatal(err)
	}
	if !removeCalled || callbackCalled || appendCalled {
		t.Fatalf("remove=%v callback=%v append=%v", removeCalled, callbackCalled, appendCalled)
	}
	if reactedMsg != 7 || reactedEmoji != eyesEmoji {
		t.Fatalf("reacted msg=%d emoji=%q", reactedMsg, reactedEmoji)
	}
}

func TestRunPromptTimesOutAndNotifiesChat(t *testing.T) {
	var sends []string
	var removeCalled bool
	app := testApplication(map[string]string{botTokenEnv: "secret", chatIDEnv: "42"})
	app.send = func(_ context.Context, _ string, _ any, message outgoing) (int, error) {
		if message.buttons != nil {
			return 1, nil
		}
		sends = append(sends, message.text)
		return 1, nil
	}
	app.awaitAnswer = func(context.Context, string, int64, int, []string, time.Time) (promptAnswer, error) {
		return promptAnswer{}, errPromptTimeout
	}
	app.removeKeyboard = func(context.Context, string, int64, int) error {
		removeCalled = true
		return nil
	}

	err := app.run(context.Background(), []string{"--text", "hello?", "--prompt", "--button", "Yes"})
	if !errors.Is(err, errPromptTimeout) {
		t.Fatalf("got %v", err)
	}
	if !removeCalled || len(sends) != 1 || sends[0] != timeoutText {
		t.Fatalf("remove=%v sends=%#v", removeCalled, sends)
	}
}

func TestRunPromptTimesOutWithoutButtonsSkipsRemove(t *testing.T) {
	var removeCalled bool
	app := testApplication(map[string]string{botTokenEnv: "secret", chatIDEnv: "42"})
	app.send = func(context.Context, string, any, outgoing) (int, error) { return 1, nil }
	app.awaitAnswer = func(context.Context, string, int64, int, []string, time.Time) (promptAnswer, error) {
		return promptAnswer{}, errPromptTimeout
	}
	app.removeKeyboard = func(context.Context, string, int64, int) error {
		removeCalled = true
		return nil
	}

	err := app.run(context.Background(), []string{"--text", "hello?", "--prompt"})
	if !errors.Is(err, errPromptTimeout) {
		t.Fatalf("got %v", err)
	}
	if removeCalled {
		t.Fatal("removeKeyboard should not run without buttons")
	}
}

func TestRunPromptRequiresChat(t *testing.T) {
	app := testApplication(map[string]string{botTokenEnv: "secret", chatIDEnv: "@alerts"})
	err := app.run(context.Background(), []string{"--text", "hello?", "--prompt"})
	if err == nil || !strings.Contains(err.Error(), "private/group chat") {
		t.Fatalf("got %v", err)
	}
}

func TestRunPromptRequiresQuestion(t *testing.T) {
	app := testApplication(map[string]string{botTokenEnv: "secret", chatIDEnv: "42"})
	err := app.run(context.Background(), []string{"--prompt"})
	if err == nil || !strings.Contains(err.Error(), "requires --text") {
		t.Fatalf("got %v", err)
	}
}

func TestRunPromptReactFailureStillReturnsReply(t *testing.T) {
	var stdout strings.Builder
	app := testApplication(map[string]string{botTokenEnv: "secret", chatIDEnv: "42"})
	app.stdout = &stdout
	app.send = func(context.Context, string, any, outgoing) (int, error) { return 1, nil }
	app.awaitAnswer = func(context.Context, string, int64, int, []string, time.Time) (promptAnswer, error) {
		return promptAnswer{text: "ok", replyMsgID: 1}, nil
	}
	app.react = func(context.Context, string, int64, int, string) error {
		return errors.New("reaction failed")
	}

	if err := app.run(context.Background(), []string{"--text", "q", "--prompt"}); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "ok\n" {
		t.Fatalf("stdout %q", stdout.String())
	}
}

func TestMatchingReply(t *testing.T) {
	after := time.Unix(100, 0)
	valid := &models.Update{Message: &models.Message{
		Date: 100,
		Text: "answer",
		Chat: models.Chat{ID: 42},
	}}
	if msg := matchingReply(valid, 42, after); msg == nil || msg.Text != "answer" {
		t.Fatal("expected valid reply")
	}

	wrongChat := *valid.Message
	wrongChat.Chat.ID = 1
	stale := *valid.Message
	stale.Date = 99
	empty := *valid.Message
	empty.Text = "   "
	for name, update := range map[string]*models.Update{
		"wrong chat": {Message: &wrongChat},
		"stale":      {Message: &stale},
		"empty":      {Message: &empty},
		"nil":        nil,
	} {
		t.Run(name, func(t *testing.T) {
			if matchingReply(update, 42, after) != nil {
				t.Fatal("unexpected match")
			}
		})
	}
}

func TestMatchingCallback(t *testing.T) {
	buttons := []string{"A", "B", "C"}
	valid := &models.Update{CallbackQuery: &models.CallbackQuery{
		ID:   "cb1",
		Data: "opt:1",
		Message: models.MaybeInaccessibleMessage{
			Type: models.MaybeInaccessibleMessageTypeMessage,
			Message: &models.Message{
				ID:   7,
				Chat: models.Chat{ID: 42},
			},
		},
	}}
	if ans := matchingCallback(valid, 42, 7, buttons); ans == nil || ans.text != "B" || ans.callbackID != "cb1" {
		t.Fatalf("got %#v", ans)
	}

	wrongChat := *valid.CallbackQuery
	wrongChat.Message.Message.Chat.ID = 1
	wrongMsg := *valid.CallbackQuery
	wrongMsg.Message.Message.ID = 8
	badData := *valid.CallbackQuery
	badData.Data = "opt:9"
	noButtons := *valid.CallbackQuery
	for name, cq := range map[string]*models.CallbackQuery{
		"nil":        nil,
		"wrong chat": &wrongChat,
		"wrong msg":  &wrongMsg,
		"bad index":  &badData,
		"no buttons": &noButtons,
	} {
		t.Run(name, func(t *testing.T) {
			var update *models.Update
			if cq != nil {
				update = &models.Update{CallbackQuery: cq}
			}
			btn := buttons
			if name == "no buttons" {
				btn = nil
			}
			if matchingCallback(update, 42, 7, btn) != nil {
				t.Fatal("unexpected match")
			}
		})
	}
}

func TestMatchesCheckinReaction(t *testing.T) {
	valid := &models.Update{MessageReaction: &models.MessageReactionUpdated{
		Chat:        models.Chat{ID: 42},
		MessageID:   7,
		NewReaction: []models.ReactionType{{Type: models.ReactionTypeTypeEmoji}},
	}}
	if !matchesCheckinReaction(valid, 42, 7) {
		t.Fatal("expected match")
	}

	wrongChat := *valid.MessageReaction
	wrongChat.Chat.ID = 1
	wrongMsg := *valid.MessageReaction
	wrongMsg.MessageID = 8
	empty := *valid.MessageReaction
	empty.NewReaction = nil
	for name, update := range map[string]*models.Update{
		"nil update":  nil,
		"no reaction": {Message: &models.Message{}},
		"wrong chat":  {MessageReaction: &wrongChat},
		"wrong msg":   {MessageReaction: &wrongMsg},
		"empty":       {MessageReaction: &empty},
	} {
		t.Run(name, func(t *testing.T) {
			if matchesCheckinReaction(update, 42, 7) {
				t.Fatal("unexpected match")
			}
		})
	}
}

func TestInlineKeyboard(t *testing.T) {
	if inlineKeyboard(nil) != nil {
		t.Fatal("expected nil for empty buttons")
	}
	m := inlineKeyboard([]string{"Yes", "No"})
	if m == nil || len(m.InlineKeyboard) != 1 || len(m.InlineKeyboard[0]) != 2 {
		t.Fatalf("unexpected markup %#v", m)
	}
	if m.InlineKeyboard[0][0].Text != "Yes" || m.InlineKeyboard[0][0].CallbackData != "opt:0" {
		t.Fatalf("first button %#v", m.InlineKeyboard[0][0])
	}
}
