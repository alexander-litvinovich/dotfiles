package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	botTokenEnv      = "TG_BOT_TOKEN"
	chatIDEnv        = "TG_CHAT_ID"
	configDirName    = "telegram-notify"
	configName       = "config.json"
	requestTimeout   = 10 * time.Second
	probeTimeout     = 5 * time.Second
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
	errPromptTimeout = errors.New("timed out waiting for a reply")
)

type config struct {
	BotToken    string      `json:"bot_token,omitempty"`
	ChatID      *chatTarget `json:"chat_id,omitempty"`
	BotUsername string      `json:"bot_username,omitempty"`
}

type chatTarget struct {
	value any
}

func (c chatTarget) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.value)
}

func (c *chatTarget) UnmarshalJSON(data []byte) error {
	var id int64
	if err := json.Unmarshal(data, &id); err == nil {
		if id == 0 {
			return errors.New("chat ID must not be zero")
		}
		c.value = id
		return nil
	}

	var username string
	if err := json.Unmarshal(data, &username); err != nil {
		return errors.New("chat ID must be an integer or an @channel username")
	}
	target, err := parseChatTarget(username, "chat ID")
	if err != nil {
		return err
	}
	c.value = target.value
	return nil
}

type stringOption struct {
	value string
	set   bool
}

func (o *stringOption) Set(value string) error {
	o.value = value
	o.set = true
	return nil
}

func (o *stringOption) String() string { return o.value }

type stringList []string

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func (s *stringList) String() string { return strings.Join(*s, ",") }

type cliOptions struct {
	text      stringOption
	token     stringOption
	chatID    stringOption
	setToken  stringOption
	setChatID stringOption
	buttons   stringList
	prompt    bool
	learn     bool
	help      bool
}

type settings struct {
	token  string
	chatID *chatTarget
}

type promptAnswer struct {
	text       string
	replyMsgID int
	callbackID string
}

type application struct {
	getenv         func(string) string
	stdout         io.Writer
	stderr         io.Writer
	now            func() time.Time
	configPath     func() (string, error)
	validateToken  func(context.Context, string) (string, error)
	checkAPI       func(context.Context) error
	lookPath       func(string) (string, error)
	learn          func(context.Context, string, time.Time) (int64, error)
	send           func(context.Context, string, any, string) error
	sendTracked    func(context.Context, string, any, string) (int, error)
	sendPrompt     func(context.Context, string, any, string, []string) (int, error)
	awaitAnswer    func(context.Context, string, int64, int, []string, time.Time) (promptAnswer, error)
	answerCallback func(context.Context, string, string) error
	removeKeyboard func(context.Context, string, int64, int) error
	appendAnswer   func(context.Context, string, int64, int, string) error
	react          func(context.Context, string, int64, int, string) error
	redactions     *[]string
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	redactions := []string{os.Getenv(botTokenEnv)}
	app := application{
		getenv:         os.Getenv,
		stdout:         os.Stdout,
		stderr:         os.Stderr,
		now:            time.Now,
		configPath:     defaultConfigPath,
		validateToken:  validateBotToken,
		checkAPI:       checkTelegramAPI,
		lookPath:       exec.LookPath,
		learn:          learnChat,
		send:           sendMessage,
		sendTracked:    sendMessageTracked,
		sendPrompt:     sendPromptMessage,
		awaitAnswer:    awaitAnswer,
		answerCallback: answerCallbackQuery,
		removeKeyboard: removeInlineKeyboard,
		appendAnswer:   appendAnswerText,
		react:          react,
		redactions:     &redactions,
	}

	if err := app.run(ctx, os.Args[1:]); err != nil {
		var silent silentExitError
		if errors.As(err, &silent) {
			os.Exit(silent.code)
		}
		fmt.Fprintf(os.Stderr, "telegram-notify: %s\n", redactAll(err.Error(), redactions))
		os.Exit(1)
	}
}

