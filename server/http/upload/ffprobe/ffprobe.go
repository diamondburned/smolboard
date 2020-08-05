package ffprobe

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/image/bmp"
	"golang.org/x/sync/semaphore"
)

const waitDura = 5 * time.Second

var sema = semaphore.NewWeighted(int64(runtime.GOMAXPROCS(-1) * 2))

func acq() error {
	ctx, cancel := context.WithTimeout(context.Background(), waitDura)
	defer cancel()

	err := sema.Acquire(ctx, 1)
	return errors.Wrap(err, "Failed to wait for pending jobs")
}

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

// FirstFrame gets the roughly-resized first frame.
func FirstFrame(path string, maxw, maxh int) (image.Image, error) {
	if err := acq(); err != nil {
		return nil, err
	}
	defer sema.Release(1)

	cmd := exec.Command(
		"ffmpeg",
		"-v", "quiet",
		"-i", path, "-vframes", "1",
		"-vf", fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=decrease", maxw, maxh),
		"-sws_flags", "neighbor", // prioritize speed
		"-c:v", "bmp", "-f", "rawvideo", "-",
	)

	b, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to execute FFmpeg")
	}

	i, err := bmp.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to decode BMP")
	}

	return i, nil
}
