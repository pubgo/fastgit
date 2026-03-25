package agentlineapp

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	acp "github.com/coder/acp-go-sdk"

	"github.com/pubgo/redant"
	agentacp "github.com/pubgo/redant/cmds/agentlineapp/acp"
)

type slashCommand struct {
	Name        string
	Aliases     []string
	Description string
}

type slashHandler func(m *agentlineModel, raw, cmdText, argText string) tea.Cmd

type slashBuiltin struct {
	Name        string
	Aliases     []string
	Description string
	Handler     slashHandler
}

var slashBuiltins = []slashBuiltin{
	{
		Name:        "chat",
		Description: "绑定聊天粘性模式（/chat <command ...>）",
		Handler: func(m *agentlineModel, _, _, argText string) tea.Cmd {
			sticky, err := buildStickyInvocation(m.root, argText, m.agentOnlyMode)
			if err != nil {
				m.appendBlock(sessionBlock{Kind: blockKindError, Title: "/chat", Lines: []string{err.Error()}})
				return nil
			}
			m.bindStickyInvocation(sticky)
			m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/chat", Lines: []string{
				"已进入聊天粘性模式。",
				fmt.Sprintf("后续普通输入将自动补全为: %s <文本>", strings.Join(append(append([]string(nil), sticky.BaseArgs...), sticky.PromptFlag), " ")),
				"输入 /unbind 退出聊天粘性模式。",
			}})
			return nil
		},
	},
	{
		Name:        "run",
		Aliases:     []string{"r"},
		Description: "执行 redant 命令（tool -> command -> result）",
		Handler: func(m *agentlineModel, _, _, argText string) tea.Cmd {
			if argText == "" {
				m.appendBlock(sessionBlock{Kind: blockKindError, Title: "/run", Lines: []string{"用法：/run <command ...>"}})
				return nil
			}
			return m.startCommandRun(argText)
		},
	},
	{
		Name:        "acp-demo",
		Description: "启动 ACP 权限回合演示（可配合 /permissions /allow /deny）",
		Handler: func(m *agentlineModel, _, _, argText string) tea.Cmd {
			prompt := strings.TrimSpace(argText)
			if prompt == "" {
				prompt = "请执行一次需要权限确认的文件修改"
			}
			m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/acp-demo", Lines: []string{
				"ACP demo 已启动：可用 /permissions 查看待审批项。",
				"输入 /allow 或 /deny 继续。",
			}})
			return m.startACPDemoTurn(prompt)
		},
	},
	{
		Name:        "acp-events",
		Description: "查看 ACP 事件时间线（/acp-events [N]）",
		Handler: func(m *agentlineModel, _, _, argText string) tea.Cmd {
			limit := parsePositiveIntOrDefault(argText, 40)
			m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/acp-events", Lines: m.acpEventsTimelineLines(limit)})
			m.outputOffset = 0
			return nil
		},
	},
	{
		Name:        "acp-events-summary",
		Description: "查看 ACP 事件统计摘要",
		Handler: func(m *agentlineModel, _, _, _ string) tea.Cmd {
			m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/acp-events-summary", Lines: m.acpEventsSummaryLines()})
			m.outputOffset = 0
			return nil
		},
	},
	{
		Name:        "acp-events-export",
		Description: "导出 ACP 事件为 JSONL（默认 .local/data.jsonl）",
		Handler: func(m *agentlineModel, _, _, argText string) tea.Cmd {
			path := strings.TrimSpace(argText)
			if path == "" {
				path = ".local/data.jsonl"
			}
			n, err := m.exportACPEventsJSONL(path)
			if err != nil {
				m.appendBlock(sessionBlock{Kind: blockKindError, Title: "/acp-events-export", Lines: []string{err.Error()}})
				return nil
			}
			if n == 0 {
				m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/acp-events-export", Lines: []string{"暂无 ACP 事件可导出。"}})
				return nil
			}
			m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/acp-events-export", Lines: []string{fmt.Sprintf("已导出 %d 条事件到 %s", n, path)}})
			return nil
		},
	},
	{
		Name:        "output",
		Aliases:     []string{"o", "out"},
		Description: "进入输出滚动模式",
		Handler: func(m *agentlineModel, _, _, _ string) tea.Cmd {
			m.outputFocus = true
			m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/output", Lines: []string{
				"已进入输出滚动模式。",
				"使用 ↑/↓ 单行滚动，PgUp/PgDn 翻页，Home/End 顶/底。",
				"输入 /input 返回普通输入模式。",
			}})
			return nil
		},
	},
	{
		Name:        "input",
		Aliases:     []string{"i"},
		Description: "返回输入模式",
		Handler: func(m *agentlineModel, _, _, _ string) tea.Cmd {
			m.outputFocus = false
			m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/input", Lines: []string{"已返回输入模式。"}})
			return nil
		},
	},
	{Name: "top", Description: "跳到历史顶部", Handler: func(m *agentlineModel, _, _, _ string) tea.Cmd { m.scrollOutputTop(); return nil }},
	{Name: "bottom", Aliases: []string{"end"}, Description: "跳到历史底部", Handler: func(m *agentlineModel, _, _, _ string) tea.Cmd { m.scrollOutputBottom(); return nil }},
	{Name: "up", Description: "历史按行向上滚动", Handler: func(m *agentlineModel, _, _, _ string) tea.Cmd { m.scrollOutputLines(1); return nil }},
	{Name: "down", Description: "历史按行向下滚动", Handler: func(m *agentlineModel, _, _, _ string) tea.Cmd { m.scrollOutputLines(-1); return nil }},
	{Name: "pgup", Description: "历史按页向上滚动", Handler: func(m *agentlineModel, _, _, _ string) tea.Cmd { m.scrollOutputPage(1); return nil }},
	{Name: "pgdown", Description: "历史按页向下滚动", Handler: func(m *agentlineModel, _, _, _ string) tea.Cmd { m.scrollOutputPage(-1); return nil }},
	{
		Name:        "history",
		Aliases:     []string{"his"},
		Description: "查看输入历史（默认最近 20 条，可 /history 50）",
		Handler: func(m *agentlineModel, _, _, argText string) tea.Cmd {
			limit := 20
			if argText != "" {
				n, err := strconv.Atoi(argText)
				if err != nil || n <= 0 {
					m.appendBlock(sessionBlock{Kind: blockKindError, Title: "/history", Lines: []string{"用法：/history [正整数]，例如 /history 50"}})
					return nil
				}
				limit = n
			}

			m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/history", Lines: m.historyPreviewLines(limit)})
			m.outputOffset = 0
			return nil
		},
	},
	{
		Name:        "permissions",
		Aliases:     []string{"perm"},
		Description: "查看待处理权限请求",
		Handler: func(m *agentlineModel, _, _, _ string) tea.Cmd {
			if m.permissionBroker == nil {
				m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/permissions", Lines: []string{"权限队列未初始化。"}})
				return nil
			}
			pending := m.permissionBroker.Pending()
			if len(pending) == 0 {
				m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/permissions", Lines: []string{"当前无待处理权限请求。"}})
				return nil
			}
			lines := make([]string, 0, len(pending)*3)
			for i, item := range pending {
				lines = append(lines, fmt.Sprintf("%d) request=%s session=%s tool=%s", i+1, item.RequestID, strings.TrimSpace(string(item.SessionID)), strings.TrimSpace(string(item.ToolCallID))))
				if strings.TrimSpace(item.Title) != "" {
					lines = append(lines, "   title: "+strings.TrimSpace(item.Title))
				}
				for idx, option := range item.Options {
					lines = append(lines, fmt.Sprintf("   option %d: id=%s kind=%s name=%s", idx+1, strings.TrimSpace(string(option.OptionId)), strings.TrimSpace(string(option.Kind)), strings.TrimSpace(option.Name)))
				}
			}
			m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/permissions", Lines: lines})
			return nil
		},
	},
	{
		Name:        "questions",
		Description: "查看待回答问题队列",
		Handler: func(m *agentlineModel, _, _, _ string) tea.Cmd {
			if m == nil || m.questionBroker == nil {
				m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/questions", Lines: []string{"问题队列未初始化。"}})
				return nil
			}
			pending := m.questionBroker.Pending()
			if len(pending) == 0 {
				m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/questions", Lines: []string{"当前无待回答问题。"}})
				return nil
			}
			lines := make([]string, 0, len(pending)+1)
			lines = append(lines, fmt.Sprintf("pending: %d", len(pending)))
			for i, q := range pending {
				lines = append(lines, fmt.Sprintf("%d) id=%s time=%s", i+1, q.ID, q.CreatedAt.Format("15:04:05")))
				lines = append(lines, "   prompt: "+strings.TrimSpace(q.Prompt))
			}
			m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/questions", Lines: lines})
			return nil
		},
	},
	{
		Name:        "reply",
		Description: "回答问题（/reply [ask_id] <answer>）",
		Handler: func(m *agentlineModel, _, _, argText string) tea.Cmd {
			if m == nil || m.questionBroker == nil {
				return nil
			}
			parts := strings.Fields(strings.TrimSpace(argText))
			if len(parts) == 0 {
				m.appendBlock(sessionBlock{Kind: blockKindError, Title: "/reply", Lines: []string{"用法：/reply [ask_id] <answer>"}})
				return nil
			}
			targetID := ""
			answer := strings.TrimSpace(argText)
			if strings.HasPrefix(parts[0], "ask_") {
				targetID = strings.TrimSpace(parts[0])
				answer = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(argText), targetID))
			} else if idx, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
				pending := m.questionBroker.Pending()
				if idx <= 0 || idx > len(pending) {
					m.appendBlock(sessionBlock{Kind: blockKindError, Title: "/reply", Lines: []string{fmt.Sprintf("问题序号超出范围: %d", idx)}})
					return nil
				}
				targetID = strings.TrimSpace(pending[idx-1].ID)
				answer = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(argText), strings.TrimSpace(parts[0])))
			}
			if strings.TrimSpace(answer) == "" {
				m.appendBlock(sessionBlock{Kind: blockKindError, Title: "/reply", Lines: []string{"answer 不能为空。"}})
				return nil
			}
			if err := m.questionBroker.Reply(targetID, answer); err != nil {
				m.appendBlock(sessionBlock{Kind: blockKindError, Title: "/reply", Lines: []string{err.Error()}})
				return nil
			}
			if strings.TrimSpace(targetID) == "" {
				targetID = "(latest)"
			}
			m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/reply", Lines: []string{fmt.Sprintf("已回复问题: %s", targetID)}})
			return nil
		},
	},
	{
		Name:        "skip",
		Description: "跳过问题（/skip [ask_id]）",
		Handler: func(m *agentlineModel, _, _, argText string) tea.Cmd {
			if m == nil || m.questionBroker == nil {
				return nil
			}
			targetID := strings.TrimSpace(argText)
			if err := m.questionBroker.Cancel(targetID); err != nil {
				m.appendBlock(sessionBlock{Kind: blockKindError, Title: "/skip", Lines: []string{err.Error()}})
				return nil
			}
			if targetID == "" {
				targetID = "(latest)"
			}
			m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/skip", Lines: []string{fmt.Sprintf("已跳过问题: %s", targetID)}})
			return nil
		},
	},
	{
		Name:        "allow",
		Description: "同意权限请求（/allow [request-id] [option-id|index]）",
		Handler: func(m *agentlineModel, _, _, argText string) tea.Cmd {
			return resolvePermissionSlash(m, true, argText)
		},
	},
	{
		Name:        "deny",
		Description: "拒绝权限请求（/deny [request-id] [option-id|index]）",
		Handler: func(m *agentlineModel, _, _, argText string) tea.Cmd {
			return resolvePermissionSlash(m, false, argText)
		},
	},
	{
		Name:        "cancel",
		Aliases:     []string{"stop"},
		Description: "中断当前运行中的任务",
		Handler: func(m *agentlineModel, _, _, _ string) tea.Cmd {
			if m.cancelActiveRun("收到 /cancel，正在尝试中断当前任务...") {
				return nil
			}
			m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/cancel", Lines: []string{"当前没有可中断的运行任务。"}})
			return nil
		},
	},
	{
		Name:        "fold",
		Description: "折叠 assistant/tool 详情块",
		Handler: func(m *agentlineModel, _, _, _ string) tea.Cmd {
			if m.foldDetails {
				m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/fold", Lines: []string{"当前已是折叠状态。"}})
			} else {
				m.foldDetails = true
				m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/fold", Lines: []string{"已折叠 assistant/tool 详情。输入 /unfold 可恢复。"}})
			}
			return nil
		},
	},
	{
		Name:        "unfold",
		Description: "展开 assistant/tool 详情块",
		Handler: func(m *agentlineModel, _, _, _ string) tea.Cmd {
			if !m.foldDetails {
				m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/unfold", Lines: []string{"当前已是展开状态。"}})
			} else {
				m.foldDetails = false
				m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/unfold", Lines: []string{"已展开 assistant/tool 详情。"}})
			}
			return nil
		},
	},
	{
		Name:        "clear",
		Aliases:     []string{"cls"},
		Description: "清空历史块（保留欢迎信息）",
		Handler: func(m *agentlineModel, _, _, _ string) tea.Cmd {
			m.blocks = []sessionBlock{{
				Kind:  blockKindSystem,
				Title: "system",
				Lines: []string{"输出历史已清空。", "输入 /help 查看可用命令。"},
			}}
			m.outputOffset = 0
			return nil
		},
	},
	{
		Name:        "unbind",
		Aliases:     []string{"chat-off"},
		Description: "退出聊天粘性模式",
		Handler: func(m *agentlineModel, _, _, _ string) tea.Cmd {
			if !m.isChatMode() {
				m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/unbind", Lines: []string{"当前未启用聊天粘性模式。"}})
			} else {
				m.unbindStickyInvocation()
				m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/unbind", Lines: []string{"已退出聊天粘性模式。"}})
			}
			return nil
		},
	},
	{
		Name:        "help",
		Aliases:     []string{"?"},
		Description: "显示 slash 命令帮助",
		Handler: func(m *agentlineModel, _, _, _ string) tea.Cmd {
			m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: "/help", Lines: slashHelpLines(m.root, m.agentOnlyMode)})
			return nil
		},
	},
	{Name: "quit", Aliases: []string{"exit", "q"}, Description: "退出 agentline", Handler: func(_ *agentlineModel, _, _, _ string) tea.Cmd { return tea.Quit }},
}

