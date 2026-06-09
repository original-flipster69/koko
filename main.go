package main

import (
	"bufio"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/original-flipster69/koko/internal/agent"
	"github.com/original-flipster69/koko/internal/audit"
	"github.com/original-flipster69/koko/internal/config"
	"github.com/original-flipster69/koko/internal/ignore"
	"github.com/original-flipster69/koko/internal/memory"
	"github.com/original-flipster69/koko/internal/plays"
	"github.com/original-flipster69/koko/internal/policy"
	"github.com/original-flipster69/koko/internal/project"
	"github.com/original-flipster69/koko/internal/provider"
	"github.com/original-flipster69/koko/internal/sandbox"
	"github.com/original-flipster69/koko/internal/terminal"
	"github.com/original-flipster69/koko/internal/ui"
)

var version = "dev"

const llmStreamTimeout = 5 * time.Minute

func main() {
	providerFlag := flag.String("provider", "", "LLM provider: anthropic, mistral, ollama")
	modelFlag := flag.String("model", "", "Model name to use")
	llmUrlFlag := flag.String("llm-url", "", "URL for LLM API (useful for local LLMs)")
	sandboxFlag := flag.String("sandbox", "", "Sandbox root directory (defaults to cwd)")
	configFlag := flag.String("config", "", "Config file path")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println(version)
		return
	}

	kokoDir := getKokoDir()

	cfgPath := config.Path(kokoDir)
	if *configFlag != "" {
		cfgPath = *configFlag
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.DefaultScheme().Error(err.Error()))
		os.Exit(1)
	}

	cfg.ApplyFlags(*providerFlag, *modelFlag, *llmUrlFlag, *sandboxFlag)
	cfg.ApplyEnv()

	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, ui.DefaultScheme().Error(err.Error()))
		os.Exit(1)
	}

	scheme, err := ui.DefaultScheme().With(cfg.Style.ColorScheme)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.DefaultScheme().Error(err.Error()))
		os.Exit(1)
	}

	llm, err := provider.New(&cfg.Llm)
	if err != nil {
		fmt.Fprintln(os.Stderr, scheme.Error(err.Error()))
		os.Exit(1)
	}

	sb, err := sandbox.New(cfg.Sandbox.Root, cfg.Sandbox.AllowedDirs(), cfg.Sandbox.DenyFiles, cfg.Sandbox.MaxFileSize)
	if err != nil {
		fmt.Fprintln(os.Stderr, scheme.Error(err.Error()))
		os.Exit(1)
	}
	auditLog, err := audit.NewLog(filepath.Join(kokoDir, "audit.jsonl"))
	if err != nil {
		fmt.Fprintln(os.Stderr, scheme.Error(fmt.Sprintf("cannot open audit log: %v", err)))
		os.Exit(1)
	}
	defer auditLog.Close()

	logFile, err := os.OpenFile(filepath.Join(kokoDir, "koko.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err == nil {
		slog.SetDefault(slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{Level: slog.LevelInfo})))
		defer logFile.Close()
	}
	slog.Info("session started", "provider", llm.Name(), "model", cfg.Llm.Model, "sandbox", cfg.Sandbox.Root)

	stack := project.Scan(cfg.Sandbox.Root)
	playsDir := filepath.Join(kokoDir, "plays")
	playRegistry, err := plays.Load(playsDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, scheme.Error(fmt.Sprintf("cannot load plays: %v", err)))
		playRegistry, _ = plays.Load("")
	}
	memoryStore, err := memory.Open(filepath.Join(kokoDir, "memory"))
	if err != nil {
		fmt.Fprintln(os.Stderr, scheme.Error(fmt.Sprintf("cannot open memory store: %v", err)))
		os.Exit(1)
	}
	extraContext := stack.Summary()
	if idx := playRegistry.Index(); idx != "" {
		if extraContext != "" {
			extraContext += "\n\n"
		}
		extraContext += idx
	}
	if idx := memoryStore.Index(); idx != "" {
		if extraContext != "" {
			extraContext += "\n\n"
		}
		extraContext += "Stored memories (use list_memories to read bodies, save_memory/delete_memory to modify):\n" + idx
	}
	cmdPolicy, err := policy.NewCommandPolicy(cfg.Sandbox.Exec.Allow, cfg.Sandbox.Exec.Deny)
	if err != nil {
		fmt.Fprintln(os.Stderr, scheme.Error(fmt.Sprintf("command policy: %v", err)))
		os.Exit(1)
	}

	confirm := func(action string) bool {
		fmt.Printf("  %s%srun:%s %s%s%s  [y/N] ", ui.Bold, scheme.Secondary, ui.Reset, scheme.Label, action, ui.Reset)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		return answer == "y" || answer == "yes"
	}
	cpuSec, memMB, fileMB := cfg.Sandbox.Exec.Limits()
	var ignoreMatcher *ignore.Matcher
	if cfg.Ignore.Mode == config.Custom {
		ignoreMatcher = ignore.NewFromPatterns(cfg.Ignore.Files)
	} else {
		ignoreMatcher = ignore.LoadGitignore(cfg.Sandbox.Root)
	}
	var outboundFilters []agent.OutboundFilter
	if cfg.Sandbox.ScrubPII {
		outboundFilters = append(outboundFilters, agent.ScrubPIIFilter)
	}
	a := agent.New(llm, sb, os.Stdout, confirm, auditLog, agent.Options{
		Memory:           memoryStore,
		CmdPolicy:        cmdPolicy,
		Ignore:           ignoreMatcher,
		Scheme:           scheme,
		ProjectCtx:       extraContext,
		ThinkingVerbs:    cfg.Style.ThinkingVerbs,
		MaxSessionTokens: cfg.Llm.MaxSessionTokens,
		StreamTimeout:    llmStreamTimeout,
		OutboundFilters:  outboundFilters,
		ExecCPUSeconds:   cpuSec,
		ExecMemoryMB:     memMB,
		ExecMaxFileMB:    fileMB,
	})

	mascotFrames := ui.MascotFrames(scheme)
	splashes := make([]string, len(mascotFrames))
	for i, m := range mascotFrames {
		splashes[i] = "\n" + scheme.Splashscreen(m, llm.Name(), cfg.Llm.Model, cfg.Sandbox.Root, version, stack.Detected) + "\n\n"
	}

	cmdHandlers := cmdHandler(cfg, llm, kokoDir, cfg.Sandbox.Root, playRegistry, scheme)

	if err := terminal.Run(a, llm.Name(), kokoDir, splashes, cmdHandlers, scheme); err != nil {
		fmt.Fprintln(os.Stderr, scheme.Error(err.Error()))
		os.Exit(1)
	}
	fmt.Println(ui.Goodbye(scheme))
}

