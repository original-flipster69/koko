package cli

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/original-flipster69/koko/internal/editor"
	"github.com/original-flipster69/koko/internal/ignore"
)

const visionMaxFiles = 1000

type vision struct{}

func (v vision) name() string { return "vision" }
func (v vision) desc() string { return "List files visible to the agent (after deny & ignore)" }
func (v vision) args() string { return "" }
func (v vision) do(opts cmdOpts) (bool, string, string) {
	scheme := opts.scheme()
	files, capped, err := visibleFiles(opts.a.Editor(), opts.a.Ignore())
	if err != nil {
		return true, "", scheme.Error(fmt.Sprintf("vision failed: %v", err))
	}
	if len(files) == 0 {
		return true, "", scheme.Info("vision", "no files visible")
	}
	var b strings.Builder
	for _, f := range files {
		b.WriteString("  " + f + "\n")
	}
	summary := fmt.Sprintf("%d files visible", len(files))
	if capped {
		summary += fmt.Sprintf(" (showing first %d)", visionMaxFiles)
	}
	return true, "", scheme.Info("vision", summary) + "\n" + strings.TrimRight(b.String(), "\n")
}

func visibleFiles(ed *editor.Editor, ig *ignore.Matcher) ([]string, bool, error) {
	var out []string
	capped := false
	err := ed.Walk(func(rel string, isDir bool) error {
		if ig.IsIgnored(rel, isDir) {
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}
		if isDir {
			return nil
		}
		if len(out) >= visionMaxFiles {
			capped = true
			return filepath.SkipAll
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		return nil, capped, err
	}
	sort.Strings(out)
	return out, capped, nil
}
