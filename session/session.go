package session

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/meeseeks/koko/provider"
	"github.com/meeseeks/koko/secrets"
)

type Session struct {
	History []provider.Message `json:"history"`
}

func Save(dir string, history []provider.Message) error {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	redacted := make([]provider.Message, len(history))
	for i, m := range history {
		if m.Role == provider.RoleSystem {
			redacted[i] = m
			continue
		}
		content, _ := secrets.RedactAll(m.Content)
		redacted[i] = provider.Message{Role: m.Role, Content: content}
	}
	s := Session{History: redacted}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "session.json"), data, 0600)
}

func Load(dir string) ([]provider.Message, error) {
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
