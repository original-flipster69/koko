package verify

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

const defaultStageTimeout = 1 * time.Minute

type Stage struct {
	Name    string
	Command string
	Fast    bool
	Timeout time.Duration
}

type Pipeline struct {
	Stages []Stage
	Hash   string
}

type rawPipeline struct {
	Stages []rawStage `toml:"stage"`
}

type rawStage struct {
	Name    string `toml:"name"`
	Command string `toml:"cmd"`
	Fast    bool   `toml:"fast"`
	Timeout string `toml:"timeout"`
}

func Load(path string) (Pipeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Pipeline{}, err
	}
	return parse(data)
}

func parse(data []byte) (Pipeline, error) {
	var raw rawPipeline
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return Pipeline{}, fmt.Errorf("invalid pipeline definition: %w", err)
	}
	if len(raw.Stages) == 0 {
		return Pipeline{}, errors.New("pipeline defines no stages")
	}

	stages := make([]Stage, 0, len(raw.Stages))
	for i, rs := range raw.Stages {
		if rs.Name == "" || rs.Command == "" {
			return Pipeline{}, fmt.Errorf("stage %d: both name and cmd are required", i)
		}
		timeout := defaultStageTimeout
		if rs.Timeout != "" {
			d, err := time.ParseDuration(rs.Timeout)
			if err != nil {
				return Pipeline{}, fmt.Errorf("stage %q: invalid timeout %q: %w", rs.Name, rs.Timeout, err)
			}
			timeout = d
		}
		stages = append(stages, Stage{Name: rs.Name, Command: rs.Command, Fast: rs.Fast, Timeout: timeout})
	}

	sum := sha256.Sum256(data)
	return Pipeline{Stages: stages, Hash: hex.EncodeToString(sum[:])}, nil
}

func (p Pipeline) Commands() []string {
	cmds := make([]string, len(p.Stages))
	for i, s := range p.Stages {
		cmds[i] = s.Command
	}
	return cmds
}
