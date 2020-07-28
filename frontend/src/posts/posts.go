package posts

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"strconv"

	"github.com/bbrks/go-blurhash"
	"github.com/diamondburned/smolboard/client"
	"github.com/diamondburned/smolboard/smolboard"
	"github.com/vugu/vgrouter"
)

const (
	BlurPreviewWidth  = 150
	BlurPreviewHeight = 150
)

var encoder = png.Encoder{
	CompressionLevel: png.NoCompression,
}

type Posts struct {
	vgrouter.NavigatorRef
	Items   []smolboard.Post
	Total   int
	Session *client.Session
}

func NewPosts(s *client.Session) *Posts {
	return &Posts{
		Session: s,
	}
}

func (p *Posts) SetResults(results smolboard.SearchResults) {
	p.Items = results.Posts
	p.Total = results.Total
}

func (p *Posts) GoTo(post smolboard.Post) {
	p.Navigate("/posts/"+post.Filename(), nil)
}

// func (p *Posts) SetPages()

func (p *Posts) BackgroundImage(post smolboard.Post) string {
	if inline := BlurInlineImage(post); inline != "" {
		return fmt.Sprintf(
			"background-image: url('%s'), url(%s), url(%s)",
			BlurInlineImage(post),
			p.Session.PostThumbURL(post),
			p.Session.PostImageURL(post),
		)
	}

	return fmt.Sprintf(
		"background-image: url(%s), url(%s)",
		p.Session.PostThumbURL(post),
		p.Session.PostImageURL(post),
	)
}

func PostURL(post smolboard.Post) string {
	return "/posts/" + strconv.FormatInt(post.ID, 10)
}

func BlurInlineImage(post smolboard.Post) string {
	if post.Attributes.Blurhash == "" {
		return ""
	}

	// Cap the width and height.
	var w, h = maxSize(
		post.Attributes.Width, post.Attributes.Height,
		BlurPreviewWidth, BlurPreviewHeight,
	)

	i, err := blurhash.Decode(post.Attributes.Blurhash, w, h, 1)
	if err != nil {
		return ""
	}

	var buf bytes.Buffer

	if err := encoder.Encode(&buf, i); err != nil {
		return ""
	}

	return "data:image/png;base64," +
		base64.RawURLEncoding.EncodeToString(buf.Bytes())
}

// from imgutil
func maxSize(w, h, maxW, maxH int) (int, int) {
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
