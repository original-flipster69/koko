package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/original-flipster69/koko/internal/agent"
	"github.com/original-flipster69/koko/internal/ignore"
	"github.com/original-flipster69/koko/internal/sandbox"
	"github.com/original-flipster69/koko/internal/ui"
)

const visionMaxFiles = 1000

type read struct{ sb *sandbox.Sandbox }

func (r read) name() string { return "read" }
func (r read) desc() string { return "Read a file" }
func (r read) args() string { return "<path>" }
func (r read) do(input string, parts []string, a *agent.Agent, scheme ui.Scheme) (bool, string, string) {
	if len(parts) < 2 {
		return true, "", scheme.Error("usage: :read <path>")
	}
	vp, err := r.sb.ValidatePath(strings.TrimPrefix(input, ":read "))
	if err != nil {
		return true, "", scheme.Error(fmt.Sprintf("read failed: %v", err))
	}
	content, err := a.Editor().Read(vp)
	if err != nil {
		return true, "", scheme.Error(fmt.Sprintf("read failed: %v", err))
	}
	return true, "", content
}

type write struct{ sb *sandbox.Sandbox }

func (w write) name() string { return "write" }
func (w write) desc() string { return "Write a file" }
func (w write) args() string { return "<path> <content>" }
func (w write) do(input string, parts []string, a *agent.Agent, scheme ui.Scheme) (bool, string, string) {
	if len(parts) < 3 {
		return true, "", scheme.Error("usage: :write <path> <content>")
	}
	path := parts[1]
	content := strings.TrimPrefix(input, ":write "+path+" ")
	vp, err := w.sb.ValidatePath(path)
	if err != nil {
		return true, "", scheme.Error(fmt.Sprintf("write failed: %v", err))
	}
	if err := a.Editor().Write(vp, content, false); err != nil {
		return true, "", scheme.Error(fmt.Sprintf("write failed: %v", err))
	}
	return true, "", scheme.Info("wrote", path)
}

type replace struct{ sb *sandbox.Sandbox }

func (r replace) name() string { return "replace" }
func (r replace) desc() string { return "Replace text in a file" }
func (r replace) args() string { return "<path> <old_text> <new_text>" }
func (r replace) do(input string, parts []string, a *agent.Agent, scheme ui.Scheme) (bool, string, string) {
	if len(parts) < 4 {
		return true, "", scheme.Error("usage: :replace <path> <old_text> <new_text>")
	}
	path := parts[1]
	oldText := parts[2]
	newText := strings.TrimPrefix(input, fmt.Sprintf(":replace %s %s ", path, oldText))
	vp, err := r.sb.ValidatePath(path)
	if err != nil {
		return true, "", scheme.Error(fmt.Sprintf("replace failed: %v", err))
	}
	if _, _, err := a.Editor().Replace(vp, oldText, newText); err != nil {
		return true, "", scheme.Error(fmt.Sprintf("replace failed: %v", err))
	}
	return true, "", scheme.Info("replaced", path)
}

type deleteCmd struct{ sb *sandbox.Sandbox }

func (d deleteCmd) name() string { return "delete" }
func (d deleteCmd) desc() string { return "Delete a file" }
func (d deleteCmd) args() string { return "<path>" }
func (d deleteCmd) do(input string, parts []string, a *agent.Agent, scheme ui.Scheme) (bool, string, string) {
	if len(parts) < 2 {
		return true, "", scheme.Error("usage: :delete <path>")
	}
	path := strings.TrimPrefix(input, ":delete ")
	vp, err := d.sb.ValidatePath(path)
	if err != nil {
		return true, "", scheme.Error(fmt.Sprintf("delete failed: %v", err))
	}
	if err := a.Editor().Delete(vp); err != nil {
		return true, "", scheme.Error(fmt.Sprintf("delete failed: %v", err))
	}
	return true, "", scheme.Info("deleted", path)
}

type list struct{ sb *sandbox.Sandbox }

func (l list) name() string { return "list" }
func (l list) desc() string { return "List directory contents" }
func (l list) args() string { return "[path]" }
func (l list) do(input string, parts []string, a *agent.Agent, scheme ui.Scheme) (bool, string, string) {
	path := "."
	if len(parts) > 1 {
		path = strings.TrimPrefix(input, ":list ")
	}
	vp, err := l.sb.ValidatePath(path)
	if err != nil {
		return true, "", scheme.Error(fmt.Sprintf("list failed: %v", err))
	}
	_, entries, err := a.Editor().List(vp)
	if err != nil {
		return true, "", scheme.Error(fmt.Sprintf("list failed: %v", err))
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}
	return true, "", strings.Join(names, "\n")
}

type vision struct {
	sb     *sandbox.Sandbox
	ignore *ignore.Matcher
}

func (v vision) name() string { return "vision" }
func (v vision) desc() string { return "List files visible to the agent (after deny & ignore)" }
func (v vision) args() string { return "" }
func (v vision) do(input string, parts []string, a *agent.Agent, scheme ui.Scheme) (bool, string, string) {
	files, capped, err := visibleFiles(v.sb, v.ignore)
	if err != nil {
		return true, "", scheme.Error(fmt.Sprintf("vision failed: %v", err))
	}
	if len(files) == 0 {
		return true, "", scheme.Info("vision", "no files visible")
	}
	var b strings.Builder
	for _, f := range files {
		b.WriteString("  " + f + "\n")
	}
	summary := fmt.Sprintf("%d files visible", len(files))
	if capped {
		summary += fmt.Sprintf(" (showing first %d)", visionMaxFiles)
	}
	return true, "", scheme.Info("vision", summary) + "\n" + strings.TrimRight(b.String(), "\n")
}

func visibleFiles(sb *sandbox.Sandbox, ig *ignore.Matcher) ([]string, bool, error) {
	root := sb.Root()
	var out []string
	capped := false
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." {
			return nil
		}
		if ig.IsIgnored(rel, info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if _, err := sb.ValidatePath(path); err != nil {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if len(out) >= visionMaxFiles {
			capped = true
			return filepath.SkipAll
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, capped, err
	}
	sort.Strings(out)
	return out, capped, nil
}
