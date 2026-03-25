package agentlineapp

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/pubgo/redant"
	agentlinemodule "github.com/pubgo/redant/pkg/agentline"
)

func TestCollectSlashCompletionItems(t *testing.T) {
	root := buildTestRoot()
	items := collectSlashCompletionItems(root, "/", false)
	if len(items) == 0 {
		t.Fatalf("expected slash suggestions for '/'")
	}
	if _, ok := findCompletion(items, "/run"); !ok {
		t.Fatalf("expected /run in slash suggestions")
	}
	if _, ok := findCompletion(items, "/history"); !ok {
		t.Fatalf("expected /history in slash suggestions")
	}
	if _, ok := findCompletion(items, "/commit"); !ok {
		t.Fatalf("expected /commit in slash suggestions")
	}
	if _, ok := findCompletion(items, "/a"); ok {
		t.Fatalf("expected /a alias hidden from slash suggestions")
	}
	if _, ok := findCompletion(items, "/q"); ok {
		t.Fatalf("expected /q alias hidden from slash suggestions")
	}
}

func TestCollectSlashCompletionItems_AgentOnly(t *testing.T) {
	root := buildTestRoot()

	items := collectSlashCompletionItems(root, "/", true)
	if _, ok := findCompletion(items, "/commit"); !ok {
		t.Fatalf("expected /commit in agent-only slash suggestions")
	}
	if _, ok := findCompletion(items, "/wait"); ok {
		t.Fatalf("expected /wait excluded in agent-only slash suggestions")
	}
}

func TestCollectSlashCompletionItems_StrictAgentOnlyExcludesUnmarked(t *testing.T) {
	root := buildTestRoot()
	root.Children[0].Metadata = nil // commit no longer marked as agent

	items := collectSlashCompletionItems(root, "/", true)
	if _, ok := findCompletion(items, "/commit"); ok {
		t.Fatalf("expected /commit excluded when command is not explicitly marked as agent")
	}
}

func TestCollectSlashCompletionItems_ExcludeCommandAliases(t *testing.T) {
	root := buildTestRoot()
	root.Children[0].Aliases = []string{"ci"} // commit alias

	items := collectSlashCompletionItems(root, "/", false)
	if _, ok := findCompletion(items, "/commit"); !ok {
		t.Fatalf("expected /commit in slash suggestions")
	}
	if _, ok := findCompletion(items, "/ci"); ok {
		t.Fatalf("expected /ci alias not shown in slash suggestions")
	}
}

func TestCollectSlashCompletionItems_CommandFlags(t *testing.T) {
	root := buildTestRoot()

	items := collectSlashCompletionItems(root, "/commit ", false)
	if _, ok := findCompletion(items, "--message"); !ok {
		t.Fatalf("expected --message in slash flag suggestions")
	}

	items = collectSlashCompletionItems(root, "/commit --m", false)
	if _, ok := findCompletion(items, "--message"); !ok {
		t.Fatalf("expected --message in slash flag prefix suggestions")
	}
}

func TestCollectSlashCompletionItems_ChatSuggestsCommands(t *testing.T) {
	root := buildTestRoot()

	items := collectSlashCompletionItems(root, "/chat ", false)
	if _, ok := findCompletion(items, "/chat commit"); !ok {
		t.Fatalf("expected '/chat commit' suggestion when typing '/chat '")
	}

	items = collectSlashCompletionItems(root, "/chat com", false)
	if _, ok := findCompletion(items, "/chat commit"); !ok {
		t.Fatalf("expected '/chat commit' suggestion when typing '/chat com'")
	}
}

func TestCollectSlashCompletionItems_ChatCommandFlags(t *testing.T) {
	root := buildTestRoot()

	items := collectSlashCompletionItems(root, "/chat commit ", false)
	if _, ok := findCompletion(items, "--message"); !ok {
		t.Fatalf("expected --message in '/chat commit ' flag suggestions")
	}

	items = collectSlashCompletionItems(root, "/chat commit --m", false)
	if _, ok := findCompletion(items, "--message"); !ok {
		t.Fatalf("expected --message in '/chat commit --m' flag suggestions")
	}
}

