package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/original-flipster69/koko/internal/agent"
	"github.com/original-flipster69/koko/internal/audit"
	"github.com/original-flipster69/koko/internal/cage"
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
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}

	cfg.ApplyFlags(*providerFlag, *modelFlag, *llmUrlFlag, *sandboxFlag)
	cfg.ApplyEnv()

	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}

	if !cfg.Sandbox.SuppressElevatedWarning && isElevated() && !confirmElevated(os.Stdin, os.Stdout) {
		fmt.Println(ui.Info("aborted", "not starting with elevated privileges"))
		os.Exit(1)
	}

	llm, err := provider.New(&cfg.Llm)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}

	sb, err := sandbox.New(cfg.Sandbox.Root, cfg.Sandbox.AllowedDirs(), cfg.Sandbox.DenyFiles, cfg.Sandbox.MaxFileSize)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}
	auditLog, err := audit.NewLog(filepath.Join(kokoDir, "audit.jsonl"))
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("cannot open audit log: %v", err)))
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
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("cannot load plays: %v", err)))
		playRegistry, _ = plays.Load("")
	}
	memoryStore, err := memory.Open(filepath.Join(kokoDir, "memory"))
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("cannot open memory store: %v", err)))
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
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("command policy: %v", err)))
		os.Exit(1)
	}

	confirm := func(action string) bool {
		fmt.Printf("  %s%srun:%s %s%s%s  [y/N] ", ui.Bold, ui.LavenderIndigo, ui.Reset, ui.BrightLavender, action, ui.Reset)
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
		ProjectCtx:       extraContext,
		ThinkingVerbs:    cfg.Style.ThinkingVerbs,
		MaxSessionTokens: cfg.Llm.MaxSessionTokens,
		StreamTimeout:    llmStreamTimeout,
		OutboundFilters:  outboundFilters,
		ExecCPUSeconds:   cpuSec,
		ExecMemoryMB:     memMB,
		ExecMaxFileMB:    fileMB,
	})

	mascotFrames := ui.MascotFrames()
	splashes := make([]string, len(mascotFrames))
	for i, m := range mascotFrames {
		splash := "\n" + ui.Splash(m, llm.Name(), cfg.Llm.Model, cfg.Sandbox.Root, version, stack.Detected) + "\n\n"
		if llm.Name() == "ollama" {
			splash += ui.Dim + ui.Gray + "  note: tool support depends on model (llama3.1+, mistral, command-r)" + ui.Reset + "\n\n"
		}
		if warning := ui.PrivacyWarning(llm.Name()); warning != "" {
			splash += warning + "\n\n"
		}
		splashes[i] = splash
	}

	cmdHandlers := cmdHandler(cfg, llm, kokoDir, cfg.Sandbox.Root, playRegistry)

	if err := terminal.Run(a, kokoDir, splashes, cmdHandlers); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}
	fmt.Println(ui.Goodbye())
}

type command struct {
	desc string
	args string
	fn   func(input string, parts []string, a *agent.Agent) (handled bool, prompt string, output string)
}

