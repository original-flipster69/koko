package editor

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/original-flipster69/koko/internal/sandbox"
)

var lockfiles = map[string]string{
	"package-lock.json": "npm install / npm ci",
	"yarn.lock":         "yarn install",
	"pnpm-lock.yaml":    "pnpm install",
	"go.sum":            "go mod tidy / go get",
	"Cargo.lock":        "cargo update / cargo build",
	"poetry.lock":       "poetry lock / poetry install",
	"Pipfile.lock":      "pipenv lock / pipenv install",
	"Gemfile.lock":      "bundle install",
	"composer.lock":     "composer update",
	"mix.lock":          "mix deps.get",
	"pdm.lock":          "pdm lock / pdm install",
	"uv.lock":           "uv lock",
}

func lockfileGuard(path sandbox.ValidPath) error {
	base := filepath.Base(string(path))
	if manager, ok := lockfiles[base]; ok {
		return fmt.Errorf("refusing to modify lockfile %q directly — run `%s` via exec_command instead", base, manager)
	}
	return nil
}

type undoEntry struct {
	path    sandbox.ValidPath
	content string
	existed bool
}

type Editor struct {
	sandbox   *sandbox.Sandbox
	mu        sync.Mutex
	undoStack []undoEntry
	reads     map[sandbox.ValidPath][32]byte
}

func New(sb *sandbox.Sandbox) *Editor {
	return &Editor{sandbox: sb, reads: make(map[sandbox.ValidPath][32]byte)}
}

func (e *Editor) MarkRead(path sandbox.ValidPath, content string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.reads[path] = sha256.Sum256([]byte(content))
}

func (e *Editor) ForgetRead(path sandbox.ValidPath) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.reads, path)
}

func (e *Editor) readHash(path sandbox.ValidPath) ([32]byte, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	h, ok := e.reads[path]
	return h, ok
}

func (e *Editor) Undo() (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.undoStack) == 0 {
		return "", nil
	}
	entry := e.undoStack[len(e.undoStack)-1]
	e.undoStack = e.undoStack[:len(e.undoStack)-1]
	pathStr := string(entry.path)
	if !entry.existed {
		if err := e.sandbox.DeleteFile(entry.path); err != nil && !os.IsNotExist(err) {
			return pathStr, fmt.Errorf("removing %s: %w", pathStr, err)
		}
		return pathStr, nil
	}
	if err := e.sandbox.WriteFile(entry.path, entry.content); err != nil {
		return pathStr, fmt.Errorf("restoring %s: %w", pathStr, err)
	}
	return pathStr, nil
}

func (e *Editor) saveUndo(path sandbox.ValidPath) {
	e.mu.Lock()
	defer e.mu.Unlock()

	content, err := e.sandbox.ReadFile(path)
	if err != nil {
		e.undoStack = append(e.undoStack, undoEntry{path: path, existed: false})
		return
	}
	e.undoStack = append(e.undoStack, undoEntry{path: path, content: content, existed: true})
}

func (e *Editor) ReadFile(path sandbox.ValidPath) (string, error) {
	return e.sandbox.ReadFile(path)
}

func (e *Editor) WriteFile(path sandbox.ValidPath, content string, overwrite bool) error {
	if err := lockfileGuard(path); err != nil {
		return err
	}
	if _, err := e.sandbox.ReadFile(path); err == nil && !overwrite {
		return fmt.Errorf("refusing to write %s: file already exists. Use replace_in_file for modifications, or pass overwrite=true ONLY when you explicitly intend a full rewrite", path)
	}
	e.saveUndo(path)
	if err := e.sandbox.WriteFile(path, content); err != nil {
		return err
	}
	e.MarkRead(path, content)
	return nil
}

