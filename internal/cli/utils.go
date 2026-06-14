package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/original-flipster69/koko/internal/config"
)

const (
	llmStreamTimeout = 5 * time.Minute
)

func KokoDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("APPDATA"), "koko")
	}
	return filepath.Join(os.Getenv("HOME"), ".koko")
}

type ReloadSources struct {
	cfgPath  string
	provider string
	model    string
	llmURL   string
	sandbox  string
}

func loadConfig(src ReloadSources) (*config.Config, error) {
	cfg, err := config.Load(src.cfgPath)
	if err != nil {
		return nil, err
	}
	if src.provider != "" {
		cfg.Llm.Provider = config.Provider(src.provider)
	}
	if src.model != "" {
		cfg.Llm.Model = src.model
	}
	if src.llmURL != "" {
		cfg.Llm.Url = src.llmURL
	}
	if src.sandbox != "" {
		cfg.Sandbox.Root = src.sandbox
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
