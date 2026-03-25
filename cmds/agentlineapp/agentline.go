package agentlineapp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	acp "github.com/coder/acp-go-sdk"

	"github.com/pubgo/redant"
	agentacp "github.com/pubgo/redant/cmds/agentlineapp/acp"
	agentlinemodule "github.com/pubgo/redant/pkg/agentline"
)

const (
	defaultSuggestionRows = 8
	defaultOutputRows     = 20
	defaultInputRows      = 4
	minOutputRows         = 6
	maxOutputBlocks       = 500
	maxOutputLines        = 4000
)

type mouseRegion string

const (
	mouseRegionOutput mouseRegion = "output"
	mouseRegionInput  mouseRegion = "input"
)

type mouseScrollMsg struct {
	Region mouseRegion
	Delta  int
}

type mouseFocusMsg struct {
	Region mouseRegion
}

type mouseSelectHistoryMsg struct {
	HistoryIndex int
}

type blockKind string

const (
	blockKindSystem    blockKind = "system"
	blockKindUser      blockKind = "user"
	blockKindAssistant blockKind = "assistant"
	blockKindTool      blockKind = "tool"
	blockKindCommand   blockKind = "command"
	blockKindResult    blockKind = "result"
	blockKindError     blockKind = "error"
)

type sessionBlock struct {
	Kind  blockKind
	Title string
	Lines []string
}

type completionItem struct {
	Insert      string
	Description string
}

type stickyInvocation struct {
	BaseArgs   []string
	PromptFlag string
}

type interactionMode string

const (
	interactionModeCommand interactionMode = "command"
	interactionModeChat    interactionMode = "chat"
)

var (
	stylePrompt          = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	styleInputText       = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	styleHeader          = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	styleHint            = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styleSelected        = lipgloss.NewStyle().Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230")).Bold(true)
	styleHistorySelected = lipgloss.NewStyle().Background(lipgloss.Color("60")).Foreground(lipgloss.Color("230"))
	styleRunning         = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	styleDesc            = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	styleStatusIdle      = lipgloss.NewStyle().Foreground(lipgloss.Color("114")).Bold(true)
	styleStatusBusy      = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)

	styleKindSystem    = lipgloss.NewStyle().Foreground(lipgloss.Color("110")).Bold(true)
	styleKindUser      = lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	styleKindAssistant = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true)
	styleKindTool      = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
	styleKindCommand   = lipgloss.NewStyle().Foreground(lipgloss.Color("150")).Bold(true)
	styleKindResult    = lipgloss.NewStyle().Foreground(lipgloss.Color("121")).Bold(true)
	styleKindError     = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
)

type RuntimeOptions struct {
	Prompt      string
	HistoryFile string
	NoHistory   bool
	InitialArgv []string
	Stdin       io.Reader
	Stdout      io.Writer
}

func Run(ctx context.Context, root *redant.Command, opts *RuntimeOptions) error {
	if root == nil {
		return errors.New("agentline runtime requires non-nil root command")
	}

	cfg := RuntimeOptions{}
	if opts != nil {
		cfg = *opts
	}

	historyFile := strings.TrimSpace(cfg.HistoryFile)
	if historyFile == "" && !cfg.NoHistory {
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			historyFile = filepath.Join(home, ".redant_agentline_history")
		}
	}

	historyLines := []string{}
	if !cfg.NoHistory && historyFile != "" {
		historyLines = loadHistory(historyFile)
	}

	input := cfg.Stdin
	if input == nil {
		input = os.Stdin
	}
	output := cfg.Stdout
	if output == nil {
		output = os.Stdout
	}

	model := newAgentlineModel(ctx, root, strings.TrimSpace(cfg.Prompt), historyLines, historyFile, !cfg.NoHistory, append([]string(nil), cfg.InitialArgv...))
	p := tea.NewProgram(model, tea.WithInput(input), tea.WithOutput(output))

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			p.Quit()
		case <-done:
		}
	}()

	_, err := p.Run()
	close(done)
	return err
}

func (m *agentlineModel) buildStickyCommandLine(prompt string) string {
	if m == nil || m.stickyInvocation == nil || m.root == nil {
		return ""
	}

	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		prompt = "继续"
	}

	args := append([]string(nil), m.stickyInvocation.BaseArgs...)
	args = append(args, m.stickyInvocation.PromptFlag, prompt)
	return formatCommandLine(m.root.Name(), args)
}

func buildStickyInvocation(root *redant.Command, commandLine string, agentOnly bool) (*stickyInvocation, error) {
	args, err := splitCommandLine(commandLine)
	if err != nil {
		return nil, fmt.Errorf("解析命令失败: %w", err)
	}
	if len(args) == 0 {
		return nil, errors.New("/chat 需要指定命令，例如 /chat commit --message hi")
	}

	resolvedLine := strings.Join(args, " ")
	cmd, ok := resolveCommandLikeInput(root, resolvedLine, false)
	if !ok || cmd == nil {
		return nil, errors.New("/chat 仅支持可执行命令")
	}
	if agentOnly && !agentlinemodule.IsAgentCommand(cmd.Metadata) {
		return nil, errors.New("当前命令未标记为 agent 命令，无法进入聊天粘性模式")
	}

	promptFlag := strings.TrimSpace(agentlinemodule.Meta(cmd.Metadata, "agentline.prompt-flag"))
	if promptFlag == "" {
		promptFlag = "--prompt"
	}

	baseArgs := stripRootIfPresent(root, args)
	baseArgs = stripPromptArg(baseArgs, promptFlag)
	if len(baseArgs) == 0 {
		return nil, errors.New("无法提取聊天粘性命令参数")
	}

	return &stickyInvocation{BaseArgs: baseArgs, PromptFlag: promptFlag}, nil
}

