package imgsrv

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/diamondburned/smolboard/smolboard/http/internal/tx"
	"github.com/go-chi/chi"
	"github.com/pkg/errors"
)

type ctxKey uint8

const (
	keyPostID ctxKey = iota
	keyFileName
)

func parseID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var fileName = chi.URLParam(r, "file")

		i, err := strconv.ParseInt(trimExt(fileName), 10, 64)
		if err != nil {
			tx.RenderError(w, errors.Wrap(err, "Failed to parse post ID"))
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, keyPostID, i)
		ctx = context.WithValue(ctx, keyFileName, fileName)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getStored(r tx.Request) (postID int64, fileName string) {
	var ctx = r.Context()

	if v, ok := ctx.Value(keyPostID).(int64); ok {
		postID = v
	}

	if s, ok := ctx.Value(keyFileName).(string); ok {
		fileName = s
	}

	return
}

func trimExt(name string) string {
	switch parts := strings.Split(name, "."); len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	default:
		return strings.Join(parts[:len(parts)-1], ".")
	}
}