func TestCollectSlashCompletionItems_ChatCommandShowsFlagsAndArgsWithoutTrailingSpace(t *testing.T) {
	var (
		sessionID string
		prompt    string
	)
	root := &redant.Command{
		Use: "app",
		Children: []*redant.Command{
			{
				Use:      "resume",
				Short:    "恢复会话",
				Metadata: agentlinemodule.AgentCommandMetadata(),
				Options: redant.OptionSet{
					{Flag: "session-id", Value: redant.StringOf(&sessionID), Description: "会话 ID"},
					{Flag: "prompt", Value: redant.StringOf(&prompt), Description: "提示词"},
				},
				Args: redant.ArgSet{
					{Name: "topic", Description: "对话主题", Required: true},
				},
			},
		},
	}

	items := collectSlashCompletionItems(root, "/chat resume", true)
	if _, ok := findCompletion(items, "--session-id"); !ok {
		t.Fatalf("expected --session-id in '/chat resume' suggestions")
	}
	if _, ok := findCompletion(items, "topic"); !ok {
		t.Fatalf("expected positional arg 'topic' in '/chat resume' suggestions")
	}
}

func TestApplySelectedCompletion_ChatDoesNotDuplicatePrefix(t *testing.T) {
	got := applySelectedCompletion("/chat ", "/chat commit ")
	if got != "/chat commit " {
		t.Fatalf("expected '/chat commit ', got %q", got)
	}
}

func TestApplySelectedCompletion_ChatPrefixTokenDoesNotDuplicatePrefix(t *testing.T) {
	got := applySelectedCompletion("/chat r", "/chat resume ")
	if got != "/chat resume " {
		t.Fatalf("expected '/chat resume ', got %q", got)
	}
}

func TestCollectSlashCompletionItems_TypoSuggestsChat(t *testing.T) {
	root := buildTestRoot()
	items := collectSlashCompletionItems(root, "/caht", false)
	if _, ok := findCompletion(items, "/chat"); !ok {
		t.Fatalf("expected typo '/caht' to suggest '/chat'")
	}
}

func TestHandleSlashInput_ModeSwitch(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	handled, cmd := m.handleSlashInput("/output")
	if !handled || cmd != nil {
		t.Fatalf("expected /output handled without cmd, handled=%v cmd=%v", handled, cmd)
	}
	if !m.outputFocus {
		t.Fatalf("expected outputFocus=true after /output")
	}

	handled, cmd = m.handleSlashInput("/input")
	if !handled || cmd != nil {
		t.Fatalf("expected /input handled without cmd, handled=%v cmd=%v", handled, cmd)
	}
	if m.outputFocus {
		t.Fatalf("expected outputFocus=false after /input")
	}
}

func TestView_MouseWheelDispatchByRegion(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)
	m.width = 100
	m.height = 24
	m.history = []string{"/ask one", "/ask two", "/run commit --message hi", "/plan test"}
	m.historyPos = len(m.history)
	m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "output", Lines: []string{"line-1", "line-2", "line-3", "line-4", "line-5"}})

	v := m.View()
	if v.OnMouse == nil {
		t.Fatalf("expected mouse handler configured")
	}

	lines := strings.Split(v.Content, "\n")
	outputY := findLineContaining(lines, "输出区域")
	inputY := findLineContaining(lines, "输入区域")
	if outputY < 0 || inputY < 0 {
		t.Fatalf("expected output/input region markers in view")
	}

	cmd := v.OnMouse(tea.MouseWheelMsg{X: 0, Y: outputY, Button: tea.MouseWheelUp})
	if cmd == nil {
		t.Fatalf("expected output region wheel event produce cmd")
	}
	msg := cmd()
	scroll, ok := msg.(mouseScrollMsg)
	if !ok {
		t.Fatalf("expected mouseScrollMsg, got %T", msg)
	}
	if scroll.Region != mouseRegionOutput || scroll.Delta != 1 {
		t.Fatalf("expected output region delta=1, got region=%s delta=%d", scroll.Region, scroll.Delta)
	}

	cmd = v.OnMouse(tea.MouseWheelMsg{X: 0, Y: inputY, Button: tea.MouseWheelDown})
	if cmd == nil {
		t.Fatalf("expected input region wheel event produce cmd")
	}
	msg = cmd()
	scroll, ok = msg.(mouseScrollMsg)
	if !ok {
		t.Fatalf("expected mouseScrollMsg, got %T", msg)
	}
	if scroll.Region != mouseRegionInput || scroll.Delta != -1 {
		t.Fatalf("expected input region delta=-1, got region=%s delta=%d", scroll.Region, scroll.Delta)
	}

	// Shift+滚轮：应旁路给终端原生行为（通常用于选择/复制场景）。
	cmd = v.OnMouse(tea.MouseWheelMsg{X: 0, Y: outputY, Button: tea.MouseWheelUp, Mod: tea.ModShift})
	if cmd != nil {
		t.Fatalf("expected shift+wheel to be bypassed")
	}
}

