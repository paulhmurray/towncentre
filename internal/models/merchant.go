package models

import (
	"database/sql"
	"golang.org/x/crypto/bcrypt"
	"time"
)

type Merchant struct {
	ID           int64
	BusinessName string
	Email        string
	Phone        string
	BusinessType string
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

	stmt := `
        INSERT INTO merchants (business_name, email, phone, business_type, password_hash, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, NOW(), NOW())`

	_, err = m.DB.Exec(stmt, businessName, email, phone, businessType, hashedPassword)
	return err
}

// Authenticate verifies a merchant's email and password
func (m *MerchantModel) Authenticate(email, password string) (*Merchant, error) {
	merchant := &Merchant{}
	var hashedPassword []byte

	stmt := `SELECT id, business_name, email, phone, business_type, password_hash, created_at, updated_at 
             FROM merchants WHERE email = ?`

	err := m.DB.QueryRow(stmt, email).Scan(
		&merchant.ID,
		&merchant.BusinessName,
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
