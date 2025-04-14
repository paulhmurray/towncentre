package handlers

import (
	"log"
	"net/http"
	"strconv"
)

// QuickStockUpdate handles quick stock updates from the dashboard
func (app *Application) QuickStockUpdate(w http.ResponseWriter, r *http.Request) {
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

	// Parse form
	err = r.ParseForm()
	if err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Get stock count from form
	stockCountStr := r.PostForm.Get("stock_count")
	stockCount, err := strconv.Atoi(stockCountStr)
	if err != nil {
		http.Error(w, "Invalid stock count", http.StatusBadRequest)
		return
	}

	// Update stock in database
	err = app.Products.UpdateStock(productID, merchantID, stockCount)
	if err != nil {
		log.Printf("Error updating stock: %v", err)
		http.Error(w, "Error updating stock", http.StatusInternalServerError)
		return
	}

	// Redirect back to dashboard
	http.Redirect(w, r, "/merchant/dashboard", http.StatusSeeOther)
}
