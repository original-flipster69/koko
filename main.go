package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/meeseeks/koko/agent"
	"github.com/meeseeks/koko/audit"
	"github.com/meeseeks/koko/config"
	"github.com/meeseeks/koko/detect"
	"github.com/meeseeks/koko/memory"
	"github.com/meeseeks/koko/plays"
	"github.com/meeseeks/koko/policy"
	"github.com/meeseeks/koko/provider"
	"github.com/meeseeks/koko/sandbox"
	"github.com/meeseeks/koko/ui"
)

func main() {
	providerFlag := flag.String("provider", "", "LLM provider: anthropic, mistral, ollama")
	modelFlag := flag.String("model", "", "Model name to use")
	baseURLFlag := flag.String("base-url", "", "Base URL for API (useful for local LLMs)")
	sandboxFlag := flag.String("sandbox", "", "Sandbox root directory (defaults to cwd)")
	configFlag := flag.String("config", "", "Config file path")
	promptFlag := flag.String("prompt", "", "Single prompt (non-interactive mode)")
	flag.Parse()

	cfgPath := config.ConfigPath()
	if *configFlag != "" {
		cfgPath = *configFlag
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}

	if *providerFlag != "" {
		cfg.Provider = config.ProviderType(*providerFlag)
	}
	if *modelFlag != "" {
		cfg.Model = *modelFlag
	}
	if *baseURLFlag != "" {
		cfg.BaseURL = *baseURLFlag
	}
	if *sandboxFlag != "" {
		cfg.SandboxRoot = *sandboxFlag
		cfg.AllowedDirs = []string{*sandboxFlag}
	}

	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv(apiKeyEnvVar(cfg.Provider))
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}

	llm, err := provider.New(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}

	sb := sandbox.New(cfg)
	confirm := func(action string) bool {
		fmt.Printf("  %s%srun:%s %s%s%s  [y/N] ", ui.Bold, ui.Purple, ui.Reset, ui.Violet, action, ui.Reset)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		return answer == "y" || answer == "yes"
	}
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
	slog.Info("session started", "provider", llm.Name(), "model", cfg.Model, "sandbox", cfg.SandboxRoot)

	project := detect.Project(cfg.SandboxRoot)
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
	extraContext := project.Summary()
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
	cmdPolicy, err := policy.NewCommandPolicy(cfg.CommandAllowlist, cfg.CommandDenyPatterns)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(fmt.Sprintf("command policy: %v", err)))
		os.Exit(1)
	}
	a := agent.New(llm, sb, os.Stdout, confirm, auditLog, extraContext)
	a.SetThinkingVerbs(cfg.ThinkingVerbs)
	a.SetMemory(memoryStore)
	a.SetCommandPolicy(cmdPolicy)
	a.SetLimits(cfg.MaxToolCalls, cfg.MaxSessionTokens)
	a.SetScrubPII(cfg.ScrubPII)
	a.SetExecLimits(cfg.ExecCPUSeconds, cfg.ExecMemoryMB, cfg.ExecMaxFileMB)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println()
		fmt.Println(ui.Goodbye())
		os.Exit(0)
	}()

	if *promptFlag != "" {
		if err := a.Run(ctx, *promptFlag); err != nil {
			fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
			os.Exit(1)
		}
		return
	}

	fmt.Println()
	fmt.Print(ui.Splash(llm.Name(), cfg.Model, cfg.SandboxRoot, project.Languages, project.BuildTools))
	fmt.Println()
	fmt.Printf("  %stype %sexit%s%s or %sctrl+c%s%s to quit%s\n\n", ui.Gray, ui.Violet, ui.Reset, ui.Gray, ui.Violet, ui.Reset, ui.Gray, ui.Reset)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for {
		fmt.Print(ui.Prompt())
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if input == `"""` {
			var lines []string
			fmt.Print(ui.MultilinePrompt())
			for scanner.Scan() {
				line := scanner.Text()
				if strings.TrimSpace(line) == `"""` {
					break
				}
				lines = append(lines, line)
				fmt.Print(ui.MultilinePrompt())
			}
			input = strings.Join(lines, "\n")
			if strings.TrimSpace(input) == "" {
				continue
			}
		}

		if input == "exit" || input == "quit" {
			fmt.Println(ui.Goodbye())
			break
		}

		if strings.HasPrefix(input, ":") {
			handled, prompt := handleSlashCommand(input, a, llm, kokoDir, cfg.SandboxRoot, playRegistry)
			if handled {
				continue
			}
			if prompt != "" {
				if err := a.Run(ctx, prompt); err != nil {
					fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
					continue
				}
				fmt.Println(ui.TokenStats(a.TotalInput, a.TotalOutput))
				fmt.Println()
				_ = a.SaveSession(kokoDir)
				continue
			}
		}

		if err := a.Run(ctx, input); err != nil {
			fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
			continue
		}
		fmt.Println(ui.TokenStats(a.TotalInput, a.TotalOutput))
		fmt.Println()
		_ = a.SaveSession(kokoDir)
	}
}