func stripRootIfPresent(root *redant.Command, args []string) []string {
	if root == nil || len(args) == 0 {
		return append([]string(nil), args...)
	}
	if strings.TrimSpace(args[0]) == root.Name() {
		return append([]string(nil), args[1:]...)
	}
	return append([]string(nil), args...)
}

func stripPromptArg(args []string, promptFlag string) []string {
	promptFlag = strings.TrimSpace(promptFlag)
	if promptFlag == "" {
		return append([]string(nil), args...)
	}

	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		item := strings.TrimSpace(args[i])
		if item == promptFlag {
			if i+1 < len(args) {
				i++
			}
			continue
		}
		if strings.HasPrefix(item, promptFlag+"=") {
			continue
		}
		out = append(out, args[i])
	}
	return out
}

type agentlineModel struct {
	ctx              context.Context
	root             *redant.Command
	input            textinput.Model
	prompt           string
	mode             interactionMode
	sessionCWD       string
	sessionGitBranch string
	sessionGitDirty  bool
	stickyInvocation *stickyInvocation
	history          []string
	historyPos       int
	historyFile      string
	persistHistory   bool
	blocks           []sessionBlock
	suggestions      []completionItem
	selected         int
	running          bool
	width            int
	height           int
	outputOffset     int
	outputFocus      bool
	inputOffset      int
	selectedHistory  int
	foldDetails      bool
	currentCancel    context.CancelFunc
	initialArgv      []string
	agentOnlyMode    bool
	permissionBroker *agentacp.PermissionBroker
	questionBroker   *QuestionBroker
	acpEventSeq      int64
	acpEventEntries  []acpEventEntry
}

type runResultMsg struct {
	blocks []sessionBlock
	quit   bool
}

type acpDemoResultMsg struct {
	blocks []sessionBlock
	err    error
}

func newAgentlineModel(ctx context.Context, root *redant.Command, prompt string, history []string, historyFile string, persist bool, initialArgv []string) *agentlineModel {
	ti := textinput.New()
	ti.Prompt = prompt
	styles := textinput.DefaultStyles(true)
	styles.Focused.Prompt = stylePrompt
	styles.Focused.Text = styleInputText
	styles.Blurred.Prompt = stylePrompt
	styles.Blurred.Text = styleInputText
	ti.SetStyles(styles)
	ti.Focus()
	ti.CharLimit = 0
	ti.SetValue("")

	if strings.TrimSpace(prompt) == "" {
		prompt = "agent> "
		ti.Prompt = prompt
	}

	agentOnlyMode := true
	hasAgentCommands := hasAnyAgentCommand(root)
	sessionCWD, sessionGitBranch, sessionGitDirty := detectSessionContext()

	m := &agentlineModel{
		ctx:              ctx,
		root:             root,
		input:            ti,
		prompt:           prompt,
		mode:             interactionModeCommand,
		sessionCWD:       sessionCWD,
		sessionGitBranch: sessionGitBranch,
		sessionGitDirty:  sessionGitDirty,
		history:          append([]string(nil), history...),
		historyPos:       len(history),
		historyFile:      historyFile,
		persistHistory:   persist,
		selectedHistory:  -1,
		initialArgv:      append([]string(nil), initialArgv...),
		agentOnlyMode:    agentOnlyMode,
		permissionBroker: agentacp.NewPermissionBroker(),
		questionBroker:   NewQuestionBroker(),
		blocks: []sessionBlock{{
			Kind:  blockKindSystem,
			Title: "system",
			Lines: []string{
				"agentline started. 默认输入会自动识别为命令执行。",
				fmt.Sprintf("cwd: %s", displayPath(sessionCWD)),
				fmt.Sprintf("git: %s", displayGitBranch(sessionGitBranch, sessionGitDirty)),
				"试试：/run commit --help、/history、/output",
				"快捷键：Tab 补全，↑/↓ 选择候选，Ctrl+O 切换输出滚动，Ctrl+C 退出。",
				"复制提示：支持直接鼠标拖拽选择并复制。",
			},
		}},
	}
	if agentOnlyMode {
		m.blocks[0].Lines = append(m.blocks[0].Lines, "仅加载显式声明为 agent 的命令（metadata: agent.command=true 或等价 agent entry）。")
		if !hasAgentCommands {
			m.blocks[0].Lines = append(m.blocks[0].Lines, "当前未检测到任何 agent 命令，请先为目标命令设置 metadata。")
		}
	}
	m.recomputeSuggestions()
	return m
}

func (m *agentlineModel) Init() tea.Cmd {
	if len(m.initialArgv) == 0 {
		return nil
	}

	request := formatCommandLine(m.root.Name(), m.initialArgv)
	return m.startCommandRun(request)
}

