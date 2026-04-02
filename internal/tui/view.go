package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hwangjungmin/lazycc/internal/claude"
)

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	leftWidth := m.width * 3 / 10
	rightWidth := m.width - leftWidth

	leftView := m.renderSessionList(leftWidth, m.height-2)
	rightView := m.renderTranscript(rightWidth, m.height-2)
	helpView := m.renderHelp()

	main := lipgloss.JoinHorizontal(lipgloss.Top, leftView, rightView)
	return lipgloss.JoinVertical(lipgloss.Left, main, helpView)
}

func (m Model) renderSessionList(width, height int) string {
	filtered := m.filteredSessions()
	innerWidth := width - 2

	var rows []string
	header := titleStyle.Render("Sessions")
	rows = append(rows, header)

	if len(filtered) == 0 {
		rows = append(rows, deadStyle.Render(" No sessions"))
	}

	for i, s := range filtered {
		row := formatSessionRow(s, innerWidth)
		if i == m.cursor {
			row = selectedRowStyle.Width(innerWidth).Render(row)
		}
		rows = append(rows, row)
	}

	content := strings.Join(rows, "\n")
	return panelStyle.Width(width).Height(height).Render(content)
}

func formatSessionRow(s claude.Session, width int) string {
	var icon string
	var style lipgloss.Style

	if !s.Alive {
		icon = "○"
		style = deadStyle
		return style.Render(fmt.Sprintf("%s Dead  %-12s  --   --", icon, truncateStr(s.Project, 12)))
	}

	switch s.Status {
	case "Work":
		icon = "●"
		style = workStyle
	case "Thinking":
		icon = "~"
		style = thinkStyle
	case "Wait":
		icon = "○"
		style = waitStyle
	default:
		icon = "○"
		style = deadStyle
	}

	statusLabel := s.Status
	if len(statusLabel) > 5 {
		statusLabel = statusLabel[:5]
	}

	elapsed := claude.FormatElapsed(s.ElapsedSec)
	return style.Render(fmt.Sprintf("%s %-5s %-12s %3d%%  %s",
		icon,
		statusLabel,
		truncateStr(s.Project, 12),
		s.CtxPercent,
		elapsed,
	))
}

func (m Model) renderTranscript(width, height int) string {
	s := m.selected()
	if s == nil {
		return panelStyle.Width(width).Height(height).Render(deadStyle.Render(" No session selected"))
	}

	// Header
	sessionShort := s.SessionID
	if len(sessionShort) > 8 {
		sessionShort = sessionShort[:8]
	}

	inputK := float64(s.InputTokens) / 1000
	outputK := float64(s.OutputTokens) / 1000

	header := titleStyle.Render(fmt.Sprintf(
		"%s / %s | %s | %s | Ctx %d%% | Turns %d | ↑%.0fK ↓%.0fK | ~$%.2f | %s | %s",
		s.Project, sessionShort,
		s.Model, s.Status,
		s.CtxPercent, s.Turns,
		inputK, outputK,
		s.CostUSD,
		s.GitBranch,
		claude.FormatElapsed(s.ElapsedSec),
	))

	// Transcript body via viewport
	body := m.viewport.View()

	// Todo overlay
	var todoOverlay string
	if m.showTodos && len(s.Tasks)+len(s.Todos) > 0 {
		todoOverlay = renderTodoOverlay(s)
	}

	content := header + "\n" + strings.Repeat("─", width-2) + "\n" + body
	if todoOverlay != "" {
		content += "\n" + todoOverlay
	}

	return panelStyle.Width(width).Height(height).Render(content)
}

func renderTranscriptContent(s *claude.Session) string {
	if s == nil || len(s.Transcript) == 0 {
		return ""
	}

	var lines []string
	for _, entry := range s.Transcript {
		switch entry.Role {
		case "user":
			lines = append(lines, userStyle.Render("user: ")+truncateStr(entry.Text, 200))
		case "assistant":
			text := truncateStr(entry.Text, 200)
			lines = append(lines, assistantStyle.Render("assistant: ")+text)
			if entry.ToolUse != "" {
				lines = append(lines, toolStyle.Render("  ↳ "+entry.ToolUse))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func renderTodoOverlay(s *claude.Session) string {
	var lines []string
	lines = append(lines, titleStyle.Render("Tasks / Todos"))

	for _, t := range s.Tasks {
		icon := "○"
		if t.Status == "completed" {
			icon = "✓"
		} else if t.Status == "in_progress" {
			icon = "●"
		}
		lines = append(lines, fmt.Sprintf(" %s %s", icon, truncateStr(t.Subject, 60)))
	}
	for _, t := range s.Todos {
		icon := "○"
		if t.Status == "completed" {
			icon = "✓"
		}
		lines = append(lines, fmt.Sprintf(" %s %s", icon, truncateStr(t.Content, 60)))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderHelp() string {
	help := lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render(
		" ↑↓/jk select | enter attach | x kill | t todo | A all | r refresh | q quit",
	)
	return help
}

func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}
