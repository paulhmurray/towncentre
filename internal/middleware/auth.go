package middleware

import (
	"net/http"
	"strings"

	"github.com/alexedwards/scs/v2"
)

func RequireAuth(next http.HandlerFunc, sessions *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/merchant") {
			if !strings.HasPrefix(r.URL.Path, "/merchant/login") && !strings.HasPrefix(r.URL.Path, "/merchant/register") {
				// Check if user is authenticated
				if !sessions.Exists(r.Context(), "merchantID") {
					http.Redirect(w, r, "/merchant/login", http.StatusSeeOther)
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	}
}