func TestUpdate_MouseScrollMsgScrollsInputAndOutput(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)
	m.width = 100
	m.height = 14
	m.history = []string{"h1", "h2", "h3", "h4", "h5", "h6", "h7", "h8", "h9", "h10"}
	m.historyPos = len(m.history)
	m.blocks = []sessionBlock{{Kind: blockKindSystem, Title: "system", Lines: []string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen"}}}

	model, _ := m.Update(mouseScrollMsg{Region: mouseRegionInput, Delta: 1})
	m = model.(*agentlineModel)
	if m.inputOffset <= 0 {
		t.Fatalf("expected inputOffset > 0 after input wheel up, got %d", m.inputOffset)
	}
	if m.outputFocus {
		t.Fatalf("expected outputFocus=false when scrolling input region")
	}

	model, _ = m.Update(mouseScrollMsg{Region: mouseRegionOutput, Delta: 1})
	m = model.(*agentlineModel)
	if m.outputOffset <= 0 {
		t.Fatalf("expected outputOffset > 0 after output wheel up, got %d", m.outputOffset)
	}
	if !m.outputFocus {
		t.Fatalf("expected outputFocus=true when scrolling output region")
	}
}

func TestView_MouseClickInputRegionFocusInput(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)
	m.width = 100
	m.height = 24
	m.history = []string{"/ask one", "/ask two", "/run commit --message hi", "/plan test"}
	m.historyPos = len(m.history)

	v := m.View()
	if v.OnMouse == nil {
		t.Fatalf("expected mouse handler configured")
	}

	lines := strings.Split(v.Content, "\n")
	clickY := findLineContaining(lines, "输入区域")
	if clickY < 0 {
		t.Fatalf("expected input region rendered in view")
	}

	cmd := v.OnMouse(tea.MouseClickMsg{X: 0, Y: clickY, Button: tea.MouseLeft})
	if cmd == nil {
		t.Fatalf("expected click in input region to emit focus message")
	}
	msg := cmd()
	focusMsg, ok := msg.(mouseFocusMsg)
	if !ok {
		t.Fatalf("expected mouseFocusMsg, got %T", msg)
	}
	if focusMsg.Region != mouseRegionInput {
		t.Fatalf("expected focus region input, got %q", focusMsg.Region)
	}

	// Shift+点击：应旁路给终端原生选择行为。
	cmd = v.OnMouse(tea.MouseClickMsg{X: 0, Y: clickY, Button: tea.MouseLeft, Mod: tea.ModShift})
	if cmd != nil {
		t.Fatalf("expected shift+click to be bypassed")
	}
}

func TestView_MouseClickHistoryRowSelectsHistory(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)
	m.width = 100
	m.height = 24
	m.history = []string{"/ask one", "/ask two", "/run commit --message hi", "/plan test"}
	m.historyPos = len(m.history)

	v := m.View()
	if v.OnMouse == nil {
		t.Fatalf("expected mouse handler configured")
	}

	lines := strings.Split(v.Content, "\n")
	clickY := findLineContaining(lines, "002 /ask two")
	if clickY < 0 {
		t.Fatalf("expected rendered history row for '/ask two'")
	}

	cmd := v.OnMouse(tea.MouseClickMsg{X: 0, Y: clickY, Button: tea.MouseLeft})
	if cmd == nil {
		t.Fatalf("expected click on history row to emit selection message")
	}
	msg := cmd()
	selMsg, ok := msg.(mouseSelectHistoryMsg)
	if !ok {
		t.Fatalf("expected mouseSelectHistoryMsg, got %T", msg)
	}
	if selMsg.HistoryIndex != 1 {
		t.Fatalf("expected history index 1, got %d", selMsg.HistoryIndex)
	}
}

