package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	xterm "github.com/charmbracelet/x/term"
	"github.com/muesli/termenv"
)

const (
	startWaitWindow = 5 * time.Minute
	maxBodyWidth    = 60
	alignMinWidth   = 50
	binDirEnv       = "TELEGRAM_NOTIFY_BIN_DIR"
	shellEnv        = "SHELL"
	programName     = "telegram-notify"
	testMessageText = "Telegram Notifier v" + version + " is connected 👋"
	usageExamples   = "  telegram-notify --text \"Hello from terminal\"\n  telegram-notify --text \"Question?\" --prompt"
)

// silentExitError reports a setup exit whose reason was already shown in the
// transcript, so main must not print it again.
type silentExitError struct{ code int }

func (e silentExitError) Error() string { return fmt.Sprintf("setup exited with code %d", e.code) }

type wizardPhase int

const (
	phaseProbe wizardPhase = iota
	phaseTokenInput
	phaseTokenCheck
	phaseWaitStart
	phaseWaitRetry
	phaseFinishing
	phaseDone
)

type chatStatus int

const (
	statusNone chatStatus = iota
	statusOK
	statusWarn
)

type chatMsg struct {
	from   string
	body   string
	at     time.Time
	status chatStatus
}

type apiProbeMsg struct {
	err error
}

type tokenCheckedMsg struct {
	username string
	err      error
}

type startReceivedMsg struct {
	chatID int64
	err    error
}

type setupFinishedMsg struct {
	testErr error
	onPath  bool
}

type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

type wizard struct {
	app      application
	ctx      context.Context
	out      *lipgloss.Renderer
	width    int
	path     string
	cfg      config
	token    string
	persist  bool
	chat     *chatTarget
	username string
	phase    wizardPhase
	input    textinput.Model
	spin     spinner.Model
	deadline time.Time
	exit     int
	focus    tea.Cmd
	log      []string
}

func newWizard(app application, ctx context.Context, path string, cfg config, resolved settings) wizard {
	w := wizard{
		app:   app,
		ctx:   ctx,
		out:   lipgloss.NewRenderer(app.stdout),
		path:  path,
		cfg:   cfg,
		token: resolved.token,
		chat:  resolved.chatID,
		phase: phaseProbe,
		input: textinput.New(),
		spin:  spinner.New(spinner.WithSpinner(spinner.Dot)),
	}
	w.persist = w.token != "" && cfg.BotToken == w.token
	// Probe the terminal size up front so the greeting renders with the real
	// width instead of the 60-char fallback. Resize events take over after.
	if f, ok := app.stdout.(*os.File); ok {
		if width, _, err := xterm.GetSize(f.Fd()); err == nil {
			w.width = width
		}
	}
	w.input.EchoMode = textinput.EchoPassword
	w.input.EchoCharacter = '•'
	w.input.Placeholder = "paste the token from BotFather"
	w.input.Prompt = w.out.NewStyle().Bold(true).Foreground(lipgloss.Color("4")).Render("You:") + " "
	w.focus = w.input.Focus()
	return w
}

func (w wizard) Init() tea.Cmd {
	return tea.Batch(w.spin.Tick, tea.Sequence(tea.ClearScreen, w.probeCmd()))
}

// greet commits the banner and the opening message once the API probe
// succeeds, and starts token validation when a token is already known.
func (w wizard) greet() (tea.Model, tea.Cmd) {
	if w.token != "" {
		w.phase = phaseTokenCheck
		resume := w.blockMsg("TN", "Hey! Let's finish setting things up.\n\nChecking your bot token...", statusOK)
		return w, tea.Batch(w.focus, tea.Sequence(w.commit(w.versionBlock(), resume), w.checkTokenCmd()))
	}
	w.phase = phaseTokenInput
	welcome := "Hey! Let's set things up.\n\nFirst, create a Telegram bot using @BotFather:\n" +
		w.hyperlink("https://t.me/BotFather") +
		"\n\nThen paste the API token BotFather gives you.\nYour token won't be shown on screen."
	return w, tea.Batch(w.focus, w.commit(w.versionBlock(), w.blockMsg("TN", welcome, statusOK)))
}

