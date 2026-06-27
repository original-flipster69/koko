package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func (s *Sandbox) MaxFileSize() int64 {
	return s.maxFileSize
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

	if seg := protectedSegment(evaluated); seg != "" {
		return "", fmt.Errorf("path %q is inside a %s directory (always denied)", path, seg)
	}

	if s.isDenied(evaluated) {
		return "", fmt.Errorf("path %q matches a denied file pattern", path)
	}

	return ValidPath(evaluated), nil
}

var protectedDirs = []string{".git", ".koko"}

func protectedSegment(path string) string {
	for _, seg := range strings.Split(filepath.ToSlash(path), "/") {
		for _, protected := range protectedDirs {
			if seg == protected {
				return protected
			}
		}
	}
	return ""
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

func (s *Sandbox) isDenied(path string) bool {
	base := filepath.Base(path)
	for _, pattern := range s.denyFiles {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	return false
}