func TestUpdate_MouseSelectHistoryMsgFillsInput(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)
	m.history = []string{"h1", "h2", "h3"}
	m.historyPos = len(m.history)
	m.outputFocus = true

	model, _ := m.Update(mouseSelectHistoryMsg{HistoryIndex: 1})
	m = model.(*agentlineModel)
	if got := m.input.Value(); got != "h2" {
		t.Fatalf("expected input filled with h2, got %q", got)
	}
	if m.historyPos != 1 {
		t.Fatalf("expected historyPos=1, got %d", m.historyPos)
	}
	if m.selectedHistory != 1 {
		t.Fatalf("expected selectedHistory=1, got %d", m.selectedHistory)
	}
	if m.outputFocus {
		t.Fatalf("expected outputFocus=false after selecting input history")
	}
}

func TestHistoryUpDownTracksSelectedHistory(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)
	m.history = []string{"h1", "h2", "h3"}
	m.historyPos = len(m.history)

	m.historyUp()
	if m.selectedHistory != 2 {
		t.Fatalf("expected selectedHistory=2 after first up, got %d", m.selectedHistory)
	}

	m.historyUp()
	if m.selectedHistory != 1 {
		t.Fatalf("expected selectedHistory=1 after second up, got %d", m.selectedHistory)
	}

	m.historyDown()
	if m.selectedHistory != 2 {
		t.Fatalf("expected selectedHistory=2 after down, got %d", m.selectedHistory)
	}

	m.historyDown()
	if m.selectedHistory != -1 {
		t.Fatalf("expected selectedHistory=-1 when leaving history mode, got %d", m.selectedHistory)
	}
	if got := m.input.Value(); got != "" {
		t.Fatalf("expected empty input after leaving history mode, got %q", got)
	}
}

func TestRunSlashRunCmd(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)
	msg := runSlashRunCmd(context.Background(), m, root, "commit --message hello")()
	res, ok := msg.(runResultMsg)
	if !ok {
		t.Fatalf("expected runResultMsg, got %T", msg)
	}
	if len(res.blocks) < 3 {
		t.Fatalf("expected at least 3 blocks, got %d", len(res.blocks))
	}
	if res.blocks[0].Kind != blockKindTool {
		t.Fatalf("expected first block kind=tool, got %s", res.blocks[0].Kind)
	}
	if res.blocks[1].Kind != blockKindTool {
		t.Fatalf("expected second block kind=tool(parse), got %s", res.blocks[1].Kind)
	}
	if res.blocks[2].Kind != blockKindCommand {
		t.Fatalf("expected third block kind=command, got %s", res.blocks[2].Kind)
	}

	result, ok := findBlockByKind(res.blocks, blockKindResult)
	if !ok {
		t.Fatalf("expected result block in run result")
	}

	joined := strings.Join(result.Lines, "\n")
	if !strings.Contains(joined, "status: ok") {
		t.Fatalf("expected status line in result block, got: %s", joined)
	}
	if !strings.Contains(joined, "duration:") {
		t.Fatalf("expected duration line in result block, got: %s", joined)
	}
	if !strings.Contains(joined, "commit ok") {
		t.Fatalf("expected command output in result block, got: %s", joined)
	}
}

func TestRunSlashRunCmd_Canceled(t *testing.T) {
	root := buildTestRoot()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	msg := runSlashRunCmd(ctx, m, root, "wait")()
	res, ok := msg.(runResultMsg)
	if !ok {
		t.Fatalf("expected runResultMsg, got %T", msg)
	}

	result, ok := findBlockByKind(res.blocks, blockKindResult)
	if !ok {
		t.Fatalf("expected result block")
	}
	joined := strings.Join(result.Lines, "\n")
	if !strings.Contains(joined, "status: canceled") {
		t.Fatalf("expected canceled status, got: %s", joined)
	}
}

func TestHandleSlashInput_CancelRunning(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	called := false
	m.running = true
	m.currentCancel = func() { called = true }

	handled, cmd := m.handleSlashInput("/cancel")
	if !handled || cmd != nil {
		t.Fatalf("expected /cancel handled without cmd, handled=%v cmd=%v", handled, cmd)
	}
	if !called {
		t.Fatalf("expected cancel function called")
	}
}

