package audit

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type entry struct {
	Timestamp string            `json:"timestamp"`
	Tool      string            `json:"tool"`
	Args      map[string]string `json:"args"`
	Result    string            `json:"result"`
	PrevHash  string            `json:"prev_hash"`
	Hash      string            `json:"hash"`
}

type Log struct {
	mu       sync.Mutex
	file     *os.File
	lastHash string
	path     string
}

func NewLog(path string) (*Log, error) {
	last, err := loadLastHash(path)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	return &Log{file: f, lastHash: last, path: path}, nil
}

func loadLastHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	prev := ""
	line := 0
	for scanner.Scan() {
		line++
		var e entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			return "", fmt.Errorf("audit: corrupt entry at line %d: %w", line, err)
		}
		if e.Hash == "" {
			prev = ""
			continue
		}
		if e.PrevHash != prev {
			return "", fmt.Errorf("audit: chain broken at line %d (expected prev_hash %q, got %q)", line, prev, e.PrevHash)
		}
		expected := hashEntry(e)
		if expected != e.Hash {
			return "", fmt.Errorf("audit: tampered entry at line %d (hash mismatch)", line)
		}
		prev = e.Hash
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return prev, nil
}

func hashEntry(e entry) string {
	h := sha256.New()
	h.Write([]byte(e.PrevHash))
	h.Write([]byte(e.Timestamp))
	h.Write([]byte(e.Tool))
	argsJSON, _ := json.Marshal(e.Args)
	h.Write(argsJSON)
	h.Write([]byte(e.Result))
	return hex.EncodeToString(h.Sum(nil))
}

func (l *Log) Record(tool string, args map[string]string, result string) {
	if l == nil || l.file == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := entry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Tool:      tool,
		Args:      args,
		Result:    truncate(result, 2048),
		PrevHash:  l.lastHash,
	}
	entry.Hash = hashEntry(entry)
	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "audit: marshal error: %v\n", err)
		return
	}
	if _, err := l.file.Write(append(data, '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "audit: write error: %v\n", err)
		return
	}
	l.lastHash = entry.Hash
}

func (l *Log) Close() {
	if l == nil || l.file == nil {
		return
	}
	l.file.Close()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}
