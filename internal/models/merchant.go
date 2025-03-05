package models

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
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

// PasswordResetToken struct
type PasswordResetToken struct {
	ID         int64
	MerchantID int64
	Token      string
	ExpiresAt  time.Time
	Used       bool
	CreatedAt  time.Time
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

func (m *MerchantModel) GetByID(id int64) (*Merchant, error) {
	merchant := &Merchant{}

	// Use NullString for fields that could be NULL
	var description, location, openingHours, phone sql.NullString

	stmt := `
        SELECT id, business_name, store_name, store_slug, region,
               description, email, phone, business_type, location,
               opening_hours, created_at, updated_at 
        FROM merchants 
        WHERE id = ?`

	err := m.DB.QueryRow(stmt, id).Scan(
		&merchant.ID,
		&merchant.BusinessName,
		&merchant.StoreName,
		&merchant.StoreSlug,
		&merchant.Region,
		&description, // Changed from &merchant.Description
		&merchant.Email,
		&phone, // Changed from &merchant.Phone
		&merchant.BusinessType,
		&location,     // Changed from &merchant.Location
		&openingHours, // Changed from &merchant.OpeningHours
		&merchant.CreatedAt,
		&merchant.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Convert NullString to string, using empty string if NULL
	if description.Valid {
		merchant.Description = description.String
	}
	if location.Valid {
		merchant.Location = location.String
	}
	if openingHours.Valid {
		merchant.OpeningHours = openingHours.String
	}
	if phone.Valid {
		merchant.Phone = phone.String
	}

	return merchant, nil
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

	// Use NullString for fields that could be NULL
	var description, location, openingHours, phone sql.NullString

	err := m.DB.QueryRow(stmt, slug, region).Scan(
		&merchant.ID,
		&merchant.BusinessName,
		&merchant.StoreName,
		&merchant.StoreSlug,
		&merchant.Region,
		&description, // Changed from &merchant.Description
		&merchant.Email,
		&phone, // Changed from &merchant.Phone
		&merchant.BusinessType,
		&location,     // Changed from &merchant.Location
		&openingHours, // Changed from &merchant.OpeningHours
		&merchant.CreatedAt,
		&merchant.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Convert NullString to string, using empty string if NULL
	if description.Valid {
		merchant.Description = description.String
	}
	if location.Valid {
		merchant.Location = location.String
	}
	if openingHours.Valid {
		merchant.OpeningHours = openingHours.String
	}
	if phone.Valid {
		merchant.Phone = phone.String
	}

	return merchant, nil
}
func (m *MerchantModel) UpdateStoreInfo(merchant *Merchant) error {
	stmt := `
        UPDATE merchants 
        SET store_name = ?,
            description = ?,
            location = ?,
            opening_hours = ?,
            updated_at = NOW()
        WHERE id = ?`

	_, err := m.DB.Exec(stmt,
		merchant.StoreName,
		merchant.Description,
		merchant.Location,
		merchant.OpeningHours,
		merchant.ID)

	if err != nil {
		return fmt.Errorf("error updating merchant: %v", err)
	}

	return nil
}

func (m *MerchantModel) GetFeatured() ([]*Merchant, error) {
	stmt := `
        SELECT id, business_name, store_name, store_slug, region,
               description, email, phone, business_type, location,
               opening_hours, created_at, updated_at
        FROM merchants
        WHERE region = 'ballarat'
        ORDER BY created_at DESC
        LIMIT 6`

	rows, err := m.DB.Query(stmt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var merchants []*Merchant
	for rows.Next() {
		m := &Merchant{}
		// Use NullString for fields that could be NULL
		var description, location, openingHours, phone sql.NullString

		err := rows.Scan(
			&m.ID,
			&m.BusinessName,
			&m.StoreName,
			&m.StoreSlug,
			&m.Region,
			&description,
			&m.Email,
			&phone,
			&m.BusinessType,
			&location,
			&openingHours,
			&m.CreatedAt,
			&m.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Convert NullString to string, using empty string if NULL
		if description.Valid {
			m.Description = description.String
		}
		if location.Valid {
			m.Location = location.String
		}
		if openingHours.Valid {
			m.OpeningHours = openingHours.String
		}
		if phone.Valid {
			m.Phone = phone.String
		}

		merchants = append(merchants, m)
	}

	return merchants, nil
}
func (m *MerchantModel) InitiatePasswordReset(email string) (*PasswordResetToken, error) {
	// Find merchant by email
	var merchantID int64
	err := m.DB.QueryRow("SELECT id FROM merchants WHERE email = ?", email).Scan(&merchantID)
	if err == sql.ErrNoRows {
		return nil, nil // Merchant not found
	} else if err != nil {
		return nil, fmt.Errorf("error finding merchant: %v", err)
	}

	// Generate secure token
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("error generating token: %v", err)
	}

	// Set expiration (1 hour from now)
	expiresAt := time.Now().Add(1 * time.Hour)

	// Insert reset token
	stmt := `
        INSERT INTO password_reset_tokens (merchant_id, token, expires_at, created_at)
        VALUES (?, ?, ?, NOW())`
	result, err := m.DB.Exec(stmt, merchantID, token, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("error inserting reset token: %v", err)
	}

	resetID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("error getting reset token ID: %v", err)
	}

	return &PasswordResetToken{
		ID:         resetID,
		MerchantID: merchantID,
		Token:      token,
		ExpiresAt:  expiresAt,
		Used:       false,
		CreatedAt:  time.Now(),
	}, nil
}

// New method to verify and reset password
func (m *MerchantModel) ResetPassword(token, newPassword string) error {
	// Verify token
	var resetToken PasswordResetToken
	stmt := `
        SELECT id, merchant_id, token, expires_at, used, created_at
        FROM password_reset_tokens
        WHERE token = ? AND used = FALSE AND expires_at > NOW()`
	err := m.DB.QueryRow(stmt, token).Scan(
		&resetToken.ID,
		&resetToken.MerchantID,
		&resetToken.Token,
		&resetToken.ExpiresAt,
		&resetToken.Used,
		&resetToken.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return fmt.Errorf("invalid or expired token")
	} else if err != nil {
		return fmt.Errorf("error verifying token: %v", err)
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return fmt.Errorf("error hashing password: %v", err)
	}

	// Start transaction
	tx, err := m.DB.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	// Update merchant password
	_, err = tx.Exec("UPDATE merchants SET password_hash = ?, updated_at = NOW() WHERE id = ?",
		hashedPassword, resetToken.MerchantID)
	if err != nil {
		return fmt.Errorf("error updating password: %v", err)
	}

	// Mark token as used
	_, err = tx.Exec("UPDATE password_reset_tokens SET used = TRUE WHERE id = ?", resetToken.ID)
	if err != nil {
		return fmt.Errorf("error marking token as used: %v", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	return nil
}

// Helper function to generate secure token
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
