package ff

import (
	"bytes"
	"fmt"
	"image"
	"os/exec"

	"github.com/pkg/errors"
	"golang.org/x/image/bmp"
)

type ScalerAlgorithm string

const (
	// LanczosScaler is a decently fast, sharp and smooth rescaler algorithm.
	LanczosScaler ScalerAlgorithm = "lanczos"
	// NeighborScaler is a rough but very fast rescaler algorithm.
	NeighborScaler ScalerAlgorithm = "neighbor"
)

// FirstFrame gets the roughly-resized first frame.
func FirstFrame(path string, maxw, maxh int, s ScalerAlgorithm) (image.Image, error) {
	if err := acq(); err != nil {
		return nil, err
	}
	defer sema.Release(1)

	cmd := exec.Command(
		"ffmpeg",
		"-v", "quiet",
		"-i", path, "-vframes", "1",
		"-vf", fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=decrease", maxw, maxh),
		"-sws_flags", string(s),
		"-c:v", "bmp", "-pix_fmt", "bgra", "-f", "rawvideo", "-",
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

// FirstFrameJPEG gets the roughly-resized first frame in raw JPEG bytes.
func FirstFrameJPEG(path string, maxw, maxh int, s ScalerAlgorithm) ([]byte, error) {
	if err := acq(); err != nil {
		return nil, err
	}
	defer sema.Release(1)

	cmd := exec.Command(
		"ffmpeg",
		"-v", "error",
		"-i", path, "-vframes", "1",
		"-vf", fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=decrease", maxw, maxh),
		"-sws_flags", string(s),
		"-c:v", "mjpeg", "-q:v", "2", "-f", "rawvideo", "-",
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	b, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Failed to execute FFmpeg: %w\n%v", err, stderr.String())
	}

	return b, nil
}
