package session

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/original-flipster69/koko/internal/provider"
	"github.com/original-flipster69/koko/internal/secrets"
)

type Session struct {
	History []provider.Msg `json:"history"`
}

func Save(dir string, history []provider.Msg) error {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	redacted := make([]provider.Msg, len(history))
	for i, m := range history {
		if m.Role == provider.System {
			redacted[i] = m
			continue
		}
		content, _ := secrets.RedactAll(m.Content)
		redacted[i] = provider.Msg{Role: m.Role, Content: content}
	}
	s := Session{History: redacted}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "session.json"), data, 0600)
}

func Load(dir string) ([]provider.Msg, error) {
	data, err := os.ReadFile(filepath.Join(dir, "session.json"))
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return s.History, nil
}

func Clear(dir string) error {
	return os.Remove(filepath.Join(dir, "session.json"))
}