// offline commits the offline screen with the direct CLI alternative and
// quits. The probe already showed everything the user needs, so main exits
// silently with code 1.
func (w wizard) offline() (tea.Model, tea.Cmd) {
	w.exit = 1
	w.phase = phaseDone
	body := "Hey! The Telegram API seems to be unreachable.\n\n" +
		"If you already know your bot token and chat ID, you can set them directly:\n\n" +
		"  telegram-notify --set-token TOKEN\n" +
		"  telegram-notify --set-chat-id ID\n\n" +
		"Fix the connection, then run telegram-notify again to set things up interactively."
	return w, tea.Sequence(w.commit(w.versionBlock(), w.blockMsg("TN", body, statusWarn), "\n\n"+usageText), tea.Quit)
}

func (w wizard) probeCmd() tea.Cmd {
	check, ctx := w.app.checkAPI, w.ctx
	return func() tea.Msg {
		return apiProbeMsg{err: check(ctx)}
	}
}

func (w wizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w.width = msg.Width
		return w, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			w.exit = 130
			return w, tea.Quit
		}
		switch w.phase {
		case phaseProbe, phaseTokenInput:
			if msg.Type == tea.KeyEnter && w.phase == phaseTokenInput {
				return w.submitToken()
			}
			// Keystrokes during the probe are buffered in the input so
			// fast typing is not lost before the greeting appears.
			var cmd tea.Cmd
			w.input, cmd = w.input.Update(msg)
			return w, cmd
		case phaseWaitRetry:
			if msg.Type == tea.KeyEnter {
				cmds := w.startWaiting()
				return w, tea.Batch(cmds...)
			}
		}
		return w, nil
	case apiProbeMsg:
		if msg.err != nil {
			return w.offline()
		}
		return w.greet()
	case spinner.TickMsg:
		if w.phase != phaseWaitStart && w.phase != phaseProbe {
			return w, nil
		}
		var cmd tea.Cmd
		w.spin, cmd = w.spin.Update(msg)
		return w, cmd
	case tickMsg:
		if w.phase != phaseWaitStart {
			return w, nil
		}
		return w, tickCmd()
	case tokenCheckedMsg:
		return w.tokenChecked(msg)
	case startReceivedMsg:
		return w.startReceived(msg)
	case setupFinishedMsg:
		return w.setupFinished(msg)
	default:
		if w.phase == phaseTokenInput {
			var cmd tea.Cmd
			w.input, cmd = w.input.Update(msg)
			return w, cmd
		}
	}
	return w, nil
}

func (w wizard) View() string {
	switch w.phase {
	case phaseProbe:
		return w.out.NewStyle().Faint(true).Render(w.spin.View() + " Connecting to Telegram...")
	case phaseTokenInput:
		return w.input.View()
	case phaseWaitStart:
		line := w.spin.View() + " Waiting " + formatRemaining(w.deadline.Sub(w.app.now()))
		return w.out.NewStyle().Faint(true).Render(line)
	default:
		return ""
	}
}

func (w wizard) submitToken() (tea.Model, tea.Cmd) {
	raw := w.input.Value()
	w.input.SetValue("")
	w.token = strings.TrimSpace(raw)
	if w.token == "" {
		return w, w.commit(w.blockMsg("TN", "Bot token cannot be empty.", statusWarn))
	}
	w.persist = true
	w.app.rememberSecret(w.token)
	w.phase = phaseTokenCheck
	you := w.blockMsg("You", strings.Repeat("•", lipgloss.Width(raw)), statusOK)
	checking := w.blockMsg("TN", "Got it. Checking...", statusOK)
	return w, tea.Sequence(w.commit(you, checking), w.checkTokenCmd())
}

