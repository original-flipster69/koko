package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/meeseeks/koko/internal/sandbox"
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

func lockfileGuard(path string) error {
	base := filepath.Base(path)
	if manager, ok := lockfiles[base]; ok {
		return fmt.Errorf("refusing to modify lockfile %q directly — run `%s` via exec_command instead", base, manager)
	}
	return nil
}

type undoEntry struct {
	path    string
	content string
	existed bool
}

type Editor struct {
	sandbox   *sandbox.Sandbox
	mu        sync.Mutex
	undoStack []undoEntry
}

func New(sb *sandbox.Sandbox) *Editor {
	return &Editor{sandbox: sb}
}

func (e *Editor) Undo() (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.undoStack) == 0 {
		return "", nil
	}
	entry := e.undoStack[len(e.undoStack)-1]
	e.undoStack = e.undoStack[:len(e.undoStack)-1]
	if !entry.existed {
		if err := os.Remove(entry.path); err != nil && !os.IsNotExist(err) {
			return entry.path, fmt.Errorf("removing %s: %w", entry.path, err)
		}
		return entry.path, nil
	}
	if err := e.sandbox.WriteFile(entry.path, entry.content); err != nil {
		return entry.path, fmt.Errorf("restoring %s: %w", entry.path, err)
	}
	return entry.path, nil
}

func (e *Editor) saveUndo(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	content, err := e.sandbox.ReadFile(path)
	if err != nil {
		e.undoStack = append(e.undoStack, undoEntry{path: path, existed: false})
		return
	}
	e.undoStack = append(e.undoStack, undoEntry{path: path, content: content, existed: true})
}

func (e *Editor) ReadFile(path string) (string, error) {
	return e.sandbox.ReadFile(path)
}

func (e *Editor) WriteFile(path string, content string, overwrite bool) error {
	if err := lockfileGuard(path); err != nil {
		return err
	}
	if _, err := e.sandbox.ReadFile(path); err == nil && !overwrite {
		return fmt.Errorf("refusing to write %s: file already exists. Use replace_in_file for modifications, or pass overwrite=true ONLY when you explicitly intend a full rewrite", path)
	}
	e.saveUndo(path)
	return e.sandbox.WriteFile(path, content)
}

func (e *Editor) ReplaceInFile(path, oldText, newText string) (before string, after string, err error) {
	if err := lockfileGuard(path); err != nil {
		return "", "", err
	}
	content, err := e.sandbox.ReadFile(path)
	if err != nil {
		return "", "", err
	}

	if !strings.Contains(content, oldText) {
		return "", "", fmt.Errorf("old_text not found in %s — the string you passed does not appear anywhere in the file. old_text must match the file byte-for-byte including whitespace, punctuation, and line breaks. Current file content follows so you can copy the exact text:\n---\n%s\n---", path, previewForError(content))
	}

	count := strings.Count(content, oldText)
	if count > 1 {
		return "", "", fmt.Errorf("old_text appears %d times in %s — must be unique. Expand old_text with surrounding lines until it matches exactly one location", count, path)
	}

	e.saveUndo(path)
	updated := strings.Replace(content, oldText, newText, 1)
	if err := e.sandbox.WriteFile(path, updated); err != nil {
		return "", "", err
	}
	return content, updated, nil
}

func (e *Editor) RenameFile(oldPath, newPath string) error {
	e.saveUndo(oldPath)
	return e.sandbox.RenameFile(oldPath, newPath)
}

func (e *Editor) DeleteFile(path string) error {
	if err := lockfileGuard(path); err != nil {
		return err
	}
	e.saveUndo(path)
	return e.sandbox.DeleteFile(path)
}

func (e *Editor) ListDir(path string) ([]string, error) {
	return e.sandbox.ListDir(path)
}

func previewForError(content string) string {
	const maxBytes = 4096
	if len(content) <= maxBytes {
		return content
	}
	return content[:maxBytes] + "\n...(truncated, file is larger — call read_file for the full content)"
}
