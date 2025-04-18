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
	TemplateCache     map[string]*template.Template
	DB                *sql.DB
	Merchants         *models.MerchantModel
	Products          *models.ProductModel
	Sessions          *scs.SessionManager
	Messages          *models.MessageModel
	StoreViews        *models.StoreViewModel
	Services          *models.ServiceModel
	ServiceCategories *models.ServiceCategoryModel
}

type templateData struct {
	IsAuthenticated    bool
	Merchant           *models.Merchant   // Logged in merchant
	Store              *models.Merchant   // Merchant being viewed
	Merchants          []*models.Merchant // List of merchants
	Products           []*models.Product
	Product            *models.Product
	Services           []*models.Service
	Service            *models.Service
	ServiceCategories  []*models.ServiceCategory
	Error              string
	MessagesList       []*models.Message
	UnreadMessageCount int
	TotalProducts      int
	TotalViews         int
	Token              string
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

// MerchantProductCreatePost handler - processes the form submission
func (app *Application) MerchantProductCreatePost(w http.ResponseWriter, r *http.Request) {
	// Get the merchant ID from the session
	merchantID, ok := app.Sessions.Get(r.Context(), "merchantID").(int64)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse multipart form for file upload (5MB max)
	err := r.ParseMultipartForm(5 << 20)
	if err != nil {
		// This error occurs when the form size exceeds the limit
		if r.Header.Get("HX-Request") == "true" {
			w.Write([]byte(`
			<div class="rounded-md bg-red-50 p-4">
				<div class="flex">
					<div class="ml-3">
						<h3 class="text-sm font-medium text-red-800">File too large</h3>
						<div class="mt-2 text-sm text-red-700">
							<p>Image exceeds the 5MB limit. Please compress your image or choose a smaller one.</p>
						</div>
					</div>
				</div>
			</div>
			`))
			return
		}
		http.Error(w, "Image file too large (max 5MB)", http.StatusRequestEntityTooLarge)
		return
	}

	// Parse form data
	err = r.ParseForm()
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
	// Parse stock count
	stockCount, err := strconv.Atoi(r.PostForm.Get("stock_count"))
	if err != nil || stockCount < 0 {
		stockCount = 0 // Default to 0 if invalid
	}

	// Get stock status
	stockStatus := r.PostForm.Get("stock_status")
	if stockStatus == "" {
		// Determine stock status based on count if not provided
		if stockCount <= 0 {
			stockStatus = "Out of Stock"
		} else if stockCount < 5 {
			stockStatus = "Low Stock"
		} else {
			stockStatus = "In Stock"
		}
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
		StockCount:  stockCount,
		StockStatus: stockStatus,
	}

	// Handle file upload
	if file, header, err := r.FormFile("image"); err == nil {
		defer file.Close()

		// Check file size again (server-side)
		fileSize := header.Size / (1024 * 1024) // size in MB
		if fileSize > 5 {
			if r.Header.Get("HX-Request") == "true" {
				w.Write([]byte(`
				<div class="rounded-md bg-red-50 p-4">
					<div class="flex">
						<div class="ml-3">
							<h3 class="text-sm font-medium text-red-800">File too large</h3>
							<div class="mt-2 text-sm text-red-700">
								<p>Image exceeds the 5MB limit. Please compress your image or choose a smaller one.</p>
							</div>
						</div>
					</div>
				</div>
				`))
				return
			}
			http.Error(w, "Image file too large (max 5MB)", http.StatusRequestEntityTooLarge)
			return
		}

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
	// Parse stock count - new code
	stockCount, err := strconv.Atoi(r.PostForm.Get("stock_count"))
	if err != nil || stockCount < 0 {
		stockCount = 0 // Default to 0 if invalid
	}

	// Get stock status - new code
	stockStatus := r.PostForm.Get("stock_status")
	if stockStatus == "" {
		// Determine stock status based on count if not provided
		if stockCount <= 0 {
			stockStatus = "Out of Stock"
		} else if stockCount < 5 {
			stockStatus = "Low Stock"
		} else {
			stockStatus = "In Stock"
		}
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
		StockCount:    stockCount,
		StockStatus:   stockStatus,
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

	// Check business model and render appropriate dashboard
	if merchant.BusinessModel == "service" {
		// For service businesses, use the service dashboard
		app.MerchantServiceDashboard(w, r)
		return
	}

	// Continue with product dashboard - this is the default/original behavior

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
		unreadCount = 0 // Fall back to 0 if there's an error
	}

	// Get total views
	totalViews, err := app.StoreViews.GetTotalViews(merchantID)
	if err != nil {
		log.Printf("Error getting view count: %v", err)
		totalViews = 0 // Fall back to 0 if there's an error
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
			if newData.Token != "" {
				td.Token = newData.Token
			}
			if newData.ServiceCategories != nil {
				td.ServiceCategories = newData.ServiceCategories
			}
			if newData.Services != nil {
				td.Services = newData.Services
			}
			if newData.Service != nil {
				td.Service = newData.Service
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

	// EXTREME DEBUGGING FOR BUSINESS MODEL
	log.Printf("-------- STORE DEBUGGING --------")
	log.Printf("Store Name: %s", merchant.StoreName)
	log.Printf("Business Model: '%s'", merchant.BusinessModel)
	log.Printf("Business Model Type: %T", merchant.BusinessModel)
	log.Printf("Business Model Bytes: %v", []byte(merchant.BusinessModel))
	log.Printf("Is 'product': %v", merchant.BusinessModel == "product")
	log.Printf("Is 'service': %v", merchant.BusinessModel == "service")
	log.Printf("Is Equal Product: %v", strings.EqualFold(merchant.BusinessModel, "product"))
	log.Printf("Is Equal Service: %v", strings.EqualFold(merchant.BusinessModel, "service"))
	log.Printf("Contains Product: %v", strings.Contains(merchant.BusinessModel, "product"))
	log.Printf("Contains Service: %v", strings.Contains(merchant.BusinessModel, "service"))
	log.Printf("---------------------------------")

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

	// Prepare base template data
	data := &templateData{
		Store: merchant,
	}

	// Try a different approach with string comparison
	businessModel := strings.TrimSpace(merchant.BusinessModel)
	if businessModel == "product" {
		log.Printf("MATCHED PRODUCT: Using product template")

		// Get products for this merchant
		products, err := app.Products.GetByMerchantID(merchant.ID)
		if err != nil {
			log.Printf("Error fetching products: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		data.Products = products

		app.render(w, r, http.StatusOK, "store.page.html", data)
	} else if businessModel == "service" {
		log.Printf("MATCHED SERVICE: Using service template")

		// Get services for this merchant
		services, err := app.Services.GetByMerchantID(merchant.ID)
		if err != nil {
			log.Printf("Error fetching services: %v", err)
			services = []*models.Service{}
		}
		data.Services = services

		app.render(w, r, http.StatusOK, "store.service.page.html", data)
	} else {
		// Fallback with explicit logging of what's happening
		log.Printf("NO MATCH: Unknown business model '%s', defaulting to product template", businessModel)

		// Default to product if neither matches
		products, err := app.Products.GetByMerchantID(merchant.ID)
		if err != nil {
			log.Printf("Error fetching products: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		data.Products = products

		app.render(w, r, http.StatusOK, "store.page.html", data)
	}
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
func (app *Application) MerchantOrders(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := app.Sessions.Get(r.Context(), "merchantID").(int64)
	if !ok {
		http.Redirect(w, r, "/merchant/login", http.StatusSeeOther)
		return
	}
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
	app.render(w, r, http.StatusOK, "merchant.orders.page.html", data)
}

func (app *Application) StoreSettingsPost(w http.ResponseWriter, r *http.Request) {
	// Log all headers to debug the request
	log.Printf("Received headers: %v", r.Header)

	// Get the merchant ID from the session
	merchantID, ok := app.Sessions.Get(r.Context(), "merchantID").(int64)
	if !ok {
		log.Println("Unauthorized: No merchantID in session")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse multipart form for file upload (5MB max)
	err := r.ParseMultipartForm(5 << 20)
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
		Phone:        r.PostForm.Get("phone"), // Add phone field
		OpeningHours: r.PostForm.Get("opening-hours"),
	}

	// Handle logo file upload
	if file, header, err := r.FormFile("logo"); err == nil {
		defer file.Close()

		// Create unique filename
		ext := filepath.Ext(header.Filename)
		filename := fmt.Sprintf("%d-logo-%s%s", time.Now().UnixNano(), uuid.New().String(), ext)
		fullPath := filepath.Join("ui/static/images/logos", filename)
		webPath := "/static/images/logos/" + filename

		log.Printf("Saving logo to: %s", fullPath)
		log.Printf("Web path will be: %s", webPath)

		// Ensure the directory exists
		err = os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err != nil {
			log.Printf("Error creating directory: %v", err)
		}
		// Create the file
		dst, err := os.Create(fullPath)
		if err != nil {
			log.Printf("Error creating logo file: %v", err)
		} else {
			defer dst.Close()

			// Copy the uploaded file
			if _, err := io.Copy(dst, file); err != nil {
				log.Printf("Error copying logo file: %v", err)
			} else {
				// Set the logo path
				merchant.LogoPath = webPath
				log.Printf("Logo path set to: %s", webPath)
			}
		}
	} else {
		log.Printf("No logo file received or error: %v", err)
	}

	// Update the merchant
	err = app.Merchants.UpdateStoreInfo(merchant)
	if err != nil {
		log.Printf("Error updating merchant: %v", err)
		http.Error(w, "Error updating store information", http.StatusInternalServerError)
		return
	}

	// Log before HTMX check
	log.Println("Reached merchant update success, checking HX-Request")

	// Handle HTMX request
	if r.Header.Get("HX-Request") == "true" {
		log.Println("HX-Request is true, setting HX-Redirect")
		// Use HX-Redirect to instruct HTMX to redirect to the dashboard
		w.Header().Set("HX-Redirect", "/merchant/dashboard")
		w.WriteHeader(http.StatusOK)
		log.Println("HX-Redirect set, response sent")
		return
	}

	// Fallback for non-HTMX requests (e.g., JavaScript disabled)
	log.Println("No HX-Request, performing HTTP redirect")
	http.Redirect(w, r, "/merchant/dashboard", http.StatusSeeOther)
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

func (app *Application) Learn(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, http.StatusOK, "learn.page.html", nil)
}
func (app *Application) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			app.handleError(w, r, "Invalid form data", http.StatusBadRequest)
			return
		}

		email := r.PostForm.Get("email")
		if email == "" {
			app.handleError(w, r, "Please enter your email", http.StatusUnprocessableEntity)
			return
		}

		resetToken, err := app.Merchants.InitiatePasswordReset(email)
		if err != nil {
			log.Printf("Error initiating password reset: %v", err)
			app.handleError(w, r, "An error occurred while processing your request", http.StatusInternalServerError)
			return
		}
		if resetToken == nil {
			app.handleError(w, r, "Email not found", http.StatusUnprocessableEntity)
			return
		}

		if err := sendResetEmail(email, resetToken.Token); err != nil {
			log.Printf("Error sending reset email: %v", err)
			app.handleError(w, r, "Error sending reset email", http.StatusInternalServerError)
			return
		}

		if r.Header.Get("HX-Request") == "true" {
			w.Write([]byte(`
                <div class="rounded-md bg-green-50 p-4">
                    <div class="flex">
                        <div class="ml-3">
                            <h3 class="text-sm font-medium text-green-800">Reset Link Sent</h3>
                            <div class="mt-2 text-sm text-green-700">
                                <p>Please check your email for the password reset link.</p>
                            </div>
                        </div>
                    </div>
                </div>
            `))
			return
		}

		data := &templateData{
			Error: "Password reset link sent to your email",
		}
		app.render(w, r, http.StatusOK, "merchant.forgot-password.page.html", data) // Template name unchanged for now
		return
	}

	app.render(w, r, http.StatusOK, "merchant.forgot-password.page.html", nil)
}

// ResetPassword handler
func (app *Application) ResetPassword(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[1] != "reset-password" {
		http.NotFound(w, r)
		return
	}
	token := parts[2]

	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			app.handleError(w, r, "Invalid form data", http.StatusBadRequest)
			return
		}

		password := r.PostForm.Get("password")
		passwordConfirm := r.PostForm.Get("password-confirm")

		if password == "" || passwordConfirm == "" {
			app.handleError(w, r, "Please enter and confirm your new password", http.StatusUnprocessableEntity)
			return
		}

		if password != passwordConfirm {
			app.handleError(w, r, "Passwords do not match", http.StatusUnprocessableEntity)
			return
		}

		err = app.Merchants.ResetPassword(token, password)
		if err != nil {
			log.Printf("Error resetting password: %v", err)
			app.handleError(w, r, err.Error(), http.StatusUnprocessableEntity)
			return
		}

		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", "/merchant/login")
			w.Write([]byte(`
                <div class="rounded-md bg-green-50 p-4">
                    <div class="flex">
                        <div class="ml-3">
                            <h3 class="text-sm font-medium text-green-800">Password Reset Successful</h3>
                            <div class="mt-2 text-sm text-green-700">
                                <p>You can now log in with your new password.</p>
                            </div>
                        </div>
                    </div>
                </div>
            `))
			return
		}

		http.Redirect(w, r, "/merchant/login", http.StatusSeeOther)
		return
	}

	data := &templateData{
		Token: token,
	}
	app.render(w, r, http.StatusOK, "merchant.reset-password.page.html", data)
}

// Helper function to handle errors consistently
func (app *Application) handleError(w http.ResponseWriter, r *http.Request, message string, status int) {
	if r.Header.Get("HX-Request") == "true" {
		w.Write([]byte(fmt.Sprintf(`
            <div class="rounded-md bg-red-50 p-4">
                <div class="flex">
                    <div class="ml-3">
                        <h3 class="text-sm font-medium text-red-800">Error</h3>
                        <div class="mt-2 text-sm text-red-700">
                            <p>%s</p>
                        </div>
                    </div>
                </div>
            </div>
        `, message)))
		return
	}

	data := &templateData{
		Error: message,
	}
	app.render(w, r, status, "merchant.forgot-password.page.html", data)
}

// could this be in a separate utility file
func sendResetEmail(email, token string) error {
	// Implement your email sending logic here
	// Example URL: http://yourdomain.com/reset-password/{token}
	log.Printf("Sending reset email to %s with token %s", email, token)
	return nil // Replace with actual email sending implementation
}
