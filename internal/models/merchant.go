package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Merchant struct {
	ID           int64
	BusinessName string
	StoreName    string
	StoreSlug    string
	Region       string
	Description  string
	Email        string
	Phone        string
	BusinessType string
	Location     string
	OpeningHours string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type MerchantModel struct {
	DB *sql.DB
}

// Insert adds a new merchant to the database
func (m *MerchantModel) Insert(businessName, email, phone, businessType, password string) error {
	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}

	// Generate store slug from business name
	baseSlug := generateSlug(businessName)
	slug := baseSlug
	counter := 1

	// Keep checking until we find a unique slug
	for {
		var exists bool
		err := m.DB.QueryRow(
			"SELECT EXISTS(SELECT 1 FROM merchants WHERE store_slug = ? AND region = ?)",
			slug, "ballarat", // Hardcoding 'ballarat' for now
		).Scan(&exists)

		if err != nil {
			return err
		}

		if !exists {
			break
		}

		// If slug exists, append a number and try again
		slug = fmt.Sprintf("%s-%d", baseSlug, counter)
		counter++
	}

	// Insert with new fields, using business name as store name initially
	stmt := `
        INSERT INTO merchants (
            business_name, store_name, store_slug, region,
            email, phone, business_type, password_hash,
            created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`

	_, err = m.DB.Exec(stmt,
		businessName,   // business_name
		businessName,   // store_name (same as business_name initially)
		slug,           // store_slug
		"ballarat",     // region (hardcoded for now)
		email,          // email
		phone,          // phone
		businessType,   // business_type
		hashedPassword, // password_hash
	)

	return err
}

// Authenticate verifies a merchant's email and password
func (m *MerchantModel) Authenticate(email, password string) (*Merchant, error) {
	merchant := &Merchant{}
	var hashedPassword []byte

	stmt := `
        SELECT id, business_name, store_name, store_slug, region,
               email, phone, business_type, password_hash,
               created_at, updated_at 
        FROM merchants 
        WHERE email = ?`

	err := m.DB.QueryRow(stmt, email).Scan(
		&merchant.ID,
		&merchant.BusinessName,
		&merchant.StoreName,
		&merchant.StoreSlug,
		&merchant.Region,
		&merchant.Email,
		&merchant.Phone,
		&merchant.BusinessType,
		&hashedPassword,
		&merchant.CreatedAt,
		&merchant.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No matching record found
	} else if err != nil {
		return nil, err
	}

	// Check password
	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(password))
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return nil, nil // Incorrect password
	} else if err != nil {
		return nil, err
	}

	return merchant, nil
}

// GetByID retrieves a merchant by their ID
func (m *MerchantModel) GetByID(id int64) (*Merchant, error) {
	merchant := &Merchant{}

	stmt := `SELECT id, business_name, email, phone, business_type, created_at, updated_at 
             FROM merchants WHERE id = ?`

	err := m.DB.QueryRow(stmt, id).Scan(
		&merchant.ID,
		&merchant.BusinessName,
		&merchant.Email,
		&merchant.Phone,
		&merchant.BusinessType,
		&merchant.CreatedAt,
		&merchant.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	return merchant, err
}
func generateSlug(name string) string {
	// Convert to lowercase
	slug := strings.ToLower(name)

	// Replace spaces with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")

	// Remove any special characters
	slug = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return -1
	}, slug)

	// Remove any double hyphens
	slug = strings.ReplaceAll(slug, "--", "-")

	// Trim hyphens from start and end
	slug = strings.Trim(slug, "-")

	return slug
}

func (m *MerchantModel) GetByStoreSlugAndRegion(slug, region string) (*Merchant, error) {
	stmt := `
        SELECT id, business_name, store_name, store_slug, region,
               description, email, phone, business_type, location,
               opening_hours, created_at, updated_at
        FROM merchants
        WHERE store_slug = ? AND region = ?`

	merchant := &Merchant{}
	err := m.DB.QueryRow(stmt, slug, region).Scan(
		&merchant.ID, &merchant.BusinessName, &merchant.StoreName,
		&merchant.StoreSlug, &merchant.Region, &merchant.Description,
		&merchant.Email, &merchant.Phone, &merchant.BusinessType,
		&merchant.Location, &merchant.OpeningHours,
		&merchant.CreatedAt, &merchant.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return merchant, nil
}
