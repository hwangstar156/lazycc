package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

type Session struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	CWD       string `json:"cwd"`
	StartedAt int64  `json:"startedAt"`
	Kind      string `json:"kind"`

	// Runtime fields (populated from JSONL parsing)
	Alive        bool
	Project      string
	Status       string // "Work" | "Thinking" | "Wait"
	Model        string
	CtxPercent   int
	Turns        int
	Summary      string
	LastTool     string
	LastText     string
	GitBranch    string
	ElapsedSec   int64
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	Tasks        []Task
	Todos        []Todo
	Transcript   []TranscriptEntry
}

func ClaudeDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

func LoadSessions() ([]Session, error) {
	sessDir := filepath.Join(ClaudeDir(), "sessions")
	matches, err := filepath.Glob(filepath.Join(sessDir, "*.json"))
	if err != nil {
		return nil, err
	}

	seen := make(map[string]Session)

	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var s Session
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}

		s.Project = filepath.Base(s.CWD)
		s.Alive = isProcessAlive(s.PID)
		s.ElapsedSec = time.Now().Unix() - s.StartedAt/1000

		if existing, ok := seen[s.SessionID]; ok {
			if s.StartedAt > existing.StartedAt {
				seen[s.SessionID] = s
			}
		} else {
			seen[s.SessionID] = s
		}
	}

	sessions := make([]Session, 0, len(seen))
	for _, s := range seen {
		sessions = append(sessions, s)
	}

	// Alive first, then by StartedAt descending (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].Alive != sessions[j].Alive {
			return sessions[i].Alive
		}
		return sessions[i].StartedAt > sessions[j].StartedAt
	})

	return sessions, nil
}

func isProcessAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