func (app application) run(ctx context.Context, args []string) error {
	opts, err := parseCLI(args)
	if err != nil {
		return err
	}
	app.rememberSecret(opts.token.value)
	app.rememberSecret(opts.setToken.value)
	if opts.help {
		app.printHelp()
		return nil
	}

	path, err := app.configPath()
	if err != nil {
		return err
	}
	cfg, err := loadConfig(path)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	app.rememberSecret(cfg.BotToken)

	if opts.setToken.set || opts.setChatID.set {
		return app.runSet(path, cfg, opts)
	}

	resolved, err := app.resolveSettings(cfg, opts)
	if err != nil {
		return err
	}

	if opts.learn {
		if resolved.token == "" {
			return app.notConfigured("bot token")
		}
		return app.runLearn(ctx, path, cfg, resolved.token)
	}

	if opts.text.set {
		if missing := missingSettings(resolved); len(missing) > 0 {
			return app.notConfigured(missing...)
		}
		if opts.prompt {
			return app.runPrompt(ctx, resolved.token, resolved.chatID.value, opts.text.value, opts.buttons)
		}
		if err := app.send(ctx, resolved.token, resolved.chatID.value, opts.text.value); err != nil {
			return fmt.Errorf("send message: %w", err)
		}
		return nil
	}

	if resolved.token == "" || resolved.chatID == nil {
		return app.runSetup(ctx, path, cfg, resolved)
	}
	app.printHelp()
	return nil
}