var (
	slashCommands      = buildSlashCommands(slashBuiltins)
	slashHandlerByName = buildSlashHandlerByName(slashBuiltins)
)

func buildSlashCommands(builtins []slashBuiltin) []slashCommand {
	out := make([]slashCommand, 0, len(builtins))
	for _, item := range builtins {
		out = append(out, slashCommand{Name: item.Name, Aliases: append([]string(nil), item.Aliases...), Description: item.Description})
	}
	return out
}

func buildSlashHandlerByName(builtins []slashBuiltin) map[string]slashHandler {
	index := make(map[string]slashHandler, len(builtins)*2)
	for _, item := range builtins {
		if item.Handler == nil {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(item.Name))
		if name != "" {
			index[name] = item.Handler
		}
		for _, alias := range item.Aliases {
			key := strings.ToLower(strings.TrimSpace(alias))
			if key == "" {
				continue
			}
			index[key] = item.Handler
		}
	}
	return index
}

func (m *agentlineModel) handleSlashInput(line string) (bool, tea.Cmd) {
	raw := strings.TrimSpace(line)
	if !strings.HasPrefix(raw, "/") {
		return false, nil
	}

	cmdText := strings.TrimSpace(strings.TrimPrefix(raw, "/"))
	if cmdText == "" {
		cmdText = "help"
	}
	parts := strings.Fields(cmdText)
	cmd := strings.ToLower(parts[0])
	argText := strings.TrimSpace(strings.TrimPrefix(cmdText, parts[0]))

	if handler, ok := slashHandlerByName[cmd]; ok {
		cmdOut := handler(m, raw, cmdText, argText)
		m.normalizeOutputOffset()
		return true, cmdOut
	}

	if isCommandLikeInputWithAlias(m.root, cmdText, m.agentOnlyMode, false) {
		return true, m.startCommandRun(cmdText)
	}

	lines := []string{
		fmt.Sprintf("未知 slash 命令: %s", cmd),
		"可尝试 /run <command...>，或直接使用 /<command ...> 形式。",
		"输入 /help 查看可用命令。",
	}
	if guessed := suggestClosestSlashNames(cmd, 3); len(guessed) > 0 {
		lines = append(lines, "你可能想输入："+formatSlashGuessList(guessed))
	}

	m.appendBlock(sessionBlock{Kind: blockKindError, Title: raw, Lines: lines})
	m.normalizeOutputOffset()
	return true, nil
}

