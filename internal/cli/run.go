package cli

import (
	"bufio"
	"fmt"
	"io"
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
	Version  string
}

func Run(opts Flags) error {
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

	cfgPath := config.Path(kokoRoot)
	scheme, err := ui.DefaultScheme().With(cfg.Style.ColorScheme)
	if err != nil {
		return fmt.Errorf("failed to initialize UI scheme: %v", err)
	}

	if !cfg.Sandbox.SuppressElevatedWarning && isElevated() && !confirmElevated(os.Stdin, os.Stdout) {
		return fmt.Errorf("aborted: not starting with elevated privileges")
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

	stack := project.Scan(cfg.Sandbox.Root)
	projectCtx := stack.Summary()
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

	apply := func(next config.Config) (applied, restart []string) {
		return applyConfig(cfg, next, a)
	}

	cmds := make(map[string]command)
	register(cmds, commandList(sb, memoStore, playsReg, cfg, kokoRoot, cfgPath, sb.Root(), opts, apply)...)
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
		cur := a.Scheme()
		name := parts[0]
		if c, ok := cmds[name]; ok {
			return true, "", c.fn(cmdOpts{input: input, a: a})
		}
		playName := strings.TrimPrefix(name, ":")
		if p, ok := playsReg.Get(playName); ok {
			extra := strings.TrimSpace(strings.TrimPrefix(input, name))
			prompt := fmt.Sprintf("Run the '%s' play:\n\n%s", p.Name, p.Render(extra))
			return false, prompt, ""
		}
		return true, "", cur.Error(fmt.Sprintf("unknown command: %s (try :help)", name))
	}

	projectConfigNote := ""
	if p := config.ProjectConfigPath(cfg.Sandbox.Root); fileExists(p) {
		projectConfigNote = ui.Dim + scheme.Muted + "  note: project config applied from " + p + ui.Reset + "\n\n"
	}

	mascotFrames := ui.MascotFrames(scheme)
	splashes := make([]string, len(mascotFrames))
	for i, mascot := range mascotFrames {
		splash := "\n" + scheme.Splashscreen(mascot, llm.Name(), cfg.Llm.Model, cfg.Sandbox.Root, opts.Version, stack.Detected) + "\n\n"
		if llm.Name() == "ollama" {
			splash += ui.Dim + scheme.Muted + "  note: tool support depends on model (llama3.1+, mistral, command-r)" + ui.Reset + "\n\n"
		}
		if warning := ui.PrivacyWarning(llm.Name()); !cfg.Sandbox.SuppressPrivacyWarning && warning != "" {
			splash += warning + "\n\n"
		}
		splash += projectConfigNote
		splashes[i] = splash
	}

	return terminal.Run(a, kokoRoot, splashes, colonHandler, knownCommands, scheme)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func applyConfig(cur *config.Config, next config.Config, a *agent.Agent) (applied, restart []string) {
	d := cur.Diff(&next)

	if d.Provider {
		if p, err := provider.New(&next.Llm); err == nil {
			a.SetProvider(p)
			cur.Llm.Provider = next.Llm.Provider
			cur.Llm.Url = next.Llm.Url
			cur.Llm.ApiKey = next.Llm.ApiKey
			cur.Llm.Model = next.Llm.Model
			applied = append(applied, "provider")
		}
	} else if d.Model {
		a.SetModel(next.Llm.Model)
		cur.Llm.Model = next.Llm.Model
		applied = append(applied, "model")
	}
	if d.Verbs {
		a.SetThinkingVerbs(next.Style.ThinkingVerbs)
		cur.Style.ThinkingVerbs = next.Style.ThinkingVerbs
		applied = append(applied, "thinking verbs")
	}
	if d.MaxTokens {
		a.SetMaxSessionTokens(next.Llm.MaxSessionTokens)
		cur.Llm.MaxSessionTokens = next.Llm.MaxSessionTokens
		applied = append(applied, "max session tokens")
	}
	if d.ScrubPII {
		var filters []agent.OutboundFilter
		if next.Sandbox.ScrubPII {
			filters = append(filters, agent.ScrubPIIFilter)
		}
		a.SetOutboundFilters(filters)
		cur.Sandbox.ScrubPII = next.Sandbox.ScrubPII
		applied = append(applied, "scrub_pii")
	}
	if d.SuppressPrivacyWarning {
		applied = append(applied, "suppress privacy warning")
	}
	if d.ExecLimits {
		cpuSec, memMB, fileMB := next.Sandbox.Exec.Limits()
		a.SetExecLimits(cpuSec, memMB, fileMB)
		cur.Sandbox.Exec.Profile = next.Sandbox.Exec.Profile
		applied = append(applied, "exec profile")
	}
	if d.CmdPolicy {
		if p, err := policy.NewCommandPolicy(next.Sandbox.Exec.Allow, next.Sandbox.Exec.Deny); err == nil {
			a.SetCmdPolicy(p)
			cur.Sandbox.Exec.Allow = next.Sandbox.Exec.Allow
			cur.Sandbox.Exec.Deny = next.Sandbox.Exec.Deny
			applied = append(applied, "command policy")
		}
	}
	if d.Ignore {
		var m *ignore.Matcher
		if next.Ignore.Mode == config.Custom {
			m = ignore.NewFromPatterns(next.Ignore.Files)
		} else {
			m = ignore.LoadGitignore(next.Sandbox.Root)
		}
		a.SetIgnore(m)
		cur.Ignore = next.Ignore
		applied = append(applied, "ignore")
	}
	if d.ColorScheme {
		if s, err := ui.DefaultScheme().With(next.Style.ColorScheme); err == nil {
			a.SetScheme(s)
			cur.Style.ColorScheme = next.Style.ColorScheme
			applied = append(applied, "color scheme")
		}
	}

	return applied, append(restart, d.RestartLabels()...)
}

func register(cmds map[string]command, list ...cmdDef) {
	for _, c := range list {
		cmds[":"+c.name()] = command{c.desc(), c.args(), c.do}
	}
}

// commandList is the single source of truth for the built-in colon commands
// (everything except :help, which is registered separately because it needs the
// finished command map). name()/desc()/args() ignore the injected deps, so it
// can be called with zero values purely to enumerate commands.
func commandList(sb *sandbox.Sandbox, memo *memories.Store, plays *plays.Registry, cfg *config.Config, kokoRoot, cfgPath, sandboxRoot string, flags Flags, apply func(config.Config) (applied, restart []string)) []cmdDef {
	return []cmdDef{
		koko{}, clear{}, history{}, undo{}, tokens{}, compact{}, plan{}, vision{},
		run{sb},
		memoriesCmd{memo},
		playsCmd{plays},
		model{}, effort{}, configCmd{cfg, kokoRoot}, save{kokoRoot}, resume{kokoRoot},
		reload{cfgPath, flags, apply}, cageCmd{kokoRoot, sandboxRoot},
	}
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

func isElevated() bool {
	if runtime.GOOS == "windows" {
		return os.Geteuid() == 0
	}
	return os.Getuid() == 0
}

func confirmElevated(r io.Reader, w io.Writer) bool {
	fmt.Fprintf(w, "Running with elevated privileges. Continue? [y/N] ")
	reader := bufio.NewReader(r)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}
