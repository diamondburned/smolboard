package unblur

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/jpeg"

	"github.com/bbrks/go-blurhash"
	"github.com/diamondburned/smolboard/server/httperr"
	"github.com/diamondburned/smolboard/smolboard"
	"github.com/pkg/errors"
)

const ThumbSize = 45
const Prefix = "data:image/jpeg;base64,"

var ErrHashUnavailable = httperr.New(404, "hash not available")

func InlinePost(p smolboard.Post) (string, error) {
	if p.Attributes.Blurhash == "" || p.Attributes.Height == 0 || p.Attributes.Width == 0 {
		return "", ErrHashUnavailable
	}

	return InlineJPEG(p.Attributes.Blurhash, p.Attributes.Width, p.Attributes.Height)
}

var JPEGOptions = &jpeg.Options{
	Quality: 65,
}

func InlineJPEG(hash string, w, h int) (string, error) {
	w, h = MaxSize(w, h, ThumbSize, ThumbSize)

	var rgba = image.NewRGBA(image.Rect(0, 0, w, h))

	if err := blurhash.DecodeDraw(rgba, hash, 1); err != nil {
		return "", errors.Wrap(err, "Failed to decode blurhash")
	}

	var b bytes.Buffer

	if err := jpeg.Encode(&b, rgba, JPEGOptions); err != nil {
		return "", errors.Wrap(err, "Failed to encode JPEG")
	}

	return Prefix + base64.StdEncoding.EncodeToString(b.Bytes()), nil
}

// from imgutil
func MaxSize(w, h, maxW, maxH int) (int, int) {
	if w < maxW && h < maxH {
		return w, h
	}

	if w > h {
		h = h * maxW / w
		w = maxW
	} else {
		w = w * maxH / h
		h = maxH
	}

	return w, h
}
