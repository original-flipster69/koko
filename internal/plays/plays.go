package plays

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Play struct {
	Name        string
	Description string
	Body        string
	Path        string
}

const argsPlaceholder = "{{args}}"

func (p Play) Render(args string) string {
	if strings.Contains(p.Body, argsPlaceholder) {
		return strings.ReplaceAll(p.Body, argsPlaceholder, args)
	}
	if args == "" {
		return p.Body
	}
	return p.Body + "\n\nUser request:\n" + args
}

type Registry struct {
	dir   string
	plays map[string]Play
}

func Load(dir string) (*Registry, error) {
	r := &Registry{dir: dir, plays: make(map[string]Play)}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("plays: skipping unreadable entry", "path", path, "err", err)
			continue
		}
		p := parse(string(data))
		p.Name = strings.TrimSuffix(e.Name(), ".md")
		p.Path = path
		r.plays[p.Name] = p
	}
	return r, nil
}

func (r *Registry) Get(name string) (Play, bool) {
	p, ok := r.plays[name]
	return p, ok
}

func (r *Registry) List() []Play {
	out := make([]Play, 0, len(r.plays))
	for _, p := range r.plays {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (r *Registry) Dir() string { return r.dir }

func (r *Registry) Index() string {
	list := r.List()
	if len(list) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Available plays (invoke via :<name>):\n")
	for _, p := range list {
		desc := p.Description
		if desc == "" {
			desc = "(no description)"
		}
		b.WriteString(fmt.Sprintf("- %s: %s\n", p.Name, desc))
	}
	return b.String()
}

func parse(raw string) Play {
	var p Play
	rest := raw
	if strings.HasPrefix(raw, "---\n") {
		end := strings.Index(raw[4:], "\n---\n")
		if end >= 0 {
			fm := raw[4 : 4+end]
			rest = raw[4+end+5:]
			for _, line := range strings.Split(fm, "\n") {
				k, v, ok := splitKV(line)
				if !ok {
					continue
				}
				switch strings.ToLower(k) {
				case "description":
					p.Description = v
				}
			}
		}
	}
	p.Body = strings.TrimSpace(rest)
	return p
}

func splitKV(line string) (string, string, bool) {
	idx := strings.IndexByte(line, ':')
	if idx < 0 {
		return "", "", false
	}
	k := strings.TrimSpace(line[:idx])
	v := strings.TrimSpace(line[idx+1:])
	v = strings.Trim(v, `"'`)
	return k, v, true
}