func (w wizard) tokenChecked(msg tokenCheckedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		if w.ctx.Err() != nil {
			w.exit = 130
			return w, tea.Quit
		}
		if isInvalidBotToken(msg.err) {
			w.phase = phaseTokenInput
			body := "That token didn't work. Check if you copied it in full.\n\nPaste it again and press Enter."
			return w, w.commit(w.blockMsg("TN", body, statusWarn))
		}
		return w.fail("Couldn't reach Telegram: " + w.app.redact(msg.err.Error()))
	}
	w.username = msg.username
	if w.persist {
		w.cfg.BotToken = w.token
		w.cfg.BotUsername = w.username
		if err := saveConfig(w.path, w.cfg); err != nil {
			return w.fail("Couldn't save the bot token: " + err.Error())
		}
	}
	if w.chat != nil {
		w.phase = phaseFinishing
		return w, tea.Sequence(w.commit(w.blockMsg("TN", "Bot connected: @"+w.username+".", statusOK)), w.finishCmd(w.chat.value))
	}
	body := "Bot connected: @" + w.username + "\n\nNow let's connect your Telegram chat.\n\nOpen the bot and send:\n\n  /start\n\n" +
		w.hyperlink("https://t.me/"+w.username) + "\n\nI'll wait for 5 minutes."
	cmds := w.startWaiting()
	return w, tea.Sequence(w.commit(w.blockMsg("TN", body, statusOK)), tea.Batch(cmds...))
}

func (w *wizard) startWaiting() []tea.Cmd {
	w.phase = phaseWaitStart
	w.deadline = w.app.now().Add(startWaitWindow)
	return []tea.Cmd{w.waitStartCmd(), tickCmd(), w.spin.Tick}
}

func (w wizard) startReceived(msg startReceivedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		if w.ctx.Err() != nil {
			w.exit = 130
			return w, tea.Quit
		}
		if errors.Is(msg.err, context.DeadlineExceeded) {
			w.phase = phaseWaitRetry
			body := "I didn't receive /start.\n\nMake sure you've opened a private chat with:\n@" + w.username +
				"\n\nPress Enter to wait another 5 minutes,\nor Ctrl+C to exit."
			return w, w.commit(w.blockMsg("TN", body, statusWarn))
		}
		return w.fail("Couldn't reach Telegram while waiting for /start: " + w.app.redact(msg.err.Error()))
	}
	w.cfg.ChatID = &chatTarget{value: msg.chatID}
	if err := saveConfig(w.path, w.cfg); err != nil {
		return w.fail("Chat connected, but I couldn't save it: " + err.Error())
	}
	w.phase = phaseFinishing
	return w, tea.Sequence(w.commit(w.blockMsg("TN", "Chat connected. You can now send messages from this machine.", statusOK)), w.finishCmd(msg.chatID))
}

func (w wizard) setupFinished(msg setupFinishedMsg) (tea.Model, tea.Cmd) {
	w.phase = phaseDone
	body := "Setup complete. I sent a test message to your chat."
	status := statusOK
	if msg.testErr != nil {
		w.exit = 1
		status = statusWarn
		body = "Setup is complete, but the test message didn't arrive:\n\n" + w.app.redact(msg.testErr.Error())
	}
	return w, tea.Sequence(w.commit(w.blockMsg("TN", body, status), w.pathBlock(msg)), tea.Quit)
}

func (w wizard) fail(body string) (tea.Model, tea.Cmd) {
	w.exit = 1
	w.phase = phaseDone
	return w, tea.Sequence(w.commit(w.blockMsg("TN", body, statusWarn)), tea.Quit)
}

func (w wizard) pathBlock(msg setupFinishedMsg) string {
	if msg.onPath {
		return w.blockMsg("TN", "telegram-notify is ready to use:\n\n"+usageExamples, statusOK)
	}
	dir := strings.TrimSpace(w.app.getenv(binDirEnv))
	shell := filepath.Base(strings.TrimSpace(w.app.getenv(shellEnv)))
	var cfgFile, snippet string
	switch shell {
	case "zsh":
		cfgFile, snippet = "~/.zshrc", "export PATH=\""+dir+":$PATH\""
	case "bash":
		cfgFile, snippet = "~/.bashrc", "export PATH=\""+dir+":$PATH\""
	case "fish":
		cfgFile, snippet = "~/.config/fish/config.fish", "fish_add_path "+dir
	}
	body := "One more thing: your shell can't find telegram-notify yet."
	if cfgFile != "" && dir != "" {
		body += "\n\nAdd this to " + cfgFile + ":\n\n  " + snippet + "\n\nThen run:\n\n" + usageExamples
	} else {
		body += "\n\nAdd telegram-notify's directory to your PATH, then run:\n\n" + usageExamples
	}
	return w.blockMsg("TN", body, statusWarn)
}

