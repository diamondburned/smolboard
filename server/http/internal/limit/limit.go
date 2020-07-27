package limit

import (
	"net/http"
	"time"

	"github.com/diamondburned/smolboard/server/http/internal/middleware"
	"github.com/didip/tollbooth/v6"
	"github.com/didip/tollbooth/v6/limiter"
)

func RateLimit(n float64) func(http.Handler) http.Handler {
	l := tollbooth.NewLimiter(n, &limiter.ExpirableOptions{
		DefaultExpirationTTL: time.Hour,
	})
	l.SetHeader("Cookie", []string{"token"})

	return middleware.P(func(w http.ResponseWriter, r *http.Request) bool {
		if err := tollbooth.LimitByRequest(l, w, r); err != nil {
			return false
		}
		return true
	})
}