func TestHandleSlashInput_FoldUnfold(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	handled, cmd := m.handleSlashInput("/fold")
	if !handled || cmd != nil {
		t.Fatalf("expected /fold handled without cmd")
	}
	if !m.foldDetails {
		t.Fatalf("expected foldDetails=true after /fold")
	}

	handled, cmd = m.handleSlashInput("/unfold")
	if !handled || cmd != nil {
		t.Fatalf("expected /unfold handled without cmd")
	}
	if m.foldDetails {
		t.Fatalf("expected foldDetails=false after /unfold")
	}
}

func TestHandleSlashInput_CommandAsSlash(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	handled, cmd := m.handleSlashInput("/commit --message hi")
	if !handled {
		t.Fatalf("expected slash command handled")
	}
	if cmd == nil {
		t.Fatalf("expected slash command to return run cmd")
	}
	if !m.running {
		t.Fatalf("expected running=true after slash command run")
	}
}

func TestHandleSlashInput_CommandAliasNotUsedAsSlash(t *testing.T) {
	root := buildTestRoot()
	root.Children[0].Aliases = []string{"ci"} // commit alias
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	handled, cmd := m.handleSlashInput("/ci --message hi")
	if !handled {
		t.Fatalf("expected slash input handled as slash flow")
	}
	if cmd != nil {
		t.Fatalf("expected alias not treated as runnable slash command")
	}
	if m.running {
		t.Fatalf("expected running=false when alias is not accepted in slash")
	}
}

func TestHandleSlashInput_HistoryDefault(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)
	m.history = []string{"/ask one", "/run commit --message hi", "/plan release"}
	m.outputOffset = 7

	handled, cmd := m.handleSlashInput("/history")
	if !handled || cmd != nil {
		t.Fatalf("expected /history handled without cmd, handled=%v cmd=%v", handled, cmd)
	}

	last := m.blocks[len(m.blocks)-1]
	if last.Title != "/history" {
		t.Fatalf("expected last block title /history, got %q", last.Title)
	}
	joined := strings.Join(last.Lines, "\n")
	if !strings.Contains(joined, "total: 3") {
		t.Fatalf("expected total line in /history output, got: %s", joined)
	}
	if !strings.Contains(joined, "003 /plan release") {
		t.Fatalf("expected numbered history line, got: %s", joined)
	}
	if m.outputOffset != 0 {
		t.Fatalf("expected outputOffset reset to 0 after /history, got %d", m.outputOffset)
	}
}

func TestHandleSlashInput_HistoryWithLimit(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)
	m.history = []string{"h1", "h2", "h3", "h4", "h5"}

	handled, cmd := m.handleSlashInput("/history 2")
	if !handled || cmd != nil {
		t.Fatalf("expected /history 2 handled without cmd, handled=%v cmd=%v", handled, cmd)
	}

	last := m.blocks[len(m.blocks)-1]
	if len(last.Lines) != 3 {
		t.Fatalf("expected 3 lines(total+2 entries), got %d", len(last.Lines))
	}
	joined := strings.Join(last.Lines, "\n")
	if strings.Contains(joined, "003 h3") {
		t.Fatalf("did not expect older history entry in limited output, got: %s", joined)
	}
	if !strings.Contains(joined, "004 h4") || !strings.Contains(joined, "005 h5") {
		t.Fatalf("expected latest 2 entries, got: %s", joined)
	}
}

func TestHandleSlashInput_HistoryInvalidArg(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	handled, cmd := m.handleSlashInput("/history abc")
	if !handled || cmd != nil {
		t.Fatalf("expected invalid /history handled without cmd, handled=%v cmd=%v", handled, cmd)
	}

	last := m.blocks[len(m.blocks)-1]
	if last.Kind != blockKindError {
		t.Fatalf("expected error block for invalid /history arg, got %s", last.Kind)
	}
	if !strings.Contains(strings.Join(last.Lines, "\n"), "用法：/history") {
		t.Fatalf("expected usage hint for invalid /history")
	}
}

