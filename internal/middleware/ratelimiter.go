package middleware

import (
	"net"
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

func IPRateLimiter(rps float64, burst int) func(next http.Handler) http.Handler {
	clients := make(map[string]*rate.Limiter)
	var mu sync.Mutex

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			mu.Lock()
			if _, found := clients[ip]; !found {
				clients[ip] = rate.NewLimiter(rate.Limit(rps), burst)
			}
			limiter := clients[ip]
			mu.Unlock()

			if !limiter.Allow() {
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
