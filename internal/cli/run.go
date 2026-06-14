package cli

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/original-flipster69/koko/internal/agent"
	"github.com/original-flipster69/koko/internal/audit"
	"github.com/original-flipster69/koko/internal/config"
	"github.com/original-flipster69/koko/internal/ignore"
	"github.com/original-flipster69/koko/internal/memories"
	"github.com/original-flipster69/koko/internal/plays"
	"github.com/original-flipster69/koko/internal/policy"
	"github.com/original-flipster69/koko/internal/project"
	"github.com/original-flipster69/koko/internal/provider"
	"github.com/original-flipster69/koko/internal/sandbox"
	"github.com/original-flipster69/koko/internal/terminal"
	"github.com/original-flipster69/koko/internal/ui"
)

// Options holds the raw command-line inputs used to bootstrap the CLI.
type Options struct {
	Provider   string
	Model      string
	LlmURL     string
	Sandbox    string
	ConfigPath string
}

// Main is the package entrypoint. It loads configuration, constructs the
// provider, sandbox and play registry, then starts the CLI. Keeping all wiring
// here means cmd/koko/main.go never has to touch unexported helpers.
func Main(opts Options) error {
	kokoDir := KokoDir()
	cfgPath := config.Path(kokoDir)
	if opts.ConfigPath != "" {
		cfgPath = opts.ConfigPath
	}

	cfg, err := loadConfig(ReloadSources{
		cfgPath:  cfgPath,
		provider: opts.Provider,
		model:    opts.Model,
		llmURL:   opts.LlmURL,
		sandbox:  opts.Sandbox,
	})
	if err != nil {
		return err
	}

	llm, err := provider.New(&cfg.Llm)
	if err != nil {
		return err
	}

	sb, err := sandbox.New(cfg.Sandbox.Root, cfg.Sandbox.AllowedDirs(), cfg.Sandbox.DenyFiles, cfg.Sandbox.MaxFileSize)
	if err != nil {
		return err
	}

	playsDir := filepath.Join(kokoDir, "plays")
	playRegistry, err := plays.Load(playsDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, ui.DefaultScheme().Error(fmt.Sprintf("cannot load plays: %v", err)))
		playRegistry, _ = plays.Load("")
	}

	return Run(llm, sb, cfg, playRegistry)
}

// Run initializes the CLI and starts the terminal.
func Run(llm provider.Provider, sb *sandbox.Sandbox, cfg *config.Config, playRegistry *plays.Registry) error {
	// Initialize UI scheme
	scheme, err := ui.DefaultScheme().With(cfg.Style.ColorScheme)
	if err != nil {
		return fmt.Errorf("failed to initialize UI scheme: %v", err)
	}

	// Initialize audit log
	kokoDir := KokoDir()
	auditLog, err := audit.NewLog(filepath.Join(kokoDir, "audit.jsonl"))
	if err != nil {
		return fmt.Errorf("failed to open audit log: %v", err)
	}
	defer auditLog.Close()

	// Initialize log file
	logFile, err := os.OpenFile(filepath.Join(kokoDir, "koko.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err == nil {
		slog.SetDefault(slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{Level: slog.LevelInfo})))
		defer logFile.Close()
	}

	// Initialize memory store
	memoryStore, err := memories.Open(filepath.Join(kokoDir, "memories"))
	if err != nil {
		return fmt.Errorf("failed to open memories store: %v", err)
	}

	// Initialize ignore matcher
	var ignoreMatcher *ignore.Matcher
	if cfg.Ignore.Mode == config.Custom {
		ignoreMatcher = ignore.NewFromPatterns(cfg.Ignore.Files)
	} else {
		ignoreMatcher = ignore.LoadGitignore(cfg.Sandbox.Root)
	}

	// Initialize command policy
	cmdPolicy, err := policy.NewCommandPolicy(cfg.Sandbox.Exec.Allow, cfg.Sandbox.Exec.Deny)
	if err != nil {
		return fmt.Errorf("failed to initialize command policy: %v", err)
	}

	// Build project context summary
	extraContext := project.Scan(cfg.Sandbox.Root).Summary()

	// Build outbound message filters
	var outboundFilters []agent.OutboundFilter
	if cfg.Sandbox.ScrubPII {
		outboundFilters = append(outboundFilters, agent.ScrubPIIFilter)
	}

	// Initialize agent
	confirm := func(action string) bool {
		fmt.Printf("  %s%srun:%s %s%s%s  [y/N] ", ui.Bold, scheme.Secondary, ui.Reset, scheme.Label, action, ui.Reset)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		return answer == "y" || answer == "yes"
	}

	cpuSec, memMB, fileMB := cfg.Sandbox.Exec.Limits()
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

	// Register all commands
	commands := make(map[string]Command)
	for k, v := range registerCoreCommands(a, scheme) {
		commands[k] = v
	}
	for k, v := range registerFileCommands(sb, scheme) {
		commands[k] = v
	}
	for k, v := range registerMemoryCommands(a, scheme) {
		commands[k] = v
	}
	for k, v := range registerExecCommands(sb, scheme) {
		commands[k] = v
	}
	for k, v := range registerPlayCommands(playRegistry, scheme) {
		commands[k] = v
	}

	// Build the list of known command names (for tab-completion / hints).
	knownCommands := make([]string, 0, len(commands))
	for name := range commands {
		knownCommands = append(knownCommands, name)
	}

	// Adapt the command map into the terminal's CmdHandler signature.
	// The terminal passes the raw input line; we split it into parts and
	// dispatch to the matching command's Fn.
	slashHandler := func(input string, a *agent.Agent) (bool, string, string) {
		parts := strings.Fields(input)
		if len(parts) == 0 {
			return false, "", ""
		}
		cmd, ok := commands[parts[0]]
		if !ok {
			return false, "", ""
		}
		return cmd.Fn(input, parts, a)
	}

	// Start the terminal
	return terminal.Run(a, kokoDir, ui.MascotFrames(scheme), slashHandler, knownCommands, scheme)
}
