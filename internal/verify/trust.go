package verify

import (
	"bytes"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

type approval struct {
	Hash       string `toml:"hash"`
	ApprovedAt string `toml:"approved_at"`
}

type trustFile struct {
	Projects map[string]approval `toml:"projects"`
}

type TrustStore struct {
	path string
}

func NewTrustStore(homeKokoDir string) *TrustStore {
	return &TrustStore{path: filepath.Join(homeKokoDir, "trust.toml")}
}

func (s *TrustStore) Approved(projectPath, hash string) (bool, error) {
	tf, err := s.load()
	if err != nil {
		return false, err
	}
	a, ok := tf.Projects[projectPath]
	return ok && a.Hash == hash, nil
}

func (s *TrustStore) Approve(projectPath, hash string) error {
	tf, err := s.load()
	if err != nil {
		return err
	}
	if tf.Projects == nil {
		tf.Projects = make(map[string]approval)
	}
	tf.Projects[projectPath] = approval{
		Hash:       hash,
		ApprovedAt: time.Now().UTC().Format(time.RFC3339),
	}
	return s.save(tf)
}

func (s *TrustStore) load() (trustFile, error) {
	tf := trustFile{Projects: make(map[string]approval)}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return tf, nil
		}
		return trustFile{}, err
	}
	if _, err := toml.Decode(string(data), &tf); err != nil {
		return trustFile{}, err
	}
	return tf, nil
}

func (s *TrustStore) save(tf trustFile) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(tf); err != nil {
		return err
	}
	return os.WriteFile(s.path, buf.Bytes(), 0o600)
}
