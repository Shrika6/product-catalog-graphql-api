package middleware

import (
	"net/http"
	"strconv"

	"github.com/shrika/product-catalog-graphql-api/pkg/sqlstats"
)

type sqlStatsResponseWriter struct {
	http.ResponseWriter
	counter       *sqlstats.Counter
	headerWritten bool
}

func (w *sqlStatsResponseWriter) ensureHeader() {
	if w.headerWritten {
		return
	}
	w.Header().Set("X-SQL-Statements", strconv.FormatInt(w.counter.Value(), 10))
	w.headerWritten = true
}

func (w *sqlStatsResponseWriter) WriteHeader(statusCode int) {
	w.ensureHeader()
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *sqlStatsResponseWriter) Write(b []byte) (int, error) {
	w.ensureHeader()
	return w.ResponseWriter.Write(b)
}

func SQLStats() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			counter := sqlstats.NewCounter()
			ctx := sqlstats.WithCounter(r.Context(), counter)
			wrapped := &sqlStatsResponseWriter{ResponseWriter: w, counter: counter}
			next.ServeHTTP(wrapped, r.WithContext(ctx))
		})
	}
}
