package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/meeseeks/koko/internal/config"
)

type Sandbox struct {
	root        string
	allowedDirs []string
	denyFiles   []string
	ignore      *ignoreMatcher
	maxFileSize int64
}

func New(cfg *config.Config) *Sandbox {
	return &Sandbox{
		root:        cfg.SandboxRoot,
		allowedDirs: cfg.AllowedDirs,
		denyFiles:   cfg.DenyFiles,
		ignore:      newIgnoreMatcher(cfg.IgnoreFiles),
		maxFileSize: cfg.MaxFileSize,
	}
}

func (s *Sandbox) IsIgnored(path string) bool {
	if s.ignore == nil {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(s.root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return false
	}
	return s.ignore.matches(rel)
}

func (s *Sandbox) Root() string {
	return s.root
}

func (s *Sandbox) ValidatePath(path string) (string, error) {
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	evaluated, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("resolving symlinks: %w", err)
		}
		dirResolved, dirErr := filepath.EvalSymlinks(filepath.Dir(resolved))
		if dirErr != nil {
			if !os.IsNotExist(dirErr) {
				return "", fmt.Errorf("resolving symlinks: %w", dirErr)
			}
			dirResolved, _ = filepath.Abs(filepath.Dir(resolved))
		}
		evaluated = filepath.Join(dirResolved, filepath.Base(resolved))
	}

	allowed := false
	for _, dir := range s.allowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if evaluated == absDir {
			allowed = true
			break
		}
		rel, err := filepath.Rel(absDir, evaluated)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(rel, "..") {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", fmt.Errorf("path %q is outside allowed directories", path)
	}

	if s.IsIgnored(evaluated) {
		return "", fmt.Errorf("no such file or directory: %q", path)
	}

	if s.isDenied(evaluated) {
		return "", fmt.Errorf("path %q matches a denied file pattern", path)
	}

	return evaluated, nil
}

var binaryExtensions = map[string]bool{
	".exe": true, ".dll": true, ".so": true, ".dylib": true,
	".a": true, ".o": true, ".obj": true, ".class": true,
	".jar": true, ".war": true, ".pyc": true, ".pyo": true,
	".bin": true, ".dat": true, ".iso": true, ".img": true,
	".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".7z": true, ".rar": true,
	".mp3": true, ".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".wav": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true, ".tiff": true, ".ico": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
	".sqlite": true, ".db": true,
}

func hasBinaryExtension(path string) bool {
	return binaryExtensions[strings.ToLower(filepath.Ext(path))]
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

func ImageMimeType(path string) (string, bool) {
	mime, ok := imageExtensions[strings.ToLower(filepath.Ext(path))]
	return mime, ok
}

func (s *Sandbox) ReadImageFile(path string) ([]byte, string, error) {
	resolved, err := s.ValidatePath(path)
	if err != nil {
		return nil, "", err
	}
	mime, ok := imageExtensions[strings.ToLower(filepath.Ext(resolved))]
	if !ok {
		return nil, "", fmt.Errorf("%q is not a supported image format", path)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, "", fmt.Errorf("stat: %w", err)
	}
	maxImage := int64(10 * 1024 * 1024)
	if info.Size() > maxImage {
		return nil, "", fmt.Errorf("image %q exceeds 10MB limit", path)
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, "", fmt.Errorf("reading image: %w", err)
	}
	return data, mime, nil
}

func (s *Sandbox) ReadFile(path string) (string, error) {
	resolved, err := s.ValidatePath(path)
	if err != nil {
		return "", err
	}

	if hasBinaryExtension(resolved) {
		return "", fmt.Errorf("refusing to read binary file %q (extension blocked)", path)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat: %w", err)
	}
	if info.Size() > s.maxFileSize {
		return "", fmt.Errorf("file %q exceeds max size (%d bytes)", path, s.maxFileSize)
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}
	if looksBinary(data) {
		return "", fmt.Errorf("refusing to read %q: content appears to be binary", path)
	}
	return string(data), nil
}

func (s *Sandbox) WriteFile(path string, content string) error {
	resolved, err := s.ValidatePath(path)
	if err != nil {
		return err
	}

	if hasBinaryExtension(resolved) {
		return fmt.Errorf("refusing to write binary file %q (extension blocked)", path)
	}
	if looksBinary([]byte(content)) {
		return fmt.Errorf("refusing to write %q: content appears to be binary", path)
	}

	if int64(len(content)) > s.maxFileSize {
		return fmt.Errorf("content exceeds max file size (%d bytes)", s.maxFileSize)
	}

	dir := filepath.Dir(resolved)
	if err := os.MkdirAll(dir, 0775); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	if err := os.WriteFile(resolved, []byte(content), 0664); err != nil {
		return err
	}
	return os.Chmod(resolved, 0664)
}

func (s *Sandbox) ListDir(path string) ([]string, error) {
	resolved, err := s.ValidatePath(path)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %w", err)
	}

	var names []string
	for _, e := range entries {
		entryPath := filepath.Join(resolved, e.Name())
		if s.IsIgnored(entryPath) {
			continue
		}
		name := e.Name()
		if e.IsDir() {
			name += "/"
		} else if info, err := e.Info(); err == nil {
			name += fmt.Sprintf(" (%s)", humanSize(info.Size()))
		}
		names = append(names, name)
	}
	return names, nil
}

func (s *Sandbox) RenameFile(oldPath, newPath string) error {
	resolvedOld, err := s.ValidatePath(oldPath)
	if err != nil {
		return err
	}
	resolvedNew, err := s.ValidatePath(newPath)
	if err != nil {
		return err
	}
	dir := filepath.Dir(resolvedNew)
	if err := os.MkdirAll(dir, 0775); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}
	return os.Rename(resolvedOld, resolvedNew)
}

func (s *Sandbox) DeleteFile(path string) error {
	resolved, err := s.ValidatePath(path)
	if err != nil {
		return err
	}
	return os.Remove(resolved)
}

func humanSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1fM", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1fK", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
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
