package memory

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Type string

const (
	TypeUser      Type = "user"
	TypeFeedback  Type = "feedback"
	TypeProject   Type = "project"
	TypeReference Type = "reference"
)

type Memory struct {
	Name        string
	Description string
	Type        Type
	Body        string
	Path        string
}

type Store struct {
	dir string
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("creating memory dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

func (s *Store) Dir() string { return s.dir }

func (s *Store) List() ([]Memory, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var out []Memory
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if e.Name() == "MEMORY.md" {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("memory: skipping unreadable entry", "path", path, "err", err)
			continue
		}
		m := parse(string(data))
		if m.Name == "" {
			m.Name = strings.TrimSuffix(e.Name(), ".md")
		}
		m.Path = path
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Store) Get(name string) (Memory, bool, error) {
	list, err := s.List()
	if err != nil {
		return Memory{}, false, err
	}
	for _, m := range list {
		if m.Name == name {
			return m, true, nil
		}
	}
	return Memory{}, false, nil
}

var slugRe = regexp.MustCompile(`[^a-z0-9_-]+`)

func slug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "_")
	s = slugRe.ReplaceAllString(s, "")
	if s == "" {
		s = "memory"
	}
	return s
}

func (s *Store) Save(m Memory) (string, error) {
	if m.Name == "" {
		return "", fmt.Errorf("memory name required")
	}
	if m.Type == "" {
		m.Type = TypeProject
	}
	filename := slug(m.Name) + ".md"
	path := filepath.Join(s.dir, filename)
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("name: %s\n", m.Name))
	b.WriteString(fmt.Sprintf("description: %s\n", m.Description))
	b.WriteString(fmt.Sprintf("type: %s\n", m.Type))
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimSpace(m.Body))
	b.WriteString("\n")
	if err := os.WriteFile(path, []byte(b.String()), 0640); err != nil {
		return "", err
	}
	if err := s.rebuildIndex(); err != nil {
		return path, err
	}
	return path, nil
}

func (s *Store) Delete(name string) error {
	m, ok, err := s.Get(name)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no memory named %q", name)
	}
	if err := os.Remove(m.Path); err != nil {
		return err
	}
	return s.rebuildIndex()
}

func (s *Store) rebuildIndex() error {
	list, err := s.List()
	if err != nil {
		return err
	}
	var b strings.Builder
	for _, m := range list {
		desc := m.Description
		if desc == "" {
			desc = "(no description)"
		}
		rel := filepath.Base(m.Path)
		b.WriteString(fmt.Sprintf("- [%s](%s) — %s\n", m.Name, rel, desc))
	}
	return os.WriteFile(filepath.Join(s.dir, "MEMORY.md"), []byte(b.String()), 0640)
}

func (s *Store) Index() string {
	data, err := os.ReadFile(filepath.Join(s.dir, "MEMORY.md"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func parse(raw string) Memory {
	var m Memory
	rest := raw
	if strings.HasPrefix(raw, "---\n") {
		end := strings.Index(raw[4:], "\n---\n")
		if end >= 0 {
			fm := raw[4 : 4+end]
			rest = raw[4+end+5:]
			for _, line := range strings.Split(fm, "\n") {
				idx := strings.IndexByte(line, ':')
				if idx < 0 {
					continue
				}
				k := strings.TrimSpace(line[:idx])
				v := strings.TrimSpace(line[idx+1:])
				v = strings.Trim(v, `"'`)
				switch strings.ToLower(k) {
				case "name":
					m.Name = v
				case "description":
					m.Description = v
				case "type":
					m.Type = Type(v)
				}
			}
		}
	}
	m.Body = strings.TrimSpace(rest)
	return m
}