func TestHandleSlashInput_UnknownCommandSuggestsClosest(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	handled, cmd := m.handleSlashInput("/caht")
	if !handled || cmd != nil {
		t.Fatalf("expected unknown slash handled without cmd, handled=%v cmd=%v", handled, cmd)
	}

	last := m.blocks[len(m.blocks)-1]
	if last.Kind != blockKindError {
		t.Fatalf("expected error block, got %s", last.Kind)
	}
	joined := strings.Join(last.Lines, "\n")
	if !strings.Contains(joined, "你可能想输入") || !strings.Contains(joined, "/chat") {
		t.Fatalf("expected closest command suggestion '/chat', got: %s", joined)
	}
}

func TestRenderOutputLines_FoldDetails(t *testing.T) {
	m := &agentlineModel{
		foldDetails: true,
		blocks: []sessionBlock{
			{Kind: blockKindAssistant, Title: "assistant", Lines: []string{"line1", "line2", "line3"}},
		},
	}

	lines := m.renderOutputLines(80)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "folded") {
		t.Fatalf("expected folded hint in output, got: %s", joined)
	}
}

func TestTabOnEmptyInputShowsStarterSlashSuggestions(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	if len(m.suggestions) != 0 {
		t.Fatalf("expected no suggestions on init empty input")
	}

	model, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	m = model.(*agentlineModel)
	if len(m.suggestions) == 0 {
		t.Fatalf("expected starter suggestions on first TAB")
	}
	if _, ok := findCompletion(m.suggestions, "/run"); !ok {
		t.Fatalf("expected /run suggestion")
	}
	if got := m.input.Value(); got != "" {
		t.Fatalf("expected first TAB not applying suggestion, got input=%q", got)
	}
}

func TestEnterPlainTextShowsSimplifiedHint(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)
	m.input.SetValue("请帮我总结今天改动")

	model, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = model.(*agentlineModel)
	if cmd != nil {
		t.Fatalf("expected no async cmd for plain text in simplified mode")
	}
	if m.running {
		t.Fatalf("expected running=false for plain text in simplified mode")
	}
	last := m.blocks[len(m.blocks)-1]
	if last.Title != "input" {
		t.Fatalf("expected input hint block, got %q", last.Title)
	}
	if !strings.Contains(strings.Join(last.Lines, "\n"), "精简命令模式") {
		t.Fatalf("expected simplified mode hint in input block")
	}
}

func TestEnterPlainTextInChatStickyModeRunsCommand(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)
	m.input.SetValue("/chat commit")

	model, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = model.(*agentlineModel)
	if cmd != nil {
		t.Fatalf("expected no async cmd while entering chat sticky mode")
	}
	if m.stickyInvocation == nil {
		t.Fatalf("expected sticky invocation to be configured")
	}

	m.input.SetValue("接着上次的话题")

	model, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = model.(*agentlineModel)
	if cmd == nil {
		t.Fatalf("expected async cmd for plain text in chat sticky mode")
	}
	if !m.running {
		t.Fatalf("expected running=true in chat sticky mode")
	}
	last := m.blocks[len(m.blocks)-1]
	if last.Kind != blockKindUser || last.Title != "user" {
		t.Fatalf("expected user echo block before command run, got kind=%s title=%q", last.Kind, last.Title)
	}
	if !strings.Contains(strings.Join(last.Lines, "\n"), "接着上次的话题") {
		t.Fatalf("expected user echo contains latest input")
	}
}

func TestEnterCommandLikeInputInChatModeRunsStickyCommand(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	m.input.SetValue("/chat commit")
	model, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = model.(*agentlineModel)
	if cmd != nil {
		t.Fatalf("expected no async cmd while entering chat sticky mode")
	}

	m.input.SetValue("commit --message hi")
	model, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = model.(*agentlineModel)
	if cmd == nil {
		t.Fatalf("expected async cmd in chat sticky mode even for command-like text")
	}
	if !m.running {
		t.Fatalf("expected running=true in chat sticky mode")
	}
}

func TestView_ShowsChatModeHintLine(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)
	m.width = 100
	m.height = 24
	m.bindStickyInvocation(&stickyInvocation{BaseArgs: []string{"commit"}, PromptFlag: "--prompt"})

	v := m.View()
	if !strings.Contains(v.Content, "当前为聊天模式") {
		t.Fatalf("expected view to contain chat mode hint line")
	}
	if !strings.Contains(v.Content, "chat=commit --prompt <text>") {
		t.Fatalf("expected view to contain chat binding line")
	}
}

