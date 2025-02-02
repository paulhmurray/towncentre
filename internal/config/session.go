package config

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/alexedwards/scs/mysqlstore"
	"github.com/alexedwards/scs/v2"
)

func InitSession(db *sql.DB) *scs.SessionManager {
	// Create a new session manager
	sessionManager := scs.New()

	// Configure session behavior
	sessionManager.Store = mysqlstore.New(db)
	sessionManager.Lifetime = 24 * time.Hour
	sessionManager.Cookie.Secure = false // Set to true in production with HTTPS
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	return sessionManager
}
