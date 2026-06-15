package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/original-flipster69/koko/internal/cage"
	"github.com/original-flipster69/koko/internal/config"
	"github.com/original-flipster69/koko/internal/ui"
)

type model struct{}

func (m model) name() string { return "model" }
func (m model) desc() string { return "Show or switch model" }
func (m model) args() string { return "[name]" }
func (m model) do(opts cmdOpts) (bool, string, string) {
	parts := opts.parts()
	if len(parts) < 2 {
		return true, "", opts.scheme.Info("model", opts.a.Model())
	}
	opts.a.SetModel(parts[1])
	return true, "", opts.scheme.Info("model", fmt.Sprintf("switched to %s", parts[1]))
}

type configCmd struct {
	cfg     *config.Config
	kokoDir string
}

func (c configCmd) name() string { return "config" }
func (c configCmd) desc() string { return "Show active configuration" }
func (c configCmd) args() string { return "" }
func (c configCmd) do(opts cmdOpts) (bool, string, string) {
	scheme := opts.scheme
	var b strings.Builder
	b.WriteString(scheme.Info("provider", string(c.cfg.Llm.Provider)) + "\n")
	b.WriteString(scheme.Info("model", c.cfg.Llm.Model) + "\n")
	b.WriteString(scheme.Info("sandbox", c.cfg.Sandbox.Root) + "\n")
	b.WriteString(scheme.Info("max_tok", fmt.Sprintf("%d", c.cfg.Llm.MaxTokens)) + "\n")
	b.WriteString(scheme.Info("session", fmt.Sprintf("%d max tokens", c.cfg.Llm.MaxSessionTokens)) + "\n")
	cpuSec, memMB, fileMB := c.cfg.Sandbox.Exec.Limits()
	b.WriteString(scheme.Info("exec", fmt.Sprintf("%s (%ds cpu, %dMB mem, %dMB file)", c.cfg.Sandbox.Exec.Profile, cpuSec, memMB, fileMB)) + "\n")
	b.WriteString(scheme.Info("scrub_pii", fmt.Sprintf("%v", c.cfg.Sandbox.ScrubPII)) + "\n")
	b.WriteString(scheme.Info("verbs", strings.Join(c.cfg.Style.ThinkingVerbs, ", ")) + "\n")
	b.WriteString(scheme.Info("config", config.Path(c.kokoDir)))
	return true, "", b.String()
}

type save struct{ kokoDir string }

func (s save) name() string { return "save" }
func (s save) desc() string { return "Save session to disk" }
func (s save) args() string { return "" }
func (s save) do(opts cmdOpts) (bool, string, string) {
	if err := opts.a.SaveSession(s.kokoDir); err != nil {
		return true, "", opts.scheme.Error(fmt.Sprintf("save failed: %v", err))
	}
	return true, "", opts.scheme.Info("saved", "session written to disk")
}

type resume struct{ kokoDir string }

func (r resume) name() string { return "resume" }
func (r resume) desc() string { return "Restore saved session" }
func (r resume) args() string { return "" }
func (r resume) do(opts cmdOpts) (bool, string, string) {
	if err := opts.a.LoadSession(r.kokoDir); err != nil {
		return true, "", opts.scheme.Error(fmt.Sprintf("resume failed: %v", err))
	}
	return true, "", opts.scheme.Info("resumed", fmt.Sprintf("loaded %d messages", opts.a.HistoryLen()))
}

type reload struct {
	cfgPath string
	opts    Flags
	apply   func(next config.Config) (applied, restart []string)
}

func (r reload) name() string { return "reload" }
func (r reload) desc() string { return "Reload config from its sources" }
func (r reload) args() string { return "" }
func (r reload) do(opts cmdOpts) (bool, string, string) {
	scheme := opts.scheme
	newCfg, err := loadConfig(r.cfgPath, r.opts)
	if err != nil {
		return true, "", scheme.Error(fmt.Sprintf("reload failed (keeping current config): %v", err))
	}
	applied, restart := r.apply(*newCfg)
	if len(applied) == 0 && len(restart) == 0 {
		return true, "", scheme.Info("reload", "config reloaded — no changes detected")
	}
	var b strings.Builder
	if len(applied) > 0 {
		b.WriteString(scheme.Info("applied", strings.Join(applied, ", ")) + "\n")
	}
	if len(restart) > 0 {
		b.WriteString(scheme.Info("restart", "changed but needs restart: "+strings.Join(restart, ", ")) + "\n")
	}
	if slices.Contains(applied, "provider") && !newCfg.Sandbox.SuppressPrivacyWarning {
		if warning := ui.PrivacyWarning(opts.a.ProviderName()); warning != "" {
			b.WriteString(warning + "\n")
		}
	}
	return true, "", strings.TrimRight(b.String(), "\n")
}

type cageCmd struct {
	kokoDir     string
	sandboxRoot string
}

func (c cageCmd) name() string { return "cage" }
func (c cageCmd) desc() string { return "Generate a low-privilege user setup script" }
func (c cageCmd) args() string { return "<username> [dir=…] [group=…] [os=darwin|linux]" }
func (c cageCmd) do(opts cmdOpts) (bool, string, string) {
	scheme := opts.scheme
	parts := opts.parts()
	if len(parts) < 2 {
		return true, "", scheme.Error("usage: :cage <username> [dir=PATH] [group=NAME] [os=darwin|linux]")
	}
	co := cage.Options{Username: parts[1], GOOS: runtime.GOOS}
	outDir := c.kokoDir
	for _, tok := range parts[2:] {
		k, v, ok := strings.Cut(tok, "=")
		if !ok || v == "" {
			return true, "", scheme.Error(fmt.Sprintf("invalid option %q (use key=value)", tok))
		}
		switch k {
		case "dir":
			outDir = v
		case "group":
			co.Group = v
		case "os":
			co.GOOS = v
		default:
			return true, "", scheme.Error(fmt.Sprintf("unknown option %q (allowed: dir, group, os)", k))
		}
	}
	if !filepath.IsAbs(outDir) {
		outDir = filepath.Join(c.sandboxRoot, outDir)
	}
	script, err := cage.Generate(co)
	if err != nil {
		return true, "", scheme.Error(err.Error())
	}
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return true, "", scheme.Error(fmt.Sprintf("cannot create output dir: %v", err))
	}
	dest := filepath.Join(outDir, script.Filename)
	if err := os.WriteFile(dest, []byte(script.Body), 0o700); err != nil {
		return true, "", scheme.Error(fmt.Sprintf("cannot write cage script: %v", err))
	}
	var b strings.Builder
	b.WriteString(scheme.Info("cage", fmt.Sprintf("setup script for user %q (group %q, %s)", script.Username, script.Group, co.GOOS)) + "\n")
	b.WriteString(scheme.Info("path", dest) + "\n")
	b.WriteString(scheme.Info("note", "a random password was generated inside — change it there before running") + "\n")
	b.WriteString(scheme.Info("run", fmt.Sprintf("review it, then: sudo sh %s", dest)))
	return true, "", b.String()
}
