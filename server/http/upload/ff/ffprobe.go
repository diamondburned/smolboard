package ff

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"

	"github.com/pkg/errors"
)

type Size struct {
	Width  int
	Height int
}

func ProbeSize(path string) (*Size, error) {
	if err := acq(); err != nil {
		return nil, err
	}
	defer sema.Release(1)

	cmd := exec.Command(
		"ffprobe",
		"-v", "quiet",
		"-read_intervals", "%+#1", // 1 frame only
		"-select_streams", "v:0",
		"-print_format", "default=noprint_wrappers=1",
		"-show_entries", "stream=width,height", path,
	)

	// The output is small enough, so whatever.
	b, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to execute FFprobe")
	}

	var size Size

	for _, t := range bytes.Fields(b) {
		p := bytes.Split(t, []byte("="))
		if len(p) != 2 {
			return nil, fmt.Errorf("invalid line: %q", t)
		}

		i, err := strconv.Atoi(string(p[1]))
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse int from line %q", t)
		}

		switch string(p[0]) {
		case "width":
			size.Width = i
		case "height":
			size.Height = i
		}
	}

	return &size, nil
}
