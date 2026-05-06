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

	"github.com/meeseeks/koko/internal/agent"
	"github.com/meeseeks/koko/internal/audit"
	"github.com/meeseeks/koko/internal/config"
	"github.com/meeseeks/koko/internal/detect"
	"github.com/meeseeks/koko/internal/memory"
	"github.com/meeseeks/koko/internal/plays"
	"github.com/meeseeks/koko/internal/policy"
	"github.com/meeseeks/koko/internal/provider"
	"github.com/meeseeks/koko/internal/sandbox"
	"github.com/meeseeks/koko/internal/tui"
	"github.com/meeseeks/koko/internal/ui"
)

var version = "dev"

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
		cfg.APIKey = os.Getenv(config.APIKeyEnvVar(cfg.Provider))
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

	if *promptFlag != "" {
		confirm := func(action string) bool {
			fmt.Printf("  %s%srun:%s %s%s%s  [y/N] ", ui.Bold, ui.Purple, ui.Reset, ui.Violet, action, ui.Reset)
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			return answer == "y" || answer == "yes"
		}
		a := agent.New(llm, sb, os.Stdout, confirm, auditLog, extraContext)
		a.SetThinkingVerbs(cfg.ThinkingVerbs)
		a.SetMemory(memoryStore)
		a.SetCommandPolicy(cmdPolicy)
		a.SetLimits(cfg.MaxToolCalls, cfg.MaxSessionTokens)
		a.SetScrubPII(cfg.ScrubPII)
		a.SetQuietTools(cfg.QuietToolOutputs)
		a.SetExecLimits(cfg.ExecCPUSeconds, cfg.ExecMemoryMB, cfg.ExecMaxFileMB)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigCh
			cancel()
			os.Exit(0)
		}()

		if err := a.Run(ctx, *promptFlag); err != nil {
			fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
			os.Exit(1)
		}
		return
	}

	splash := "\n" + ui.Splash(llm.Name(), cfg.Model, cfg.SandboxRoot, version, project.Languages, project.BuildTools) + "\n\n"

	slashHandler := makeSlashHandler(cfg, llm, kokoDir, cfg.SandboxRoot, playRegistry)

	if err := tui.Run(cfg, llm, sb, auditLog, memoryStore, cmdPolicy, playRegistry, extraContext, kokoDir, splash, slashHandler); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}
	fmt.Println(ui.Goodbye())
}

