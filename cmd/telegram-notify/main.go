package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	botTokenEnv    = "TG_BOT_TOKEN"
	chatIDEnv      = "TG_CHAT_ID"
	configDirName  = "telegram-notify"
	configName     = "config.json"
	requestTimeout = 10 * time.Second
	pollTimeout    = 30 * time.Second
)

var errNoSavedChat = errors.New("no saved chat ID")

type config struct {
	ChatID int64 `json:"chat_id"`
}

type application struct {
	getenv     func(string) string
	stdout     io.Writer
	now        func() time.Time
	configPath func() (string, error)
	learn      func(context.Context, string, time.Time) (int64, error)
	send       func(context.Context, string, any, string) error
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := application{
		getenv:     os.Getenv,
		stdout:     os.Stdout,
		now:        time.Now,
		configPath: defaultConfigPath,
		learn:      learnChat,
		send:       sendMessage,
	}

	if err := app.run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "telegram-notify: %s\n", redact(err.Error(), os.Getenv(botTokenEnv)))
		os.Exit(1)
	}
}

func (app application) run(ctx context.Context, args []string) error {
	token := strings.TrimSpace(app.getenv(botTokenEnv))
	if token == "" {
		return fmt.Errorf("%s is not set", botTokenEnv)
	}

	if len(args) == 1 && args[0] == "--learn" {
		return app.runLearn(ctx, token)
	}
	if len(args) > 0 && args[0] == "--learn" {
		return errors.New("--learn does not accept arguments")
	}

	message := strings.Join(args, " ")
	if strings.TrimSpace(message) == "" {
		return errors.New("message is required; usage: telegram-notify <message>")
	}

	chatID, err := app.resolveChatID()
	if err != nil {
		return err
	}

	if err := app.send(ctx, token, chatID, message); err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	return nil
}

func (app application) runLearn(ctx context.Context, token string) error {
	fmt.Fprintln(app.stdout, "Waiting for /start in a private chat...")

	chatID, err := app.learn(ctx, token, app.now())
	if err != nil {
		return fmt.Errorf("learn chat ID: %w", err)
	}

	path, err := app.configPath()
	if err != nil {
		return err
	}
	if err := saveConfig(path, config{ChatID: chatID}); err != nil {
		return fmt.Errorf("save chat ID: %w", err)
	}

	if err := app.send(ctx, token, chatID, "Telegram notifier connected."); err != nil {
		return fmt.Errorf("chat ID was saved, but confirmation failed: %w", err)
	}

	fmt.Fprintf(app.stdout, "Connected to chat %d.\n", chatID)
	return nil
}

func (app application) resolveChatID() (any, error) {
	if value := strings.TrimSpace(app.getenv(chatIDEnv)); value != "" {
		if id, err := strconv.ParseInt(value, 10, 64); err == nil {
			if id == 0 {
				return nil, fmt.Errorf("%s must not be zero", chatIDEnv)
			}
			return id, nil
		}
		if strings.HasPrefix(value, "@") {
			return value, nil
		}
		return nil, fmt.Errorf("%s must be an integer or an @channel username", chatIDEnv)
	}

	path, err := app.configPath()
	if err != nil {
		return nil, err
	}
	cfg, err := loadConfig(path)
	if errors.Is(err, errNoSavedChat) {
		return nil, errors.New("chat ID is not configured; set TG_CHAT_ID or run telegram-notify --learn")
	}
	if err != nil {
		return nil, fmt.Errorf("load chat ID: %w", err)
	}
	return cfg.ChatID, nil
}

func defaultConfigPath() (string, error) {
	base := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, configDirName, configName), nil
}

func loadConfig(path string) (config, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return config{}, errNoSavedChat
	}
	if err != nil {
		return config{}, err
	}
	defer file.Close()

	var cfg config
	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		return config{}, err
	}
	if cfg.ChatID == 0 {
		return config{}, errors.New("saved chat ID is empty")
	}
	return cfg, nil
}

func saveConfig(path string, cfg config) error {
	if cfg.ChatID == 0 {
		return errors.New("chat ID is empty")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := json.NewEncoder(tmp).Encode(cfg); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func sendMessage(ctx context.Context, token string, chatID any, message string) error {
	client, err := bot.New(
		token,
		bot.WithSkipGetMe(),
		bot.WithHTTPClient(requestTimeout, &http.Client{Timeout: requestTimeout}),
	)
	if err != nil {
		return err
	}

	requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	_, err = client.SendMessage(requestCtx, &bot.SendMessageParams{ChatID: chatID, Text: message})
	return err
}

func learnChat(ctx context.Context, token string, startedAt time.Time) (int64, error) {
	learnCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	found := make(chan int64, 1)
	failures := make(chan error, 1)
	handler := func(_ context.Context, _ *bot.Bot, update *models.Update) {
		if !isFreshPrivateStart(update, startedAt) {
			return
		}
		select {
		case found <- update.Message.Chat.ID:
			cancel()
		default:
		}
	}
	errorHandler := func(err error) {
		select {
		case failures <- err:
			cancel()
		default:
		}
	}

	client, err := bot.New(
		token,
		bot.WithSkipGetMe(),
		bot.WithDefaultHandler(handler),
		bot.WithErrorsHandler(errorHandler),
		bot.WithAllowedUpdates(bot.AllowedUpdates{models.AllowedUpdateMessage}),
		bot.WithHTTPClient(pollTimeout, &http.Client{Timeout: pollTimeout + 5*time.Second}),
	)
	if err != nil {
		return 0, err
	}

	client.Start(learnCtx)

	select {
	case id := <-found:
		return id, nil
	default:
	}
	select {
	case err := <-failures:
		return 0, err
	default:
	}
	return 0, ctx.Err()
}

func isFreshPrivateStart(update *models.Update, startedAt time.Time) bool {
	if update == nil || update.Message == nil {
		return false
	}
	message := update.Message
	if message.Chat.Type != models.ChatTypePrivate || int64(message.Date) < startedAt.Unix() {
		return false
	}
	fields := strings.Fields(message.Text)
	if len(fields) == 0 {
		return false
	}
	command := fields[0]
	return command == "/start"
}

func redact(message, secret string) string {
	if secret == "" {
		return message
	}
	return strings.ReplaceAll(message, secret, "[REDACTED]")
}
