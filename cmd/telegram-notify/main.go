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
	callbackDataPref = "opt:"
	eyesEmoji        = "\U0001F440"
	// checkmarkEmoji acknowledges a button-tap answer. Telegram's setMessageReaction
	// only accepts a fixed emoji set for bots and rejects "\u2705" (✅) with
	// REACTION_INVALID, so thumbs-up is used instead.
	checkmarkEmoji = "\U0001F44D"
)

var (
	errNoSavedChat   = errors.New("no saved chat ID")
	errPromptTimeout = errors.New("timed out waiting for a reply")
)

type config struct {
	ChatID int64 `json:"chat_id"`
}

type promptAnswer struct {
	text       string
	replyMsgID int
	callbackID string
}

type application struct {
	getenv         func(string) string
	stdout         io.Writer
	now            func() time.Time
	configPath     func() (string, error)
	learn          func(context.Context, string, time.Time) (int64, error)
	send           func(context.Context, string, any, string) error
	sendTracked    func(context.Context, string, any, string) (int, error)
	sendPrompt     func(context.Context, string, any, string, []string) (int, error)
	awaitAnswer    func(context.Context, string, int64, int, []string, time.Time) (promptAnswer, error)
	answerCallback func(context.Context, string, string) error
	removeKeyboard func(context.Context, string, int64, int) error
	appendAnswer   func(context.Context, string, int64, int, string) error
	react          func(context.Context, string, int64, int, string) error
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := application{
		getenv:         os.Getenv,
		stdout:         os.Stdout,
		now:            time.Now,
		configPath:     defaultConfigPath,
		learn:          learnChat,
		send:           sendMessage,
		sendTracked:    sendMessageTracked,
		sendPrompt:     sendPromptMessage,
		awaitAnswer:    awaitAnswer,
		answerCallback: answerCallbackQuery,
		removeKeyboard: removeInlineKeyboard,
		appendAnswer:   appendAnswerText,
		react:          react,
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
		question, buttons, err := parsePromptArgs(args[1:])
		if err != nil {
			return err
		}
		if strings.TrimSpace(question) == "" {
			return errors.New("question is required; usage: telegram-notify --prompt <question> [--button <label> ...]")
		}
		return app.runPrompt(ctx, token, question, buttons)
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

func parsePromptArgs(args []string) (question string, buttons []string, err error) {
	var questionParts []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--button" {
			if i+1 >= len(args) {
				return "", nil, errors.New("--button requires a label")
			}
			i++
			buttons = append(buttons, args[i])
			continue
		}
		questionParts = append(questionParts, args[i])
	}
	return strings.Join(questionParts, " "), buttons, nil
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

func (app application) runPrompt(ctx context.Context, token, question string, buttons []string) error {
	chatID, err := app.resolveChatID()
	if err != nil {
		return err
	}
	numericChatID, ok := chatID.(int64)
	if !ok {
		return errors.New("--prompt requires a private/group chat, not a channel username")
	}

	sentAt := app.now()
	msgID, err := app.sendPrompt(ctx, token, chatID, question, buttons)
	if err != nil {
		return fmt.Errorf("send prompt: %w", err)
	}

	ans, err := app.awaitAnswer(ctx, token, numericChatID, msgID, buttons, sentAt)
	if err != nil {
		if len(buttons) > 0 {
			if rmErr := app.removeKeyboard(ctx, token, numericChatID, msgID); rmErr != nil {
				fmt.Fprintf(os.Stderr, "telegram-notify: failed to remove keyboard: %s\n", rmErr)
			}
		}
		if errors.Is(err, errPromptTimeout) {
			if sendErr := app.send(ctx, token, chatID, timeoutText); sendErr != nil {
				fmt.Fprintf(os.Stderr, "telegram-notify: failed to send timeout notice: %s\n", sendErr)
			}
		}
		return err
	}

	if ans.callbackID != "" {
		if err := app.answerCallback(ctx, token, ans.callbackID); err != nil {
			fmt.Fprintf(os.Stderr, "telegram-notify: answer callback failed: %s\n", err)
		}
		answeredText := question + "\n\nAnswer: " + ans.text
		if err := app.appendAnswer(ctx, token, numericChatID, msgID, answeredText); err != nil {
			fmt.Fprintf(os.Stderr, "telegram-notify: failed to record answer on message: %s\n", err)
		}
		if err := app.react(ctx, token, numericChatID, msgID, checkmarkEmoji); err != nil {
			fmt.Fprintf(os.Stderr, "telegram-notify: react with checkmark failed: %s\n", err)
		}
	} else {
		if len(buttons) > 0 {
			if err := app.removeKeyboard(ctx, token, numericChatID, msgID); err != nil {
				fmt.Fprintf(os.Stderr, "telegram-notify: failed to remove keyboard: %s\n", err)
			}
		}
		if err := app.react(ctx, token, numericChatID, ans.replyMsgID, eyesEmoji); err != nil {
			fmt.Fprintf(os.Stderr, "telegram-notify: react with eyes failed: %s\n", err)
		}
	}

	fmt.Fprintln(app.stdout, ans.text)
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

func inlineKeyboard(buttons []string) *models.InlineKeyboardMarkup {
	if len(buttons) == 0 {
		return nil
	}
	row := make([]models.InlineKeyboardButton, len(buttons))
	for i, label := range buttons {
		row[i] = models.InlineKeyboardButton{
			Text:         label,
			CallbackData: callbackDataPref + strconv.Itoa(i),
		}
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{row}}
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

func sendPromptMessage(ctx context.Context, token string, chatID any, question string, buttons []string) (int, error) {
	client, err := bot.New(
		token,
		bot.WithSkipGetMe(),
		bot.WithHTTPClient(requestTimeout, &http.Client{Timeout: requestTimeout}),
	)
	if err != nil {
		return 0, err
	}

	params := &bot.SendMessageParams{ChatID: chatID, Text: question}
	if markup := inlineKeyboard(buttons); markup != nil {
		params.ReplyMarkup = markup
	}

	requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	msg, err := client.SendMessage(requestCtx, params)
	if err != nil {
		return 0, err
	}
	return msg.ID, nil
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

func answerCallbackQuery(ctx context.Context, token, callbackQueryID string) error {
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
	_, err = client.AnswerCallbackQuery(requestCtx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
	})
	return err
}

func removeInlineKeyboard(ctx context.Context, token string, chatID int64, messageID int) error {
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
	_, err = client.EditMessageReplyMarkup(requestCtx, &bot.EditMessageReplyMarkupParams{
		ChatID:    chatID,
		MessageID: messageID,
		ReplyMarkup: models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{},
		},
	})
	return err
}

func appendAnswerText(ctx context.Context, token string, chatID int64, messageID int, text string) error {
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
	_, err = client.EditMessageText(requestCtx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      text,
		ReplyMarkup: models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{},
		},
	})
	return err
}

