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
	BusinessName  string // For displaying the store name with the product
	Location      string // For displaying the store location
}

type ProductModel struct {
	DB *sql.DB
}

func (m *ProductModel) Insert(p *Product) error {
	log.Printf("ProductModel.Insert called for product: %s", p.Name)
	
	stmt := `
        INSERT INTO products (
            merchant_id, name, description, price, category, 
            image_path, thumbnail_path, has_delivery, has_pickup, 
            created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`

	var imagePath, thumbnailPath *string
	if p.ImagePath != nil {
		imagePath = p.ImagePath
		log.Printf("Using image path: %s", *imagePath)
	} else {
		log.Printf("No image path provided")
	}
	if p.ThumbnailPath != nil {
		thumbnailPath = p.ThumbnailPath
		log.Printf("Using thumbnail path: %s", *thumbnailPath)
	} else {
		log.Printf("No thumbnail path provided")
	}
	
	log.Printf("Executing SQL with parameters - MerchantID: %d, Name: %s, Category: %s, ImagePath: %v, ThumbnailPath: %v",
		p.MerchantID, p.Name, p.Category, imagePath, thumbnailPath)
	
	result, err := m.DB.Exec(stmt,
		p.MerchantID, p.Name, p.Description, p.Price,
		p.Category, imagePath, thumbnailPath,
		p.HasDelivery, p.HasPickup)
	if err != nil {
		log.Printf("Error executing SQL: %v", err)
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("Error getting last insert ID: %v", err)
		return err
	}

	p.ID = id
	log.Printf("Product %s inserted with ID: %d", p.Name, p.ID)
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
func (m *ProductModel) GetByID(productID int64, merchantID int64) (*Product, error) {
	log.Printf("Getting product %d for merchant %d", productID, merchantID)

	var stmt string
	var args []interface{}

	if merchantID > 0 {
		// For merchant viewing (checks ownership)
		stmt = `
            SELECT id, merchant_id, name, description, price, 
                   category, image_path, thumbnail_path,
                   has_delivery, has_pickup, created_at, updated_at
            FROM products 
            WHERE id = ? AND merchant_id = ?`
		args = []interface{}{productID, merchantID}
	} else {
		// For public viewing (no ownership check)
		stmt = `
            SELECT id, merchant_id, name, description, price, 
                   category, image_path, thumbnail_path,
                   has_delivery, has_pickup, created_at, updated_at
            FROM products 
            WHERE id = ?`
		args = []interface{}{productID}
	}

	p := &Product{}
	var imagePath, thumbnailPath sql.NullString

	err := m.DB.QueryRow(stmt, args...).Scan(
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

// for home page featured products
func (m *ProductModel) GetFeatured() ([]*Product, error) {
	stmt := `
        SELECT p.id, p.merchant_id, p.name, p.description, p.price, 
               p.category, p.image_path, p.thumbnail_path,
               p.has_delivery, p.has_pickup, p.created_at, p.updated_at,
               m.business_name, m.location
        FROM products p
        JOIN merchants m ON p.merchant_id = m.id
        ORDER BY p.created_at DESC
        LIMIT 8`

	rows, err := m.DB.Query(stmt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []*Product
	for rows.Next() {
		p := &Product{}
		var imagePath, thumbnailPath sql.NullString
		var businessName, location sql.NullString

		err := rows.Scan(
			&p.ID, &p.MerchantID, &p.Name, &p.Description, &p.Price,
			&p.Category, &imagePath, &thumbnailPath,
			&p.HasDelivery, &p.HasPickup, &p.CreatedAt, &p.UpdatedAt,
			&businessName, &location,
		)
		if err != nil {
			return nil, err
		}

		if imagePath.Valid {
			p.ImagePath = &imagePath.String
		}
		if thumbnailPath.Valid {
			p.ThumbnailPath = &thumbnailPath.String
		}
		if businessName.Valid {
			p.BusinessName = businessName.String
		}
		if location.Valid {
			p.Location = location.String
		} else {
			p.Location = "Ballarat" // Default location
		}

		products = append(products, p)
	}

	return products, nil
}
func (m *ProductModel) GetTotalCount(merchantID int64) (int, error) {
	var count int
	stmt := `
        SELECT COUNT(*) 
        FROM products 
        WHERE merchant_id = ?
    `
	err := m.DB.QueryRow(stmt, merchantID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}
