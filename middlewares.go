package tabloid

import (
	"context"
	"net/http"
)

// middleware is a convenient type for declaring middlewares.
type middleware func(http.Handler) http.Handler

// loadCurrentUserMiddleware fetches the user session data through the AuthService
// and assigns it to request context.
func loadCurrentUserMiddleware(s *Server) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userData, err := s.authService.CurrentUser(r)
			if err != nil {
				s.Logger.Warn().Err(err).Msg("Failed to fetch session data")
				http.Error(w, "Failed to fetch session data", http.StatusInternalServerError)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyUser, userData)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
