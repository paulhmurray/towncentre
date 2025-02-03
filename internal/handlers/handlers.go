package handlers

import (
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
}

type templateData struct {
	IsAuthenticated bool
	Merchant        *models.Merchant
	Products        []*models.Product
	Product         *models.Product
	Store           *models.Merchant
}

// Home handler
func (app *Application) Home(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, http.StatusOK, "home.page.html", nil)
}

// ProductView handler
func (app *Application) ProductView(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}
	msg := fmt.Sprintf("Display a specific snippet with ID %d...", id)
	w.Write([]byte(msg))
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
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		email := r.PostForm.Get("email")
		password := r.PostForm.Get("password")

		merchant, err := app.Merchants.Authenticate(email, password)
		if err != nil {
			log.Printf("Authentication error: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if merchant == nil {
			if r.Header.Get("HX-Request") == "true" {
				w.Write([]byte(`
                    <div class="rounded-md bg-red-50 p-4 mt-4">
                        <div class="flex">
                            <div class="ml-3">
                                <h3 class="text-sm font-medium text-red-800">Invalid credentials</h3>
                                <div class="mt-2 text-sm text-red-700">
                                    <p>Please check your email and password and try again.</p>
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

		app.Sessions.Put(r.Context(), "merchantID", merchant.ID)

		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", "/merchant/dashboard")
			return
		}
		http.Redirect(w, r, "/merchant/dashboard", http.StatusSeeOther)
	}
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

	data := &templateData{
		IsAuthenticated: true,
		Merchant:        merchant,
		Products:        products,
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
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	td := &templateData{
		IsAuthenticated: app.Sessions.Exists(r.Context(), "merchantID"),
	}

	if td.IsAuthenticated {
		if merchantID, ok := app.Sessions.Get(r.Context(), "merchantID").(int64); ok {
			merchant, err := app.Merchants.GetByID(merchantID)
			if err == nil {
				td.Merchant = merchant
			}
		}
	}

	if data != nil {
		td = data.(*templateData)
	}

	w.WriteHeader(status)
	err := ts.ExecuteTemplate(w, "base", td)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
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

	if storeSlug == "" {
		http.NotFound(w, r)
		return
	}

	// Get merchant by store slug and region
	merchant, err := app.Merchants.GetByStoreSlugAndRegion(storeSlug, region)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
		} else {
			log.Printf("Error fetching store: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
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
