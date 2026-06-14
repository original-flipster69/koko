package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/original-flipster69/koko/internal/cli"
	"github.com/original-flipster69/koko/internal/ui"
)

var version = "dev"

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

	if err := cli.Main(cli.Options{
		Provider:   *providerFlag,
		Model:      *modelFlag,
		LlmURL:     *llmUrlFlag,
		Sandbox:    *sandboxFlag,
		ConfigPath: *configFlag,
	}); err != nil {
		fmt.Fprintln(os.Stderr, ui.DefaultScheme().Error(err.Error()))
		os.Exit(1)
	}
}
