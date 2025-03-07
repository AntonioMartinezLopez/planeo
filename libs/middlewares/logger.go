package middlewares

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"planeo/libs/logger" // Replace with your actual import path
)

// LoggerMiddleware adds a request-scoped logger to the context and logs request details
func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Get or generate request ID
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
			w.Header().Set("X-Request-ID", requestID)
		}

		// Create request logger
		requestLogger := log.With().
			Str(logger.FieldRequestID, requestID).
			Str(logger.FieldMethod, r.Method).
			Str(logger.FieldPath, r.URL.Path).
			Logger()

		// Log request start
		requestLogger.Info().Msg("Request started")

		// Create a wrapper to capture response details
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		// Add logger to context
		ctx := logger.WithContext(r.Context(), requestLogger)

		// Process request
		next.ServeHTTP(ww, r.WithContext(ctx))

		// Log request completion
		duration := time.Since(start)
		requestLogger.Info().
			Int(logger.FieldStatusCode, ww.Status()).
			Dur(logger.FieldDuration, duration).
			Int("bytes_written", ww.BytesWritten()).
			Msg("Request completed")
	})
}
