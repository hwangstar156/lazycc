package claude

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type PollMsg struct {
	Sessions []Session
	Err      error
}

type TickMsg time.Time

func Poll() tea.Msg {
	sessions, err := LoadSessions()
	if err != nil {
		return PollMsg{Err: err}
	}
	for i := range sessions {
		if sessions[i].Alive {
			_ = ParseTranscript(&sessions[i])
		}
	}
	return PollMsg{Sessions: sessions}
}

func TickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}
