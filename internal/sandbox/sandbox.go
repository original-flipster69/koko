package sandbox

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type ValidPath string

type Sandbox struct {
	root        string
	allowedDirs []string
	denyFiles   []string
	maxFileSize int64
}

func New(root string, allowedDirs, denyFiles []string, maxFileSize int64) (*Sandbox, error) {
	absRoot, err := canonicalize(root)
	if err != nil {
		return nil, fmt.Errorf("sandbox root %q: %w", root, err)
	}
	canonical := make([]string, 0, len(allowedDirs))
	for _, d := range allowedDirs {
		abs, err := canonicalize(d)
		if err != nil {
			return nil, fmt.Errorf("allowed dir %q: %w", d, err)
		}
		canonical = append(canonical, abs)
	}
	for _, p := range denyFiles {
		if _, err := filepath.Match(p, ""); err != nil {
			return nil, fmt.Errorf("deny pattern %q: %w", p, err)
		}
	}
	return &Sandbox{
		root:        absRoot,
		allowedDirs: canonical,
		denyFiles:   denyFiles,
		maxFileSize: maxFileSize,
	}, nil
}

func canonicalize(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return resolveSymlinks(abs)
}

func resolveSymlinks(absPath string) (string, error) {
	if evaluated, err := filepath.EvalSymlinks(absPath); err == nil {
		return evaluated, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("resolving symlinks: %w", err)
	}
	cur := absPath
	var suffix []string
	for {
		parent := filepath.Dir(cur)
		if parent == cur {
			return absPath, nil
		}
		suffix = append([]string{filepath.Base(cur)}, suffix...)
		cur = parent
		existing, err := filepath.EvalSymlinks(cur)
		if err == nil {
			parts := append([]string{existing}, suffix...)
			return filepath.Join(parts...), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("resolving symlinks: %w", err)
		}
	}
}

func (s *Sandbox) Root() string {
	return s.root
}

func (s *Sandbox) ValidatePath(path string) (ValidPath, error) {
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	evaluated, err := resolveSymlinks(resolved)
	if err != nil {
		return "", err
	}

	allowed := false
	for _, absDir := range s.allowedDirs {
		if evaluated == absDir {
			allowed = true
			break
		}
		rel, err := filepath.Rel(absDir, evaluated)
		if err != nil {
			continue
		}
		if rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", fmt.Errorf("path %q is outside allowed directories", path)
	}

	if s.isDenied(evaluated) {
		return "", fmt.Errorf("path %q matches a denied file pattern", path)
	}

	return ValidPath(evaluated), nil
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

var imageExtensions = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
}

func ImgMimeType(path string) (string, bool) {
	mime, ok := imageExtensions[strings.ToLower(filepath.Ext(path))]
	return mime, ok
}

func (s *Sandbox) ReadImg(p ValidPath) ([]byte, string, error) {
	resolved := string(p)
	mime, ok := imageExtensions[strings.ToLower(filepath.Ext(resolved))]
	if !ok {
		return nil, "", fmt.Errorf("%q is not a supported image format", resolved)
	}
	file, err := os.OpenFile(resolved, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, "", fmt.Errorf("reading image: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("stat: %w", err)
	}
	if info.Size() > s.maxFileSize {
		return nil, "", fmt.Errorf("image %q exceeds max size (%d bytes)", resolved, s.maxFileSize)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, "", fmt.Errorf("reading image: %w", err)
	}
	return data, mime, nil
}

func (s *Sandbox) ReadFile(p ValidPath) (string, error) {
	resolved := string(p)
	file, err := os.OpenFile(resolved, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat: %w", err)
	}
	if info.Size() > s.maxFileSize {
		return "", fmt.Errorf("file %q exceeds max size (%d bytes)", resolved, s.maxFileSize)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}
	if looksBinary(data) {
		return "", fmt.Errorf("refusing to read %q: content appears to be binary", resolved)
	}
	return string(data), nil
}

func (s *Sandbox) WriteFile(p ValidPath, content string) error {
	resolved := string(p)
	if looksBinary([]byte(content)) {
		return fmt.Errorf("refusing to write %q: content appears to be binary", resolved)
	}

	if int64(len(content)) > s.maxFileSize {
		return fmt.Errorf("content exceeds max file size (%d bytes)", s.maxFileSize)
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

func (s *Sandbox) ListDir(p ValidPath) (string, []os.DirEntry, error) {
	resolved := string(p)
	entries, err := os.ReadDir(resolved)
	if err != nil {
		return "", nil, fmt.Errorf("reading directory: %w", err)
	}
	return resolved, entries, nil
}

func (s *Sandbox) RenameFile(oldPath, newPath ValidPath) error {
	resolvedNew := string(newPath)
	dir := filepath.Dir(resolvedNew)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}
	return os.Rename(string(oldPath), resolvedNew)
}

func (s *Sandbox) DeleteFile(p ValidPath) error {
	return os.Remove(string(p))
}

func (s *Sandbox) isDenied(path string) bool {
	base := filepath.Base(path)
	for _, pattern := range s.denyFiles {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	return false
}