func parseCLI(args []string) (cliOptions, error) {
	var opts cliOptions
	flags := flag.NewFlagSet("telegram-notify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Var(&opts.text, "text", "message or question text")
	flags.BoolVar(&opts.prompt, "prompt", false, "wait for a Telegram reply")
	flags.Var(&opts.buttons, "button", "prompt button label; repeatable")
	flags.Var(&opts.token, "token", "temporary bot token override")
	flags.Var(&opts.chatID, "chat-id", "temporary chat ID override")
	flags.Var(&opts.setToken, "set-token", "save a bot token without API validation")
	flags.Var(&opts.setChatID, "set-chat-id", "save a chat ID without contacting Telegram")
	flags.BoolVar(&opts.learn, "learn", false, "learn and save a private chat ID from /start")
	flags.BoolVar(&opts.help, "help", false, "show help")
	flags.BoolVar(&opts.help, "h", false, "show help")

	if err := flags.Parse(args); err != nil {
		return cliOptions{}, fmt.Errorf("%w\n\n%s", err, usageText)
	}
	if opts.help {
		return opts, nil
	}
	if flags.NArg() != 0 {
		return cliOptions{}, fmt.Errorf("unexpected positional arguments: %s\n\n%s", strings.Join(flags.Args(), " "), usageText)
	}

	setMode := opts.setToken.set || opts.setChatID.set
	if setMode {
		if opts.text.set || opts.prompt || len(opts.buttons) > 0 || opts.learn || opts.token.set || opts.chatID.set {
			return cliOptions{}, errors.New("--set-token and --set-chat-id cannot be combined with action or override flags")
		}
		if opts.setToken.set && strings.TrimSpace(opts.setToken.value) == "" {
			return cliOptions{}, errors.New("--set-token requires a non-empty value")
		}
		if opts.setChatID.set {
			if _, err := parseChatTarget(opts.setChatID.value, "--set-chat-id"); err != nil {
				return cliOptions{}, err
			}
		}
		return opts, nil
	}

	if opts.learn {
		if opts.text.set || opts.prompt || len(opts.buttons) > 0 {
			return cliOptions{}, errors.New("--learn cannot be combined with --text, --prompt, or --button")
		}
		if opts.chatID.set {
			return cliOptions{}, errors.New("--learn cannot be combined with --chat-id")
		}
	}
	if opts.text.set && strings.TrimSpace(opts.text.value) == "" {
		return cliOptions{}, errors.New("--text requires a non-empty value")
	}
	if opts.prompt && !opts.text.set {
		return cliOptions{}, errors.New("--prompt requires --text")
	}
	if len(opts.buttons) > 0 && !opts.prompt {
		return cliOptions{}, errors.New("--button requires --prompt")
	}
	for _, button := range opts.buttons {
		if strings.TrimSpace(button) == "" {
			return cliOptions{}, errors.New("--button requires a non-empty label")
		}
	}
	return opts, nil
}

const usageText = `Usage:
  telegram-notify --text "message"
  telegram-notify --text "question" --prompt [--button LABEL ...]
  telegram-notify --learn [--token TOKEN]
  telegram-notify --set-token TOKEN [--set-chat-id ID]
  telegram-notify --set-chat-id ID

Options:
  --text TEXT        Message or question text.
  --prompt           Wait for a text reply or button tap.
  --button LABEL     Add a prompt button. Repeat for more buttons.
  --token TOKEN      Override the bot token for this invocation.
  --chat-id ID       Override the chat for this invocation.
  --set-token TOKEN  Save a bot token without API validation.
  --set-chat-id ID   Save a numeric chat ID or @username.
  --learn            Replace the saved chat ID after receiving /start.
  --help, -h         Show this help.`

func (app application) printHelp() {
	fmt.Fprintln(app.stdout, usageText)
}

func (app application) resolveSettings(cfg config, opts cliOptions) (settings, error) {
	token := strings.TrimSpace(app.getenv(botTokenEnv))
	if strings.TrimSpace(cfg.BotToken) != "" {
		token = strings.TrimSpace(cfg.BotToken)
	}
	if opts.token.set {
		token = strings.TrimSpace(opts.token.value)
		if token == "" {
			return settings{}, errors.New("--token requires a non-empty value")
		}
	}

	var chatID *chatTarget
	if opts.chatID.set {
		parsed, err := parseChatTarget(opts.chatID.value, "--chat-id")
		if err != nil {
			return settings{}, err
		}
		chatID = parsed
	} else if cfg.ChatID != nil {
		chatID = cfg.ChatID
	} else if raw := strings.TrimSpace(app.getenv(chatIDEnv)); raw != "" {
		parsed, err := parseChatTarget(raw, chatIDEnv)
		if err != nil {
			return settings{}, err
		}
		chatID = parsed
	}
	return settings{token: token, chatID: chatID}, nil
}

func missingSettings(resolved settings) []string {
	var missing []string
	if resolved.token == "" {
		missing = append(missing, "bot token")
	}
	if resolved.chatID == nil {
		missing = append(missing, "chat ID")
	}
	return missing
}

// notConfigured prints what the command was missing to stdout and returns a
// silent exit so the message is not duplicated on stderr. Scripts and agents
// parse this line to decide how to finish setup.
func (app application) notConfigured(missing ...string) error {
	fmt.Fprintf(app.stdout, "not configured: missing %s\n", strings.Join(missing, " and "))
	return silentExitError{code: 1}
}

func parseChatTarget(value, source string) (*chatTarget, error) {
	value = strings.TrimSpace(value)
	if id, err := strconv.ParseInt(value, 10, 64); err == nil {
		if id == 0 {
			return nil, fmt.Errorf("%s must not be zero", source)
		}
		return &chatTarget{value: id}, nil
	}
	if strings.HasPrefix(value, "@") && len(value) > 1 && !strings.ContainsAny(value, " \t\r\n") {
		return &chatTarget{value: value}, nil
	}
	return nil, fmt.Errorf("%s must be an integer or an @channel username", source)
}

func (app application) runSet(path string, cfg config, opts cliOptions) error {
	if opts.setToken.set {
		cfg.BotToken = strings.TrimSpace(opts.setToken.value)
	}
	if opts.setChatID.set {
		chatID, err := parseChatTarget(opts.setChatID.value, "--set-chat-id")
		if err != nil {
			return err
		}
		cfg.ChatID = chatID
	}
	if err := saveConfig(path, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	if opts.setToken.set && opts.setChatID.set {
		fmt.Fprintln(app.stdout, "Bot token and chat ID saved.")
	} else if opts.setToken.set {
		fmt.Fprintln(app.stdout, "Bot token saved.")
	} else {
		fmt.Fprintln(app.stdout, "Chat ID saved.")
	}
	return nil
}

func (app application) runSetup(ctx context.Context, path string, cfg config, resolved settings) error {
	program := tea.NewProgram(newWizard(app, ctx, path, cfg, resolved), tea.WithContext(ctx), tea.WithOutput(app.stdout))
	final, err := program.Run()
	if err != nil {
		if errors.Is(err, tea.ErrProgramKilled) || ctx.Err() != nil {
			return silentExitError{code: 130}
		}
		return fmt.Errorf("run setup: %w", err)
	}
	if w, ok := final.(wizard); ok && w.exit != 0 {
		return silentExitError{code: w.exit}
	}
	return nil
}

func (app application) runLearn(ctx context.Context, path string, cfg config, token string) error {
	if _, err := app.validateToken(ctx, token); err != nil {
		return fmt.Errorf("validate bot token: %w", err)
	}
	fmt.Fprintln(app.stdout, "Send /start to the bot in a private chat. Waiting...")

	chatID, err := app.learn(ctx, token, app.now())
	if err != nil {
		return fmt.Errorf("learn chat ID: %w", err)
	}

	cfg.ChatID = &chatTarget{value: chatID}
	if err := saveConfig(path, cfg); err != nil {
		return fmt.Errorf("save chat ID: %w", err)
	}
	return app.confirmConnection(ctx, token, cfg.ChatID)
}

func (app application) confirmConnection(ctx context.Context, token string, chatID *chatTarget) error {
	if err := app.send(ctx, token, chatID.value, testMessageText); err != nil {
		return fmt.Errorf("chat ID was saved, but confirmation failed: %w", err)
	}
	fmt.Fprintf(app.stdout, "Connected to chat %v.\n", chatID.value)
	return nil
}

func (app application) runPrompt(ctx context.Context, token string, chatID any, question string, buttons []string) error {
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
				app.warn("failed to remove keyboard", rmErr)
			}
		}
		if errors.Is(err, errPromptTimeout) {
			if sendErr := app.send(ctx, token, chatID, timeoutText); sendErr != nil {
				app.warn("failed to send timeout notice", sendErr)
			}
		}
		return err
	}

	if ans.callbackID != "" {
		if err := app.answerCallback(ctx, token, ans.callbackID); err != nil {
			app.warn("answer callback failed", err)
		}
		answeredText := question + "\n\nAnswer: " + ans.text
		if err := app.appendAnswer(ctx, token, numericChatID, msgID, answeredText); err != nil {
			app.warn("failed to record answer on message", err)
		}
		if err := app.react(ctx, token, numericChatID, msgID, checkmarkEmoji); err != nil {
			app.warn("react with checkmark failed", err)
		}
	} else {
		if len(buttons) > 0 {
			if err := app.removeKeyboard(ctx, token, numericChatID, msgID); err != nil {
				app.warn("failed to remove keyboard", err)
			}
		}
		if err := app.react(ctx, token, numericChatID, ans.replyMsgID, eyesEmoji); err != nil {
			app.warn("react with eyes failed", err)
		}
	}

	fmt.Fprintln(app.stdout, ans.text)
	return nil
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
		return config{}, nil
	}
	if err != nil {
		return config{}, err
	}
	defer file.Close()

	var cfg config
	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		return config{}, err
	}
	cfg.BotToken = strings.TrimSpace(cfg.BotToken)
	return cfg, nil
}

