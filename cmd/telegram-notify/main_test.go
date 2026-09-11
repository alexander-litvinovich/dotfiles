package main

import (
	"context"
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
	app.send = func(_ context.Context, token string, chatID any, message string) error {
		gotToken, gotChatID, gotMessage = token, chatID, message
		return nil
	}

	if err := app.run(context.Background(), []string{"job", "finished"}); err != nil {
		t.Fatal(err)
	}
	if gotToken != "secret" || gotChatID != int64(-123) || gotMessage != "job finished" {
		t.Fatalf("unexpected send: token=%q chat=%v message=%q", gotToken, gotChatID, gotMessage)
	}
}

func TestRunUsesSavedChat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := saveConfig(path, config{ChatID: 456}); err != nil {
		t.Fatal(err)
	}

	var gotChatID any
	app := testApplication(map[string]string{botTokenEnv: "secret"})
	app.configPath = func() (string, error) { return path, nil }
	app.send = func(_ context.Context, _ string, chatID any, _ string) error {
		gotChatID = chatID
		return nil
	}

	if err := app.run(context.Background(), []string{"done"}); err != nil {
		t.Fatal(err)
	}
	if gotChatID != int64(456) {
		t.Fatalf("got chat ID %v", gotChatID)
	}
}

func TestRunAllowsChannelUsername(t *testing.T) {
	app := testApplication(map[string]string{botTokenEnv: "secret", chatIDEnv: "@alerts"})
	app.send = func(_ context.Context, _ string, chatID any, _ string) error {
		if chatID != "@alerts" {
			t.Fatalf("got chat ID %v", chatID)
		}
		return nil
	}
	if err := app.run(context.Background(), []string{"done"}); err != nil {
		t.Fatal(err)
	}
}

