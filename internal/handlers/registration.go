package handlers

import (
	"fmt"
	"log"
	"net/http"
)

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
		otherBusinessType := r.PostForm.Get("other-business-type") // New field
		businessModel := r.PostForm.Get("business-model")
		password := r.PostForm.Get("password")
		passwordConfirm := r.PostForm.Get("password-confirm")

		// If "other" is selected and a custom type is provided, use it
		if businessType == "other" && otherBusinessType != "" {
			businessType = otherBusinessType
		}
		// Validate business model
		if businessModel != "product" && businessModel != "service" {
			businessModel = "product" // Default to product if invalid
		}

		// Validate required fields
		if businessName == "" || email == "" || businessType == "" || password == "" {
			if r.Header.Get("HX-Request") == "true" {
				w.Write([]byte(`
                    <div class="rounded-md bg-red-50 p-4 mt-4">
                        <div class="flex">
                            <div class="ml-3">
                                <h3 class="text-sm font-medium text-red-800">Registration Failed</h3>
                                <div class="mt-2 text-sm text-red-700">
                                    <p>Please fill in all required fields.</p>
                                </div>
                            </div>
                        </div>
                    </div>
                `))
				return
			}
			http.Error(w, "Please fill in all required fields", http.StatusBadRequest)
			return
		}

		if password != passwordConfirm {
			if r.Header.Get("HX-Request") == "true" {
				w.Write([]byte(`
                    <div class="rounded-md bg-red-50 p-4 mt-4">
                        <div class="flex">
                            <div class="ml-3">
                                <h3 class="text-sm font-medium text-red-800">Registration Failed</h3>
                                <div class="mt-2 text-sm text-red-700">
                                    <p>Passwords do not match.</p>
                                </div>
                            </div>
                        </div>
                    </div>
                `))
				return
			}
			http.Error(w, "Passwords do not match", http.StatusBadRequest)
			return
		}

		// Insert the merchant with the resolved businessType and get the merchant ID
		merchantID, err := app.Merchants.Insert(businessName, email, phone, businessType, businessModel, password)
		if err != nil {
			log.Printf("Error registering merchant: %v", err)
			if r.Header.Get("HX-Request") == "true" {
				w.Write([]byte(`
                    <div class="rounded-md bg-red-50 p-4 mt-4">
                        <div class="flex">
                            <div class="ml-3">
                                <h3 class="text-sm font-medium text-red-800">Registration Failed</h3>
                                <div class="mt-2 text-sm text-red-700">
                                    <p>Error creating account. Please try again later.</p>
                                </div>
                            </div>
                        </div>
                    </div>
                `))
				return
			}
			http.Error(w, "Error creating account", http.StatusInternalServerError)
			return
		}

		// Create default vouchers for the new merchant using the merchant model's function
		err = app.Merchants.InsertDefaultVouchers(merchantID, app.Products)
		if err != nil {
			log.Printf("Warning: Failed to create default vouchers for merchant %d: %v", merchantID, err)
			// Continue with registration even if voucher creation fails
		}

		// After successful registration, authenticate the user
		merchant, err := app.Merchants.Authenticate(email, password)
		if err != nil {
			log.Printf("Error authenticating new merchant: %v", err)
			// Fall back to redirecting to login page if automatic login faill
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/merchant/login")
				return
			}
			http.Redirect(w, r, "/merchant/login", http.StatusSeeOther)
			return
		}

		// Set the merchant ID in the session
		app.Sessions.Put(r.Context(), "merchantID", merchant.ID)

		// Redirect to the dashboard
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", "/merchant/dashboard")
			return
		}

		http.Redirect(w, r, "/merchant/dashboard", http.StatusSeeOther)
	}
}

func (app *Application) Terms(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, http.StatusOK, "terms.page.html", nil)
}

// CheckBusinessType handles the HTMX request to show/hide the "Other" text field
func (app *Application) CheckBusinessType(w http.ResponseWriter, r *http.Request) {
	businessType := r.URL.Query().Get("business-type")
	if businessType == "other" {
		// Return the text field
		fmt.Fprint(w, `
			<div id="other-business-type-container" class="">
				<label for="other-business-type" class="block text-sm font-medium text-gray-700">Please specify your business type</label>
				<input type="text" name="other-business-type" id="other-business-type" class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500" />
			</div>
		`)
	} else {
		// Return an empty hidden div
		fmt.Fprint(w, `<div id="other-business-type-container" class="hidden"></div>`)
	}
}

// BusinessCategories returns different category options based on the business model
func (app *Application) BusinessCategories(w http.ResponseWriter, r *http.Request) {
	businessType := r.URL.Query().Get("type")
	w.Header().Set("Content-Type", "text/html")

	// Default to product categories if not specified
	if businessType != "service" {
		// Product categories
		w.Write([]byte(`
			<option value="">Select a category</option>
			<option value="retail">Retail</option>
			<option value="food">Food & Drink</option>
			<option value="crafts">Local Crafts</option>
			<option value="health">Health & Wellness Products</option>
			<option value="arts">Creative Arts</option>
			<option value="home">Home & Garden</option>
			<option value="other">Other</option>
		`))
	} else {
		// Service categories
		w.Write([]byte(`
			<option value="">Select a category</option>
			<option value="trades">Trades & Construction</option>
			<option value="professional">Professional Services</option>
			<option value="health">Health & Wellness Services</option>
			<option value="tech">Technology</option>
			<option value="automotive">Automotive</option>
			<option value="education">Education & Training</option>
			<option value="creative">Creative Services</option>
			<option value="personal">Personal Services</option>
			<option value="other">Other</option>
		`))
	}
}