func makeSlashHandler(cfg *config.Config, llm provider.Provider, dataDir string, sandboxRoot string, playRegistry *plays.Registry) tui.SlashHandler {
	return func(input string, a *agent.Agent) (bool, string, string) {
		parts := strings.Fields(input)
		cmd := parts[0]
		var out strings.Builder

		switch cmd {
		case ":koko":
			out.WriteString("\n")
			out.WriteString(ui.Mascot())
			return true, "", out.String()

		case ":help":
			out.WriteString(":clear        — reset conversation history\n")
			out.WriteString(":history      — show message count\n")
			out.WriteString(":undo         — revert last file change\n")
			out.WriteString(":run <cmd>    — run a shell command directly\n")
			out.WriteString(":tokens       — show token usage stats\n")
			out.WriteString(":compact      — compress history to free context\n")
			out.WriteString(":model [name] — show or switch model\n")
			out.WriteString(":config       — show active configuration\n")
			out.WriteString(":save         — save session to disk\n")
			out.WriteString(":resume       — restore saved session\n")
			out.WriteString(":plays        — list installed plays\n")
			out.WriteString(":<name>       — run a play by name (e.g. :review)\n")
			out.WriteString(":plan         — toggle plan mode (read-only)\n")
			out.WriteString(":koko         — print the koko mascot\n")
			out.WriteString(":help         — show this help\n")
			return true, "", out.String()

		case ":clear":
			a.ClearHistory()
			out.WriteString(ui.Info("cleared", "conversation history reset"))
			return true, "", out.String()

		case ":history":
			out.WriteString(ui.Info("messages", fmt.Sprintf("%d", a.HistoryLen())))
			return true, "", out.String()

		case ":undo":
			path, err := a.Undo()
			if err != nil {
				out.WriteString(ui.Error(fmt.Sprintf("undo failed: %v", err)))
			} else if path == "" {
				out.WriteString(ui.Info("undo", "nothing to undo"))
			} else {
				out.WriteString(ui.Info("undo", fmt.Sprintf("reverted %s", path)))
			}
			return true, "", out.String()

		case ":tokens":
			out.WriteString(ui.Info("input   ", fmt.Sprintf("%d tokens", a.TotalInput)) + "\n")
			out.WriteString(ui.Info("output  ", fmt.Sprintf("%d tokens", a.TotalOutput)) + "\n")
			out.WriteString(ui.Info("total   ", fmt.Sprintf("%d tokens", a.TotalInput+a.TotalOutput)) + "\n")
			out.WriteString(ui.Info("messages", fmt.Sprintf("%d", a.HistoryLen())))
			return true, "", out.String()

		case ":run":
			if len(parts) < 2 {
				out.WriteString(ui.Error("usage: :run <command>"))
			} else {
				cmdStr := strings.TrimPrefix(input, ":run ")
				runCmd := exec.Command("sh", "-c", cmdStr)
				runCmd.Dir = sandboxRoot
				result, err := runCmd.CombinedOutput()
				text := strings.TrimRight(string(result), "\n")
				if err != nil {
					out.WriteString(ui.Error(text))
				} else if text != "" {
					out.WriteString(text)
				}
			}
			return true, "", out.String()

		case ":compact":
			oldTokens, newTokens := a.Compact()
			out.WriteString(ui.Info("compact", fmt.Sprintf("~%d → ~%d tokens", oldTokens, newTokens)))
			return true, "", out.String()

		case ":model":
			if len(parts) < 2 {
				out.WriteString(ui.Info("model", llm.Model()))
			} else {
				llm.SetModel(parts[1])
				out.WriteString(ui.Info("model", fmt.Sprintf("switched to %s", parts[1])))
			}
			return true, "", out.String()

		case ":config":
			out.WriteString(ui.Info("provider ", string(cfg.Provider)) + "\n")
			out.WriteString(ui.Info("model    ", cfg.Model) + "\n")
			out.WriteString(ui.Info("sandbox  ", cfg.SandboxRoot) + "\n")
			out.WriteString(ui.Info("max_tok  ", fmt.Sprintf("%d", cfg.MaxTokens)) + "\n")
			out.WriteString(ui.Info("tools    ", fmt.Sprintf("%d max", cfg.MaxToolCalls)) + "\n")
			out.WriteString(ui.Info("session  ", fmt.Sprintf("%d max tokens", cfg.MaxSessionTokens)) + "\n")
			out.WriteString(ui.Info("exec     ", fmt.Sprintf("%ds cpu, %dMB mem, %dMB file", cfg.ExecCPUSeconds, cfg.ExecMemoryMB, cfg.ExecMaxFileMB)) + "\n")
			out.WriteString(ui.Info("scrub_pii", fmt.Sprintf("%v", cfg.ScrubPII)) + "\n")
			out.WriteString(ui.Info("verbs    ", strings.Join(cfg.ThinkingVerbs, ", ")) + "\n")
			out.WriteString(ui.Info("quiet    ", strings.Join(cfg.QuietToolOutputs, ", ")) + "\n")
			out.WriteString(ui.Info("config   ", config.ConfigPath()))
			return true, "", out.String()

		case ":save":
			if err := a.SaveSession(dataDir); err != nil {
				out.WriteString(ui.Error(fmt.Sprintf("save failed: %v", err)))
			} else {
				out.WriteString(ui.Info("saved", "session written to disk"))
			}
			return true, "", out.String()

		case ":resume":
			if err := a.LoadSession(dataDir); err != nil {
				out.WriteString(ui.Error(fmt.Sprintf("resume failed: %v", err)))
			} else {
				out.WriteString(ui.Info("resumed", fmt.Sprintf("loaded %d messages", a.HistoryLen())))
			}
			return true, "", out.String()

		case ":plays":
			list := playRegistry.List()
			if len(list) == 0 {
				out.WriteString(ui.Info("plays", fmt.Sprintf("none installed — add *.md files in %s", playRegistry.Dir())))
			} else {
				for _, p := range list {
					desc := p.Description
					if desc == "" {
						desc = "(no description)"
					}
					out.WriteString(ui.Info(p.Name, desc) + "\n")
				}
			}
			return true, "", out.String()

		case ":plan":
			mode := a.TogglePlanMode()
			if mode {
				out.WriteString(ui.Info("plan", "mode on — read-only; call :plan again to exit"))
			} else {
				out.WriteString(ui.Info("plan", "mode off — full tools restored"))
			}
			return true, "", out.String()

		default:
			name := strings.TrimPrefix(cmd, ":")
			if p, ok := playRegistry.Get(name); ok {
				extra := strings.TrimSpace(strings.TrimPrefix(input, cmd))
				prompt := fmt.Sprintf("Run the '%s' play:\n\n%s", p.Name, p.Body)
				if extra != "" {
					prompt += "\n\nUser request:\n" + extra
				}
				return false, prompt, ""
			}
			out.WriteString(ui.Error(fmt.Sprintf("unknown command: %s (try :help)", cmd)))
			return true, "", out.String()
		}
	}
}