type command struct {
	desc string
	args string
	fn   func(input string, parts []string, a *agent.Agent) (handled bool, prompt string, output string)
}

func cmdHandler(cfg *config.Config, llm provider.Provider, dataDir string, sandboxRoot string, playRegistry *plays.Registry, scheme ui.Scheme) terminal.CmdHandler {
	var commands map[string]command
	commands = map[string]command{
		":koko": {desc: "print the koko mascot", fn: func(string, []string, *agent.Agent) (bool, string, string) {
			return true, "", "\n" + ui.Mascot(scheme)
		}},
		":clear": {desc: "reset conversation history", fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
			a.ClearHistory()
			return true, "", scheme.Info("cleared", "conversation history reset")
		}},
		":history": {desc: "show message count", fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
			return true, "", scheme.Info("messages", fmt.Sprintf("%d", a.HistoryLen()))
		}},
		":undo": {desc: "revert last file change", fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
			path, err := a.Undo()
			switch {
			case err != nil:
				return true, "", scheme.Error(fmt.Sprintf("undo failed: %v", err))
			case path == "":
				return true, "", scheme.Info("undo", "nothing to undo")
			default:
				return true, "", scheme.Info("undo", fmt.Sprintf("reverted %s", path))
			}
		}},
		":tokens": {desc: "show token usage stats", fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
			var b strings.Builder
			b.WriteString(scheme.Info("input   ", fmt.Sprintf("%d tokens", a.TotalInput)) + "\n")
			b.WriteString(scheme.Info("output  ", fmt.Sprintf("%d tokens", a.TotalOutput)) + "\n")
			b.WriteString(scheme.Info("total   ", fmt.Sprintf("%d tokens", a.TotalInput+a.TotalOutput)) + "\n")
			b.WriteString(scheme.Info("messages", fmt.Sprintf("%d", a.HistoryLen())))
			return true, "", b.String()
		}},
		":run": {desc: "run a shell command directly", args: "<cmd>", fn: func(input string, parts []string, _ *agent.Agent) (bool, string, string) {
			if len(parts) < 2 {
				return true, "", scheme.Error("usage: :run <command>")
			}
			cmdStr := strings.TrimPrefix(input, ":run ")
			runCmd := exec.Command("sh", "-c", cmdStr)
			runCmd.Dir = sandboxRoot
			result, err := runCmd.CombinedOutput()
			text := strings.TrimRight(string(result), "\n")
			if err != nil {
				return true, "", scheme.Error(text)
			}
			return true, "", text
		}},
		":compact": {desc: "compress history to free context", fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
			oldTokens, newTokens := a.Compact()
			return true, "", scheme.Info("compact", fmt.Sprintf("~%d → ~%d tokens", oldTokens, newTokens))
		}},
		":model": {desc: "show or switch model", args: "[name]", fn: func(_ string, parts []string, _ *agent.Agent) (bool, string, string) {
			if len(parts) < 2 {
				return true, "", scheme.Info("model", llm.Model())
			}
			llm.SetModel(parts[1])
			return true, "", scheme.Info("model", fmt.Sprintf("switched to %s", parts[1]))
		}},
		":config": {desc: "show active configuration", fn: func(string, []string, *agent.Agent) (bool, string, string) {
			var b strings.Builder
			b.WriteString(scheme.Info("provider", string(cfg.Llm.Provider)) + "\n")
			b.WriteString(scheme.Info("model", cfg.Llm.Model) + "\n")
			b.WriteString(scheme.Info("sandbox", cfg.Sandbox.Root) + "\n")
			b.WriteString(scheme.Info("max_tok", fmt.Sprintf("%d", cfg.Llm.MaxTokens)) + "\n")
			b.WriteString(scheme.Info("session", fmt.Sprintf("%d max tokens", cfg.Llm.MaxSessionTokens)) + "\n")
			cpuSec, memMB, fileMB := cfg.Sandbox.Exec.Limits()
			b.WriteString(scheme.Info("exec", fmt.Sprintf("%s (%ds cpu, %dMB mem, %dMB file)", cfg.Sandbox.Exec.Profile, cpuSec, memMB, fileMB)) + "\n")
			b.WriteString(scheme.Info("scrub_pii", fmt.Sprintf("%v", cfg.Sandbox.ScrubPII)) + "\n")
			b.WriteString(scheme.Info("verbs", strings.Join(cfg.Style.ThinkingVerbs, ", ")) + "\n")
			b.WriteString(scheme.Info("config", config.Path(dataDir)))
			return true, "", b.String()
		}},
		":save": {desc: "save session to disk", fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
			if err := a.SaveSession(dataDir); err != nil {
				return true, "", scheme.Error(fmt.Sprintf("save failed: %v", err))
			}
			return true, "", scheme.Info("saved", "session written to disk")
		}},
		":resume": {desc: "restore saved session", fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
			if err := a.LoadSession(dataDir); err != nil {
				return true, "", scheme.Error(fmt.Sprintf("resume failed: %v", err))
			}
			return true, "", scheme.Info("resumed", fmt.Sprintf("loaded %d messages", a.HistoryLen()))
		}},
		":plays": {desc: "list installed plays", fn: func(string, []string, *agent.Agent) (bool, string, string) {
			list := playRegistry.List()
			if len(list) == 0 {
				return true, "", scheme.Info("plays", fmt.Sprintf("none installed — add *.md files in %s", playRegistry.Dir()))
			}
			var b strings.Builder
			for _, p := range list {
				desc := p.Description
				if desc == "" {
					desc = "(no description)"
				}
				b.WriteString(scheme.Info(p.Name, desc) + "\n")
			}
			return true, "", b.String()
		}},
		":plan": {desc: "toggle plan mode (read-only)", fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
			if a.TogglePlanMode() {
				return true, "", scheme.Info("plan", "mode on — read-only; call :plan again to exit")
			}
			return true, "", scheme.Info("plan", "mode off — full tools restored")
		}},
		":help": {desc: "show this help", fn: func(string, []string, *agent.Agent) (bool, string, string) {
			names := make([]string, 0, len(commands))
			for n := range commands {
				names = append(names, n)
			}
			sort.Strings(names)
			var b strings.Builder
			for _, n := range names {
				display := n
				if commands[n].args != "" {
					display = n + " " + commands[n].args
				}
				b.WriteString(fmt.Sprintf("%-14s— %s\n", display, commands[n].desc))
			}
			b.WriteString(fmt.Sprintf("%-14s— %s", ":<name>", "run a play by name (e.g. :review)"))
			return true, "", b.String()
		}},
	}

	return func(input string, a *agent.Agent) (bool, string, string) {
		parts := strings.Fields(input)
		if len(parts) == 0 {
			return true, "", ""
		}
		name := parts[0]
		if c, ok := commands[name]; ok {
			return c.fn(input, parts, a)
		}
		playName := strings.TrimPrefix(name, ":")
		if p, ok := playRegistry.Get(playName); ok {
			extra := strings.TrimSpace(strings.TrimPrefix(input, name))
			prompt := fmt.Sprintf("Run the '%s' play:\n\n%s", p.Name, p.Render(extra))
			return false, prompt, ""
		}
		return true, "", scheme.Error(fmt.Sprintf("unknown command: %s (try :help)", name))
	}
}

func getKokoDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.DefaultScheme().Error(fmt.Sprintf("cannot determine home directory: %v", err)))
		os.Exit(1)
	}
	kokoDir := filepath.Join(home, ".koko")
	if err := os.MkdirAll(kokoDir, 0750); err != nil {
		fmt.Fprintln(os.Stderr, ui.DefaultScheme().Error(fmt.Sprintf("cannot create data directory: %v", err)))
		os.Exit(1)
	}
	return kokoDir
}
