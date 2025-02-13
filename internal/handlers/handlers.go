package handlers

import (
	"bytes"
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"github.com/paulhmurray/towncentre/internal/models"
	// Add your models import
)

type Application struct {
	TemplateCache map[string]*template.Template
	DB            *sql.DB
	Merchants     *models.MerchantModel
	Products      *models.ProductModel
	Sessions      *scs.SessionManager
	Messages      *models.MessageModel
	StoreViews    *models.StoreViewModel
}

type templateData struct {
	IsAuthenticated    bool
	Merchant           *models.Merchant   // Logged in merchant
	Store              *models.Merchant   // Merchant being viewed
	Merchants          []*models.Merchant // List of merchants
	Products           []*models.Product
	Product            *models.Product
	Error              string
	MessagesList       []*models.Message
	UnreadMessageCount int
	TotalProducts      int
	TotalViews         int
}

// Home handler
func (app *Application) Home(w http.ResponseWriter, r *http.Request) {
	// Get featured products
	products, err := app.Products.GetFeatured()
	if err != nil {
		log.Printf("Error fetching featured products: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get featured stores (previously "local shops")
	merchants, err := app.Merchants.GetFeatured()
	if err != nil {
		log.Printf("Error fetching featured stores: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := &templateData{
		Products:  products,
		Merchants: merchants, // Use new field name
	}

	app.render(w, r, http.StatusOK, "home.page.html", data)
}

// Launch handler (temporary landing page)
func (app *Application) Launch(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, http.StatusOK, "launch.page.html", nil)
}

// ProductView handler
func (app *Application) ProductView(w http.ResponseWriter, r *http.Request) {
	log.Printf("ProductView - URL Path: %s", r.URL.Path)

	// Get the product ID from the URL
	productIDStr := r.PathValue("id")
	log.Printf("ProductView - ID from URL: %s", productIDStr)

	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil || productID < 1 {
		log.Printf("ProductView - Invalid product ID: %v", err)
		http.NotFound(w, r)
		return
	}

	// Get the product details
	product, err := app.Products.GetByID(productID, 0) // Using 0 as merchantID to bypass ownership check
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("ProductView - Product not found in database: %d", productID)
			http.NotFound(w, r)
		} else {
			log.Printf("ProductView - Database error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	if product == nil {
		log.Printf("ProductView - Product is nil after database query")
		http.NotFound(w, r)
		return
	}

	log.Printf("ProductView - Found product in database: %+v", product)

	// Get the merchant (store) details
	merchant, err := app.Merchants.GetByID(product.MerchantID)
	if err != nil {
		log.Printf("ProductView - Error fetching merchant: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data := &templateData{
		Product: product,
		Store:   merchant,
	}

	// Check if template exists in cache
	if _, ok := app.TemplateCache["product.view.page.html"]; !ok {
		log.Printf("ProductView - Template 'product.view.page.html' not found in cache")
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	log.Printf("ProductView - About to render template")
	app.render(w, r, http.StatusOK, "product.view.page.html", data)
}

// MerchantProductView handler
func (app *Application) MerchantProductView(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, http.StatusOK, "merchant.products.page.html", nil)
}

// MerchantProductCreate handler - shows the form
func (app *Application) MerchantProductCreate(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, http.StatusOK, "merchant.product.create.page.html", nil)
}

// MerchantProductCreate handler - processes the form submission
func (app *Application) MerchantProductCreatePost(w http.ResponseWriter, r *http.Request) {
	// Get the merchant ID from the session
	merchantID, ok := app.Sessions.Get(r.Context(), "merchantID").(int64)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse multipart form for file upload (5MB max)
	r.ParseMultipartForm(5 << 20)

	// Parse form data first
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Parse the price
	price, err := strconv.ParseFloat(r.PostForm.Get("price"), 64)
	if err != nil {
		http.Error(w, "Invalid price", http.StatusBadRequest)
		return
	}

	// Create product struct early
	product := &models.Product{
		MerchantID:  merchantID,
		Name:        r.PostForm.Get("name"),
		Description: r.PostForm.Get("description"),
		Price:       price,
		Category:    r.PostForm.Get("category"),
		HasDelivery: r.PostForm.Get("delivery") == "on",
		HasPickup:   r.PostForm.Get("pickup") == "on",
	}

	// Handle file upload
	if file, header, err := r.FormFile("image"); err == nil {
		defer file.Close()

		// Create unique filename
		ext := filepath.Ext(header.Filename)
		filename := fmt.Sprintf("%d-%s%s", time.Now().UnixNano(), uuid.New().String(), ext)
		fullPath := filepath.Join("ui/static/images/products", filename)
		webPath := "/static/images/products/" + filename

		log.Printf("Full path: %s", fullPath)
		log.Printf("Web path: %s", webPath)

		// Create the file
		dst, err := os.Create(fullPath)
		if err != nil {
			log.Printf("Error creating file: %v", err)
			http.Error(w, "Error uploading file", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		// Copy the uploaded file
		if _, err := io.Copy(dst, file); err != nil {
			log.Printf("Error copying file: %v", err)
			http.Error(w, "Error uploading file", http.StatusInternalServerError)
			return
		}

		// Set the main image path
		imagePath := webPath
		product.ImagePath = &imagePath

		// Create thumbnail
		if thumbPath, err := createThumbnail(fullPath); err == nil {
			webThumbPath := "/static/images/products/" + filepath.Base(thumbPath)
			thumbnailPath := webThumbPath
			product.ThumbnailPath = &thumbnailPath
		} else {
			log.Printf("Error creating thumbnail: %v", err)
		}
	}

	// Debug logging
	//log.Printf("Product Image Path: %s", product.ImagePath)
	//log.Printf("Product Thumbnail Path: %s", product.ThumbnailPath)

	// Insert the product
	err = app.Products.Insert(product)
	if err != nil {
		log.Printf("Error inserting product: %v", err)
		http.Error(w, "Error creating product", http.StatusInternalServerError)
		return
	}

	// Handle HTMX request
	if r.Header.Get("HX-Request") == "true" {
		w.Write([]byte(`
            <div class="rounded-md bg-green-50 p-4">
                <div class="flex">
                    <div class="ml-3">
                        <h3 class="text-sm font-medium text-green-800">Product Created Successfully</h3>
                        <div class="mt-2 text-sm text-green-700">
                            <p>Your product has been listed. <a href="/merchant/dashboard" class="font-medium text-green-800 hover:text-green-900">Return to Dashboard</a></p>
                        </div>
                    </div>
                </div>
            </div>
        `))
		return
	}

	// Regular form submission - redirect to dashboard
	http.Redirect(w, r, "/merchant/dashboard", http.StatusSeeOther)
}

// MerchantProductEdit - shows the edit form
func (app *Application) MerchantProductEdit(w http.ResponseWriter, r *http.Request) {
	// Get the merchant ID from the session
	merchantID, ok := app.Sessions.Get(r.Context(), "merchantID").(int64)
	if !ok {
		log.Printf("No merchant ID in session")
		http.Redirect(w, r, "/merchant/login", http.StatusSeeOther)
		return
	}

	// Get the merchant data
	merchant, err := app.Merchants.GetByID(merchantID)
	if err != nil {
		log.Printf("Error fetching merchant: %v", err)
		http.Error(w, "Error fetching merchant data", http.StatusInternalServerError)
		return
	}

	// Get the product ID from the URL
	productIDStr := r.PathValue("id")
	log.Printf("Attempting to edit product ID: %s", productIDStr)

	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil {
		log.Printf("Invalid product ID: %v", err)
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	// Fetch the product
	product, err := app.Products.GetByID(productID, merchantID)
	if err != nil {
		log.Printf("Error fetching product %d for merchant %d: %v", productID, merchantID, err)
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}

	// Add debug logging
	log.Printf("Retrieved product: %+v", product)

	// Render the edit form with both merchant and product data
	data := &templateData{
		IsAuthenticated: true,
		Merchant:        merchant,
		Product:         product,
	}

	log.Printf("Rendering edit form with data: %+v", data)
	app.render(w, r, http.StatusOK, "merchant.product.edit.page.html", data)
}

// MerchantProductEditPost - processes the edit form submission
func (app *Application) MerchantProductEditPost(w http.ResponseWriter, r *http.Request) {
	// Get the merchant ID from the session
	merchantID, ok := app.Sessions.Get(r.Context(), "merchantID").(int64)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get the product ID from the URL
	productIDStr := r.PathValue("id")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	// Parse form data
	err = r.ParseMultipartForm(5 << 20) // 5MB max
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Parse the price
	price, err := strconv.ParseFloat(r.PostForm.Get("price"), 64)
	if err != nil {
		http.Error(w, "Invalid price", http.StatusBadRequest)
		return
	}

	// First get the existing product
	existingProduct, err := app.Products.GetByID(productID, merchantID)
	if err != nil {
		log.Printf("Error fetching existing product: %v", err)
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}

	// Create product struct with existing image paths
	product := &models.Product{
		ID:            productID,
		MerchantID:    merchantID,
		Name:          r.PostForm.Get("name"),
		Description:   r.PostForm.Get("description"),
		Price:         price,
		Category:      r.PostForm.Get("category"),
		HasDelivery:   r.PostForm.Get("delivery") == "on",
		HasPickup:     r.PostForm.Get("pickup") == "on",
		ImagePath:     existingProduct.ImagePath,     // Keep existing image path
		ThumbnailPath: existingProduct.ThumbnailPath, // Keep existing thumbnail path
	}

	// Handle file upload (optional)
	if file, header, err := r.FormFile("image"); err == nil {
		defer file.Close()

		// Create unique filename
		ext := filepath.Ext(header.Filename)
		filename := fmt.Sprintf("%d-%s%s", time.Now().UnixNano(), uuid.New().String(), ext)
		fullPath := filepath.Join("ui/static/images/products", filename)
		webPath := "/static/images/products/" + filename

		// Create the file
		dst, err := os.Create(fullPath)
		if err != nil {
			log.Printf("Error creating file: %v", err)
			http.Error(w, "Error uploading file", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		// Copy the uploaded file
		if _, err := io.Copy(dst, file); err != nil {
			log.Printf("Error copying file: %v", err)
			http.Error(w, "Error uploading file", http.StatusInternalServerError)
			return
		}

		// Set the main image path
		imagePath := webPath
		product.ImagePath = &imagePath

		// Create thumbnail
		if thumbPath, err := createThumbnail(fullPath); err == nil {
			webThumbPath := "/static/images/products/" + filepath.Base(thumbPath)
			thumbnailPath := webThumbPath
			product.ThumbnailPath = &thumbnailPath
		} else {
			log.Printf("Error creating thumbnail: %v", err)
		}
	}

	// Update the product
	err = app.Products.Update(product)
	if err != nil {
		log.Printf("Error updating product: %v", err)
		http.Error(w, "Error updating product", http.StatusInternalServerError)
		return
	}

	// For HTMX requests, use HX-Redirect header
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/merchant/dashboard")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Redirect for non-HTMX requests
	http.Redirect(w, r, "/merchant/dashboard", http.StatusSeeOther)
}

func (app *Application) MerchantProductDelete(w http.ResponseWriter, r *http.Request) {
	// Get the merchant ID from the session
	merchantID, ok := app.Sessions.Get(r.Context(), "merchantID").(int64)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get the product ID from the URL
	productIDStr := r.PathValue("id")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	// Attempt to delete the product
	err = app.Products.Delete(productID, merchantID)
	if err != nil {
		log.Printf("Error deleting product: %v", err)
		http.Error(w, "Error deleting product", http.StatusInternalServerError)
		return
	}

	// For HTMX requests, use HX-Redirect header
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/merchant/dashboard")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Redirect for non-HTMX requests
	http.Redirect(w, r, "/merchant/dashboard", http.StatusSeeOther)
}

// Search handler
func (app *Application) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	w.Write([]byte("Search results for: " + query))
}

// CategoryProducts handler
func (app *Application) CategoryProducts(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	w.Write([]byte("Products in category: " + category))
}

// MerchantRegister handler
func (app *Application) MerchantRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		app.render(w, r, http.StatusOK, "merchant.register.page.html", nil)
		return
	}

	if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			log.Printf("Error parsing form: %v", err)
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		businessName := r.PostForm.Get("business-name")
		email := r.PostForm.Get("email")
		phone := r.PostForm.Get("phone")
		businessType := r.PostForm.Get("business-type")
		password := r.PostForm.Get("password")
		passwordConfirm := r.PostForm.Get("password-confirm")

		if businessName == "" || email == "" || businessType == "" || password == "" {
			http.Error(w, "Please fill in all required fields", http.StatusBadRequest)
			return
		}

		if password != passwordConfirm {
			http.Error(w, "Passwords do not match", http.StatusBadRequest)
			return
		}

		err = app.Merchants.Insert(businessName, email, phone, businessType, password)
		if err != nil {
			log.Printf("Error registering merchant: %v", err)
			http.Error(w, "Error creating account", http.StatusInternalServerError)
			return
		}

		if r.Header.Get("HX-Request") == "true" {
			w.Write([]byte(`
                <div class="rounded-md bg-green-50 p-4">
                    <div class="flex">
                        <div class="ml-3">
                            <h3 class="text-sm font-medium text-green-800">Registration Successful</h3>
                            <div class="mt-2 text-sm text-green-700">
                                <p>Your account has been created. <a href="/merchant/login" class="font-medium text-green-800 hover:text-green-900">Log in</a></p>
                            </div>
                        </div>
                    </div>
                </div>
            `))
			return
		}

		http.Redirect(w, r, "/merchant/login", http.StatusSeeOther)
	}
}

// MerchantLogin handler
func (app *Application) MerchantLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		app.render(w, r, http.StatusOK, "merchant.login.page.html", nil)
		return
	}

	if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			log.Printf("Error parsing form: %v", err)
			app.handleLoginError(w, r, "Invalid form data")
			return
		}

		email := r.PostForm.Get("email")
		password := r.PostForm.Get("password")

		// Basic validation
		if email == "" || password == "" {
			app.handleLoginError(w, r, "Please enter both email and password")
			return
		}

		merchant, err := app.Merchants.Authenticate(email, password)
		if err != nil {
			log.Printf("Authentication error: %v", err)
			app.handleLoginError(w, r, "An error occurred while trying to log in")
			return
		}

		if merchant == nil {
			app.handleLoginError(w, r, "Invalid email or password")
			return
		}

		// Success path
		app.Sessions.Put(r.Context(), "merchantID", merchant.ID)

		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", "/merchant/dashboard")
			return
		}
		http.Redirect(w, r, "/merchant/dashboard", http.StatusSeeOther)
	}
}