func (m *agentlineModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case runResultMsg:
		m.running = false
		m.currentCancel = nil
		if len(msg.blocks) > 0 {
			m.appendBlocks(msg.blocks)
			m.outputOffset = 0
		}
		if msg.quit {
			return m, tea.Quit
		}
		m.normalizeOutputOffset()
		m.normalizeInputOffset()
		m.recomputeSuggestions()
		return m, nil

	case acpDemoResultMsg:
		m.running = false
		m.currentCancel = nil
		if len(msg.blocks) > 0 {
			m.appendBlocks(msg.blocks)
			m.outputOffset = 0
		}
		if msg.err != nil {
			m.appendBlock(sessionBlock{Kind: blockKindError, Title: "acp.demo", Lines: []string{fmt.Sprintf("%v", msg.err)}})
		}
		m.normalizeOutputOffset()
		m.normalizeInputOffset()
		m.recomputeSuggestions()
		return m, nil

	case mouseScrollMsg:
		switch msg.Region {
		case mouseRegionInput:
			m.outputFocus = false
			m.scrollInputLines(msg.Delta)
		default:
			m.outputFocus = true
			m.scrollOutputLines(msg.Delta)
		}
		return m, nil

	case mouseSelectHistoryMsg:
		if msg.HistoryIndex < 0 || msg.HistoryIndex >= len(m.history) {
			return m, nil
		}
		m.outputFocus = false
		m.historyPos = msg.HistoryIndex
		m.selectedHistory = msg.HistoryIndex
		m.input.SetValue(m.history[msg.HistoryIndex])
		m.input.CursorEnd()
		m.recomputeSuggestions()
		m.normalizeInputOffset()
		return m, nil

	case mouseFocusMsg:
		switch msg.Region {
		case mouseRegionInput:
			m.outputFocus = false
		case mouseRegionOutput:
			m.outputFocus = true
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.width > len([]rune(m.prompt))+4 {
			m.input.SetWidth(m.width - len([]rune(m.prompt)) - 4)
		}
		m.normalizeOutputOffset()
		m.normalizeInputOffset()
		return m, nil

	case tea.KeyMsg:
		key := msg.String()
		switch key {
		case "ctrl+c":
			if m.running {
				if m.cancelActiveRun("收到 Ctrl+C，正在尝试中断当前任务...") {
					return m, nil
				}
				m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "interrupt", Lines: []string{"当前任务不支持中断，等待完成中..."}})
				m.normalizeOutputOffset()
				return m, nil
			}
			return m, tea.Quit
		case "ctrl+o":
			m.outputFocus = !m.outputFocus
			return m, nil
		case "esc":
			if m.outputFocus {
				m.outputFocus = false
				return m, nil
			}
			m.suggestions = nil
			m.selected = 0
			return m, nil
		}

		if m.outputFocus && len(m.suggestions) == 0 {
			switch key {
			case "up":
				m.scrollOutputLines(1)
				return m, nil
			case "down":
				m.scrollOutputLines(-1)
				return m, nil
			case "pgup":
				m.scrollOutputPage(1)
				return m, nil
			case "pgdown":
				m.scrollOutputPage(-1)
				return m, nil
			case "home":
				m.scrollOutputTop()
				return m, nil
			case "end":
				m.scrollOutputBottom()
				return m, nil
			}
		}

		switch key {
		case "tab":
			if strings.TrimSpace(m.input.Value()) == "" && len(m.suggestions) == 0 {
				m.suggestions = collectStarterSlashItems()
				m.suggestions = append(m.suggestions, collectCommandSlashItems(m.root, m.agentOnlyMode, "")...)
				m.suggestions = uniqueCompletionItems(m.suggestions)
				m.selected = 0
				return m, nil
			}
			m.applySuggestion()
			m.recomputeSuggestions()
			m.normalizeOutputOffset()
			return m, nil

		case "home":
			if len(m.suggestions) > 0 {
				m.selected = 0
				return m, nil
			}
			m.scrollOutputTop()
			return m, nil

		case "end":
			if len(m.suggestions) > 0 {
				m.selected = len(m.suggestions) - 1
				return m, nil
			}
			m.scrollOutputBottom()
			return m, nil

		case "pgup":
			if len(m.suggestions) > 0 {
				m.selected -= m.suggestionRows(len(m.suggestions))
				if m.selected < 0 {
					m.selected = 0
				}
				return m, nil
			}
			m.scrollOutputPage(1)
			return m, nil

		case "pgdown":
			if len(m.suggestions) > 0 {
				m.selected += m.suggestionRows(len(m.suggestions))
				if m.selected >= len(m.suggestions) {
					m.selected = len(m.suggestions) - 1
				}
				return m, nil
			}
			m.scrollOutputPage(-1)
			return m, nil

		case "up":
			if len(m.suggestions) > 0 {
				if m.selected > 0 {
					m.selected--
				}
				return m, nil
			}
			m.historyUp()
			m.recomputeSuggestions()
			m.normalizeInputOffset()
			return m, nil

		case "down":
			if len(m.suggestions) > 0 {
				if m.selected < len(m.suggestions)-1 {
					m.selected++
				}
				return m, nil
			}
			m.historyDown()
			m.recomputeSuggestions()
			m.normalizeInputOffset()
			return m, nil

		case "enter":
			line := strings.TrimSpace(m.input.Value())
			if line == "" {
				return m, nil
			}

			if m.running {
				if !strings.HasPrefix(line, "/") && m.questionBroker != nil && len(m.questionBroker.Pending()) > 0 {
					m.appendHistory(line)
					m.input.SetValue("")
					m.historyPos = len(m.history)
					m.suggestions = nil
					m.selected = 0
					m.inputOffset = 0
					m.selectedHistory = -1

					if err := m.questionBroker.Reply("", line); err != nil {
						m.appendBlock(sessionBlock{Kind: blockKindError, Title: "reply.direct", Lines: []string{err.Error()}})
					} else {
						m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "reply.direct", Lines: []string{"已作为问题回复提交（latest）。"}})
					}
					m.normalizeOutputOffset()
					return m, nil
				}

				if !isAllowedWhileRunning(line) {
					m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "running", Lines: []string{
						"当前任务执行中，仅支持 /permissions、/allow、/deny、/questions、/reply、/skip、/cancel。",
						"若存在待回答问题，也可直接输入文本并回车（默认回复最新问题）。",
					}})
					m.normalizeOutputOffset()
					return m, nil
				}
			}

			m.appendHistory(line)
			m.input.SetValue("")
			m.historyPos = len(m.history)
			m.suggestions = nil
			m.selected = 0
			m.inputOffset = 0
			m.selectedHistory = -1

			return m, m.dispatchInputLine(line)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.recomputeSuggestions()
	m.normalizeOutputOffset()
	m.normalizeInputOffset()
	return m, cmd
}