func suggestClosestSlashNames(input string, limit int) []string {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" || limit <= 0 {
		return nil
	}

	type scored struct {
		name  string
		score int
	}

	scores := make([]scored, 0, len(slashCommands))
	seen := map[string]struct{}{}
	for _, sc := range slashCommands {
		name := strings.ToLower(strings.TrimSpace(sc.Name))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}

		d := damerauLevenshteinDistance(input, name)
		if d <= 2 {
			scores = append(scores, scored{name: name, score: d})
		}
	}

	if len(scores) == 0 {
		return nil
	}

	sort.Slice(scores, func(i, j int) bool {
		if scores[i].score != scores[j].score {
			return scores[i].score < scores[j].score
		}
		return scores[i].name < scores[j].name
	})

	if len(scores) > limit {
		scores = scores[:limit]
	}

	out := make([]string, 0, len(scores))
	for _, item := range scores {
		out = append(out, item.name)
	}
	return out
}

func formatSlashGuessList(names []string) string {
	if len(names) == 0 {
		return ""
	}
	parts := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		parts = append(parts, "/"+name)
	}
	return strings.Join(parts, "、")
}

func damerauLevenshteinDistance(a, b string) int {
	ra := []rune(a)
	rb := []rune(b)
	la, lb := len(ra), len(rb)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	dp := make([][]int, la+1)
	for i := 0; i <= la; i++ {
		dp[i] = make([]int, lb+1)
		dp[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		dp[0][j] = j
	}

	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 0
			if ra[i-1] != rb[j-1] {
				cost = 1
			}

			deletion := dp[i-1][j] + 1
			insertion := dp[i][j-1] + 1
			substitution := dp[i-1][j-1] + cost

			best := deletion
			if insertion < best {
				best = insertion
			}
			if substitution < best {
				best = substitution
			}

			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				transposition := dp[i-2][j-2] + 1
				if transposition < best {
					best = transposition
				}
			}

			dp[i][j] = best
		}
	}

	return dp[la][lb]
}

