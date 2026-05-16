package project

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type Stack struct {
	Detected []string
	HasGit   bool
}

var markers = []struct {
	file  string
	label string
}{
	{"go.mod", "Go"},
	{"Cargo.toml", "Rust"},
	{"package.json", "JavaScript/TypeScript"},
	{"yarn.lock", "yarn"},
	{"pnpm-lock.yaml", "pnpm"},
	{"tsconfig.json", "TypeScript"},
	{"requirements.txt", "Python"},
	{"pyproject.toml", "Python"},
	{"setup.py", "Python"},
	{"Pipfile", "Python"},
	{"Gemfile", "Ruby"},
	{"pom.xml", "Java"},
	{"build.gradle", "Java/Kotlin"},
	{"CMakeLists.txt", "C/C++"},
	{"Makefile", "make"},
	{"Dockerfile", "Docker"},
	{"docker-compose.yml", "Docker Compose"},
	{"next.config.js", "Next.js"},
	{"next.config.mjs", "Next.js"},
	{"nuxt.config.ts", "Nuxt"},
	{"angular.json", "Angular"},
	{"svelte.config.js", "Svelte"},
	{"remix.config.js", "Remix"},
	{"tailwind.config.js", "Tailwind CSS"},
	{".eslintrc.json", "eslint"},
	{".prettierrc", "prettier"},
}

func Scan(root string) Stack {
	info := Stack{}

	_, err := os.Stat(filepath.Join(root, ".git"))
	info.HasGit = err == nil

	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(root, m.file)); err != nil {
			continue
		}
		if !slices.Contains(info.Detected, m.label) {
			info.Detected = append(info.Detected, m.label)
		}
	}

	return info
}

func (p Stack) Summary() string {
	if len(p.Detected) == 0 && !p.HasGit {
		return ""
	}
	var parts []string
	if len(p.Detected) > 0 {
		parts = append(parts, "Stack: "+strings.Join(p.Detected, ", "))
	}
	if p.HasGit {
		parts = append(parts, "Version control: git")
	}
	return strings.Join(parts, "\n")
}