func (m *agentlineModel) dispatchInputLine(line string) tea.Cmd {
	if handled, cmd := m.handleSlashInput(line); handled {
		return cmd
	}

	if m.shouldRenderChatUserInput(line) {
		m.appendBlock(sessionBlock{Kind: blockKindUser, Title: "user", Lines: []string{strings.TrimSpace(line)}})
		m.outputOffset = 0
		m.normalizeOutputOffset()
	}

	if request, ok := m.resolveExecutionRequest(line); ok {
		return m.startCommandRun(request)
	}

	m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "input", Lines: []string{
		"当前为精简命令模式：请使用 /run <command...> 或 /<command ...>。",
		"输入 /help 查看可用命令。",
	}})
	m.outputOffset = 0
	m.normalizeOutputOffset()
	return nil
}

func (m *agentlineModel) shouldRenderChatUserInput(line string) bool {
	if m == nil || !m.isChatMode() {
		return false
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "/") {
		return false
	}
	// 若本身是显式命令输入，则按命令处理，不作为自然语言 chat 轮次回显。
	if isCommandLikeInput(m.root, trimmed, m.agentOnlyMode) {
		return false
	}
	return true
}

func (m *agentlineModel) resolveExecutionRequest(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", false
	}

	if isCommandLikeInput(m.root, line, m.agentOnlyMode) {
		return line, true
	}

	if m.isChatMode() {
		request := m.buildStickyCommandLine(line)
		if strings.TrimSpace(request) != "" {
			return request, true
		}
	}

	return "", false
}

func (m *agentlineModel) bindStickyInvocation(sticky *stickyInvocation) {
	m.stickyInvocation = sticky
	if sticky == nil {
		m.mode = interactionModeCommand
		return
	}
	m.mode = interactionModeChat
}

func (m *agentlineModel) unbindStickyInvocation() {
	m.stickyInvocation = nil
	m.mode = interactionModeCommand
}

func (m *agentlineModel) isChatMode() bool {
	return m.mode == interactionModeChat && m.stickyInvocation != nil
}

func runSlashRunCmd(ctx context.Context, m *agentlineModel, root *redant.Command, commandLine string) tea.Cmd {
	return func() tea.Msg {
		startedAt := time.Now()
		request := strings.TrimSpace(commandLine)
		blocks := []sessionBlock{{
			Kind:  blockKindTool,
			Title: "tool.run",
			Lines: []string{fmt.Sprintf("request: %s", request)},
		}}

		appendResult := func(status string, output []string, runErr error) runResultMsg {
			lines := make([]string, 0, len(output)+2)
			lines = append(lines, fmt.Sprintf("status: %s", status))
			lines = append(lines, fmt.Sprintf("duration: %s", formatDuration(time.Since(startedAt))))
			if len(output) == 0 {
				lines = append(lines, "(no output)")
			} else {
				lines = append(lines, output...)
			}
			resultBlocks := append(blocks, sessionBlock{Kind: blockKindResult, Title: "result", Lines: lines})
			if runErr != nil {
				resultBlocks = append(resultBlocks, sessionBlock{Kind: blockKindError, Title: "error", Lines: []string{fmt.Sprintf("%v", runErr)}})
			}
			return runResultMsg{blocks: resultBlocks}
		}

		args, parseErr := splitCommandLine(request)
		if parseErr != nil {
			blocks = append(blocks, sessionBlock{Kind: blockKindError, Title: "parse", Lines: []string{fmt.Sprintf("parse input failed: %v", parseErr)}})
			return appendResult("failed", nil, parseErr)
		}
		if len(args) == 0 {
			err := errors.New("empty command")
			blocks = append(blocks, sessionBlock{Kind: blockKindError, Title: "parse", Lines: []string{err.Error()}})
			return appendResult("failed", nil, err)
		}
		if args[0] == root.Name() {
			args = args[1:]
		}
		if len(args) == 0 {
			err := errors.New("missing target command after root name")
			blocks = append(blocks, sessionBlock{Kind: blockKindError, Title: "parse", Lines: []string{err.Error()}})
			return appendResult("failed", nil, err)
		}

		blocks = append(blocks, sessionBlock{Kind: blockKindTool, Title: "tool.parse", Lines: []string{
			fmt.Sprintf("argv: %v", args),
			fmt.Sprintf("argc: %d", len(args)),
		}})

		title := "$ " + formatCommandLine(root.Name(), args)
		blocks = append(blocks, sessionBlock{Kind: blockKindCommand, Title: title, Lines: []string{"dispatching..."}})

		stdout := bytes.NewBuffer(nil)
		stderr := bytes.NewBuffer(nil)

		runInv := root.Invoke(args...)
		runInv.Stdout = stdout
		runInv.Stderr = stderr
		runInv.Stdin = strings.NewReader("")
		if runInv.Annotations == nil {
			runInv.Annotations = map[string]any{}
		}
		runInv.Annotations[InteractionAnnotationKey] = &runtimeInteractionBridge{
			emitFn: func(_ context.Context, event InteractionEvent) error {
				kind := interactionKindToBlockKind(event.Kind)
				title := strings.TrimSpace(event.Title)
				if title == "" {
					title = "interaction"
				}
				lines := append([]string(nil), event.Lines...)
				if len(lines) == 0 {
					lines = []string{"(empty)"}
				}
				blocks = append(blocks, sessionBlock{Kind: kind, Title: title, Lines: lines})
				return nil
			},
			askFn: func(ctx context.Context, req AskRequest) (AskResponse, error) {
				if m == nil || m.questionBroker == nil {
					return AskResponse{Cancelled: true}, nil
				}
				prompt := strings.TrimSpace(req.Prompt)
				if prompt == "" {
					return AskResponse{}, errors.New("ask prompt is empty")
				}
				blocks = append(blocks, sessionBlock{Kind: blockKindSystem, Title: "ask.pending", Lines: []string{
					prompt,
					"使用 /questions 查看待回答问题，使用 /reply [ask_id] <answer> 回复，/skip [ask_id] 跳过。",
					"也可直接输入文本并回车（默认回复最新问题）。",
				}})
				return m.questionBroker.Request(ctx, req)
			},
		}

		runErr := runInv.WithContext(ctx).Run()
		status := runStatus(runErr)

		resultLines := make([]string, 0, 8)
		if out := strings.TrimSpace(stdout.String()); out != "" {
			resultLines = append(resultLines, strings.Split(out, "\n")...)
		}
		if out := strings.TrimSpace(stderr.String()); out != "" {
			resultLines = append(resultLines, strings.Split(out, "\n")...)
		}
		return appendResult(status, resultLines, runErr)
	}
}