func (e *Editor) ReplaceInFile(path sandbox.ValidPath, oldText, newText string) (before string, after string, err error) {
	if err := lockfileGuard(path); err != nil {
		return "", "", err
	}
	known, ok := e.readHash(path)
	if !ok {
		return "", "", fmt.Errorf("refusing to edit %s — file has not been read in this session. Call read_file on this path first (with no offset/limit, so you see the full content), then retry replace_in_file with byte-exact old_text copied from the read output", path)
	}
	content, err := e.sandbox.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	current := sha256.Sum256([]byte(content))
	if current != known {
		e.ForgetRead(path)
		return "", "", fmt.Errorf("refusing to edit %s — file content changed on disk since last read. Re-read the file and retry replace_in_file with fresh old_text", path)
	}

	commit := func(updated string) (string, string, error) {
		e.saveUndo(path)
		if err := e.sandbox.WriteFile(path, updated); err != nil {
			return "", "", err
		}
		e.MarkRead(path, updated)
		return content, updated, nil
	}

	count := strings.Count(content, oldText)
	if count == 1 {
		return commit(strings.Replace(content, oldText, newText, 1))
	}
	if count > 1 {
		return "", "", fmt.Errorf("old_text appears %d times in %s — must be unique. Expand old_text with surrounding lines until it matches exactly one location", count, path)
	}

	if start, end, matches, ok := fuzzyWhitespaceMatch(content, oldText); ok {
		return commit(content[:start] + newText + content[end:])
	} else if matches > 1 {
		return "", "", fmt.Errorf("old_text not found exactly, and whitespace-tolerant match is ambiguous (%d candidates) in %s — expand old_text with more surrounding context", matches, path)
	}

	if start, end, matches, ok := trimmedLineMatch(content, oldText); ok {
		replacement := newText
		if end > 0 && end <= len(content) && content[end-1] == '\n' && !strings.HasSuffix(replacement, "\n") {
			replacement += "\n"
		}
		return commit(content[:start] + replacement + content[end:])
	} else if matches > 1 {
		return "", "", fmt.Errorf("old_text not found exactly, and trimmed-line match is ambiguous (%d candidates) in %s — expand old_text with more surrounding context", matches, path)
	}

	return "", "", fmt.Errorf("old_text not found in %s — the string you passed does not appear anywhere in the file. old_text must match the file byte-for-byte including whitespace, punctuation, and line breaks. Current file content follows so you can copy the exact text:\n---\n%s\n---", path, previewForError(content))
}

func trimmedLineMatch(content, oldText string) (int, int, int, bool) {
	rawOld := strings.Split(oldText, "\n")
	oldLines := make([]string, 0, len(rawOld))
	for _, l := range rawOld {
		oldLines = append(oldLines, strings.TrimSpace(l))
	}
	for len(oldLines) > 0 && oldLines[0] == "" {
		oldLines = oldLines[1:]
	}
	for len(oldLines) > 0 && oldLines[len(oldLines)-1] == "" {
		oldLines = oldLines[:len(oldLines)-1]
	}
	if len(oldLines) == 0 {
		return 0, 0, 0, false
	}

	contentLines := strings.Split(content, "\n")
	trimmed := make([]string, len(contentLines))
	for i, l := range contentLines {
		trimmed[i] = strings.TrimSpace(l)
	}

	var hits [][2]int
	for i := 0; i+len(oldLines) <= len(trimmed); i++ {
		ok := true
		for j, ol := range oldLines {
			if trimmed[i+j] != ol {
				ok = false
				break
			}
		}
		if ok {
			hits = append(hits, [2]int{i, i + len(oldLines)})
		}
	}
	if len(hits) != 1 {
		return 0, 0, len(hits), false
	}

	startLine, endLine := hits[0][0], hits[0][1]
	start := 0
	for i := 0; i < startLine; i++ {
		start += len(contentLines[i]) + 1
	}
	end := start
	for i := startLine; i < endLine; i++ {
		end += len(contentLines[i]) + 1
	}
	if end > len(content) {
		end = len(content)
	}
	return start, end, 1, true
}

func fuzzyWhitespaceMatch(content, oldText string) (int, int, int, bool) {
	if strings.TrimSpace(oldText) == "" {
		return 0, 0, 0, false
	}
	var b strings.Builder
	inWS := false
	hasWS := false
	for _, r := range oldText {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !inWS {
				b.WriteString(`\s+`)
				inWS = true
				hasWS = true
			}
			continue
		}
		inWS = false
		b.WriteString(regexp.QuoteMeta(string(r)))
	}
	if !hasWS {
		return 0, 0, 0, false
	}
	re, err := regexp.Compile(b.String())
	if err != nil {
		return 0, 0, 0, false
	}
	matches := re.FindAllStringIndex(content, -1)
	if len(matches) == 1 {
		return matches[0][0], matches[0][1], 1, true
	}
	return 0, 0, len(matches), false
}

func (e *Editor) RenameFile(oldPath, newPath sandbox.ValidPath) error {
	e.saveUndo(oldPath)
	if err := e.sandbox.RenameFile(oldPath, newPath); err != nil {
		return err
	}
	e.ForgetRead(oldPath)
	e.ForgetRead(newPath)
	return nil
}

func (e *Editor) DeleteFile(path sandbox.ValidPath) error {
	if err := lockfileGuard(path); err != nil {
		return err
	}
	e.saveUndo(path)
	if err := e.sandbox.DeleteFile(path); err != nil {
		return err
	}
	e.ForgetRead(path)
	return nil
}

func (e *Editor) ListDir(path sandbox.ValidPath) (string, []os.DirEntry, error) {
	return e.sandbox.ListDir(path)
}

func previewForError(content string) string {
	const maxBytes = 4096
	if len(content) <= maxBytes {
		return content
	}
	return content[:maxBytes] + "\n...(truncated, file is larger — call read_file for the full content)"
}
