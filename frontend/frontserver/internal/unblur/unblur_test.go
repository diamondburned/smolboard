package unblur

import (
	"testing"

	"github.com/diamondburned/smolboard/smolboard"
)

var testdata = smolboard.PostAttribute{
	Width:    389,
	Height:   550,
	Blurhash: "LHQco0*0.m-V.Sn$=_-otSf6ROM{",
}

const jpegdata = "data:image/jpeg;base64,/9j/2wCEAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDIBCQkJDAsMGA0NGDIhHCEyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMv/AABEIAC0AHwMBIgACEQEDEQH/xAGiAAABBQEBAQEBAQAAAAAAAAAAAQIDBAUGBwgJCgsQAAIBAwMCBAMFBQQEAAABfQECAwAEEQUSITFBBhNRYQcicRQygZGhCCNCscEVUtHwJDNicoIJChYXGBkaJSYnKCkqNDU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6g4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2drh4uPk5ebn6Onq8fLz9PX29/j5+gEAAwEBAQEBAQEBAQAAAAAAAAECAwQFBgcICQoLEQACAQIEBAMEBwUEBAABAncAAQIDEQQFITEGEkFRB2FxEyIygQgUQpGhscEJIzNS8BVictEKFiQ04SXxFxgZGiYnKCkqNTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqCg4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2dri4+Tl5ufo6ery8/T19vf4+fr/2gAMAwEAAhEDEQA/APfX+6aypz89akpwhrFnf56uOxD3LERqfNU4W4qffU31LktC5cNiM1hTt89bE7ZWsacfNTeiFHVkkLcVPvqnGcVNuzUo0mtC67ZFUplq1UM3SrkZ09yn0NSqeKhb71Sp0qEazP/Z"

func TestUnblur(t *testing.T) {
	b, err := InlineJPEG(testdata.Blurhash, testdata.Width, testdata.Height)
	if err != nil {
		t.Fatal("JPEG failed:", err)
	}

	// if b != jpegdata {
	// 	t.Fatalf("Unexpected JPEG data: %q", b)
	// }

	t.Logf("JPEG size: %q", b)
}

func BenchmarkUnblur(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := InlineJPEG(testdata.Blurhash, testdata.Width, testdata.Height)
		if err != nil {
			b.Fatal("JPEG failed:", err)
		}
	}
}
