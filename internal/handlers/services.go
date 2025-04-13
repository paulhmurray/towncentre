// internal/handlers/services.go
package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/paulhmurray/towncentre/internal/models"
)

// MerchantServicesView displays all services for the current merchant
func (app *Application) MerchantServicesView(w http.ResponseWriter, r *http.Request) {
	// Get merchant ID from session
	merchantID, ok := app.Sessions.Get(r.Context(), "merchantID").(int64)
	if !ok {
		http.Redirect(w, r, "/merchant/login", http.StatusSeeOther)
		return
	}

	// Get merchant data
	merchant, err := app.Merchants.GetByID(merchantID)
	if err != nil {
		log.Printf("Error fetching merchant: %v", err)
		http.Error(w, "Error fetching merchant data", http.StatusInternalServerError)
		return
	}

	// Get merchant's services
	services, err := app.Services.GetByMerchantID(merchantID)
	if err != nil {
		log.Printf("Error fetching services: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := &templateData{
		IsAuthenticated: true,
		Merchant:        merchant,
		Services:        services,
	}

	app.render(w, r, http.StatusOK, "merchant.services.page.html", data)
}

// Merchant service create shows the form for creating a new service
func (app *Application) MerchantServiceCreate(w http.ResponseWriter, r *http.Request) {
	// Get the merchant ID from the session
	merchantID, ok := app.Sessions.Get(r.Context(), "merchantID").(int64)
	if !ok {
		http.Redirect(w, r, "/merchant/login", http.StatusSeeOther)
		return
	}
	merchant, err := app.Merchants.GetByID(merchantID)
	if err != nil {
		log.Printf("Error fetching merchant: %v", err)
		http.Error(w, "Error fetching merchant data", http.StatusInternalServerError)
		return
	}
	// Now I need to get service categories for the form dropdown
	categories, err := app.ServiceCategories.GetAll()
	log.Printf("Found %d categories", len(categories))
	if err != nil {
		log.Printf("Error fetching service categories: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
	data := &templateData{
		IsAuthenticated:   true,
		Merchant:          merchant,
		ServiceCategories: categories,
	}
	log.Printf("Template data ServiceCategories: %d", len(data.ServiceCategories))
	app.render(w, r, http.StatusOK, "merchant.service.create.page.html", data)
}

// Add this to internal/handlers/services.go

// MerchantServiceCreatePost processes the form submission for creating a new service
func (app *Application) MerchantServiceCreatePost(w http.ResponseWriter, r *http.Request) {
	// Get the merchant ID from the session
	merchantID, ok := app.Sessions.Get(r.Context(), "merchantID").(int64)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse form data
	err := r.ParseForm()
	if err != nil {
		log.Printf("Error parsing form: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Parse price fields if provided
	var priceFrom, priceTo *float64

	if pfStr := r.PostForm.Get("price_from"); pfStr != "" {
		pf, err := strconv.ParseFloat(pfStr, 64)
		if err != nil {
			http.Error(w, "Invalid price format", http.StatusBadRequest)
			return
		}
		priceFrom = &pf
	}

	if ptStr := r.PostForm.Get("price_to"); ptStr != "" {
		pt, err := strconv.ParseFloat(ptStr, 64)
		if err != nil {
			http.Error(w, "Invalid price format", http.StatusBadRequest)
			return
		}
		priceTo = &pt
	}

	// Create service struct
	service := &models.Service{
		MerchantID:   merchantID,
		ServiceName:  r.PostForm.Get("service_name"),
		Description:  r.PostForm.Get("description"),
		Category:     r.PostForm.Get("category"),
		PriceType:    r.PostForm.Get("price_type"),
		PriceFrom:    priceFrom,
		PriceTo:      priceTo,
		Availability: r.PostForm.Get("availability"),
		ServiceArea:  r.PostForm.Get("service_area"),
		IsFeatured:   false, // Default to false, admin can change later
	}

	// Validate required fields
	if service.ServiceName == "" || service.Category == "" || service.PriceType == "" {
		http.Error(w, "Required fields missing", http.StatusBadRequest)
		return
	}

	// Insert the service
	err = app.Services.Insert(service)
	if err != nil {
		log.Printf("Error inserting service: %v", err)
		http.Error(w, "Error creating service", http.StatusInternalServerError)
		return
	}

	// Handle HTMX request
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/merchant/dashboard")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Regular form submission - redirect to services page
	http.Redirect(w, r, "/merchant/dashboard", http.StatusSeeOther)
}
func (app *Application) MerchantServiceEdit(w http.ResponseWriter, r *http.Request) {
	// Get the merchant ID from the session
	merchantID, ok := app.Sessions.Get(r.Context(), "merchantID").(int64)
	if !ok {
		http.Redirect(w, r, "/merchant/login", http.StatusSeeOther)
		return
	}
	// Get the service ID from the URL
	serviceIDStr := r.PathValue("id")
	serviceID, err := strconv.ParseInt(serviceIDStr, 10, 64)
	if err != nil {
		log.Printf("Invalid service ID: %v", err)
		http.Error(w, "Invalid service ID", http.StatusBadRequest)
		return
	}

	// Get merchant data
	merchant, err := app.Merchants.GetByID(merchantID)
	if err != nil {
		log.Printf("Error fetching merchant: %v", err)
		http.Error(w, "Error fetching merchant data", http.StatusInternalServerError)
		return
	}

	// Fetch the service
	service, err := app.Services.GetByID(serviceID)
	if err != nil {
		log.Printf("Error fetching service %d: %v", serviceID, err)
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Verify ownership
	if service.MerchantID != merchantID {
		log.Printf("Unauthorized attempt to edit service %d by merchant %d", serviceID, merchantID)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get service categories for form
	categories, err := app.ServiceCategories.GetAll()
	if err != nil {
		log.Printf("Error fetching service categories: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := &templateData{
		IsAuthenticated:   true,
		Merchant:          merchant,
		Service:           service,
		ServiceCategories: categories,
	}

	app.render(w, r, http.StatusOK, "merchant.service.edit.page.html", data)
}

// MerchantServiceEditPost processes the form submission for editing a service
func (app *Application) MerchantServiceEditPost(w http.ResponseWriter, r *http.Request) {
	// Get merchant ID from session
	merchantID, ok := app.Sessions.Get(r.Context(), "merchantID").(int64)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get service ID from URL
	serviceIDStr := r.PathValue("id")
	serviceID, err := strconv.ParseInt(serviceIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid service ID", http.StatusBadRequest)
		return
	}

	// Parse form data
	err = r.ParseForm()
	if err != nil {
		log.Printf("Error parsing form: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Verify service exists and belongs to this merchant
	existingService, err := app.Services.GetByID(serviceID)
	if err != nil {
		log.Printf("Error fetching service: %v", err)
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}
	if existingService.MerchantID != merchantID {
		log.Printf("Unauthorized attempt to edit service")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse price fields
	var priceFrom, priceTo *float64

	if pfStr := r.PostForm.Get("price_from"); pfStr != "" {
		pf, err := strconv.ParseFloat(pfStr, 64)
		if err != nil {
			http.Error(w, "Invalid price format", http.StatusBadRequest)
			return
		}
		priceFrom = &pf
	}

	if ptStr := r.PostForm.Get("price_to"); ptStr != "" {
		pt, err := strconv.ParseFloat(ptStr, 64)
		if err != nil {
			http.Error(w, "Invalid price format", http.StatusBadRequest)
			return
		}
		priceTo = &pt
	}

	// Create service struct with updated values
	service := &models.Service{
		ID:           serviceID,
		MerchantID:   merchantID,
		ServiceName:  r.PostForm.Get("service_name"),
		Description:  r.PostForm.Get("description"),
		Category:     r.PostForm.Get("category"),
		PriceType:    r.PostForm.Get("price_type"),
		PriceFrom:    priceFrom,
		PriceTo:      priceTo,
		Availability: r.PostForm.Get("availability"),
		ServiceArea:  r.PostForm.Get("service_area"),
		IsFeatured:   existingService.IsFeatured, // Preserve featured status
	}

	// Validate required fields
	if service.ServiceName == "" || service.Category == "" || service.PriceType == "" {
		http.Error(w, "Required fields missing", http.StatusBadRequest)
		return
	}

	// Update the service
	err = app.Services.Update(service)
	if err != nil {
		log.Printf("Error updating service: %v", err)
		http.Error(w, "Error updating service", http.StatusInternalServerError)
		return
	}

	// Handle HTMX request
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/merchant/dashboard")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Regular form submission - redirect to services page
	http.Redirect(w, r, "/merchant/dashboard", http.StatusSeeOther)
}

// MerchantServiceDashboard handler - temporary development route
func (app *Application) MerchantServiceDashboard(w http.ResponseWriter, r *http.Request) {
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

	// Initialize basic template data
	data := &templateData{
		IsAuthenticated: true,
		Merchant:        merchant,
	}

	// Get unread message count
	unreadCount, err := app.Messages.GetUnreadCount(merchantID)
	if err != nil {
		log.Printf("Error getting unread count: %v", err)
		unreadCount = 0
	}
	data.UnreadMessageCount = unreadCount

	// Get total views
	totalViews, err := app.StoreViews.GetTotalViews(merchantID)
	if err != nil {
		log.Printf("Error getting view count: %v", err)
		totalViews = 0
	}
	data.TotalViews = totalViews

	// Get services for the merchant
	services, err := app.Services.GetByMerchantID(merchantID)
	if err != nil {
		log.Printf("Error fetching services: %v", err)
		// Don't return error, just set to empty slice
		services = []*models.Service{}
	}
	data.Services = services

	app.render(w, r, http.StatusOK, "merchant.service-dashboard.page.html", data)
}