func isCommandLikeInput(root *redant.Command, line string, agentOnly bool) bool {
	return isCommandLikeInputWithAlias(root, line, agentOnly, true)
}

func isCommandLikeInputWithAlias(root *redant.Command, line string, agentOnly, allowAlias bool) bool {
	cmd, ok := resolveCommandLikeInput(root, line, allowAlias)
	if !ok {
		return false
	}

	if !agentOnly {
		return true
	}

	return agentlinemodule.IsAgentCommand(cmd.Metadata)
}

func hasAnyAgentCommand(root *redant.Command) bool {
	if root == nil {
		return false
	}

	for _, child := range root.Children {
		if agentlinemodule.IsAgentCommand(child.Metadata) {
			return true
		}
		if hasAnyAgentCommand(child) {
			return true
		}
	}

	return false
}

func resolveCommandLikeInput(root *redant.Command, line string, allowAlias bool) (*redant.Command, bool) {
	args, err := splitCommandLine(line)
	if err != nil || len(args) == 0 || root == nil {
		return nil, false
	}

	if args[0] == root.Name() {
		args = args[1:]
		if len(args) == 0 {
			return nil, false
		}
	}

	current := root
	consumed := 0
	for _, token := range args {
		if strings.HasPrefix(token, "-") || strings.HasPrefix(token, "/") || strings.Contains(token, "=") {
			break
		}

		if strings.Contains(token, ":") {
			parts := strings.Split(token, ":")
			for _, part := range parts {
				next := childByToken(current, part, allowAlias)
				if next == nil {
					if consumed == 0 {
						return nil, false
					}
					return current, true
				}
				current = next
				consumed++
			}
			continue
		}

		next := childByToken(current, token, allowAlias)
		if next == nil {
			break
		}
		current = next
		consumed++
	}

	if consumed == 0 {
		return nil, false
	}

	return current, true
}

func childByToken(parent *redant.Command, token string, allowAlias bool) *redant.Command {
	if allowAlias {
		return childByNameOrAlias(parent, token)
	}
	return childByName(parent, token)
}

func childByName(parent *redant.Command, token string) *redant.Command {
	if parent == nil {
		return nil
	}
	for _, child := range parent.Children {
		if child.Hidden {
			continue
		}
		if child.Name() == token {
			return child
		}
	}
	return nil
}

func childByNameOrAlias(parent *redant.Command, token string) *redant.Command {
	if parent == nil {
		return nil
	}
	for _, child := range parent.Children {
		if child.Hidden {
			continue
		}
		if child.Name() == token {
			return child
		}
		for _, alias := range child.Aliases {
			if strings.TrimSpace(alias) == token {
				return child
			}
		}
	}
	return nil
}

func (m *agentlineModel) recomputeSuggestions() {
	line := m.input.Value()
	trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
	if strings.TrimSpace(line) == "" {
		m.suggestions = nil
		m.selected = 0
		return
	}

	if strings.HasPrefix(trimmed, "/") {
		m.suggestions = collectSlashCompletionItems(m.root, line, m.agentOnlyMode)
	} else {
		m.suggestions = nil
	}

	if len(m.suggestions) == 0 {
		m.selected = 0
		return
	}
	if m.selected >= len(m.suggestions) {
		m.selected = len(m.suggestions) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

func collectStarterSlashItems() []completionItem {
	return uniqueCompletionItems([]completionItem{
		{Insert: "/run ", Description: "执行命令"},
		{Insert: "/history", Description: "查看输入历史"},
		{Insert: "/help", Description: "查看 slash 帮助"},
		{Insert: "/output", Description: "进入输出滚动"},
	})
}

func collectSlashCompletionItems(root *redant.Command, input string, agentOnly bool) []completionItem {
	trimmedRight := strings.TrimRightFunc(input, unicode.IsSpace)
	if trimmedRight == "" {
		return nil
	}

	fields := strings.Fields(trimmedRight)
	if len(fields) == 0 {
		return nil
	}
	first := fields[0]
	if !strings.HasPrefix(first, "/") {
		return nil
	}

	firstName := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(first, "/")))
	if firstName == "chat" {
		chatPrefix := ""
		chatTokens := []string{}
		if len(fields) > 1 {
			chatTokens = append(chatTokens, fields[1:]...)
			chatPrefix = strings.TrimSpace(strings.Join(chatTokens, " "))
		}

		if len(fields) == 1 && len(trimmedRight) == len(input) {
			return collectSlashNameSuggestions(root, agentOnly, firstName)
		}

		if len(chatTokens) > 0 {
			cmd, ok := resolveCommandLikeInput(root, strings.Join(chatTokens, " "), false)
			if ok && cmd != nil {
				if len(chatTokens) == 1 && strings.EqualFold(strings.TrimSpace(chatTokens[0]), cmd.Name()) {
					return collectCommandInputItems(cmd, "")
				}
				if len(chatTokens) == 1 && len(trimmedRight) < len(input) {
					return collectCommandInputItems(cmd, "")
				}

				last := chatTokens[len(chatTokens)-1]
				if strings.HasPrefix(last, "-") {
					return collectCommandInputItems(cmd, last)
				}

				if len(trimmedRight) < len(input) {
					return collectCommandInputItems(cmd, "")
				}
			}
		}

		return collectChatCommandItems(root, agentOnly, chatPrefix)
	}

	if len(fields) > 1 || len(trimmedRight) < len(input) {
		probeTokens := append([]string{strings.TrimPrefix(first, "/")}, fields[1:]...)
		probeLine := strings.TrimSpace(strings.Join(probeTokens, " "))
		cmd, ok := resolveCommandLikeInput(root, probeLine, false)

		// 命令还未解析成功时，继续给出命令名补全。
		if !ok {
			return collectSlashNameSuggestions(root, agentOnly, strings.TrimPrefix(first, "/"))
		}

		// 场景：/commit <TAB>
		if len(fields) == 1 && len(trimmedRight) < len(input) {
			return collectCommandFlagItems(cmd, "")
		}

		// 场景：/commit --m<TAB>
		last := fields[len(fields)-1]
		if strings.HasPrefix(last, "-") {
			return collectCommandFlagItems(cmd, last)
		}

		// 场景：/commit --message hi <TAB>
		if len(trimmedRight) < len(input) {
			return collectCommandFlagItems(cmd, "")
		}

		return nil
	}

	return collectSlashNameSuggestions(root, agentOnly, strings.TrimPrefix(first, "/"))
}

