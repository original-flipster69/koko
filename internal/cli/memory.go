package cli

import (
	"fmt"
	"strings"

	"github.com/original-flipster69/koko/internal/agent"
	"github.com/original-flipster69/koko/internal/memories"
	"github.com/original-flipster69/koko/internal/ui"
)

// registerMemoryCommands registers memory operations like :save_memory, :list_memories, etc.
func registerMemoryCommands(a *agent.Agent, scheme ui.Scheme) map[string]Command {
	return map[string]Command{
		":save_memory": {
			Desc: "Save a memory",
			Args: "<name> <type> <description> <body>",
			Fn: func(input string, parts []string, a *agent.Agent) (bool, string, string) {
				if len(parts) < 5 {
					return true, "", scheme.Error("usage: :save_memory <name> <type> <description> <body>")
				}
				name := parts[1]
				type_ := parts[2]
				description := parts[3]
				body := strings.TrimPrefix(input, fmt.Sprintf(":save_memory %s %s %s ", name, type_, description))
				if err := a.SaveMemory(name, type_, description, body); err != nil {
					return true, "", scheme.Error(fmt.Sprintf("save_memory failed: %v", err))
				}
				return true, "", scheme.Info("saved", name)
			},
		},
		":list_memories": {
			Desc: "List all memories",
			Fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
				memories, err := a.ListMemories()
				if err != nil {
					return true, "", scheme.Error(fmt.Sprintf("list_memories failed: %v", err))
				}
				return true, "", memories
			},
		},
		":delete_memory": {
			Desc: "Delete a memory",
			Args: "<name>",
			Fn: func(input string, parts []string, a *agent.Agent) (bool, string, string) {
				if len(parts) < 2 {
					return true, "", scheme.Error("usage: :delete_memory <name>")
				}
				name := strings.TrimPrefix(input, ":delete_memory ")
				if err := a.DeleteMemory(name); err != nil {
					return true, "", scheme.Error(fmt.Sprintf("delete_memory failed: %v", err))
				}
				return true, "", scheme.Info("deleted", name)
			},
		},
	}
}