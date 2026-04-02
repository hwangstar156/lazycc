package tui

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hwangjungmin/lazycc/internal/claude"
)

type Model struct {
	sessions  []claude.Session
	cursor    int
	width     int
	height    int
	showDead  bool
	showTodos bool
	viewport  viewport.Model
	err       error
}

func NewModel() Model {
	return Model{}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		claude.Poll,
		claude.TickCmd(),
	)
}

func (m Model) filteredSessions() []claude.Session {
	if m.showDead {
		return m.sessions
	}
	var alive []claude.Session
	for _, s := range m.sessions {
		if s.Alive {
			alive = append(alive, s)
		}
	}
	return alive
}

func (m Model) selected() *claude.Session {
	filtered := m.filteredSessions()
	if m.cursor >= 0 && m.cursor < len(filtered) {
		return &filtered[m.cursor]
	}
	return nil
}
