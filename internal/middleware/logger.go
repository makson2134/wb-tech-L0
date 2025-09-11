package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

//Стандартный middleware.Logger chi использует простой текстовый вывд вместо структурированных json-логов.
//Для единообразия и в целом правильного  логгирования, логично сделать кастомный логгер, который так же будет
//логгировать по настройкам установленного slog.setDefault()

func NewCustomSlogLogger() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Инициализация - основной log
		log := slog.With(
			slog.String("component", "middleware/logger"),
		)

		log.Info("logger middleware enabled")

		// Внешний слой для каждого запроса
		fn := func(w http.ResponseWriter, r *http.Request) {
			// Содержание лога
			entry := log.With(
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("user_agent", r.UserAgent()),
				slog.String("request_id", middleware.GetReqID(r.Context())),
			)

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			t1 := time.Now()

			defer func() {
				// Финальный лог
				entry.Info("request completed",
					slog.Int("status", ww.Status()),
					slog.Int("bytes", ww.BytesWritten()),
					slog.String("duration", time.Since(t1).String()),
				)
			}()

			next.ServeHTTP(ww, r)
		}

		return http.HandlerFunc(fn)
	}
}