func collectChatCommandItems(root *redant.Command, agentOnly bool, prefix string) []completionItem {
	prefix = strings.TrimSpace(prefix)
	cmdItems := collectCommandSlashItems(root, agentOnly, prefix)
	if len(cmdItems) == 0 {
		return nil
	}

	out := make([]completionItem, 0, len(cmdItems))
	for _, item := range cmdItems {
		insert := strings.TrimSpace(item.Insert)
		insert = strings.TrimPrefix(insert, "/")
		if insert == "" {
			continue
		}
		desc := item.Description
		if strings.TrimSpace(desc) == "" {
			desc = "绑定聊天命令"
		} else {
			desc = "绑定聊天命令 · " + desc
		}
		out = append(out, completionItem{Insert: "/chat " + insert, Description: desc})
	}

	return uniqueCompletionItems(out)
}

func collectSlashNameSuggestions(root *redant.Command, agentOnly bool, prefix string) []completionItem {
	prefix = strings.TrimSpace(prefix)
	out := make([]completionItem, 0, len(slashCommands)+8)
	matchedByPrefix := false
	addCandidate := func(name, desc string) {
		candidate := "/" + strings.TrimSpace(name)
		if candidate == "/" {
			return
		}
		if prefix == "" || strings.HasPrefix(strings.TrimPrefix(candidate, "/"), prefix) {
			matchedByPrefix = true
			out = append(out, completionItem{Insert: candidate, Description: desc})
		}
	}

	for _, sc := range slashCommands {
		addCandidate(sc.Name, sc.Description)
	}

	if prefix != "" && !matchedByPrefix {
		for _, guessed := range suggestClosestSlashNames(prefix, 3) {
			out = append(out, completionItem{Insert: "/" + guessed, Description: "你可能想输入这个命令"})
		}
	}

	out = append(out, collectCommandSlashItems(root, agentOnly, prefix)...)

	return uniqueCompletionItems(out)
}

func collectCommandFlagItems(cmd *redant.Command, prefix string) []completionItem {
	if cmd == nil {
		return nil
	}

	prefix = strings.TrimSpace(prefix)
	var out []completionItem
	for _, opt := range cmd.FullOptions() {
		if opt.Hidden || strings.TrimSpace(opt.Flag) == "" {
			continue
		}
		flagName := "--" + strings.TrimSpace(opt.Flag)
		if prefix != "" && !strings.HasPrefix(flagName, prefix) {
			continue
		}
		desc := strings.TrimSpace(opt.Description)
		if desc == "" {
			desc = "命令参数"
		}
		out = append(out, completionItem{Insert: flagName + " ", Description: desc})
	}

	return uniqueCompletionItems(out)
}

func collectCommandArgItems(cmd *redant.Command, prefix string) []completionItem {
	if cmd == nil || len(cmd.Args) == 0 {
		return nil
	}

	prefix = strings.TrimSpace(prefix)
	var out []completionItem
	for i, arg := range cmd.Args {
		name := strings.TrimSpace(arg.Name)
		if name == "" {
			name = fmt.Sprintf("arg%d", i+1)
		}
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}

		desc := strings.TrimSpace(arg.Description)
		if desc == "" {
			desc = "命令参数"
		}
		if arg.Required {
			desc = desc + " (required)"
		}
		if strings.TrimSpace(arg.Default) != "" {
			desc = desc + fmt.Sprintf(" (default=%s)", strings.TrimSpace(arg.Default))
		}

		out = append(out, completionItem{Insert: name + " ", Description: desc})
	}

	return uniqueCompletionItems(out)
}

func collectCommandInputItems(cmd *redant.Command, prefix string) []completionItem {
	prefix = strings.TrimSpace(prefix)
	if strings.HasPrefix(prefix, "-") {
		return collectCommandFlagItems(cmd, prefix)
	}

	out := make([]completionItem, 0, 8)
	out = append(out, collectCommandArgItems(cmd, prefix)...)
	out = append(out, collectCommandFlagItems(cmd, prefix)...)
	return uniqueCompletionItems(out)
}

