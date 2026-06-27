package editor

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"

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
	return &Editor{
		sandbox: sb,
		reads:   make(map[sandbox.ValidPath][32]byte),
	}
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
		if err := os.Remove(pathStr); err != nil && !os.IsNotExist(err) {
			return pathStr, fmt.Errorf("removing %s: %w", pathStr, err)
		}
		return pathStr, nil
	}
	if err := e.writeBytes(entry.path, entry.content); err != nil {
		return pathStr, fmt.Errorf("restoring %s: %w", pathStr, err)
	}
	return pathStr, nil
}

func (e *Editor) saveUndo(path sandbox.ValidPath) {
	e.mu.Lock()
	defer e.mu.Unlock()

	content, err := e.readBytes(path)
	if err != nil {
		e.undoStack = append(e.undoStack, undoEntry{path: path, existed: false})
		return
	}
	e.undoStack = append(e.undoStack, undoEntry{path: path, content: content, existed: true})
}

func (e *Editor) readBytes(path sandbox.ValidPath) (string, error) {
	resolved := string(path)
	file, err := os.OpenFile(resolved, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat: %w", err)
	}
	if info.Size() > e.sandbox.MaxFileSize() {
		return "", fmt.Errorf("file %q exceeds max size (%d bytes)", resolved, e.sandbox.MaxFileSize())
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}
	return string(data), nil
}

