package cli

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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

const (
	playsDir  = "plays"
	auditFile = "audit.jsonl"
	logFile   = "koko.log"
	memoDir   = "memories"

	llmStreamTimeout = 5 * time.Minute
)

type Flags struct {
	Provider string
	Model    string
	LlmURL   string
	Sandbox  string
}

func Main(opts Flags) error {
	kokoRoot := kokoDir()

	cfg, err := loadConfig(config.Path(kokoRoot), opts)
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

	return Run(llm, sb, cfg, kokoRoot, config.Path(kokoRoot), opts)
}

func Run(llm provider.Provider, sb *sandbox.Sandbox, cfg *config.Config, kokoRoot string, cfgPath string, opts Flags) error {
	scheme, err := ui.DefaultScheme().With(cfg.Style.ColorScheme)
	if err != nil {
		return fmt.Errorf("failed to initialize UI scheme: %v", err)
	}

	playDir := filepath.Join(kokoRoot, playsDir)
	playsReg, err := plays.Load(playDir)
	if err != nil {
		return fmt.Errorf("failed to load plays: %v", err)
	}

	auditLog, err := audit.NewLog(filepath.Join(kokoRoot, auditFile))
	if err != nil {
		return fmt.Errorf("failed to open audit log: %v", err)
	}
	defer auditLog.Close()

	log, err := os.OpenFile(filepath.Join(kokoRoot, logFile), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err == nil {
		slog.SetDefault(slog.New(slog.NewJSONHandler(log, &slog.HandlerOptions{Level: slog.LevelInfo})))
		defer log.Close()
	}

	memoStore, err := memories.Open(filepath.Join(kokoRoot, memoDir))
	if err != nil {
		return fmt.Errorf("failed to open memories store: %v", err)
	}

	var ignoreMatcher *ignore.Matcher
	if cfg.Ignore.Mode == config.Custom {
		ignoreMatcher = ignore.NewFromPatterns(cfg.Ignore.Files)
	} else {
		ignoreMatcher = ignore.LoadGitignore(cfg.Sandbox.Root)
	}

	cmdPolicy, err := policy.NewCommandPolicy(cfg.Sandbox.Exec.Allow, cfg.Sandbox.Exec.Deny)
	if err != nil {
		return fmt.Errorf("failed to initialize command policy: %v", err)
	}

	projectCtx := project.Scan(cfg.Sandbox.Root).Summary()
	if idx := memoStore.Index(); idx != "" {
		if projectCtx != "" {
			projectCtx += "\n\n"
		}
		projectCtx += "Stored memories (use list_memories to read bodies, save_memory/delete_memory to modify):\n" + idx
	}

	var outFilters []agent.OutboundFilter
	if cfg.Sandbox.ScrubPII {
		outFilters = append(outFilters, agent.ScrubPIIFilter)
	}

	confirm := func(action string) bool {
		fmt.Printf("  %s%srun:%s %s%s%s  [y/N] ", ui.Bold, scheme.Secondary, ui.Reset, scheme.Label, action, ui.Reset)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		return answer == "y" || answer == "yes"
	}

	cpuSec, memMB, fileMB := cfg.Sandbox.Exec.Limits()
	a := agent.New(llm, sb, os.Stdout, confirm, auditLog, agent.Options{
		Memory:           memoStore,
		CmdPolicy:        cmdPolicy,
		Ignore:           ignoreMatcher,
		Scheme:           scheme,
		ProjectCtx:       projectCtx,
		ThinkingVerbs:    cfg.Style.ThinkingVerbs,
		MaxSessionTokens: cfg.Llm.MaxSessionTokens,
		StreamTimeout:    llmStreamTimeout,
		OutboundFilters:  outFilters,
		ExecCPUSeconds:   cpuSec,
		ExecMemoryMB:     memMB,
		ExecMaxFileMB:    fileMB,
	})

	cmds := make(map[string]command)
	register(cmds,
		koko{}, clear{}, history{}, undo{}, tokens{}, compact{}, plan{}, vision{sb, ignoreMatcher},
		run{sb},
		memoriesCmd{memoStore},
		playsCmd{playsReg},
		model{llm}, configCmd{cfg, kokoRoot}, save{kokoRoot}, resume{kokoRoot},
		reload{cfg, llm, cfgPath, opts}, cageCmd{kokoRoot, sb.Root()},
	)
	register(cmds, help{cmds})

	knownCommands := make([]string, 0, len(cmds))
	for name := range cmds {
		knownCommands = append(knownCommands, name)
	}
	for _, p := range playsReg.List() {
		knownCommands = append(knownCommands, ":"+p.Name)
	}

	colonHandler := func(input string, a *agent.Agent) (bool, string, string) {
		parts := strings.Fields(input)
		if len(parts) == 0 {
			return true, "", ""
		}
		name := parts[0]
		if c, ok := cmds[name]; ok {
			return c.fn(input, parts, a, scheme)
		}
		playName := strings.TrimPrefix(name, ":")
		if p, ok := playsReg.Get(playName); ok {
			extra := strings.TrimSpace(strings.TrimPrefix(input, name))
			prompt := fmt.Sprintf("Run the '%s' play:\n\n%s", p.Name, p.Render(extra))
			return false, prompt, ""
		}
		return true, "", scheme.Error(fmt.Sprintf("unknown command: %s (try :help)", name))
	}

	return terminal.Run(a, kokoRoot, ui.MascotFrames(scheme), colonHandler, knownCommands, scheme)
}

func kokoDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("APPDATA"), "koko")
	}
	return filepath.Join(os.Getenv("HOME"), ".koko")
}