func collectCommandSlashItems(root *redant.Command, agentOnly bool, prefix string) []completionItem {
	if root == nil {
		return nil
	}

	prefix = strings.TrimSpace(prefix)
	var out []completionItem

	var walk func(parent *redant.Command, path []string)
	walk = func(parent *redant.Command, path []string) {
		if parent == nil {
			return
		}

		for _, child := range parent.Children {
			if child == nil || child.Hidden {
				continue
			}
			if child.Name() == agentlinemodule.CommandName {
				continue
			}

			cmdPath := append(path, child.Name())

			if !agentOnly || agentlinemodule.IsAgentCommand(child.Metadata) {
				pathText := strings.Join(cmdPath, " ")
				if prefix == "" || strings.HasPrefix(pathText, prefix) {
					desc := strings.TrimSpace(child.Short)
					if desc == "" {
						desc = "执行命令"
					}
					out = append(out, completionItem{
						Insert:      "/" + pathText + " ",
						Description: desc,
					})
				}
			}

			walk(child, cmdPath)
		}
	}

	walk(root, nil)
	return out
}

func (m *agentlineModel) applySuggestion() {
	if len(m.suggestions) == 0 {
		m.recomputeSuggestions()
		if len(m.suggestions) == 0 {
			return
		}
	}

	idx := m.selected
	if idx < 0 || idx >= len(m.suggestions) {
		idx = 0
	}

	newLine := applySelectedCompletion(m.input.Value(), m.suggestions[idx].Insert)
	m.input.SetValue(newLine)
	m.input.CursorEnd()
}

func applySelectedCompletion(input, selected string) string {
	trimmedRight := strings.TrimRightFunc(input, unicode.IsSpace)
	if trimmedRight == "" {
		if strings.HasSuffix(selected, " ") {
			return selected
		}
		return selected + " "
	}
	if strings.HasPrefix(strings.TrimSpace(selected), "/") {
		if strings.HasSuffix(selected, " ") {
			return selected
		}
		return selected + " "
	}
	if len(trimmedRight) < len(input) {
		if strings.HasPrefix(selected, "/") {
			if strings.HasSuffix(selected, " ") {
				return selected
			}
			return selected + " "
		}
		if strings.HasSuffix(selected, " ") {
			return trimmedRight + " " + selected
		}
		return trimmedRight + " " + selected + " "
	}
	idx := strings.LastIndexFunc(trimmedRight, unicode.IsSpace)
	if idx < 0 {
		if strings.HasSuffix(selected, " ") {
			return selected
		}
		return selected + " "
	}
	if strings.HasSuffix(selected, " ") {
		return trimmedRight[:idx+1] + selected
	}
	return trimmedRight[:idx+1] + selected + " "
}

func (m *agentlineModel) appendBlocks(blocks []sessionBlock) {
	for _, block := range blocks {
		m.appendBlock(block)
	}
}

func (m *agentlineModel) appendBlock(block sessionBlock) {
	title := strings.TrimSpace(block.Title)
	if title == "" {
		title = string(block.Kind)
	}

	kind := block.Kind
	if kind == "" {
		kind = blockKindSystem
	}

	normalized := make([]string, 0, len(block.Lines))
	for _, line := range block.Lines {
		normalized = append(normalized, normalizeOutputLines(line)...)
	}

	m.blocks = append(m.blocks, sessionBlock{Kind: kind, Title: title, Lines: normalized})
	m.trimOutputHistory()
}

func (m *agentlineModel) trimOutputHistory() {
	if len(m.blocks) == 0 {
		return
	}

	total := 0
	for _, b := range m.blocks {
		total += len(b.Lines)
	}

	for len(m.blocks) > 1 && (len(m.blocks) > maxOutputBlocks || total > maxOutputLines) {
		total -= len(m.blocks[0].Lines)
		m.blocks = m.blocks[1:]
	}
}

func (m *agentlineModel) appendHistory(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	if len(m.history) > 0 && m.history[len(m.history)-1] == line {
		return
	}
	m.history = append(m.history, line)
	if m.persistHistory && m.historyFile != "" {
		_ = appendHistoryLine(m.historyFile, line)
	}
}

func (m *agentlineModel) historyUp() {
	if len(m.history) == 0 {
		return
	}
	if m.historyPos <= 0 {
		m.historyPos = 0
		m.selectedHistory = m.historyPos
		m.input.SetValue(m.history[m.historyPos])
		m.input.CursorEnd()
		return
	}
	m.historyPos--
	m.selectedHistory = m.historyPos
	m.input.SetValue(m.history[m.historyPos])
	m.input.CursorEnd()
}

func (m *agentlineModel) historyDown() {
	if len(m.history) == 0 {
		return
	}
	if m.historyPos >= len(m.history)-1 {
		m.historyPos = len(m.history)
		m.selectedHistory = -1
		m.input.SetValue("")
		m.input.CursorEnd()
		return
	}
	m.historyPos++
	m.selectedHistory = m.historyPos
	m.input.SetValue(m.history[m.historyPos])
	m.input.CursorEnd()
}

func (m *agentlineModel) historyPreviewLines(limit int) []string {
	if len(m.history) == 0 {
		return []string{"暂无输入历史。"}
	}

	if limit <= 0 {
		limit = 20
	}

	start := len(m.history) - limit
	if start < 0 {
		start = 0
	}

	lines := make([]string, 0, len(m.history)-start+1)
	lines = append(lines, fmt.Sprintf("total: %d, showing: %d-%d", len(m.history), start+1, len(m.history)))
	for i := start; i < len(m.history); i++ {
		lines = append(lines, fmt.Sprintf("%03d %s", i+1, strings.TrimSpace(m.history[i])))
	}
	return lines
}