func handleSlashCommand(input string, a *agent.Agent, llm provider.Provider, dataDir string, sandboxRoot string, playRegistry *plays.Registry) (bool, string) {
	parts := strings.Fields(input)
	cmd := parts[0]

	switch cmd {
	case ":koko":
		fmt.Println(ui.Mascot())
		fmt.Println()
		return true, ""

	case ":help":
		fmt.Println(ui.Response("  :clear        — reset conversation history"))
		fmt.Println(ui.Response("  :history      — show message count"))
		fmt.Println(ui.Response("  :undo         — revert last file change"))
		fmt.Println(ui.Response("  :run <cmd>    — run a shell command directly"))
		fmt.Println(ui.Response("  :tokens       — show token usage stats"))
		fmt.Println(ui.Response("  :compact      — compress history to free context"))
		fmt.Println(ui.Response("  :model [name] — show or switch model"))
		fmt.Println(ui.Response("  :save         — save session to disk"))
		fmt.Println(ui.Response("  :resume       — restore saved session"))
		fmt.Println(ui.Response("  :plays        — list installed plays"))
		fmt.Println(ui.Response("  :<name>       — run a play by name (e.g. :review)"))
		fmt.Println(ui.Response("  :plan         — toggle plan mode (read-only)"))
		fmt.Println(ui.Response("  :koko         — print the koko mascot"))
		fmt.Println(ui.Response("  :help         — show this help"))
		fmt.Println(ui.Response("  \"\"\"           — start multiline input"))
		fmt.Println()
		return true, ""

	case ":clear":
		a.ClearHistory()
		fmt.Println(ui.Info("cleared", "conversation history reset"))
		fmt.Println()
		return true, ""

	case ":history":
		fmt.Println(ui.Info("messages", fmt.Sprintf("%d", a.HistoryLen())))
		fmt.Println()
		return true, ""

	case ":undo":
		path, err := a.Undo()
		if err != nil {
			fmt.Println(ui.Error(fmt.Sprintf("undo failed: %v", err)))
		} else if path == "" {
			fmt.Println(ui.Info("undo", "nothing to undo"))
		} else {
			fmt.Println(ui.Info("undo", fmt.Sprintf("reverted %s", path)))
		}
		fmt.Println()
		return true, ""

	case ":tokens":
		fmt.Println(ui.Info("input   ", fmt.Sprintf("%d tokens", a.TotalInput)))
		fmt.Println(ui.Info("output  ", fmt.Sprintf("%d tokens", a.TotalOutput)))
		fmt.Println(ui.Info("total   ", fmt.Sprintf("%d tokens", a.TotalInput+a.TotalOutput)))
		fmt.Println(ui.Info("messages", fmt.Sprintf("%d", a.HistoryLen())))
		fmt.Println()
		return true, ""

	case ":run":
		if len(parts) < 2 {
			fmt.Println(ui.Error("usage: :run <command>"))
		} else {
			cmdStr := strings.TrimPrefix(input, ":run ")
			runCmd := exec.Command("sh", "-c", cmdStr)
			runCmd.Dir = sandboxRoot
			out, err := runCmd.CombinedOutput()
			result := strings.TrimRight(string(out), "\n")
			if err != nil {
				fmt.Println(ui.Error(result))
			} else if result != "" {
				fmt.Println(ui.Response(result))
			}
		}
		fmt.Println()
		return true, ""

	case ":compact":
		oldTokens, newTokens := a.Compact()
		fmt.Println(ui.Info("compact", fmt.Sprintf("~%d → ~%d tokens", oldTokens, newTokens)))
		fmt.Println()
		return true, ""

	case ":model":
		if len(parts) < 2 {
			fmt.Println(ui.Info("model", llm.Model()))
		} else {
			llm.SetModel(parts[1])
			fmt.Println(ui.Info("model", fmt.Sprintf("switched to %s", parts[1])))
		}
		fmt.Println()
		return true, ""

	case ":save":
		if err := a.SaveSession(dataDir); err != nil {
			fmt.Println(ui.Error(fmt.Sprintf("save failed: %v", err)))
		} else {
			fmt.Println(ui.Info("saved", "session written to disk"))
		}
		fmt.Println()
		return true, ""

	case ":resume":
		if err := a.LoadSession(dataDir); err != nil {
			fmt.Println(ui.Error(fmt.Sprintf("resume failed: %v", err)))
		} else {
			fmt.Println(ui.Info("resumed", fmt.Sprintf("loaded %d messages", a.HistoryLen())))
		}
		fmt.Println()
		return true, ""

	case ":plays":
		list := playRegistry.List()
		if len(list) == 0 {
			fmt.Println(ui.Info("plays", fmt.Sprintf("none installed — add *.md files in %s", playRegistry.Dir())))
		} else {
			for _, p := range list {
				desc := p.Description
				if desc == "" {
					desc = "(no description)"
				}
				fmt.Println(ui.Info(p.Name, desc))
			}
		}
		fmt.Println()
		return true, ""

	case ":plan":
		mode := a.TogglePlanMode()
		if mode {
			fmt.Println(ui.Info("plan", "mode on — read-only; call :plan again to exit"))
		} else {
			fmt.Println(ui.Info("plan", "mode off — full tools restored"))
		}
		fmt.Println()
		return true, ""

	default:
		name := strings.TrimPrefix(cmd, ":")
		if p, ok := playRegistry.Get(name); ok {
			extra := strings.TrimSpace(strings.TrimPrefix(input, cmd))
			prompt := fmt.Sprintf("Run the '%s' play:\n\n%s", p.Name, p.Body)
			if extra != "" {
				prompt += "\n\nUser request:\n" + extra
			}
			return false, prompt
		}
		fmt.Println(ui.Error(fmt.Sprintf("unknown command: %s (try :help)", cmd)))
		fmt.Println()
		return true, ""
	}
}

func apiKeyEnvVar(p config.ProviderType) string {
	switch p {
	case config.ProviderAnthropic:
		return "ANTHROPIC_API_KEY"
	case config.ProviderMistral:
		return "MISTRAL_API_KEY"
	default:
		return ""
	}
}
