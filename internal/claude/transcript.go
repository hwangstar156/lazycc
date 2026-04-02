package claude

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxTailBytes = 512 * 1024
const maxHeadBytes = 8 * 1024

type TranscriptEntry struct {
	Role    string // "user" | "assistant" | "tool"
	Text    string
	ToolUse string // formatted tool_use summary
}

type jsonlMessage struct {
	Type       string          `json:"type"`
	Role       string          `json:"role"`
	StopReason *string         `json:"stop_reason"`
	Message    *innerMessage   `json:"message"`
	Content    json.RawMessage `json:"content"`
	Model      string          `json:"model"`
	Usage      *usageInfo      `json:"usage"`
	GitBranch  string          `json:"gitBranch"`
	Timestamp  string          `json:"timestamp"`
}

type innerMessage struct {
	Role       string          `json:"role"`
	Model      string          `json:"model"`
	StopReason *string         `json:"stop_reason"`
	Content    json.RawMessage `json:"content"`
	Usage      *usageInfo      `json:"usage"`
}

type usageInfo struct {
	InputTokens        int `json:"input_tokens"`
	OutputTokens       int `json:"output_tokens"`
	CacheCreationInput int `json:"cache_creation_input_tokens"`
	CacheReadInput     int `json:"cache_read_input_tokens"`
}

type contentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

var modelPricing = map[string][2]float64{
	"claude-opus-4-6":   {15.0, 75.0},
	"claude-sonnet-4-6": {3.0, 15.0},
	"claude-haiku-4-5":  {0.8, 4.0},
}

func ParseTranscript(session *Session) error {
	jsonlPath := findJSONLPath(session)
	if jsonlPath == "" {
		return nil
	}

	// Parse summary from head
	parseSummary(session, jsonlPath)

	// Tail read
	lines, err := readTailLines(jsonlPath)
	if err != nil {
		return err
	}

	// Parse transcript entries and find last assistant message
	var lastAssistantMsg *jsonlMessage
	var turns int
	var transcript []TranscriptEntry

	for _, line := range lines {
		var msg jsonlMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}

		role := msg.Role
		if msg.Message != nil {
			role = msg.Message.Role
		}

		switch {
		case msg.Type == "user" || role == "user":
			text := extractText(msg.Content)
			if text != "" {
				turns++
				transcript = append(transcript, TranscriptEntry{Role: "user", Text: truncate(text, 200)})
			}
		case msg.Type == "assistant" || role == "assistant":
			lastAssistantMsg = &msg
			content := msg.Content
			if msg.Message != nil {
				content = msg.Message.Content
			}
			text, toolUse := extractAssistantContent(content)
			entry := TranscriptEntry{Role: "assistant", Text: text}
			if toolUse != "" {
				entry.ToolUse = toolUse
			}
			transcript = append(transcript, entry)
		}
	}

	session.Turns = turns
	session.Transcript = transcript

	if lastAssistantMsg != nil {
		parseLastAssistant(session, lastAssistantMsg)
	}

	return nil
}

func parseLastAssistant(session *Session, msg *jsonlMessage) {
	// Model
	model := msg.Model
	if msg.Message != nil && msg.Message.Model != "" {
		model = msg.Message.Model
	}
	session.Model = shortenModel(model)

	// GitBranch
	if msg.GitBranch != "" {
		session.GitBranch = msg.GitBranch
	}

	// Usage & Context
	usage := msg.Usage
	if msg.Message != nil && msg.Message.Usage != nil {
		usage = msg.Message.Usage
	}
	if usage != nil {
		session.InputTokens = usage.InputTokens + usage.CacheCreationInput + usage.CacheReadInput
		session.OutputTokens = usage.OutputTokens
		session.CtxPercent = session.InputTokens * 100 / 200000
		if session.CtxPercent > 100 {
			session.CtxPercent = 100
		}
	}

	// Cost
	session.CostUSD = estimateCost(model, session.InputTokens, session.OutputTokens)

	// Status
	stopReason := msg.StopReason
	if msg.Message != nil && msg.Message.StopReason != nil {
		stopReason = msg.Message.StopReason
	}

	if stopReason != nil && *stopReason == "end_turn" {
		session.Status = "Wait"
	} else {
		session.Status = "Work"
		// Check for thinking
		content := msg.Content
		if msg.Message != nil {
			content = msg.Message.Content
		}
		if hasThinkingBlock(content) && isRecent(msg.Timestamp, 30) {
			session.Status = "Thinking"
		}
	}

	// LastTool & LastText
	content := msg.Content
	if msg.Message != nil {
		content = msg.Message.Content
	}
	text, toolUse := extractAssistantContent(content)
	session.LastText = truncate(text, 200)
	session.LastTool = toolUse
}

