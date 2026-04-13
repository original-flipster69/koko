package detect

import (
	"os"
	"path/filepath"
	"strings"
)

type ProjectInfo struct {
	Languages  []string
	Frameworks []string
	BuildTools []string
	HasGit     bool
}

var markers = []struct {
	file      string
	language  string
	framework string
	buildTool string
}{
	{"go.mod", "Go", "", "go"},
	{"Cargo.toml", "Rust", "", "cargo"},
	{"package.json", "JavaScript/TypeScript", "", "npm"},
	{"yarn.lock", "", "", "yarn"},
	{"pnpm-lock.yaml", "", "", "pnpm"},
	{"tsconfig.json", "TypeScript", "", ""},
	{"requirements.txt", "Python", "", "pip"},
	{"pyproject.toml", "Python", "", ""},
	{"setup.py", "Python", "", ""},
	{"Pipfile", "Python", "", "pipenv"},
	{"Gemfile", "Ruby", "", "bundler"},
	{"pom.xml", "Java", "", "maven"},
	{"build.gradle", "Java/Kotlin", "", "gradle"},
	{"CMakeLists.txt", "C/C++", "", "cmake"},
	{"Makefile", "", "", "make"},
	{"Dockerfile", "", "Docker", ""},
	{"docker-compose.yml", "", "Docker Compose", ""},
	{"next.config.js", "", "Next.js", ""},
	{"next.config.mjs", "", "Next.js", ""},
	{"nuxt.config.ts", "", "Nuxt", ""},
	{"angular.json", "", "Angular", ""},
	{"svelte.config.js", "", "Svelte", ""},
	{"remix.config.js", "", "Remix", ""},
	{"tailwind.config.js", "", "Tailwind CSS", ""},
	{".eslintrc.json", "", "", "eslint"},
	{".prettierrc", "", "", "prettier"},
}

func Project(root string) ProjectInfo {
	info := ProjectInfo{}
	seen := map[string]bool{}

	_, err := os.Stat(filepath.Join(root, ".git"))
	info.HasGit = err == nil

	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(root, m.file)); err != nil {
			continue
		}
		if m.language != "" && !seen["l:"+m.language] {
			info.Languages = append(info.Languages, m.language)
			seen["l:"+m.language] = true
		}
		if m.framework != "" && !seen["f:"+m.framework] {
			info.Frameworks = append(info.Frameworks, m.framework)
			seen["f:"+m.framework] = true
		}
		if m.buildTool != "" && !seen["b:"+m.buildTool] {
			info.BuildTools = append(info.BuildTools, m.buildTool)
			seen["b:"+m.buildTool] = true
		}
	}

	return info
}

func (p ProjectInfo) Summary() string {
	if len(p.Languages) == 0 && len(p.Frameworks) == 0 {
		return ""
	}
	var parts []string
	if len(p.Languages) > 0 {
		parts = append(parts, "Languages: "+strings.Join(p.Languages, ", "))
	}
	if len(p.Frameworks) > 0 {
		parts = append(parts, "Frameworks: "+strings.Join(p.Frameworks, ", "))
	}
	if len(p.BuildTools) > 0 {
		parts = append(parts, "Build tools: "+strings.Join(p.BuildTools, ", "))
	}
	if p.HasGit {
		parts = append(parts, "Version control: git")
	}
	return strings.Join(parts, "\n")
}
