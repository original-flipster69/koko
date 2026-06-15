package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	llmStreamTimeout = 5 * time.Minute
)

func isElevated() bool {
	if runtime.GOOS == "windows" {
		return os.Geteuid() == 0
	}
	return os.Getuid() == 0
}

func confirmElevated(r io.Reader, w io.Writer) bool {
	fmt.Fprintf(w, "Running with elevated privileges. Continue? [y/N] ")
	reader := bufio.NewReader(r)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}
