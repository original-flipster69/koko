package cli

import (
	"fmt"
	"strings"

	"github.com/original-flipster69/koko/internal/agent"
	"github.com/original-flipster69/koko/internal/memories"
	"github.com/original-flipster69/koko/internal/ui"
)

type memoriesCmd struct{ store *memories.Store }

func (m memoriesCmd) name() string { return "memories" }
func (m memoriesCmd) desc() string { return "Manage memories" }
func (m memoriesCmd) args() string { return "[<name> | add <name> <body> | delete <name>]" }
func (m memoriesCmd) do(input string, parts []string, a *agent.Agent, scheme ui.Scheme) (bool, string, string) {
	return true, "", memoryCommand(m.store, scheme, input, parts)
}

func memoryCommand(store *memories.Store, scheme ui.Scheme, input string, parts []string) string {
	if len(parts) < 2 {
		list, err := store.List()
		if err != nil {
			return scheme.Error(fmt.Sprintf("cannot list memories: %v", err))
		}
		if len(list) == 0 {
			return scheme.Info("memories", "none stored")
		}
		var b strings.Builder
		for _, mem := range list {
			desc := mem.Description
			if desc == "" {
				desc = "(no description)"
			}
			b.WriteString(scheme.Info(mem.Name, fmt.Sprintf("[%s] %s", mem.Type, desc)) + "\n")
		}
		return strings.TrimRight(b.String(), "\n")
	}

	switch parts[1] {
	case "add":
		if len(parts) < 4 {
			return scheme.Error("usage: :memories add <name> <body>")
		}
		name := parts[2]
		body := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(input, parts[0])), "add"))
		body = strings.TrimSpace(strings.TrimPrefix(body, name))
		if body == "" {
			return scheme.Error("usage: :memories add <name> <body>")
		}
		if _, err := store.Save(memories.Memory{Name: name, Type: memories.TypeProject, Body: body}); err != nil {
			return scheme.Error(fmt.Sprintf("cannot save memories: %v", err))
		}
		return scheme.Info("saved", fmt.Sprintf("memories %q", name))
	case "delete":
		if len(parts) < 3 {
			return scheme.Error("usage: :memories delete <name>")
		}
		if err := store.Delete(parts[2]); err != nil {
			return scheme.Error(err.Error())
		}
		return scheme.Info("deleted", fmt.Sprintf("memories %q", parts[2]))
	default:
		name := parts[1]
		mem, ok, err := store.Get(name)
		if err != nil {
			return scheme.Error(fmt.Sprintf("cannot read memories: %v", err))
		}
		if !ok {
			return scheme.Error(fmt.Sprintf("no memories named %q", name))
		}
		var b strings.Builder
		b.WriteString(scheme.Info(mem.Name, fmt.Sprintf("[%s] %s", mem.Type, mem.Description)) + "\n\n")
		b.WriteString(mem.Body)
		return b.String()
	}
}
