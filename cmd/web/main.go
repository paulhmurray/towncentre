package main

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/paulhmurray/towncentre/internal/config"
	"github.com/paulhmurray/towncentre/internal/handlers"
	"github.com/paulhmurray/towncentre/internal/middleware"
	"github.com/paulhmurray/towncentre/internal/models"
)

func newTemplateCache() (map[string]*template.Template, error) {
	cache := map[string]*template.Template{}

	// Get all pages from ui/html/pages
	pages, err := filepath.Glob("./ui/html/pages/*.html")
	if err != nil {
		log.Printf("Error finding pages: %v", err)
		return nil, err
	}

	// Log found pages
	log.Printf("Found pages: %v", pages)

	for _, page := range pages {
		name := filepath.Base(page)
		log.Printf("Processing page: %s", name)

		// Parse the base template
		ts, err := template.ParseFiles("./ui/html/base.html")
		if err != nil {
			log.Printf("Error parsing base template: %v", err)
			return nil, err
		}

		// Add the page template
		ts, err = ts.ParseFiles(page)
		if err != nil {
			log.Printf("Error parsing page template: %v", err)
			return nil, err
		}

		cache[name] = ts
		log.Printf("Added %s to cache", name)
	}

	return cache, nil
}

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: Error loading .env file:", err)
		// Continue execution as environment variables might be set through other means
	}
	// Database configuration
	dbConfig := config.NewDBConfig()

	// Initialize database
	db, err := config.InitDB(dbConfig)
	if err != nil {
		log.Fatal("Database initialisation failed:", err)
	}
	defer db.Close()

	// Test the connection
	err = db.Ping()
	if err != nil {
		log.Fatal("Could not ping database:", err)
	}
	log.Println("Successfully connected to database")

	//Initialise session manager
	sessionManager := config.InitSession(db)

	// Initialize template cache
	templateCache, err := newTemplateCache()
	if err != nil {
		log.Fatal(err)
	}
	// Initialise the application struct
	assetsServer := http.FileServer(http.Dir("ui/assets"))
	staticServer := http.FileServer(http.Dir("ui/static"))

	// Initialise the merchant model
	merchants := &models.MerchantModel{DB: db}

	// Initialise the product model
	products := &models.ProductModel{DB: db}
	// Initialis the store view model
	storeViews := &models.StoreViewModel{DB: db}

	app := &handlers.Application{
		TemplateCache: templateCache,
		DB:            db,
		Merchants:     merchants,
		Products:      products,
		Sessions:      sessionManager,
		Messages:      &models.MessageModel{DB: db},
		StoreViews:    storeViews,
	}
	mux := http.NewServeMux()

	// middleware for authentication and session
	combinedMiddleware := func(next http.HandlerFunc) http.Handler {
		return sessionManager.LoadAndSave(
			middleware.RequireAuth(next, sessionManager),
		)
	}

	// Routes with combined middleware
	mux.Handle("GET /{$}", combinedMiddleware(app.Home))
	mux.Handle("GET /launch", combinedMiddleware(app.Launch))
	mux.Handle("GET /product/view/{id}", combinedMiddleware(app.ProductView))
	mux.Handle("GET /merchant/product/view", combinedMiddleware(app.MerchantProductView))
	mux.Handle("GET /merchant/product/create", combinedMiddleware(app.MerchantProductCreate))
	mux.Handle("POST /merchant/product/create", combinedMiddleware(app.MerchantProductCreatePost))
	mux.Handle("GET /search", combinedMiddleware(app.Search))
	mux.Handle("GET /category/{category}", combinedMiddleware(app.CategoryProducts))
	mux.Handle("GET /merchant/product/edit/{id}", combinedMiddleware(app.MerchantProductEdit))
	mux.Handle("POST /merchant/product/edit/{id}", combinedMiddleware(app.MerchantProductEditPost))
	mux.Handle("DELETE /merchant/product/delete/{id}", combinedMiddleware(app.MerchantProductDelete))

	// For now, just Ballarat
	// In the future, this would become:
	// mux.Handle("GET /{region}/{slug}", sessionManager.LoadAndSave(http.HandlerFunc(app.StoreProfile)))
	mux.Handle("GET /ballarat/{slug}", combinedMiddleware(app.StoreProfile))

	// Store settings routes
	mux.Handle("GET /merchant/settings", combinedMiddleware(app.StoreSettings))
	mux.Handle("POST /merchant/settings", combinedMiddleware(app.StoreSettingsPost))

	// Static file servers (no middleware needed)
	mux.Handle("GET /assets/", http.StripPrefix("/assets", assetsServer))
	mux.Handle("GET /static/", http.StripPrefix("/static", staticServer))

	// Authentication routes with combined middleware
	mux.Handle("GET /merchant/register", combinedMiddleware(app.MerchantRegister))
	mux.Handle("POST /merchant/register", combinedMiddleware(app.MerchantRegister))
	mux.Handle("GET /merchant/dashboard", combinedMiddleware(app.MerchantDashboard))
	mux.Handle("GET /merchant/login", combinedMiddleware(app.MerchantLogin))
	mux.Handle("POST /merchant/login", combinedMiddleware(app.MerchantLogin))
	mux.Handle("POST /merchant/logout", combinedMiddleware(app.MerchantLogout))

	// Message routes with combined middleware
	mux.Handle("POST /store/{id}/message", combinedMiddleware(app.StoreMessageCreate))
	mux.Handle("GET /merchant/messages", combinedMiddleware(app.MerchantMessages))
	mux.Handle("POST /merchant/message/{id}/read", combinedMiddleware(app.MarkMessageAsRead))

	// no auth required for Terms, minimal middleware for CheckBusinessType)
	mux.Handle("GET /terms", combinedMiddleware(app.Terms))
	mux.Handle("GET /merchant/check-business-type", sessionManager.LoadAndSave(http.HandlerFunc(app.CheckBusinessType)))

	log.Print("starting server on :4000")
	err = http.ListenAndServe(":4000", mux)
	log.Fatal(err)
}