func saveConfig(path string, cfg config) error {
	cfg.BotToken = strings.TrimSpace(cfg.BotToken)
	if cfg.BotToken == "" && cfg.ChatID == nil {
		return errors.New("config is empty")
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

func validateBotToken(ctx context.Context, token string) (string, error) {
	client, err := bot.New(
		token,
		bot.WithSkipGetMe(),
		bot.WithHTTPClient(requestTimeout, &http.Client{Timeout: requestTimeout}),
	)
	if err != nil {
		return "", err
	}
	requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	user, err := client.GetMe(requestCtx)
	if err != nil {
		return "", err
	}
	return user.Username, nil
}

// checkTelegramAPI reports whether the Telegram API host is reachable. Any
// HTTP response counts as reachable; only transport errors mean offline.
func checkTelegramAPI(ctx context.Context) error {
	requestCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, "https://api.telegram.org", nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	return err
}

func isInvalidBotToken(err error) bool {
	return errors.Is(err, bot.ErrorUnauthorized) || errors.Is(err, bot.ErrorNotFound)
}

func (app application) rememberSecret(secret string) {
	secret = strings.TrimSpace(secret)
	if secret != "" && app.redactions != nil {
		*app.redactions = append(*app.redactions, secret)
	}
}

func (app application) redact(message string) string {
	if app.redactions == nil {
		return message
	}
	return redactAll(message, *app.redactions)
}

func (app application) warn(message string, err error) {
	stderr := app.stderr
	if stderr == nil {
		stderr = io.Discard
	}
	fmt.Fprintf(stderr, "telegram-notify: %s: %s\n", message, app.redact(err.Error()))
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
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return message
	}
	return strings.ReplaceAll(message, secret, "[REDACTED]")
}

func redactAll(message string, secrets []string) string {
	for _, secret := range secrets {
		message = redact(message, secret)
	}
	return message
}