func TestView_ShowsPendingQuestionHintWhileRunning(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)
	m.width = 120
	m.height = 28
	m.running = true

	respCh := make(chan AskResponse, 1)
	go func() {
		resp, _ := m.questionBroker.Request(context.Background(), AskRequest{Prompt: "请确认是否继续执行"})
		respCh <- resp
	}()

	for i := 0; i < 50; i++ {
		if len(m.questionBroker.Pending()) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	v := m.View()
	if !strings.Contains(v.Content, "WAITING_ANSWER") {
		t.Fatalf("expected running view status to be WAITING_ANSWER")
	}
	if !strings.Contains(v.Content, "等待问题回复") {
		t.Fatalf("expected running view contains waiting-question hint")
	}
	if !strings.Contains(v.Content, "最新问题:") {
		t.Fatalf("expected running view contains latest question line")
	}

	if err := m.questionBroker.Cancel(""); err != nil {
		t.Fatalf("cancel pending question failed: %v", err)
	}
	select {
	case <-respCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting question cancel")
	}
}

func TestHandleSlashInput_ChatRequiresCommand(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	handled, cmd := m.handleSlashInput("/chat")
	if !handled || cmd != nil {
		t.Fatalf("expected /chat handled without async cmd, handled=%v cmd=%v", handled, cmd)
	}
	last := m.blocks[len(m.blocks)-1]
	if last.Kind != blockKindError {
		t.Fatalf("expected error block for empty /chat, got %s", last.Kind)
	}
	if !strings.Contains(strings.Join(last.Lines, "\n"), "需要指定命令") {
		t.Fatalf("expected usage hint for empty /chat")
	}
}

func TestHandleSlashInput_ChatAndUnbind(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	handled, cmd := m.handleSlashInput("/chat commit")
	if !handled || cmd != nil {
		t.Fatalf("expected /chat handled without async cmd, handled=%v cmd=%v", handled, cmd)
	}
	if m.stickyInvocation == nil {
		t.Fatalf("expected sticky invocation after /chat")
	}

	handled, cmd = m.handleSlashInput("/unbind")
	if !handled || cmd != nil {
		t.Fatalf("expected /unbind handled without async cmd, handled=%v cmd=%v", handled, cmd)
	}
	if m.stickyInvocation != nil {
		t.Fatalf("expected sticky invocation cleared after /unbind")
	}
	if m.mode != interactionModeCommand {
		t.Fatalf("expected command mode after /unbind, got %q", m.mode)
	}
}

func TestHandleSlashInput_ChatSetsMode(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)

	handled, cmd := m.handleSlashInput("/chat commit")
	if !handled || cmd != nil {
		t.Fatalf("expected /chat handled without async cmd, handled=%v cmd=%v", handled, cmd)
	}
	if m.mode != interactionModeChat {
		t.Fatalf("expected chat mode after /chat, got %q", m.mode)
	}
	if !m.isChatMode() {
		t.Fatalf("expected isChatMode=true after /chat")
	}
}

func TestSuggestionNavigationTakesPriorityOverOutputFocus(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, nil)
	m.outputFocus = true
	m.input.SetValue("/")
	m.recomputeSuggestions()

	if len(m.suggestions) < 2 {
		t.Fatalf("expected at least 2 slash suggestions, got=%d", len(m.suggestions))
	}

	model, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m = model.(*agentlineModel)
	if m.selected != 1 {
		t.Fatalf("expected selected=1 after down, got=%d", m.selected)
	}
	if m.outputOffset != 0 {
		t.Fatalf("expected outputOffset unchanged when navigating suggestions, got=%d", m.outputOffset)
	}

	model, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	m = model.(*agentlineModel)
	if m.selected != 0 {
		t.Fatalf("expected selected=0 after up, got=%d", m.selected)
	}
}

func TestIsCommandLikeInput(t *testing.T) {
	root := buildTestRoot()
	if !isCommandLikeInput(root, "commit --message hi", false) {
		t.Fatalf("expected commit line recognized as command input")
	}
	if isCommandLikeInput(root, "请帮我总结一下今天改动", false) {
		t.Fatalf("expected natural language not recognized as command input")
	}
}

