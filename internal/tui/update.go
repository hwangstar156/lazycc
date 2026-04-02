package tui

import (
	"os/exec"
	"syscall"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hwangjungmin/lazycc/internal/claude"
)

type resumeDoneMsg struct{ err error }

type tasksLoadedMsg struct {
	tasks []claude.Task
	todos []claude.Todo
}

func loadTasksCmd(sessionID string) tea.Cmd {
	return func() tea.Msg {
		tasks, _ := claude.LoadTasks(sessionID)
		todos, _ := claude.LoadTodos(sessionID)
		return tasksLoadedMsg{tasks: tasks, todos: todos}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		rightWidth := m.width*7/10 - 2
		m.viewport = newViewport(rightWidth, m.height-4)

	case claude.TickMsg:
		return m, tea.Batch(claude.Poll, claude.TickCmd())

	case claude.PollMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.sessions = msg.Sessions
			filtered := m.filteredSessions()
			if m.cursor >= len(filtered) {
				m.cursor = max(0, len(filtered)-1)
			}
			// Update viewport only if content actually changed
			if s := m.selected(); s != nil {
				content := renderTranscriptContent(s)
				if content != m.lastContent {
					wasAtBottom := m.viewport.AtBottom()
					m.viewport.SetContent(content)
					m.lastContent = content
					if wasAtBottom {
						m.viewport.GotoBottom()
					}
				}
			}
		}

	case tasksLoadedMsg:
		if s := m.selected(); s != nil {
			s.Tasks = msg.tasks
			s.Todos = msg.todos
		}

	case resumeDoneMsg:
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.filteredSessions())-1 {
				m.cursor++
				if s := m.selected(); s != nil {
					m.viewport.SetContent(renderTranscriptContent(s))
					m.viewport.GotoBottom()
				}
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				if s := m.selected(); s != nil {
					m.viewport.SetContent(renderTranscriptContent(s))
					m.viewport.GotoBottom()
				}
			}
		case "enter", "a":
			if s := m.selected(); s != nil {
				c := exec.Command("claude", "--resume", s.SessionID)
				return m, tea.ExecProcess(c, func(err error) tea.Msg {
					return resumeDoneMsg{err}
				})
			}
		case "x":
			if s := m.selected(); s != nil && s.Alive {
				syscall.Kill(s.PID, syscall.SIGTERM)
			}
		case "t":
			m.showTodos = !m.showTodos
			if m.showTodos {
				if s := m.selected(); s != nil {
					return m, loadTasksCmd(s.SessionID)
				}
			}
		case "A":
			m.showDead = !m.showDead
			m.cursor = 0
		case "r":
			return m, claude.Poll
		case "q":
			return m, tea.Quit
		default:
			// Pass through to viewport for scrolling
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func newViewport(width, height int) viewport.Model {
	vp := viewport.New(width, height)
	vp.SetContent("")
	return vp
}