func react(ctx context.Context, token string, chatID int64, messageID int, emoji string) error {
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
			ReactionTypeEmoji: &models.ReactionTypeEmoji{Emoji: emoji},
		}},
	})
	return err
}

func awaitAnswer(ctx context.Context, token string, chatID int64, questionMsgID int, buttons []string, after time.Time) (promptAnswer, error) {
	found := make(chan promptAnswer, 1)
	extend := make(chan struct{}, 1)
	failures := make(chan error, 1)

	var (
		mu           sync.Mutex
		checkinSent  bool
		checkinMsgID int
	)

	handler := func(_ context.Context, _ *bot.Bot, update *models.Update) {
		if ans := matchingCallback(update, chatID, questionMsgID, buttons); ans != nil {
			select {
			case found <- *ans:
			default:
			}
			return
		}
		if msg := matchingReply(update, chatID, after); msg != nil {
			select {
			case found <- promptAnswer{text: msg.Text, replyMsgID: msg.ID}:
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
		bot.WithAllowedUpdates(bot.AllowedUpdates{
			models.AllowedUpdateMessage,
			models.AllowedUpdateMessageReaction,
			models.AllowedUpdateCallbackQuery,
		}),
		bot.WithHTTPClient(pollTimeout, &http.Client{Timeout: pollTimeout + 5*time.Second}),
	)
	if err != nil {
		return promptAnswer{}, err
	}
	go client.Start(waitCtx)

	deadline := after.Add(promptTimeout)
	timer := time.NewTimer(time.Until(deadline) - checkinLeadTime)
	defer timer.Stop()

	for {
		select {
		case ans := <-found:
			return ans, nil
		case err := <-failures:
			return promptAnswer{}, err
		case <-ctx.Done():
			return promptAnswer{}, ctx.Err()
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
			return promptAnswer{}, errPromptTimeout
		}
	}
}

func resetTimer(t *time.Timer, d time.Duration) {
	if d < 0 {
		d = 0
	}
	t.Reset(d)
}

func matchingCallback(update *models.Update, chatID int64, questionMsgID int, buttons []string) *promptAnswer {
	if update == nil || update.CallbackQuery == nil || len(buttons) == 0 {
		return nil
	}
	cq := update.CallbackQuery
	if cq.Message.Message == nil {
		return nil
	}
	msg := cq.Message.Message
	if msg.Chat.ID != chatID || msg.ID != questionMsgID {
		return nil
	}
	if !strings.HasPrefix(cq.Data, callbackDataPref) {
		return nil
	}
	idxStr := strings.TrimPrefix(cq.Data, callbackDataPref)
	idx, err := strconv.Atoi(idxStr)
	if err != nil || idx < 0 || idx >= len(buttons) {
		return nil
	}
	return &promptAnswer{text: buttons[idx], callbackID: cq.ID}
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
