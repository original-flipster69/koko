package agent

import (
	"context"

	"github.com/original-flipster69/koko/internal/provider"
)

type tool struct {
	Name        string
	Description string
	Params      provider.Schema
	Verb        string
	ReadOnly    bool
	Quiet       bool
	Handler     func(*Agent, context.Context, provider.ToolCall) string
}

var tools = []tool{
	{
		Name:        "read_file",
		Description: "Read the contents of a file. Returns numbered lines. Use offset and limit to read specific sections of large files.",
		Verb:        "◇ reading",
		ReadOnly:    true,
		Quiet:       true,
		Handler:     (*Agent).readFile,
		Params: provider.Schema{
			Type: "object",
			Properties: map[string]provider.Property{
				"path":   provider.StringParam("Path to the file to read"),
				"offset": provider.StringParam("Start line number (1-based, optional)"),
				"limit":  provider.StringParam("Number of lines to read (optional, defaults to entire file)"),
			},
			Required: []string{"path"},
		},
	},
	{
		Name:        "write_file",
		Description: "Create a NEW file. Refuses to run if the path already exists unless overwrite=true is explicitly passed (reserved for deliberate full rewrites). For ANY modification of existing files, use replace_in_file — never write_file.",
		Verb:        "✎ writing",
		Handler:     (*Agent).writeFile,
		Params: provider.Schema{
			Type: "object",
			Properties: map[string]provider.Property{
				"path":      provider.StringParam("Path to the new file"),
				"content":   provider.StringParam("Full content for the new file"),
				"overwrite": provider.StringParam("Set to \"true\" ONLY when deliberately replacing an existing file wholesale. Defaults to false; any modification should go through replace_in_file instead."),
			},
			Required: []string{"path", "content"},
		},
	},
	{
		Name:        "replace_in_file",
		Description: "Replace a unique substring in an existing file. You MUST call read_file on this path earlier in the session before calling replace_in_file — the tool will refuse otherwise. If the file changes on disk after your read, you must re-read it. old_text must match byte-for-byte — whitespace, punctuation, capitalization, and line breaks all count. Copy old_text directly from the read_file output. If a short phrase appears multiple times, expand old_text with surrounding context until it is unique.",
		Verb:        "✎ editing",
		Handler:     (*Agent).replaceInFile,
		Params: provider.Schema{
			Type: "object",
			Properties: map[string]provider.Property{
				"path":     provider.StringParam("Path to the file"),
				"old_text": provider.StringParam("Text to find and replace (must be unique in the file)"),
				"new_text": provider.StringParam("Replacement text"),
			},
			Required: []string{"path", "old_text", "new_text"},
		},
	},
	{
		Name:        "rename_file",
		Description: "Move or rename a file",
		Verb:        "⇄ moving",
		Handler:     (*Agent).renameFile,
		Params: provider.Schema{
			Type: "object",
			Properties: map[string]provider.Property{
				"old_path": provider.StringParam("Current file path"),
				"new_path": provider.StringParam("New file path"),
			},
			Required: []string{"old_path", "new_path"},
		},
	},
	{
		Name:        "delete_file",
		Description: "Delete a file. Supports undo via /undo.",
		Verb:        "✕ deleting",
		Handler:     (*Agent).deleteFile,
		Params: provider.Schema{
			Type: "object",
			Properties: map[string]provider.Property{
				"path": provider.StringParam("Path to the file to delete"),
			},
			Required: []string{"path"},
		},
	},
	{
		Name:        "list_dir",
		Description: "List the contents of a directory. Use recursive=true for a tree view.",
		Verb:        "≡ listing",
		ReadOnly:    true,
		Handler:     (*Agent).listDir,
		Params: provider.Schema{
			Type: "object",
			Properties: map[string]provider.Property{
				"path":      provider.StringParam("Path to the directory"),
				"recursive": provider.StringParam("Set to 'true' for recursive tree view"),
				"depth":     provider.StringParam("Max depth for recursive listing (1-10, default 3)"),
			},
			Required: []string{"path"},
		},
	},
	{
		Name:        "search_files",
		Description: "Search for a text pattern in files recursively. Returns matches with surrounding context lines. Use glob to filter by file type.",
		Verb:        "⌕ searching",
		ReadOnly:    true,
		Quiet:       true,
		Handler:     (*Agent).searchFiles,
		Params: provider.Schema{
			Type: "object",
			Properties: map[string]provider.Property{
				"pattern":       provider.StringParam("Text pattern to search for"),
				"path":          provider.StringParam("Directory to search in (defaults to sandbox root)"),
				"context_lines": provider.StringParam("Number of context lines before/after each match (0-10, default 2)"),
				"glob":          provider.StringParam("File name glob filter (e.g. \"*.go\", \"*.ts\", \"Makefile\")"),
			},
			Required: []string{"pattern"},
		},
	},
	{
		Name:        "exec_command",
		Description: "Execute a shell command and return its output. Runs in the sandbox root directory. Requires user approval.",
		Verb:        "⚡ running",
		Quiet:       true,
		Handler:     (*Agent).execCmd,
		Params: provider.Schema{
			Type: "object",
			Properties: map[string]provider.Property{
				"command": provider.StringParam("The shell command to execute"),
			},
			Required: []string{"command"},
		},
	},
	{
		Name:        "save_memory",
		Description: "Save a persistent memory for future sessions. Types: user (preferences, role), feedback (corrections, validated approaches), project (ongoing work context), reference (pointers to external systems).",
		Verb:        "◆ remembering",
		Handler:     (*Agent).saveMemory,
		Params: provider.Schema{
			Type: "object",
			Properties: map[string]provider.Property{
				"name":        provider.StringParam("Short unique name for the memory"),
				"description": provider.StringParam("One-line summary used when deciding relevance later"),
				"type":        provider.StringParam("One of: user, feedback, project, reference"),
				"body":        provider.StringParam("The memory content"),
			},
			Required: []string{"name", "type", "body"},
		},
	},
	{
		Name:        "delete_memory",
		Description: "Remove a stored memory by name.",
		Verb:        "◆ forgetting",
		Handler:     (*Agent).deleteMemory,
		Params: provider.Schema{
			Type: "object",
			Properties: map[string]provider.Property{
				"name": provider.StringParam("Name of the memory to delete"),
			},
			Required: []string{"name"},
		},
	},
	{
		Name:        "list_memories",
		Description: "List all stored memories with their types, descriptions, and bodies.",
		Verb:        "◆ recalling",
		ReadOnly:    true,
		Handler:     (*Agent).listMemories,
		Params:      provider.Schema{Type: "object"},
	},
	{
		Name:        "exit_plan_mode",
		Description: "Present a plan to the user for approval and exit plan mode. Only callable while plan mode is active. Call this once investigation is done and you have a concrete plan to propose.",
		ReadOnly:    true,
		Handler:     (*Agent).exitPlanMode,
		Params: provider.Schema{
			Type: "object",
			Properties: map[string]provider.Property{
				"plan": provider.StringParam("The plan as markdown — steps, files to change, high-level approach."),
			},
			Required: []string{"plan"},
		},
	},
}

var toolsByName = func() map[string]*tool {
	m := make(map[string]*tool, len(tools))
	for i := range tools {
		m[tools[i].Name] = &tools[i]
	}
	return m
}()

func toolVerb(name string) string {
	if t, ok := toolsByName[name]; ok && t.Verb != "" {
		return t.Verb
	}
	return "working"
}

func toolReadOnly(name string) bool {
	t, ok := toolsByName[name]
	return ok && t.ReadOnly
}

func toolQuiet(name string) bool {
	t, ok := toolsByName[name]
	return ok && t.Quiet
}

func (a *Agent) buildTools() []provider.ToolDef {
	out := make([]provider.ToolDef, len(tools))
	for i, t := range tools {
		out[i] = provider.ToolDef{
			Name:        t.Name,
			Description: t.Description,
			Params:      t.Params,
		}
	}
	return out
}