func (m *agentlineModel) startCommandRun(commandLine string) tea.Cmd {
	runCtx := m.ctx
	if runCtx == nil {
		runCtx = context.Background()
	}
	runCtx, cancel := context.WithCancel(runCtx)
	m.running = true
	m.currentCancel = cancel
	m.outputFocus = false
	return runSlashRunCmd(runCtx, m, m.root, commandLine)
}

func (m *agentlineModel) cancelActiveRun(message string) bool {
	if !m.running || m.currentCancel == nil {
		return false
	}
	cancel := m.currentCancel
	m.currentCancel = nil
	cancel()
	if strings.TrimSpace(message) != "" {
		m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "cancel", Lines: []string{message}})
	}
	return true
}

func isAllowedWhileRunning(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") {
		return false
	}
	cmdText := strings.TrimSpace(strings.TrimPrefix(trimmed, "/"))
	if cmdText == "" {
		return false
	}
	parts := strings.Fields(cmdText)
	if len(parts) == 0 {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(parts[0])) {
	case "permissions", "perm", "allow", "deny", "cancel", "stop":
		return true
	case "questions", "reply", "skip":
		return true
	case "acp-events", "acp-events-summary", "acp-events-export":
		return true
	default:
		return false
	}
}

func (m *agentlineModel) startACPDemoTurn(prompt string) tea.Cmd {
	runCtx := m.ctx
	if runCtx == nil {
		runCtx = context.Background()
	}
	runCtx, cancel := context.WithCancel(runCtx)
	m.running = true
	m.currentCancel = cancel
	m.outputFocus = false
	return runACPDemoTurnCmd(runCtx, strings.TrimSpace(prompt), m.permissionBroker)
}

func runACPDemoTurnCmd(ctx context.Context, prompt string, broker *agentacp.PermissionBroker) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(prompt) == "" {
			prompt = "请执行一次需要权限确认的操作"
		}

		client := &agentacp.CallbackClient{PermissionBroker: broker}
		collector := make([]acp.SessionNotification, 0, 8)
		client.OnSessionUpdate = func(_ context.Context, params acp.SessionNotification) error {
			collector = append(collector, params)
			return nil
		}

		var bridge *agentacp.AgentBridge
		exec := agentacp.PromptExecutorFunc(func(ctx context.Context, sessionID acp.SessionId, _ []acp.ContentBlock, emit func(update acp.SessionUpdate) error) (acp.StopReason, error) {
			toolID := acp.ToolCallId("call_demo_1")
			title := "demo edit"
			if err := emit(acp.StartToolCall(toolID, title,
				acp.WithStartKind(acp.ToolKindEdit),
				acp.WithStartStatus(acp.ToolCallStatusPending),
			)); err != nil {
				return "", err
			}

			resp, err := bridge.RequestPermission(ctx, acp.RequestPermissionRequest{
				SessionId: sessionID,
				ToolCall: acp.RequestPermissionToolCall{
					ToolCallId: toolID,
					Title:      acp.Ptr(title),
					Kind:       acp.Ptr(acp.ToolKindEdit),
					Status:     acp.Ptr(acp.ToolCallStatusPending),
				},
				Options: []acp.PermissionOption{
					{OptionId: "allow-once", Name: "Allow once", Kind: acp.PermissionOptionKindAllowOnce},
					{OptionId: "reject-once", Name: "Reject once", Kind: acp.PermissionOptionKindRejectOnce},
				},
			})
			if err != nil {
				return "", err
			}

			if resp.Outcome.Selected == nil || strings.TrimSpace(string(resp.Outcome.Selected.OptionId)) == "" {
				if err := emit(acp.UpdateToolCall(toolID,
					acp.WithUpdateStatus(acp.ToolCallStatusFailed),
					acp.WithUpdateContent([]acp.ToolCallContent{acp.ToolContent(acp.TextBlock("permission denied"))}),
				)); err != nil {
					return "", err
				}
				return acp.StopReasonRefusal, nil
			}

			if err := emit(acp.UpdateToolCall(toolID, acp.WithUpdateStatus(acp.ToolCallStatusInProgress))); err != nil {
				return "", err
			}
			if err := emit(acp.UpdateToolCall(toolID,
				acp.WithUpdateStatus(acp.ToolCallStatusCompleted),
				acp.WithUpdateContent([]acp.ToolCallContent{acp.ToolContent(acp.TextBlock("demo change applied"))}),
			)); err != nil {
				return "", err
			}
			if err := emit(acp.UpdateAgentMessageText("ACP demo done")); err != nil {
				return "", err
			}
			return acp.StopReasonEndTurn, nil
		})

		bridge = agentacp.NewAgentBridge(agentacp.BridgeOptions{Executor: exec, PermissionRequester: client})
		bridge.SetSessionUpdater(client)

		newResp, err := bridge.NewSession(ctx, acp.NewSessionRequest{Cwd: "/tmp", McpServers: nil})
		if err != nil {
			return acpDemoResultMsg{err: err}
		}

		_, err = bridge.Prompt(ctx, acp.PromptRequest{SessionId: newResp.SessionId, Prompt: []acp.ContentBlock{acp.TextBlock(prompt)}})
		blocks := make([]sessionBlock, 0, len(collector)+1)
		for _, n := range collector {
			blocks = append(blocks, sessionBlocksFromACP(n)...)
		}
		if len(blocks) == 0 {
			blocks = append(blocks, sessionBlock{Kind: blockKindSystem, Title: "acp.demo", Lines: []string{"no updates received"}})
		}
		return acpDemoResultMsg{blocks: blocks, err: err}
	}
}
