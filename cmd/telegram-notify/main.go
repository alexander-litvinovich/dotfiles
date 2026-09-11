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
	"sync"
	"syscall"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	botTokenEnv      = "TG_BOT_TOKEN"
	chatIDEnv        = "TG_CHAT_ID"
	configDirName    = "telegram-notify"
	configName       = "config.json"
	requestTimeout   = 10 * time.Second
	pollTimeout      = 30 * time.Second
	promptTimeout    = 5 * time.Minute
	checkinLeadTime  = 30 * time.Second
	checkinExtension = 5 * time.Minute
	checkinText      = "Still there? React to this message to get 5 more minutes."
	timeoutText      = "Request timed out waiting for a reply."
)

var (
	errNoSavedChat   = errors.New("no saved chat ID")
	errPromptTimeout = errors.New("timed out waiting for a reply")
)

type config struct {
	ChatID int64 `json:"chat_id"`
}

type application struct {
	getenv      func(string) string
	stdout      io.Writer
	now         func() time.Time
	configPath  func() (string, error)
	learn       func(context.Context, string, time.Time) (int64, error)
	send        func(context.Context, string, any, string) error
	sendTracked func(context.Context, string, any, string) (int, error)
	awaitReply  func(context.Context, string, int64, time.Time) (string, int, error)
	react       func(context.Context, string, int64, int) error
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := application{
		getenv:      os.Getenv,
		stdout:      os.Stdout,
		now:         time.Now,
		configPath:  defaultConfigPath,
		learn:       learnChat,
		send:        sendMessage,
		sendTracked: sendMessageTracked,
		awaitReply:  awaitReply,
		react:       reactEyes,
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

	if len(args) >= 1 && args[0] == "--prompt" {
		question := strings.Join(args[1:], " ")
		if strings.TrimSpace(question) == "" {
			return errors.New("question is required; usage: telegram-notify --prompt <question>")
		}
		return app.runPrompt(ctx, token, question)
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

func (app application) runPrompt(ctx context.Context, token, question string) error {
	chatID, err := app.resolveChatID()
	if err != nil {
		return err
	}
	numericChatID, ok := chatID.(int64)
	if !ok {
		return errors.New("--prompt requires a private/group chat, not a channel username")
	}

	sentAt := app.now()
	if err := app.send(ctx, token, chatID, question); err != nil {
		return fmt.Errorf("send prompt: %w", err)
	}

	replyText, messageID, err := app.awaitReply(ctx, token, numericChatID, sentAt)
	if err != nil {
		if errors.Is(err, errPromptTimeout) {
			if sendErr := app.send(ctx, token, chatID, timeoutText); sendErr != nil {
				fmt.Fprintf(os.Stderr, "telegram-notify: failed to send timeout notice: %s\n", sendErr)
			}
		}
		return err
	}

	if err := app.react(ctx, token, numericChatID, messageID); err != nil {
		fmt.Fprintf(os.Stderr, "telegram-notify: react with eyes failed: %s\n", err)
	}

	fmt.Fprintln(app.stdout, replyText)
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

func sendMessageTracked(ctx context.Context, token string, chatID any, message string) (int, error) {
	client, err := bot.New(
		token,
		bot.WithSkipGetMe(),
		bot.WithHTTPClient(requestTimeout, &http.Client{Timeout: requestTimeout}),
	)
	if err != nil {
		return 0, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	msg, err := client.SendMessage(requestCtx, &bot.SendMessageParams{ChatID: chatID, Text: message})
	if err != nil {
		return 0, err
	}
	return msg.ID, nil
}

func reactEyes(ctx context.Context, token string, chatID int64, messageID int) error {
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
	_, err = client.SetMessageReaction(requestCtx, &bot.SetMessageReactionParams{
		ChatID:    chatID,
		MessageID: messageID,
		Reaction: []models.ReactionType{{
			Type:              models.ReactionTypeTypeEmoji,
			ReactionTypeEmoji: &models.ReactionTypeEmoji{Emoji: "\U0001F440"},
		}},
	})
	return err
}

func awaitReply(ctx context.Context, token string, chatID int64, after time.Time) (string, int, error) {
	type result struct {
		text string
		id   int
	}
	found := make(chan result, 1)
	extend := make(chan struct{}, 1)
	failures := make(chan error, 1)

	var (
		mu           sync.Mutex
		checkinSent  bool
		checkinMsgID int
	)

	handler := func(_ context.Context, _ *bot.Bot, update *models.Update) {
		if msg := matchingReply(update, chatID, after); msg != nil {
			select {
			case found <- result{msg.Text, msg.ID}:
			default:
			}
			return
		}
		mu.Lock()
		id, sent := checkinMsgID, checkinSent
		mu.Unlock()
		if sent && matchesCheckinReaction(update, chatID, id) {
			select {
			case extend <- struct{}{}:
			default:
			}
		}
	}
	errorHandler := func(err error) {
		select {
		case failures <- err:
		default:
		}
	}

	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	client, err := bot.New(
		token,
		bot.WithSkipGetMe(),
		bot.WithDefaultHandler(handler),
		bot.WithErrorsHandler(errorHandler),
		bot.WithAllowedUpdates(bot.AllowedUpdates{models.AllowedUpdateMessage, models.AllowedUpdateMessageReaction}),
		bot.WithHTTPClient(pollTimeout, &http.Client{Timeout: pollTimeout + 5*time.Second}),
	)
	if err != nil {
		return "", 0, err
	}
	go client.Start(waitCtx)

	deadline := after.Add(promptTimeout)
	timer := time.NewTimer(time.Until(deadline) - checkinLeadTime)
	defer timer.Stop()

	for {
		select {
		case r := <-found:
			return r.text, r.id, nil
		case err := <-failures:
			return "", 0, err
		case <-ctx.Done():
			return "", 0, ctx.Err()
		case <-extend:
			deadline = time.Now().Add(checkinExtension)
			mu.Lock()
			checkinSent = false
			mu.Unlock()
			resetTimer(timer, time.Until(deadline)-checkinLeadTime)
		case <-timer.C:
			mu.Lock()
			sent := checkinSent
			mu.Unlock()
			if !sent {
				id, sendErr := sendMessageTracked(ctx, token, chatID, checkinText)
				if sendErr == nil {
					mu.Lock()
					checkinSent, checkinMsgID = true, id
					mu.Unlock()
				}
				resetTimer(timer, time.Until(deadline))
				continue
			}
			return "", 0, errPromptTimeout
		}
	}
}

func resetTimer(t *time.Timer, d time.Duration) {
	if d < 0 {
		d = 0
	}
	t.Reset(d)
}

func matchingReply(update *models.Update, chatID int64, after time.Time) *models.Message {
	if update == nil || update.Message == nil {
		return nil
	}
	message := update.Message
	if message.Chat.ID != chatID || int64(message.Date) < after.Unix() {
		return nil
	}
	if strings.TrimSpace(message.Text) == "" {
		return nil
	}
	return message
}

func matchesCheckinReaction(update *models.Update, chatID int64, messageID int) bool {
	if update == nil {
		return false
	}
	r := update.MessageReaction
	if r == nil {
		return false
	}
	return r.Chat.ID == chatID && r.MessageID == messageID && len(r.NewReaction) > 0
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
