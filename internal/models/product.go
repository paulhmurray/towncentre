package models

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

type Product struct {
	ID            int64
	MerchantID    int64
	Name          string
	Description   string
	Price         float64
	Category      string
	ImagePath     *string
	ThumbnailPath *string
	HasDelivery   bool
	HasPickup     bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ProductModel struct {
	DB *sql.DB
}

func (m *ProductModel) Insert(p *Product) error {
	stmt := `
        INSERT INTO products (
            merchant_id, name, description, price, category, 
            image_path, thumbnail_path, has_delivery, has_pickup, 
            created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`

	var imagePath, thumbnailPath *string
	if p.ImagePath != nil {
		imagePath = p.ImagePath
	}
	if p.ThumbnailPath != nil {
		thumbnailPath = p.ThumbnailPath
	}
	result, err := m.DB.Exec(stmt,
		p.MerchantID, p.Name, p.Description, p.Price,
		p.Category, imagePath, thumbnailPath,
		p.HasDelivery, p.HasPickup)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	p.ID = id
	return nil
}

// In models/product.go
func (m *ProductModel) Delete(productID int64, merchantID int64) error {
	stmt := `DELETE FROM products WHERE id = ? AND merchant_id = ?`

	// Execute the delete with both product ID and merchant ID to ensure
	// a merchant can only delete their own products
	result, err := m.DB.Exec(stmt, productID, merchantID)
	if err != nil {
		return err
	}

	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	// If no rows were affected, either the product doesn't exist
	// or it doesn't belong to the merchant
	if rowsAffected == 0 {
		return fmt.Errorf("no product found with ID %d for this merchant", productID)
	}

	return nil
}

// Update
func (m *ProductModel) Update(p *Product) error {
	stmt := `
        UPDATE products 
        SET name = ?, description = ?, price = ?, category = ?, 
            image_path = ?, thumbnail_path = ?, 
            has_delivery = ?, has_pickup = ?, 
            updated_at = NOW() 
        WHERE id = ? AND merchant_id = ?`

	var imagePath, thumbnailPath *string
	if p.ImagePath != nil {
		imagePath = p.ImagePath
	}
	if p.ThumbnailPath != nil {
		thumbnailPath = p.ThumbnailPath
	}

	_, err := m.DB.Exec(stmt,
		p.Name, p.Description, p.Price, p.Category,
		imagePath, thumbnailPath,
		p.HasDelivery, p.HasPickup,
		p.ID, p.MerchantID)

	return err
}

// Get a single product by ID for a specific merchant
func (m *ProductModel) GetByID(productID, merchantID int64) (*Product, error) {
	log.Printf("Getting product %d for merchant %d", productID, merchantID)

	stmt := `
        SELECT id, merchant_id, name, description, price, 
               category, image_path, thumbnail_path,
               has_delivery, has_pickup, created_at, updated_at
        FROM products 
        WHERE id = ? AND merchant_id = ?`

	p := &Product{}
	var imagePath, thumbnailPath sql.NullString

	err := m.DB.QueryRow(stmt, productID, merchantID).Scan(
		&p.ID,
		&p.MerchantID,
		&p.Name,
		&p.Description,
		&p.Price,
		&p.Category,
		&imagePath,
		&thumbnailPath,
		&p.HasDelivery,
		&p.HasPickup,
		&p.CreatedAt,
		&p.UpdatedAt,
	)

	if err != nil {
		log.Printf("Error getting product: %v", err)
		return nil, err
	}

	log.Printf("Found product: %+v", p)

	// Convert sql.NullString to *string
	if imagePath.Valid {
		p.ImagePath = &imagePath.String
		log.Printf("Image path: %s", *p.ImagePath)
	}
	if thumbnailPath.Valid {
		p.ThumbnailPath = &thumbnailPath.String
		log.Printf("Thumbnail path: %s", *p.ThumbnailPath)
	}

	return p, nil
}

func (m *ProductModel) GetByMerchantID(merchantID int64) ([]*Product, error) {
	stmt := `
        SELECT id, merchant_id, name, description, price, 
               category, image_path, thumbnail_path,
               has_delivery, has_pickup, created_at, updated_at
        FROM products 
        WHERE merchant_id = ?
        ORDER BY created_at DESC`

	rows, err := m.DB.Query(stmt, merchantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []*Product
	for rows.Next() {
		p := &Product{}
		var imagePath, thumbnailPath sql.NullString
		err := rows.Scan(
			&p.ID,
			&p.MerchantID,
			&p.Name,
			&p.Description,
			&p.Price,
			&p.Category,
			&imagePath,
			&thumbnailPath,
			&p.HasDelivery,
			&p.HasPickup,
			&p.CreatedAt,
			&p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		// Convert sql.NullString to *string
		if imagePath.Valid {
			p.ImagePath = &imagePath.String
		}
		if thumbnailPath.Valid {
			p.ThumbnailPath = &thumbnailPath.String
		}
		products = append(products, p)
	}

	return products, nil
}
