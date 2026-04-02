package tui

import "github.com/charmbracelet/lipgloss"

var (
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("241"))

	selectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Background(lipgloss.Color("236"))

	workStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	thinkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("51"))
	waitStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
	deadStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	helpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))

	userStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true)
	assistantStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("40")).Bold(true)
	toolStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)