// Helper function to handle login errors consistently
func (app *Application) handleLoginError(w http.ResponseWriter, r *http.Request, message string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Write([]byte(`
            <div class="rounded-md bg-red-50 p-4 mt-4">
                <div class="flex">
                    <div class="ml-3">
                        <h3 class="text-sm font-medium text-red-800">Invalid credentials</h3>
                        <div class="mt-2 text-sm text-red-700">
                            <p>` + message + `</p>
                        </div>
                    </div>
                </div>
            </div>
        `))
		return
	}

	// For non-HTMX requests, show error on login page
	data := &templateData{
		Error: message,
	}
	app.render(w, r, http.StatusUnprocessableEntity, "merchant.login.page.html", data)
}

// MerchantDashboard handler
func (app *Application) MerchantDashboard(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := app.Sessions.Get(r.Context(), "merchantID").(int64)
	if !ok {
		http.Redirect(w, r, "/merchant/login", http.StatusSeeOther)
		return
	}

	merchant, err := app.Merchants.GetByID(merchantID)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	// Get merchant's products
	products, err := app.Products.GetByMerchantID(merchantID)
	if err != nil {
		log.Printf("Error fetching products: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	// Get total product count
	totalProducts, err := app.Products.GetTotalCount(merchantID)
	if err != nil {
		log.Printf("Error getting product count: %v", err)
		totalProducts = 0 // Fall back to 0 if there's an error
	}
	// Get unread message count
	unreadCount, err := app.Messages.GetUnreadCount(merchantID)
	if err != nil {
		log.Printf("Error getting unread count: %v", err)
		// Don't return error, just set count to 0
		unreadCount = 0
	}
	// Get total views
	totalViews, err := app.StoreViews.GetTotalViews(merchantID)
	if err != nil {
		log.Printf("Error getting view count: %v", err)
		totalViews = 0
	}

	data := &templateData{
		IsAuthenticated:    true,
		Merchant:           merchant,
		Products:           products,
		UnreadMessageCount: unreadCount,
		TotalProducts:      totalProducts,
		TotalViews:         totalViews,
	}

	app.render(w, r, http.StatusOK, "merchant.dashboard.page.html", data)
}

// MerchantLogout handler
func (app *Application) MerchantLogout(w http.ResponseWriter, r *http.Request) {
	app.Sessions.Remove(r.Context(), "merchantID")

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/")
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// The render helper
func (app *Application) render(w http.ResponseWriter, r *http.Request, status int, page string, data interface{}) {
	ts, ok := app.TemplateCache[page]
	if !ok {
		log.Printf("Template not found: %s", page)
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	// Initialize template data
	td := &templateData{
		IsAuthenticated: false,
		Merchant:        nil,
	}

	// Check authentication and load merchant data if authenticated
	if app.Sessions.Exists(r.Context(), "merchantID") {
		if merchantID, ok := app.Sessions.Get(r.Context(), "merchantID").(int64); ok {
			merchant, err := app.Merchants.GetByID(merchantID)
			if err != nil {
				log.Printf("Error loading merchant data: %v", err)
				// Don't return error, continue with nil merchant
			} else if merchant != nil {
				td.IsAuthenticated = true
				td.Merchant = merchant
			}
		}
	}

	// If data was passed in, merge it with existing template data
	if data != nil {
		// Type assert the data
		if newData, ok := data.(*templateData); ok {
			// Preserve authentication state and merchant data if not overridden
			if newData.Merchant != nil {
				td.Merchant = newData.Merchant
			}
			if newData.Products != nil {
				td.Products = newData.Products
			}
			if newData.Product != nil {
				td.Product = newData.Product
			}
			if newData.Store != nil {
				td.Store = newData.Store
			}
			if newData.MessagesList != nil { // Add this block
				td.MessagesList = newData.MessagesList
			}
			if newData.UnreadMessageCount > 0 { // And this for the unread count
				td.UnreadMessageCount = newData.UnreadMessageCount
			}
			if newData.TotalProducts > 0 {
				td.TotalProducts = newData.TotalProducts
			}
			if newData.TotalViews > 0 {
				td.TotalViews = newData.TotalViews
			}

			td.IsAuthenticated = td.IsAuthenticated || newData.IsAuthenticated
		}
	}

	// Create a buffer to render the template first
	buf := new(bytes.Buffer)

	// Execute template into buffer
	err := ts.ExecuteTemplate(buf, "base", td)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Write status code
	w.WriteHeader(status)

	// Copy buffer to response writer
	_, err = buf.WriteTo(w)
	if err != nil {
		log.Printf("Error writing template to response: %v", err)
		return
	}
}
func createThumbnail(originalPath string) (string, error) {
	log.Printf("Starting thumbnail creation for: %s", originalPath)

	// Check if file exists
	if _, err := os.Stat(originalPath); os.IsNotExist(err) {
		return "", fmt.Errorf("original file does not exist: %v", err)
	}

	// Open the original image
	src, err := imaging.Open(originalPath)
	if err != nil {
		return "", fmt.Errorf("failed ot open image: %v", err)
	}

	log.Printf("Successfully opened image")

	// Create thumbnail (resixe to width 200px while preserving aspect ratio)
	thumbnail := imaging.Resize(src, 200, 0, imaging.Lanczos)

	log.Printf("Successfully resized image")

	// Create thumbnail filename
	originalExt := filepath.Ext(originalPath)
	thumbnailPath := strings.TrimSuffix(originalPath, originalExt) + "_thumb" + originalExt

	log.Printf("Saving thumbnail to: %s", thumbnailPath)

	err = imaging.Save(thumbnail, thumbnailPath)
	if err != nil {
		return "", fmt.Errorf("Failed to save thumbnail: %v", err)
	}

	log.Printf("Successfully saved thumbnail")
	return thumbnailPath, nil
}

func (app *Application) StoreProfile(w http.ResponseWriter, r *http.Request) {
	region := "ballarat" // Hardcoded for now
	storeSlug := r.PathValue("slug")

	log.Printf("Accessing store profile - Region: %s, Slug: %s", region, storeSlug)

	if storeSlug == "" {
		log.Printf("Store slug is empty")
		http.NotFound(w, r)
		return
	}

	// Get merchant by store slug and region
	merchant, err := app.Merchants.GetByStoreSlugAndRegion(storeSlug, region)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No store found for slug '%s' in region '%s'", storeSlug, region)
			http.NotFound(w, r)
		} else {
			log.Printf("Error fetching store: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}
	// Record the view
	viewerIP := r.RemoteAddr
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		viewerIP = forwardedFor
	}
	err = app.StoreViews.RecordView(merchant.ID, viewerIP)
	if err != nil {
		log.Printf("Error recording view: %v", err)
		// Don't return error to user, continue showing the page
	}

	// Get all products for this merchant
	products, err := app.Products.GetByMerchantID(merchant.ID)
	if err != nil {
		log.Printf("Error fetching products: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data := &templateData{
		Store:    merchant,
		Products: products,
	}

	// Render the store profile template
	app.render(w, r, http.StatusOK, "store.page.html", data)
}

func (app *Application) StoreSettings(w http.ResponseWriter, r *http.Request) {
	// Get the merchant ID from the session
	merchantID, ok := app.Sessions.Get(r.Context(), "merchantID").(int64)
	if !ok {
		http.Redirect(w, r, "/merchant/login", http.StatusSeeOther)
		return
	}

	// Get merchant data
	merchant, err := app.Merchants.GetByID(merchantID)
	if err != nil {
		log.Printf("Error fetching merchant: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := &templateData{
		IsAuthenticated: true,
		Merchant:        merchant,
	}

	app.render(w, r, http.StatusOK, "merchant.settings.page.html", data)
}

func (app *Application) StoreSettingsPost(w http.ResponseWriter, r *http.Request) {
	// Get the merchant ID from the session
	merchantID, ok := app.Sessions.Get(r.Context(), "merchantID").(int64)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	err := r.ParseForm()
	if err != nil {
		log.Printf("Error parsing form: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Create updated merchant data
	merchant := &models.Merchant{
		ID:           merchantID,
		StoreName:    r.PostForm.Get("store-name"),
		Description:  r.PostForm.Get("description"),
		Location:     r.PostForm.Get("location"),
		OpeningHours: r.PostForm.Get("opening-hours"),
	}

	// Update the merchant
	err = app.Merchants.UpdateStoreInfo(merchant)
	if err != nil {
		log.Printf("Error updating merchant: %v", err)
		http.Error(w, "Error updating store information", http.StatusInternalServerError)
		return
	}

	// Handle HTMX request
	if r.Header.Get("HX-Request") == "true" {
		w.Write([]byte(`
            <div class="rounded-md bg-green-50 p-4">
                <div class="flex">
                    <div class="ml-3">
                        <h3 class="text-sm font-medium text-green-800">Settings Updated Successfully</h3>
                        <div class="mt-2 text-sm text-green-700">
                            <p>Your store information has been updated.</p>
                        </div>
                    </div>
                </div>
            </div>
        `))
		return
	}

	http.Redirect(w, r, "/merchant/settings", http.StatusSeeOther)
}
func (app *Application) StoreMessageCreate(w http.ResponseWriter, r *http.Request) {
	// Parse store ID from URL
	storeIDStr := r.PathValue("id")
	storeID, err := strconv.ParseInt(storeIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid store ID", http.StatusBadRequest)
		return
	}

	// Parse form
	err = r.ParseForm()
	if err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Create message
	msg := &models.Message{
		MerchantID:    storeID,
		CustomerName:  r.PostForm.Get("name"),
		CustomerEmail: r.PostForm.Get("email"),
		CustomerPhone: r.PostForm.Get("phone"),
		MessageText:   r.PostForm.Get("message"),
	}

	// Insert message
	err = app.Messages.Insert(msg)
	if err != nil {
		if strings.Contains(err.Error(), "inappropriate content") {
			// Return specific error for inappropriate content
			w.Write([]byte(`
                <div class="rounded-md bg-red-50 p-4">
                    <div class="flex">
                        <div class="ml-3">
                            <h3 class="text-sm font-medium text-red-800">Message not sent</h3>
                            <div class="mt-2 text-sm text-red-700">
                                <p>Your message contains inappropriate content. Please revise and try again.</p>
                            </div>
                        </div>
                    </div>
                </div>
            `))
			return
		}

		http.Error(w, "Error sending message", http.StatusInternalServerError)
		return
	}

	// Return success message
	w.Write([]byte(`
        <div class="rounded-md bg-green-50 p-4">
            <div class="flex">
                <div class="ml-3">
                    <h3 class="text-sm font-medium text-green-800">Message sent successfully</h3>
                    <div class="mt-2 text-sm text-green-700">
                        <p>The store owner will contact you via email or phone.</p>
                    </div>
                </div>
            </div>
        </div>
    `))
}

func (app *Application) MerchantMessages(w http.ResponseWriter, r *http.Request) {
	// Get merchant ID from session
	merchantID, ok := app.Sessions.Get(r.Context(), "merchantID").(int64)
	if !ok {
		log.Printf("No merchant ID in session")
		http.Redirect(w, r, "/merchant/login", http.StatusSeeOther)
		return
	}

	log.Printf("Loading messages page for merchant ID: %d", merchantID)

	// Get merchant data
	merchant, err := app.Merchants.GetByID(merchantID)
	if err != nil {
		log.Printf("Error fetching merchant: %v", err)
		http.Error(w, "Error fetching merchant data", http.StatusInternalServerError)
		return
	}

	// Get messages
	messages, err := app.Messages.GetByMerchantID(merchantID)
	if err != nil {
		log.Printf("Error fetching messages: %v", err)
		http.Error(w, "Error fetching messages", http.StatusInternalServerError)
		return
	}

	log.Printf("Messages before creating data: %+v", messages)
	log.Printf("Number of messages found: %d", len(messages))

	// Prepare template data
	data := &templateData{
		IsAuthenticated: true,
		Merchant:        merchant,
		MessagesList:    messages,
	}

	log.Printf("Data being sent to template: %+v", data)
	log.Printf("MessagesList in data: %+v", data.MessagesList)
	log.Printf("Length of MessagesList in data: %d", len(data.MessagesList))

	app.render(w, r, http.StatusOK, "merchant.messages.page.html", data)
}

func (app *Application) MarkMessageAsRead(w http.ResponseWriter, r *http.Request) {
	// Get merchant ID from session
	merchantID, ok := app.Sessions.Get(r.Context(), "merchantID").(int64)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get message ID from URL
	messageIDStr := r.PathValue("id")
	messageID, err := strconv.ParseInt(messageIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid message ID", http.StatusBadRequest)
		return
	}

	// Mark message as read
	err = app.Messages.MarkAsRead(messageID, merchantID)
	if err != nil {
		http.Error(w, "Error marking message as read", http.StatusInternalServerError)
		return
	}

	// Get the updated message
	message, err := app.Messages.GetByID(messageID, merchantID)
	if err != nil {
		http.Error(w, "Error fetching updated message", http.StatusInternalServerError)
		return
	}

	// Construct phone number HTML if it exists
	phoneHTML := ""
	if message.CustomerPhone != "" {
		phoneHTML = fmt.Sprintf(`
            <span class="ml-4 text-sm text-gray-500">
                <a href="tel:%s" class="hover:text-indigo-600">%s</a>
            </span>
        `, message.CustomerPhone, message.CustomerPhone)
	}

	// Return just the updated message HTML
	html := fmt.Sprintf(`
        <li id="message-%d" class="hover:bg-gray-50">
            <div class="px-4 py-5 sm:px-6">
                <div class="flex items-center justify-between">
                    <div class="flex-1 min-w-0">
                        <div class="flex items-center">
                            <!-- Customer Icon -->
                            <div class="flex-shrink-0">
                                <span class="h-10 w-10 rounded-full bg-indigo-100 flex items-center justify-center">
                                    <svg class="h-6 w-6 text-indigo-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                                    </svg>
                                </span>
                            </div>
                            <!-- Customer Details -->
                            <div class="ml-4">
                                <h2 class="text-sm font-medium text-gray-900">%s</h2>
                                <div class="mt-1 flex items-center">
                                    <a href="mailto:%s" class="text-sm text-gray-500 hover:text-indigo-600">%s</a>
                                    %s
                                </div>
                            </div>
                        </div>
                        <!-- Message Content -->
                        <div class="mt-4">
                            <p class="text-sm text-gray-900 whitespace-pre-line">%s</p>
                        </div>
                    </div>
                    <!-- Time -->
                    <div class="ml-6 flex-shrink-0 flex flex-col items-end">
                        <p class="text-sm text-gray-500">%s</p>
                    </div>
                </div>
            </div>
        </li>
    `,
		message.ID,
		message.CustomerName,
		message.CustomerEmail,
		message.CustomerEmail,
		phoneHTML,
		message.MessageText,
		message.CreatedAt.Format("Jan 2, 2006 3:04 PM"),
	)

	w.Write([]byte(html))
}
