package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/original-flipster69/koko/internal/ui"
)

type koko struct{}

func (k koko) name() string { return "koko" }
func (k koko) desc() string { return "Print the koko mascot" }
func (k koko) args() string { return "" }
func (k koko) do(opts cmdOpts) (bool, string, string) {
	return true, "", "\n" + ui.Mascot(opts.scheme)
}

type clear struct{}

func (c clear) name() string { return "clear" }
func (c clear) desc() string { return "Reset conversation history" }
func (c clear) args() string { return "" }
func (c clear) do(opts cmdOpts) (bool, string, string) {
	opts.a.ClearHistory()
	return true, "", opts.scheme.Info("cleared", "conversation history reset")
}

type history struct{}

func (h history) name() string { return "history" }
func (h history) desc() string { return "Show message count" }
func (h history) args() string { return "" }
func (h history) do(opts cmdOpts) (bool, string, string) {
	return true, "", opts.scheme.Info("messages", fmt.Sprintf("%d", opts.a.HistoryLen()))
}

type undo struct{}

func (u undo) name() string { return "undo" }
func (u undo) desc() string { return "Revert last file change" }
func (u undo) args() string { return "" }
func (u undo) do(opts cmdOpts) (bool, string, string) {
	path, err := opts.a.Undo()
	switch {
	case err != nil:
		return true, "", opts.scheme.Error(fmt.Sprintf("undo failed: %v", err))
	case path == "":
		return true, "", opts.scheme.Info("undo", "nothing to undo")
	default:
		return true, "", opts.scheme.Info("undo", fmt.Sprintf("reverted %s", path))
	}
}

type tokens struct{}

func (t tokens) name() string { return "tokens" }
func (t tokens) desc() string { return "Show token usage stats" }
func (t tokens) args() string { return "" }
func (t tokens) do(opts cmdOpts) (bool, string, string) {
	a, scheme := opts.a, opts.scheme
	var b strings.Builder
	b.WriteString(scheme.Info("input   ", fmt.Sprintf("%d tokens", a.TotalInput)) + "\n")
	b.WriteString(scheme.Info("output  ", fmt.Sprintf("%d tokens", a.TotalOutput)) + "\n")
	b.WriteString(scheme.Info("total   ", fmt.Sprintf("%d tokens", a.TotalInput+a.TotalOutput)) + "\n")
	b.WriteString(scheme.Info("messages", fmt.Sprintf("%d", a.HistoryLen())))
	return true, "", b.String()
}

type compact struct{}

func (c compact) name() string { return "compact" }
func (c compact) desc() string { return "Compress history to free context" }
func (c compact) args() string { return "" }
func (c compact) do(opts cmdOpts) (bool, string, string) {
	oldTokens, newTokens := opts.a.Compact()
	return true, "", opts.scheme.Info("compact", fmt.Sprintf("~%d → ~%d tokens", oldTokens, newTokens))
}

type plan struct{}

func (p plan) name() string { return "plan" }
func (p plan) desc() string { return "Toggle plan mode (read-only)" }
func (p plan) args() string { return "" }
func (p plan) do(opts cmdOpts) (bool, string, string) {
	if opts.a.TogglePlanMode() {
		return true, "", opts.scheme.Info("plan", "mode on — read-only; call :plan again to exit")
	}
	return true, "", opts.scheme.Info("plan", "mode off — full tools restored")
}

type help struct {
	cmds map[string]command
}

func (h help) name() string { return "help" }
func (h help) desc() string { return "Show this help" }
func (h help) args() string { return "" }
func (h help) do(_ cmdOpts) (bool, string, string) {
	names := make([]string, 0, len(h.cmds))
	for n := range h.cmds {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		display := n
		if h.cmds[n].args != "" {
			display = n + " " + h.cmds[n].args
		}
		b.WriteString(fmt.Sprintf("%-14s— %s\n", display, h.cmds[n].desc))
	}
	b.WriteString(fmt.Sprintf("%-14s— %s", ":<name>", "run a play by name (e.g. :review)"))
	return true, "", b.String()
}