func TestIsCommandLikeInput_AgentOnlyMode(t *testing.T) {
	root := buildTestRoot()
	root.Children[0].Metadata = agentlinemodule.AgentCommandMetadata() // commit

	if !isCommandLikeInput(root, "commit --message hi", true) {
		t.Fatalf("expected commit recognized in agent-only mode")
	}
	if isCommandLikeInput(root, "wait", true) {
		t.Fatalf("expected non-agent command rejected in agent-only mode")
	}
}

func TestNewAgentlineModel_InitWithInitialArgv(t *testing.T) {
	root := buildTestRoot()
	m := newAgentlineModel(context.Background(), root, "agent> ", nil, "", false, []string{"commit", "--message", "hello world"})

	cmd := m.Init()
	if cmd == nil {
		t.Fatalf("expected non-nil init cmd for initial argv")
	}
	if !m.running {
		t.Fatalf("expected running=true after init bootstrap")
	}
}

func TestBuildStickyInvocation(t *testing.T) {
	root := buildTestRoot()

	t.Run("empty command", func(t *testing.T) {
		_, err := buildStickyInvocation(root, "", true)
		if err == nil {
			t.Fatalf("expected error for empty /chat command")
		}
	})

	t.Run("agent command", func(t *testing.T) {
		sticky, err := buildStickyInvocation(root, "commit --message old", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sticky == nil {
			t.Fatalf("expected sticky invocation")
		} else {
			if sticky.PromptFlag != "--prompt" {
				t.Fatalf("expected default prompt flag, got %q", sticky.PromptFlag)
			}
			joined := strings.Join(sticky.BaseArgs, " ")
			if !strings.Contains(joined, "commit") {
				t.Fatalf("expected command args keep command path, got %q", joined)
			}
		}
	})

	t.Run("reject non-agent command in agent-only mode", func(t *testing.T) {
		_, err := buildStickyInvocation(root, "wait", true)
		if err == nil {
			t.Fatalf("expected non-agent command rejected")
		}
	})
}

func TestSessionContextLine(t *testing.T) {
	m := &agentlineModel{sessionCWD: "/tmp/work", sessionGitBranch: "feat/copilot", sessionGitDirty: true}
	got := m.sessionContextLine()
	if !strings.Contains(got, "cwd=/tmp/work") {
		t.Fatalf("expected cwd in session context line, got %q", got)
	}
	if !strings.Contains(got, "git=feat/copilot*") {
		t.Fatalf("expected git branch in session context line, got %q", got)
	}
}

func TestDisplayGitBranch_NotRepo(t *testing.T) {
	if got := displayGitBranch("", false); got != "(not repo)" {
		t.Fatalf("expected (not repo), got %q", got)
	}
}

func TestDisplayGitBranch_DirtySuffix(t *testing.T) {
	if got := displayGitBranch("feat/copilot", true); got != "feat/copilot*" {
		t.Fatalf("expected dirty branch suffix, got %q", got)
	}
}

func buildTestRoot() *redant.Command {
	var msg string
	commit := &redant.Command{
		Use:      "commit",
		Short:    "提交代码",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Options: redant.OptionSet{
			{Flag: "message", Shorthand: "m", Description: "提交信息", Value: redant.StringOf(&msg)},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			_, err := inv.Stdout.Write([]byte("commit ok\n"))
			return err
		},
	}

	wait := &redant.Command{
		Use:   "wait",
		Short: "等待上下文取消",
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}

	return &redant.Command{
		Use:      "app",
		Children: []*redant.Command{commit, wait},
		Handler:  func(ctx context.Context, inv *redant.Invocation) error { return nil },
	}
}

func findBlockByKind(blocks []sessionBlock, kind blockKind) (sessionBlock, bool) {
	for _, b := range blocks {
		if b.Kind == kind {
			return b, true
		}
	}
	return sessionBlock{}, false
}

func findCompletion(items []completionItem, insert string) (completionItem, bool) {
	for _, item := range items {
		if item.Insert == insert {
			return item, true
		}
	}
	return completionItem{}, false
}

func findLineContaining(lines []string, needle string) int {
	for i, line := range lines {
		if strings.Contains(line, needle) {
			return i
		}
	}
	return -1
}
