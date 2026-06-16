package lever

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/original-flipster69/koko/internal/privacy"
	"github.com/original-flipster69/koko/internal/provider"
)

type sessionFile struct {
	History []provider.Msg `json:"history"`
}

func saveSession(dir string, history []provider.Msg) error {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	redacted := make([]provider.Msg, len(history))
	for i, m := range history {
		if m.Role == provider.System {
			redacted[i] = m
			continue
		}
		content, _ := privacy.RedactAll(m.Content)
		redacted[i] = provider.Msg{Role: m.Role, Content: content}
	}
	data, err := json.MarshalIndent(sessionFile{History: redacted}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "session.json"), data, 0600)
}

func loadSession(dir string) ([]provider.Msg, error) {
	data, err := os.ReadFile(filepath.Join(dir, "session.json"))
	if err != nil {
		return nil, err
	}
	var s sessionFile
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return s.History, nil
}