func slashHelpLines(root *redant.Command, agentOnly bool) []string {
	lines := []string{
		"slash commands:",
		"  /chat <command ...>: 绑定聊天粘性模式（保留已输入参数，后续普通输入作为 prompt 多轮复用）",
		"  /<command ...>: 直接执行命令（例如 /commit --message hi）",
		"  /run <command...>: 执行命令并输出 tool/command/result",
		"  /acp-demo [prompt]: 启动 ACP 权限回合演示",
		"  /acp-events [N]: 查看 ACP 事件时间线（默认最近 40 条）",
		"  /acp-events-summary: 查看 ACP 事件统计摘要",
		"  /acp-events-export [path]: 导出 ACP 事件到 JSONL（默认 .local/data.jsonl）",
		"  /questions: 查看待回答问题",
		"  /reply [ask_id] <answer>: 回答问题（省略 ask_id 默认回复最新）",
		"  /skip [ask_id]: 跳过问题（省略 ask_id 默认跳过最新）",
		"  /permissions: 查看待处理权限请求",
		"  /allow [request-id] [option-id|index]: 同意权限请求",
		"  /deny [request-id] [option-id|index]: 拒绝权限请求",
		"  /history [N]: 查看最近输入历史（默认 20 条）",
		"  /cancel: 中断当前运行中的任务",
		"  /fold: 折叠 assistant/tool 详情块",
		"  /unfold: 展开 assistant/tool 详情块",
		"  /unbind: 退出聊天粘性模式",
		"  /output (/o): 进入输出滚动模式",
		"  /input (/i): 返回输入模式",
		"  /top /bottom /up /down /pgup /pgdown: 浏览历史",
		"  /clear: 清空历史块",
		"  /quit: 退出 agentline",
	}

	commands := collectCommandSlashItems(root, agentOnly, "")
	if len(commands) > 0 {
		lines = append(lines, "")
		lines = append(lines, "可直接执行的命令：")
		limit := len(commands)
		if limit > 8 {
			limit = 8
		}
		for i := 0; i < limit; i++ {
			lines = append(lines, "  "+strings.TrimSpace(commands[i].Insert))
		}
		if len(commands) > limit {
			lines = append(lines, fmt.Sprintf("  ...(共 %d 个，可输入 / 查看候选)", len(commands)))
		}
	}

	return lines
}