func findJSONLPath(session *Session) string {
	encodedCWD := strings.ReplaceAll(session.CWD, "/", "-")
	if !strings.HasPrefix(encodedCWD, "-") {
		encodedCWD = "-" + encodedCWD
	}
	// Remove leading double dash if present
	encodedCWD = strings.TrimPrefix(encodedCWD, "--")
	encodedCWD = "-" + strings.TrimPrefix(encodedCWD, "-")

	projectsDir := filepath.Join(ClaudeDir(), "projects")
	jsonlPath := filepath.Join(projectsDir, encodedCWD, session.SessionID+".jsonl")
	if _, err := os.Stat(jsonlPath); err == nil {
		return jsonlPath
	}

	// Fallback: try to find matching directory
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate := filepath.Join(projectsDir, e.Name(), session.SessionID+".jsonl")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func readTailLines(path string) ([][]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := info.Size()
	offset := int64(0)
	readSize := size
	if size > maxTailBytes {
		offset = size - maxTailBytes
		readSize = maxTailBytes
	}

	buf := make([]byte, readSize)
	if _, err := f.ReadAt(buf, offset); err != nil && err != io.EOF {
		return nil, err
	}

	rawLines := strings.Split(string(buf), "\n")
	// Skip first line if we read from offset (may be truncated)
	if offset > 0 && len(rawLines) > 0 {
		rawLines = rawLines[1:]
	}

	var lines [][]byte
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, []byte(line))
		}
	}
	return lines, nil
}

func parseSummary(session *Session, path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	buf := make([]byte, maxHeadBytes)
	n, _ := f.Read(buf)
	buf = buf[:n]

	for _, line := range strings.Split(string(buf), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var msg jsonlMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.Type == "user" || msg.Role == "user" {
			text := extractText(msg.Content)
			if text != "" {
				session.Summary = truncate(text, 80)
				return
			}
		}
	}
}

func extractText(content json.RawMessage) string {
	if content == nil {
		return ""
	}

	// Try as string
	var s string
	if json.Unmarshal(content, &s) == nil {
		return s
	}

	// Try as array of content blocks
	var blocks []contentBlock
	if json.Unmarshal(content, &blocks) == nil {
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				return b.Text
			}
		}
	}
	return ""
}

func extractAssistantContent(content json.RawMessage) (text string, toolUse string) {
	if content == nil {
		return "", ""
	}

	var blocks []contentBlock
	if json.Unmarshal(content, &blocks) != nil {
		return "", ""
	}

	var lastText string
	var lastTool string

	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				lastText = b.Text
			}
		case "tool_use":
			var inputMap map[string]any
			if b.Input != nil {
				json.Unmarshal(b.Input, &inputMap)
			}
			lastTool = formatToolUse(b.Name, inputMap)
		}
	}
	return lastText, lastTool
}

func hasThinkingBlock(content json.RawMessage) bool {
	if content == nil {
		return false
	}
	var blocks []contentBlock
	if json.Unmarshal(content, &blocks) != nil {
		return false
	}
	for _, b := range blocks {
		if b.Type == "thinking" {
			return true
		}
	}
	return false
}

func isRecent(timestamp string, seconds int) bool {
	if timestamp == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		// Try other formats
		t, err = time.Parse("2006-01-02T15:04:05.000Z", timestamp)
		if err != nil {
			return true // assume recent if can't parse
		}
	}
	return time.Since(t).Seconds() < float64(seconds)
}

func formatToolUse(name string, input map[string]any) string {
	if input == nil {
		return name
	}
	switch name {
	case "Edit", "Write", "Read":
		if fp, ok := input["file_path"].(string); ok {
			return name + " " + shortenPath(fp)
		}
	case "Bash":
		if cmd, ok := input["command"].(string); ok {
			return "Bash " + truncate(cmd, 50)
		}
	case "Grep":
		if pat, ok := input["pattern"].(string); ok {
			return "Grep " + pat
		}
	case "Agent", "Skill":
		if desc, ok := input["description"].(string); ok {
			return name + " " + truncate(desc, 50)
		}
	}
	return name
}

func estimateCost(model string, inputTokens, outputTokens int) float64 {
	pricing, ok := modelPricing[model]
	if !ok {
		return 0
	}
	return float64(inputTokens)/1e6*pricing[0] + float64(outputTokens)/1e6*pricing[1]
}

func shortenModel(model string) string {
	model = strings.TrimPrefix(model, "claude-")
	return model
}

func shortenPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) <= 3 {
		return path
	}
	return strings.Join(parts[len(parts)-3:], "/")
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

func FormatElapsed(sec int64) string {
	if sec < 0 {
		return "--"
	}
	hours := sec / 3600
	minutes := (sec % 3600) / 60
	secs := sec % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dm%ds", minutes, secs)
}