func (e *Editor) writeBytes(path sandbox.ValidPath, content string) error {
	resolved := string(path)
	max := e.sandbox.MaxFileSize()
	if int64(len(content)) > max {
		return fmt.Errorf("content exceeds max file size (%d bytes)", max)
	}
	dir := filepath.Dir(resolved)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}
	existed := false
	if _, err := os.Lstat(resolved); err == nil {
		existed = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat: %w", err)
	}
	file, err := os.OpenFile(resolved, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, 0640)
	if err != nil {
		return fmt.Errorf("writing file: %w", err)
	}
	if _, err := file.Write([]byte(content)); err != nil {
		file.Close()
		return fmt.Errorf("writing file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}
	if !existed {
		return os.Chmod(resolved, 0640)
	}
	return nil
}

func (e *Editor) Read(path sandbox.ValidPath) (string, error) {
	content, err := e.readBytes(path)
	if err != nil {
		return "", err
	}
	if looksBinary([]byte(content)) {
		return "", fmt.Errorf("refusing to read %q: content appears to be binary", path)
	}
	return content, nil
}

func (e *Editor) ReadImg(path sandbox.ValidPath) ([]byte, string, error) {
	mime, ok := sandbox.ImgMimeType(string(path))
	if !ok {
		return nil, "", fmt.Errorf("%q is not a supported image format", path)
	}
	resolved := string(path)
	file, err := os.OpenFile(resolved, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, "", fmt.Errorf("reading image: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("stat: %w", err)
	}
	if info.Size() > e.sandbox.MaxFileSize() {
		return nil, "", fmt.Errorf("image %q exceeds max size (%d bytes)", resolved, e.sandbox.MaxFileSize())
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, "", fmt.Errorf("reading image: %w", err)
	}
	return data, mime, nil
}

func (e *Editor) Write(path sandbox.ValidPath, content string, overwrite bool) error {
	if err := lockfileGuard(path); err != nil {
		return err
	}
	if looksBinary([]byte(content)) {
		return fmt.Errorf("refusing to write %q: content appears to be binary", path)
	}
	if _, err := os.Stat(string(path)); err == nil && !overwrite {
		return fmt.Errorf("refusing to write %s: file already exists. Use replace_in_file for modifications, or pass overwrite=true ONLY when you explicitly intend a full rewrite", path)
	}
	e.saveUndo(path)
	if err := e.writeBytes(path, content); err != nil {
		return err
	}
	e.MarkRead(path, content)
	return nil
}

func (e *Editor) Replace(path sandbox.ValidPath, oldText, newText string) (before string, after string, err error) {
	if err := lockfileGuard(path); err != nil {
		return "", "", err
	}
	known, ok := e.readHash(path)
	if !ok {
		return "", "", fmt.Errorf("refusing to edit %s — file has not been read in this session. Call read_file on this path first (with no offset/limit, so you see the full content), then retry replace_in_file with byte-exact old_text copied from the read output", path)
	}
	content, err := e.readBytes(path)
	if err != nil {
		return "", "", err
	}
	current := sha256.Sum256([]byte(content))
	if current != known {
		e.ForgetRead(path)
		return "", "", fmt.Errorf("refusing to edit %s — file content changed on disk since last read. Re-read the file and retry replace_in_file with fresh old_text", path)
	}

	commit := func(updated string) (string, string, error) {
		if looksBinary([]byte(updated)) {
			return "", "", fmt.Errorf("refusing to write %q: content appears to be binary", path)
		}
		e.saveUndo(path)
		if err := e.writeBytes(path, updated); err != nil {
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

func (e *Editor) Rename(oldPath, newPath sandbox.ValidPath) error {
	e.saveUndo(oldPath)
	dir := filepath.Dir(string(newPath))
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}
	if err := os.Rename(string(oldPath), string(newPath)); err != nil {
		return err
	}
	e.ForgetRead(oldPath)
	e.ForgetRead(newPath)
	return nil
}

func (e *Editor) Delete(path sandbox.ValidPath) error {
	if err := lockfileGuard(path); err != nil {
		return err
	}
	e.saveUndo(path)
	if err := os.Remove(string(path)); err != nil {
		return err
	}
	e.ForgetRead(path)
	return nil
}

func (e *Editor) List(path sandbox.ValidPath) (string, []os.DirEntry, error) {
	resolved := string(path)
	entries, err := os.ReadDir(resolved)
	if err != nil {
		return "", nil, fmt.Errorf("reading directory: %w", err)
	}
	return resolved, entries, nil
}

// Walk traverses the sandbox tree depth-first, validating every directory
// through the sandbox before reading it and skipping any entry that fails
// validation (denied, outside the sandbox, or unresolvable). For each
// validated entry it calls visit with the path relative to the sandbox root.
// Returning filepath.SkipDir from visit prunes a directory; filepath.SkipAll
// stops the walk.
func (e *Editor) Walk(visit func(rel string, isDir bool) error) error {
	root := e.sandbox.Root()
	vp, err := e.sandbox.ValidatePath(root)
	if err != nil {
		return err
	}
	if err := e.walk(root, vp, visit); err != nil && !errors.Is(err, filepath.SkipAll) {
		return err
	}
	return nil
}

func (e *Editor) walk(root string, dir sandbox.ValidPath, visit func(rel string, isDir bool) error) error {
	_, entries, err := e.List(dir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		full := filepath.Join(string(dir), entry.Name())
		vp, err := e.sandbox.ValidatePath(full)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(root, full)
		if err != nil {
			continue
		}
		isDir := entry.IsDir()
		if verr := visit(filepath.ToSlash(rel), isDir); verr != nil {
			if errors.Is(verr, filepath.SkipDir) {
				continue
			}
			return verr
		}
		if isDir {
			if werr := e.walk(root, vp, visit); werr != nil {
				return werr
			}
		}
	}
	return nil
}

func looksBinary(data []byte) bool {
	n := len(data)
	if n == 0 {
		return false
	}
	if n > 512 {
		n = 512
	}
	nonPrint := 0
	for i := 0; i < n; i++ {
		c := data[i]
		if c == 0 {
			return true
		}
		if c < 9 || (c > 13 && c < 32) || c == 127 {
			nonPrint++
		}
	}
	return nonPrint*100/n > 30
}

func previewForError(content string) string {
	const maxBytes = 4096
	if len(content) <= maxBytes {
		return content
	}
	return content[:maxBytes] + "\n...(truncated, file is larger — call read_file for the full content)"
}
