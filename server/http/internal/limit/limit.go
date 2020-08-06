package limit

import (
	"net/http"
	"time"

	"github.com/diamondburned/smolboard/server/http/internal/middleware"
	"github.com/diamondburned/smolboard/server/http/internal/tx"
	"github.com/didip/tollbooth/v6"
	"github.com/didip/tollbooth/v6/errors"
	"github.com/didip/tollbooth/v6/limiter"
)

func RateLimit(n float64) func(http.Handler) http.Handler {
	l := tollbooth.NewLimiter(n, &limiter.ExpirableOptions{
		DefaultExpirationTTL: time.Hour,
	})
	l.SetIPLookups([]string{"X-Forwarded-For", "RemoteAddr", "X-Real-IP"})
	l.SetHeader("Cookie", []string{"token"})

	return middleware.P(func(w http.ResponseWriter, r *http.Request) bool {
		if err := tollbooth.LimitByRequest(l, w, r); err != nil {
			tx.RenderError(w, rateErr{err})
			return false
		}
		return true
	})
}

type rateErr struct {
	*errors.HTTPError
}

func (r rateErr) StatusCode() int {
	return r.HTTPError.StatusCode
}