func resolvePermissionSlash(m *agentlineModel, allow bool, argText string) tea.Cmd {
	if m == nil || m.permissionBroker == nil {
		return nil
	}
	pending := m.permissionBroker.Pending()
	if len(pending) == 0 {
		title := "/deny"
		if allow {
			title = "/allow"
		}
		m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: title, Lines: []string{"当前无待处理权限请求。"}})
		return nil
	}

	parts := strings.Fields(strings.TrimSpace(argText))
	target := pending[len(pending)-1]
	rest := []string{}
	if len(parts) > 0 {
		candidate := strings.TrimSpace(parts[0])
		if strings.HasPrefix(candidate, "perm_") {
			for _, item := range pending {
				if item.RequestID == candidate {
					target = item
					rest = parts[1:]
					goto resolve
				}
			}
			m.appendBlock(sessionBlock{Kind: blockKindError, Title: permissionTitle(allow), Lines: []string{fmt.Sprintf("未知 request id: %s", candidate)}})
			return nil
		}
		rest = parts
	}

resolve:
	if len(rest) > 0 {
		idx, optionID, isIndex := agentacp.ParseIndexOrOption(rest[0])
		var err error
		if isIndex {
			err = m.permissionBroker.ResolveByIndex(target.RequestID, idx)
		} else {
			err = m.permissionBroker.ResolveSelected(target.RequestID, optionID)
		}
		if err != nil {
			m.appendBlock(sessionBlock{Kind: blockKindError, Title: permissionTitle(allow), Lines: []string{err.Error()}})
			return nil
		}
		m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: permissionTitle(allow), Lines: []string{fmt.Sprintf("已处理 request=%s", target.RequestID)}})
		return nil
	}

	var err error
	if allow {
		err = m.permissionBroker.ResolveFirstByKind(target.RequestID, acp.PermissionOptionKindAllowOnce, acp.PermissionOptionKindAllowAlways)
	} else {
		err = m.permissionBroker.ResolveFirstByKind(target.RequestID, acp.PermissionOptionKindRejectOnce, acp.PermissionOptionKindRejectAlways)
	}
	if err != nil {
		if allow {
			err = m.permissionBroker.ResolveCancelled(target.RequestID)
		} else {
			// 无 reject 选项时回退 cancelled，保持可前进。
			err = m.permissionBroker.ResolveCancelled(target.RequestID)
		}
	}
	if err != nil {
		m.appendBlock(sessionBlock{Kind: blockKindError, Title: permissionTitle(allow), Lines: []string{err.Error()}})
		return nil
	}
	m.appendBlock(sessionBlock{Kind: blockKindSystem, Title: permissionTitle(allow), Lines: []string{fmt.Sprintf("已处理 request=%s", target.RequestID)}})
	return nil
}

func permissionTitle(allow bool) string {
	if allow {
		return "/allow"
	}
	return "/deny"
}

// ACPPermissionClient 返回仅处理 permission 的 ACP client 适配。
// session/update 建议通过调用方在 UI 主循环中转发到 appendACPSessionNotification。
func (m *agentlineModel) ACPPermissionClient() *agentacp.CallbackClient {
	if m == nil {
		return &agentacp.CallbackClient{}
	}
	return &agentacp.CallbackClient{
		PermissionBroker: m.permissionBroker,
		OnSessionUpdate: func(_ context.Context, params acp.SessionNotification) error {
			m.appendACPSessionNotification(params)
			return nil
		},
	}
}