func (w wizard) checkTokenCmd() tea.Cmd {
	token, validate, ctx := w.token, w.app.validateToken, w.ctx
	return func() tea.Msg {
		username, err := validate(ctx, token)
		return tokenCheckedMsg{username: username, err: err}
	}
}

func (w wizard) waitStartCmd() tea.Cmd {
	token, learn, ctx, startedAt := w.token, w.app.learn, w.ctx, w.app.now()
	return func() tea.Msg {
		waitCtx, cancel := context.WithTimeout(ctx, startWaitWindow)
		defer cancel()
		chatID, err := learn(waitCtx, token, startedAt)
		return startReceivedMsg{chatID: chatID, err: err}
	}
}

func (w wizard) finishCmd(chatID any) tea.Cmd {
	token, app, ctx := w.token, w.app, w.ctx
	return func() tea.Msg {
		_, testErr := app.send(ctx, token, chatID, outgoing{text: testMessageText})
		_, err := app.lookPath(programName)
		return setupFinishedMsg{testErr: testErr, onPath: err == nil}
	}
}

// commit records a transcript block in the model and returns a command that
// prints it above the live region, where it survives in terminal history.
func (w *wizard) commit(blocks ...string) tea.Cmd {
	w.log = append(w.log, blocks...)
	return tea.Println(strings.Join(blocks, "\n\n") + "\n")
}

func (w wizard) blockMsg(from, body string, status chatStatus) string {
	return renderChatMessage(chatMsg{from: from, body: body, at: w.app.now(), status: status}, w.width, w.out)
}

func (w wizard) hyperlink(url string) string {
	if w.out.ColorProfile() == termenv.Ascii {
		return url
	}
	text := w.out.NewStyle().Foreground(lipgloss.Color("14")).Italic(true).Render(url)
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

// versionBlock frames the version banner in a thin grey border, 60 chars
// wide (or the terminal width when narrower), with centered text.
func (w wizard) versionBlock() string {
	width := maxBodyWidth
	if w.width > 0 && w.width < maxBodyWidth {
		width = w.width
	}
	return "\n" + w.out.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("8")).
		Width(width-2).
		Align(lipgloss.Center).
		Render("Telegram Notifier v"+version)
}

func renderChatMessage(m chatMsg, width int, r *lipgloss.Renderer) string {
	left := r.NewStyle().Bold(true).Foreground(lipgloss.Color("6")).Render("TN:")
	if m.from == "You" {
		left = r.NewStyle().Bold(true).Foreground(lipgloss.Color("4")).Render("You:")
	}
	right := r.NewStyle().Faint(true).Render(m.at.Format("15:04"))
	switch m.status {
	case statusOK:
		right += " " + r.NewStyle().Faint(true).Foreground(lipgloss.Color("2")).Render("✓")
	case statusWarn:
		right += " " + r.NewStyle().Foreground(lipgloss.Color("3")).Render("!")
	}
	textWidth := width
	if textWidth == 0 || textWidth > maxBodyWidth {
		textWidth = maxBodyWidth
	}
	header := left + " " + right
	if textWidth >= alignMinWidth {
		if pad := textWidth - lipgloss.Width(left) - lipgloss.Width(right); pad >= 1 {
			header = left + strings.Repeat(" ", pad) + right
		}
	}
	body := m.body
	if textWidth > 0 {
		body = ansi.Wrap(body, textWidth, "")
	}
	return header + "\n" + body
}

func formatRemaining(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	seconds := int(d / time.Second)
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}
