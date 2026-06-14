package cli

import (
	"fmt"
	"strings"

	"github.com/original-flipster69/koko/internal/agent"
	"github.com/original-flipster69/koko/internal/sandbox"
	"github.com/original-flipster69/koko/internal/ui"
)

// registerFileCommands registers file operations like :read, :write, etc.
func registerFileCommands(sb *sandbox.Sandbox, scheme ui.Scheme) map[string]Command {
	return map[string]Command{
		":read": {
			Desc: "Read a file",
			Args: "<path>",
			Fn: func(input string, parts []string, _ *agent.Agent) (bool, string, string) {
				if len(parts) < 2 {
					return true, "", scheme.Error("usage: :read <path>")
				}
				path := strings.TrimPrefix(input, ":read ")
				content, err := sb.ReadFile(path)
				if err != nil {
					return true, "", scheme.Error(fmt.Sprintf("read failed: %v", err))
				}
				return true, "", string(content)
			},
		},
		":write": {
			Desc: "Write a file",
			Args: "<path> <content>",
			Fn: func(input string, parts []string, _ *agent.Agent) (bool, string, string) {
				if len(parts) < 3 {
					return true, "", scheme.Error("usage: :write <path> <content>")
				}
				path := parts[1]
				content := strings.TrimPrefix(input, ":write "+path+" ")
				if err := sb.WriteFile(path, []byte(content), false); err != nil {
					return true, "", scheme.Error(fmt.Sprintf("write failed: %v", err))
				}
				return true, "", scheme.Info("wrote", path)
			},
		},
		":replace": {
			Desc: "Replace text in a file",
			Args: "<path> <old_text> <new_text>",
			Fn: func(input string, parts []string, _ *agent.Agent) (bool, string, string) {
				if len(parts) < 4 {
					return true, "", scheme.Error("usage: :replace <path> <old_text> <new_text>")
				}
				path := parts[1]
				oldText := parts[2]
				newText := strings.TrimPrefix(input, fmt.Sprintf(":replace %s %s ", path, oldText))
				if err := sb.ReplaceInFile(path, oldText, newText); err != nil {
					return true, "", scheme.Error(fmt.Sprintf("replace failed: %v", err))
				}
				return true, "", scheme.Info("replaced", path)
			},
		},
		":delete": {
			Desc: "Delete a file",
			Args: "<path>",
			Fn: func(input string, parts []string, _ *agent.Agent) (bool, string, string) {
				if len(parts) < 2 {
					return true, "", scheme.Error("usage: :delete <path>")
				}
				path := strings.TrimPrefix(input, ":delete ")
				if err := sb.DeleteFile(path); err != nil {
					return true, "", scheme.Error(fmt.Sprintf("delete failed: %v", err))
				}
				return true, "", scheme.Info("deleted", path)
			},
		},
		":list": {
			Desc: "List directory contents",
			Args: "[path]",
			Fn: func(input string, parts []string, _ *agent.Agent) (bool, string, string) {
				path := "."
				if len(parts) > 1 {
					path = strings.TrimPrefix(input, ":list ")
				}
				entries, err := sb.ListDir(path)
				if err != nil {
					return true, "", scheme.Error(fmt.Sprintf("list failed: %v", err))
				}
				return true, "", strings.Join(entries, "\n")
			},
		},
	}
}