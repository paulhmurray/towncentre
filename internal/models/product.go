package models

import (
	"database/sql"
	"time"
)

type Product struct {
	ID            int64
	MerchantID    int64
	Name          string
	Description   string
	Price         float64
	Category      string
	ImagePath     string
	ThumbnailPath string
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
            has_delivery, has_pickup, created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`

	result, err := m.DB.Exec(stmt,
		p.MerchantID, p.Name, p.Description, p.Price,
		p.Category, p.HasDelivery, p.HasPickup)
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

func (m *ProductModel) GetByMerchantID(merchantID int64) ([]*Product, error) {
	stmt := `
        SELECT id, merchant_id, name, description, price, 
               category, has_delivery, has_pickup, created_at, updated_at
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
		err := rows.Scan(
			&p.ID, &p.MerchantID, &p.Name, &p.Description,
			&p.Price, &p.Category, &p.HasDelivery, &p.HasPickup,
			&p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		products = append(products, p)
	}

	return products, nil
}