func cmdHandler(cfg *config.Config, llm provider.Provider, dataDir string, sandboxRoot string, playRegistry *plays.Registry) terminal.CmdHandler {
	var commands map[string]command
	commands = map[string]command{
		":koko": {desc: "print the koko mascot", fn: func(string, []string, *agent.Agent) (bool, string, string) {
			return true, "", "\n" + ui.Mascot()
		}},
		":clear": {desc: "reset conversation history", fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
			a.ClearHistory()
			return true, "", ui.Info("cleared", "conversation history reset")
		}},
		":history": {desc: "show message count", fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
			return true, "", ui.Info("messages", fmt.Sprintf("%d", a.HistoryLen()))
		}},
		":undo": {desc: "revert last file change", fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
			path, err := a.Undo()
			switch {
			case err != nil:
				return true, "", ui.Error(fmt.Sprintf("undo failed: %v", err))
			case path == "":
				return true, "", ui.Info("undo", "nothing to undo")
			default:
				return true, "", ui.Info("undo", fmt.Sprintf("reverted %s", path))
			}
		}},
		":tokens": {desc: "show token usage stats", fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
			var b strings.Builder
			b.WriteString(ui.Info("input   ", fmt.Sprintf("%d tokens", a.TotalInput)) + "\n")
			b.WriteString(ui.Info("output  ", fmt.Sprintf("%d tokens", a.TotalOutput)) + "\n")
			b.WriteString(ui.Info("total   ", fmt.Sprintf("%d tokens", a.TotalInput+a.TotalOutput)) + "\n")
			b.WriteString(ui.Info("messages", fmt.Sprintf("%d", a.HistoryLen())))
			return true, "", b.String()
		}},
		":run": {desc: "run a shell command directly", args: "<cmd>", fn: func(input string, parts []string, _ *agent.Agent) (bool, string, string) {
			if len(parts) < 2 {
				return true, "", ui.Error("usage: :run <command>")
			}
			cmdStr := strings.TrimPrefix(input, ":run ")
			runCmd := exec.Command("sh", "-c", cmdStr)
			runCmd.Dir = sandboxRoot
			result, err := runCmd.CombinedOutput()
			text := strings.TrimRight(string(result), "\n")
			if err != nil {
				return true, "", ui.Error(text)
			}
			return true, "", text
		}},
		":compact": {desc: "compress history to free context", fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
			oldTokens, newTokens := a.Compact()
			return true, "", ui.Info("compact", fmt.Sprintf("~%d → ~%d tokens", oldTokens, newTokens))
		}},
		":cage": {desc: "generate a low-privilege user setup script", args: "<username> [dir=…] [group=…] [os=darwin|linux]", fn: func(_ string, parts []string, _ *agent.Agent) (bool, string, string) {
			if len(parts) < 2 {
				return true, "", ui.Error("usage: :cage <username> [dir=PATH] [group=NAME] [os=darwin|linux]")
			}
			opts := cage.Options{Username: parts[1], GOOS: runtime.GOOS}
			outDir := dataDir
			for _, tok := range parts[2:] {
				k, v, ok := strings.Cut(tok, "=")
				if !ok || v == "" {
					return true, "", ui.Error(fmt.Sprintf("invalid option %q (use key=value)", tok))
				}
				switch k {
				case "dir":
					outDir = v
				case "group":
					opts.Group = v
				case "os":
					opts.GOOS = v
				default:
					return true, "", ui.Error(fmt.Sprintf("unknown option %q (allowed: dir, group, os)", k))
				}
			}
			if !filepath.IsAbs(outDir) {
				outDir = filepath.Join(sandboxRoot, outDir)
			}
			script, err := cage.Generate(opts)
			if err != nil {
				return true, "", ui.Error(err.Error())
			}
			if err := os.MkdirAll(outDir, 0o700); err != nil {
				return true, "", ui.Error(fmt.Sprintf("cannot create output dir: %v", err))
			}
			dest := filepath.Join(outDir, script.Filename)
			if err := os.WriteFile(dest, []byte(script.Body), 0o700); err != nil {
				return true, "", ui.Error(fmt.Sprintf("cannot write cage script: %v", err))
			}
			var b strings.Builder
			b.WriteString(ui.Info("cage", fmt.Sprintf("setup script for user %q (group %q, %s)", script.Username, script.Group, opts.GOOS)) + "\n")
			b.WriteString(ui.Info("path", dest) + "\n")
			b.WriteString(ui.Info("note", "a random password was generated inside — change it there before running") + "\n")
			b.WriteString(ui.Info("run", fmt.Sprintf("review it, then: sudo sh %s", dest)))
			return true, "", b.String()
		}},
		":model": {desc: "show or switch model", args: "[name]", fn: func(_ string, parts []string, _ *agent.Agent) (bool, string, string) {
			if len(parts) < 2 {
				return true, "", ui.Info("model", llm.Model())
			}
			llm.SetModel(parts[1])
			return true, "", ui.Info("model", fmt.Sprintf("switched to %s", parts[1]))
		}},
		":config": {desc: "show active configuration", fn: func(string, []string, *agent.Agent) (bool, string, string) {
			var b strings.Builder
			b.WriteString(ui.Info("provider", string(cfg.Llm.Provider)) + "\n")
			b.WriteString(ui.Info("model", cfg.Llm.Model) + "\n")
			b.WriteString(ui.Info("sandbox", cfg.Sandbox.Root) + "\n")
			b.WriteString(ui.Info("max_tok", fmt.Sprintf("%d", cfg.Llm.MaxTokens)) + "\n")
			b.WriteString(ui.Info("session", fmt.Sprintf("%d max tokens", cfg.Llm.MaxSessionTokens)) + "\n")
			cpuSec, memMB, fileMB := cfg.Sandbox.Exec.Limits()
			b.WriteString(ui.Info("exec", fmt.Sprintf("%s (%ds cpu, %dMB mem, %dMB file)", cfg.Sandbox.Exec.Profile, cpuSec, memMB, fileMB)) + "\n")
			b.WriteString(ui.Info("scrub_pii", fmt.Sprintf("%v", cfg.Sandbox.ScrubPII)) + "\n")
			b.WriteString(ui.Info("verbs", strings.Join(cfg.Style.ThinkingVerbs, ", ")) + "\n")
			b.WriteString(ui.Info("config", config.Path(dataDir)))
			return true, "", b.String()
		}},
		":save": {desc: "save session to disk", fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
			if err := a.SaveSession(dataDir); err != nil {
				return true, "", ui.Error(fmt.Sprintf("save failed: %v", err))
			}
			return true, "", ui.Info("saved", "session written to disk")
		}},
		":resume": {desc: "restore saved session", fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
			if err := a.LoadSession(dataDir); err != nil {
				return true, "", ui.Error(fmt.Sprintf("resume failed: %v", err))
			}
			return true, "", ui.Info("resumed", fmt.Sprintf("loaded %d messages", a.HistoryLen()))
		}},
		":plays": {desc: "list installed plays", fn: func(string, []string, *agent.Agent) (bool, string, string) {
			list := playRegistry.List()
			if len(list) == 0 {
				return true, "", ui.Info("plays", fmt.Sprintf("none installed — add *.md files in %s", playRegistry.Dir()))
			}
			var b strings.Builder
			for _, p := range list {
				desc := p.Description
				if desc == "" {
					desc = "(no description)"
				}
				b.WriteString(ui.Info(p.Name, desc) + "\n")
			}
			return true, "", b.String()
		}},
		":plan": {desc: "toggle plan mode (read-only)", fn: func(_ string, _ []string, a *agent.Agent) (bool, string, string) {
			if a.TogglePlanMode() {
				return true, "", ui.Info("plan", "mode on — read-only; call :plan again to exit")
			}
			return true, "", ui.Info("plan", "mode off — full tools restored")
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
		return true, "", ui.Error(fmt.Sprintf("unknown command: %s (try :help)", name))
	}
}

func isElevated() bool {
	return os.Geteuid() == 0
}

func confirmElevated(in io.Reader, out io.Writer) bool {
	fmt.Fprintln(out, ui.Error("running with elevated privileges (root)"))
	fmt.Fprintln(out, "  LLMs are non-deterministic; granting an agent root access is strongly discouraged.")
	fmt.Fprintf(out, "  start koko anyway? [y/N] ")
	answer, _ := bufio.NewReader(in).ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

func getKokoDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("cannot determine home directory: %v", err)))
		os.Exit(1)
	}
	kokoDir := filepath.Join(home, ".koko")
	if err := os.MkdirAll(kokoDir, 0750); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("cannot create data directory: %v", err)))
		os.Exit(1)
	}
	return kokoDir
}
