package agent

var toolVerbs = map[string]string{
	"read_file":       "◇ reading",
	"write_file":      "✎ writing",
	"replace_in_file": "✎ editing",
	"delete_file":     "✕ deleting",
	"rename_file":     "⇄ moving",
	"list_dir":        "≡ listing",
	"search_files":    "⌕ searching",
	"exec_command":    "⚡ running",
	"save_memory":     "◆ remembering",
	"delete_memory":   "◆ forgetting",
	"list_memories":   "◆ recalling",
}

func toolVerb(name string) string {
	if v, ok := toolVerbs[name]; ok {
		return v
	}
	return "working"
}

var readOnlyTools = map[string]bool{
	"read_file":      true,
	"list_dir":       true,
	"search_files":   true,
	"list_memories":  true,
	"exit_plan_mode": true,
}

var quietTools = map[string]bool{
	"read_file":    true,
	"search_files": true,
	"exec_command": true,
}
