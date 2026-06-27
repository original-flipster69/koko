package pushpuppet

import (
	"context"
	"log/slog"
	"slices"

	"github.com/original-flipster69/koko/internal/provider"
)

type tool struct {
	Name        string
	Description string
	Params      provider.Schema
	Verb        string
	ReadOnly    bool
	Quiet       bool
	Handler     func(*PushPuppet, context.Context, provider.ToolCall) string
}

func ptr[T any](v T) *T {
	return &v
}

var tools = []tool{
	{
		Name:        "read_file",
		Description: "Read the contents of a file. Returns numbered lines. Use offset and limit to read specific sections of large files.",
		Verb:        "◇ reading",
		ReadOnly:    true,
		Quiet:       true,
		Handler:     (*PushPuppet).readFile,
		Params: provider.Schema{
			Type: "object",
			Properties: map[string]provider.Property{
				"path":   provider.StringParam("Path to the file to read"),
				"offset": provider.IntParam("Start line number (1-based, optional)", ptr(1), nil, nil),
				"limit":  provider.IntParam("Number of lines to read (optional, defaults to entire file)", ptr(1), nil, nil),
			},
			Required: []string{"path"},
		},
	},
	{
		Name:        "write_file",
		Description: "Create a NEW file. Refuses to run if the path already exists unless overwrite=true is explicitly passed (reserved for deliberate full rewrites). For ANY modification of existing files, use replace_in_file — never write_file.",
		Verb:        "✎ writing",
		Handler:     (*PushPuppet).writeFile,
		Params: provider.Schema{
			Type: "object",
			Properties: map[string]provider.Property{
				"path":      provider.StringParam("Path to the new file"),
				"content":   provider.StringParam("Full content for the new file"),
				"overwrite": provider.BoolParam("Set to `true` ONLY when deliberately replacing an existing file wholesale. Any modification should go through `replace_in_file` instead.", ptr(false)),
			},
			Required: []string{"path", "content"},
		},
	},
	{
		Name:        "replace_in_file",
		Description: "Replace a unique substring in an existing file. You MUST call read_file on this path earlier in the session before calling replace_in_file — the tool will refuse otherwise. If the file changes on disk after your read, you must re-read it. old_text must match byte-for-byte — whitespace, punctuation, capitalization, and line breaks all count. Copy old_text directly from the read_file output. If a short phrase appears multiple times, expand old_text with surrounding context until it is unique.",
		Verb:        "✎ editing",
		Handler:     (*PushPuppet).replaceInFile,
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
		Handler:     (*PushPuppet).renameFile,
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
		Handler:     (*PushPuppet).deleteFile,
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
		Handler:     (*PushPuppet).listDir,
		Params: provider.Schema{
			Type: "object",
			Properties: map[string]provider.Property{
				"path":      provider.StringParam("Path to the directory"),
				"recursive": provider.BoolParam("Set to `true` for recursive tree view", nil),
				"depth":     provider.IntParam("Max depth for recursive listing (1-10, default 3)", ptr(1), ptr(10), ptr(3)),
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
		Handler:     (*PushPuppet).searchFiles,
		Params: provider.Schema{
			Type: "object",
			Properties: map[string]provider.Property{
				"pattern":       provider.StringParam("Text pattern to search for"),
				"path":          provider.StringParam("Directory to search in (defaults to sandbox root)"),
				"context_lines": provider.IntParam("Number of context lines before/after each match", ptr(0), ptr(10), ptr(2)),
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
		Handler:     (*PushPuppet).execCmd,
		Params: provider.Schema{
			Type: "object",
			Properties: map[string]provider.Property{
				"command": provider.StringParam("The shell command to execute"),
			},
			Required: []string{"command"},
		},
	},
	{
		Name:        "verify",
		Description: "Run the project's verification pipeline (build, lint, tests, etc. as configured) and return the structured result. Use this to check your work — prefer it over running build/test/vet/lint commands yourself via exec_command. Takes no arguments. Note: koko already runs the fast checks automatically after every edit; call this when you want the full pipeline (including slower stages such as tests).",
		Verb:        "✓ verifying",
		Handler:     (*PushPuppet).verifyCmd,
		Params: provider.Schema{
			Type:       "object",
			Properties: map[string]provider.Property{},
		},
	},
	{
		Name:        "save_memory",
		Description: "Save a persistent memories for future sessions. Types: user (preferences, role), feedback (corrections, validated approaches), project (ongoing work context), reference (pointers to external systems).",
		Verb:        "◆ remembering",
		Handler:     (*PushPuppet).saveMemory,
		Params: provider.Schema{
			Type: "object",
			Properties: map[string]provider.Property{
				"name":        provider.StringParam("Short unique name for the memories"),
				"description": provider.StringParam("One-line summary used when deciding relevance later"),
				"type":        provider.StringParam("One of: user, feedback, project, reference"),
				"body":        provider.StringParam("The memories content"),
			},
			Required: []string{"name", "type", "body"},
		},
	},
	{
		Name:        "delete_memory",
		Description: "Remove a stored memories by name.",
		Verb:        "◆ forgetting",
		Handler:     (*PushPuppet).deleteMemory,
		Params: provider.Schema{
			Type: "object",
			Properties: map[string]provider.Property{
				"name": provider.StringParam("Name of the memories to delete"),
			},
			Required: []string{"name"},
		},
	},
	{
		Name:        "list_memories",
		Description: "List all stored memories with their types, descriptions, and bodies.",
		Verb:        "◆ recalling",
		ReadOnly:    true,
		Handler:     (*PushPuppet).listMemories,
		Params:      provider.Schema{Type: "object"},
	},
	{
		Name:        "exit_plan_mode",
		Description: "Present a plan to the user for approval and exit plan mode. Only callable while plan mode is active. Call this once investigation is done and you have a concrete plan to propose.",
		ReadOnly:    true,
		Handler:     (*PushPuppet).exitPlanMode,
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

var fileEditingTools = map[string]bool{
	"write_file":      true,
	"replace_in_file": true,
	"rename_file":     true,
	"delete_file":     true,
}

func toolEditsFiles(name string) bool {
	return fileEditingTools[name]
}

func toolQuiet(name string) bool {
	t, ok := toolsByName[name]
	return ok && t.Quiet
}

var excluded = []string{"save_memory", "delete_memory", "list_memories"}

func (p *PushPuppet) buildTools() []provider.ToolDef {
	out := make([]provider.ToolDef, 0, len(tools))
	for _, t := range tools {
		if slices.Contains(excluded, t.Name) {
			slog.Info("tool ignored", "tool", t.Name)
			continue
		}
		if t.Name == "verify" && p.verifier == nil {
			continue
		}
		out = append(out, provider.ToolDef{
			Name:        t.Name,
			Description: t.Description,
			Params:      t.Params,
		})
	}
	return out
}

var toolSymbols = map[string]string{
	"read_file":       "◇",
	"write_file":      "✎",
	"replace_in_file": "✎",
	"delete_file":     "✕",
	"rename_file":     "⇄",
	"list_dir":        "≡",
	"search_files":    "⌕",
	"exec_command":    "⚡",
	"verify":          "✓",
	"save_memory":     "◆",
	"delete_memory":   "◆",
	"list_memories":   "◆",
}
