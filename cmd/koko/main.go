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
	provider := flag.String("provider", "", "LLM provider: anthropic, mistral, ollama")
	model := flag.String("model", "", "Model name to use")
	llmUrl := flag.String("llm-url", "", "URL for LLM API (useful for local LLMs)")
	sandbox := flag.String("sandbox", "", "Sandbox root directory (defaults to cwd)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	if err := cli.Main(cli.Flags{
		Provider: *provider,
		Model:    *model,
		LlmURL:   *llmUrl,
		Sandbox:  *sandbox,
	}); err != nil {
		fmt.Fprintln(os.Stderr, ui.DefaultScheme().Error(err.Error()))
		os.Exit(1)
	}
}
