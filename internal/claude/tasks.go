package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Task struct {
	ID         string `json:"id"`
	Subject    string `json:"subject"`
	Status     string `json:"status"`
	ActiveForm string `json:"activeForm"`
}

type Todo struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"activeForm"`
}

func LoadTasks(sessionID string) ([]Task, error) {
	taskDir := filepath.Join(ClaudeDir(), "tasks", sessionID)
	matches, err := filepath.Glob(filepath.Join(taskDir, "*.json"))
	if err != nil {
		return nil, err
	}

	var tasks []Task
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var t Task
		if err := json.Unmarshal(data, &t); err != nil {
			continue
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func LoadTodos(sessionID string) ([]Todo, error) {
	todosDir := filepath.Join(ClaudeDir(), "todos")
	matches, err := filepath.Glob(filepath.Join(todosDir, sessionID+"-agent-*.json"))
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return nil, nil
	}

	data, err := os.ReadFile(matches[0])
	if err != nil {
		return nil, err
	}

	var todos []Todo
	if err := json.Unmarshal(data, &todos); err != nil {
		return nil, err
	}
	return todos, nil
}