func loadConfig(path string, opts Flags) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	root := opts.Sandbox
	if root == "" {
		root = cfg.Sandbox.Root
	}
	if _, err := cfg.ApplyProjectConfig(root); err != nil {
		return nil, err
	}
	cfg.ApplyFlags(opts.Provider, opts.Model, opts.LlmURL, opts.Sandbox)
	cfg.ApplyEnv()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func applyReloadedConfig(cur, next *config.Config, setModel func(string), setVerbs func([]string), setMaxTokens func(int)) (applied, restart []string) {
	if !equalStrings(next.Style.ThinkingVerbs, cur.Style.ThinkingVerbs) {
		setVerbs(next.Style.ThinkingVerbs)
		cur.Style.ThinkingVerbs = next.Style.ThinkingVerbs
		applied = append(applied, "thinking verbs")
	}
	if next.Llm.MaxSessionTokens != cur.Llm.MaxSessionTokens {
		setMaxTokens(next.Llm.MaxSessionTokens)
		cur.Llm.MaxSessionTokens = next.Llm.MaxSessionTokens
		applied = append(applied, "max session tokens")
	}
	if next.Llm.Provider == cur.Llm.Provider {
		if next.Llm.Model != cur.Llm.Model {
			setModel(next.Llm.Model)
			cur.Llm.Model = next.Llm.Model
			applied = append(applied, "model")
		}
	} else {
		restart = append(restart, "provider")
	}
	if next.Llm.Url != cur.Llm.Url {
		restart = append(restart, "llm url")
	}
	if next.Sandbox.Root != cur.Sandbox.Root {
		restart = append(restart, "sandbox root")
	}
	if !equalStrings(next.Sandbox.AdditionalDirs, cur.Sandbox.AdditionalDirs) {
		restart = append(restart, "additional dirs")
	}
	if !equalStrings(next.Sandbox.DenyFiles, cur.Sandbox.DenyFiles) {
		restart = append(restart, "deny files")
	}
	if next.Sandbox.MaxFileSize != cur.Sandbox.MaxFileSize {
		restart = append(restart, "max file size")
	}
	if next.Sandbox.ScrubPII != cur.Sandbox.ScrubPII {
		restart = append(restart, "scrub_pii")
	}
	if next.Sandbox.Exec.Profile != cur.Sandbox.Exec.Profile {
		restart = append(restart, "exec profile")
	}
	if next.Ignore.Mode != cur.Ignore.Mode {
		restart = append(restart, "ignore mode")
	}
	return applied, restart
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