func TestRunValidation(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		args []string
		want string
	}{
		{name: "missing token", env: map[string]string{}, args: []string{"done"}, want: "TG_BOT_TOKEN is not set"},
		{name: "missing message", env: map[string]string{botTokenEnv: "secret"}, want: "message is required"},
		{name: "missing chat", env: map[string]string{botTokenEnv: "secret"}, args: []string{"done"}, want: "run telegram-notify --learn"},
		{name: "zero chat", env: map[string]string{botTokenEnv: "secret", chatIDEnv: "0"}, args: []string{"done"}, want: "must not be zero"},
		{name: "invalid chat", env: map[string]string{botTokenEnv: "secret", chatIDEnv: "somewhere"}, args: []string{"done"}, want: "integer or an @channel username"},
		{name: "learn arguments", env: map[string]string{botTokenEnv: "secret"}, args: []string{"--learn", "extra"}, want: "does not accept arguments"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := testApplication(tt.env)
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
	app.learn = func(_ context.Context, token string, gotStartedAt time.Time) (int64, error) {
		if token != "secret" || !gotStartedAt.Equal(startedAt) {
			t.Fatal("unexpected learn arguments")
		}
		return 789, nil
	}
	app.send = func(_ context.Context, token string, chatID any, message string) error {
		sent = []any{token, chatID, message}
		return nil
	}

	if err := app.run(context.Background(), []string{"--learn"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ChatID != 789 {
		t.Fatalf("got saved chat ID %d", cfg.ChatID)
	}
	wantSent := []any{"secret", int64(789), "Telegram notifier connected."}
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
	return application{
		getenv: func(key string) string { return env[key] },
		stdout: io.Discard,
		now:    time.Now,
		configPath: func() (string, error) {
			return filepath.Join(os.TempDir(), "missing-telegram-notify-config"), nil
		},
		learn: func(context.Context, string, time.Time) (int64, error) {
			return 0, errors.New("unexpected learn")
		},
		send: func(context.Context, string, any, string) error {
			return errors.New("unexpected send")
		},
		sendTracked: func(context.Context, string, any, string) (int, error) {
			return 0, errors.New("unexpected sendTracked")
		},
		awaitReply: func(context.Context, string, int64, time.Time) (string, int, error) {
			return "", 0, errors.New("unexpected awaitReply")
		},
		react: func(context.Context, string, int64, int) error {
			return errors.New("unexpected react")
		},
	}
}

func TestRunPromptSendsQuestionAndReturnsReply(t *testing.T) {
	var sends []string
	var reactedChat int64
	var reactedMsg int
	var stdout strings.Builder
	app := testApplication(map[string]string{botTokenEnv: "secret", chatIDEnv: "42"})
	app.stdout = &stdout
	app.send = func(_ context.Context, _ string, chatID any, message string) error {
		if chatID != int64(42) {
			t.Fatalf("unexpected chat %v", chatID)
		}
		sends = append(sends, message)
		return nil
	}
	app.awaitReply = func(_ context.Context, _ string, chatID int64, _ time.Time) (string, int, error) {
		if chatID != 42 {
			t.Fatalf("unexpected chat %d", chatID)
		}
		return "user answer", 99, nil
	}
	app.react = func(_ context.Context, _ string, chatID int64, messageID int) error {
		reactedChat, reactedMsg = chatID, messageID
		return nil
	}

	if err := app.run(context.Background(), []string{"--prompt", "pick", "one"}); err != nil {
		t.Fatal(err)
	}
	if len(sends) != 1 || sends[0] != "pick one" {
		t.Fatalf("unexpected sends %#v", sends)
	}
	if reactedChat != 42 || reactedMsg != 99 {
		t.Fatalf("unexpected react chat=%d msg=%d", reactedChat, reactedMsg)
	}
	if stdout.String() != "user answer\n" {
		t.Fatalf("stdout %q", stdout.String())
	}
}

func TestRunPromptTimesOutAndNotifiesChat(t *testing.T) {
	var sends []string
	app := testApplication(map[string]string{botTokenEnv: "secret", chatIDEnv: "42"})
	app.send = func(_ context.Context, _ string, _ any, message string) error {
		sends = append(sends, message)
		return nil
	}
	app.awaitReply = func(context.Context, string, int64, time.Time) (string, int, error) {
		return "", 0, errPromptTimeout
	}

	err := app.run(context.Background(), []string{"--prompt", "hello?"})
	if !errors.Is(err, errPromptTimeout) {
		t.Fatalf("got %v", err)
	}
	if len(sends) != 2 || sends[0] != "hello?" || sends[1] != timeoutText {
		t.Fatalf("unexpected sends %#v", sends)
	}
}

func TestRunPromptRequiresChat(t *testing.T) {
	app := testApplication(map[string]string{botTokenEnv: "secret", chatIDEnv: "@alerts"})
	err := app.run(context.Background(), []string{"--prompt", "hello?"})
	if err == nil || !strings.Contains(err.Error(), "private/group chat") {
		t.Fatalf("got %v", err)
	}
}

func TestRunPromptRequiresQuestion(t *testing.T) {
	app := testApplication(map[string]string{botTokenEnv: "secret", chatIDEnv: "42"})
	err := app.run(context.Background(), []string{"--prompt"})
	if err == nil || !strings.Contains(err.Error(), "question is required") {
		t.Fatalf("got %v", err)
	}
}

func TestRunPromptReactFailureStillReturnsReply(t *testing.T) {
	var stdout strings.Builder
	app := testApplication(map[string]string{botTokenEnv: "secret", chatIDEnv: "42"})
	app.stdout = &stdout
	app.send = func(context.Context, string, any, string) error { return nil }
	app.awaitReply = func(context.Context, string, int64, time.Time) (string, int, error) {
		return "ok", 1, nil
	}
	app.react = func(context.Context, string, int64, int) error {
		return errors.New("reaction failed")
	}

	if err := app.run(context.Background(), []string{"--prompt", "q"}); err != nil {
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
