// models/service.go
package models

import (
	"database/sql"
	"fmt"
	"time"
)

type Service struct {
	ID           int64
	MerchantID   int64
	ServiceName  string
	Description  string
	Category     string
	PriceType    string
	PriceFrom    *float64
	PriceTo      *float64
	Availability string
	ServiceArea  string
	IsFeatured   bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ServiceModel struct {
	DB *sql.DB
}

// Insert adds a new service to the database
func (m *ServiceModel) Insert(s *Service) error {
	stmt := `
        INSERT INTO services (
            merchant_id, service_name, description, category,
            price_type, price_from, price_to, availability,
            service_area, is_featured
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `

	result, err := m.DB.Exec(
		stmt,
		s.MerchantID, s.ServiceName, s.Description, s.Category,
		s.PriceType, s.PriceFrom, s.PriceTo, s.Availability,
		s.ServiceArea, s.IsFeatured,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	s.ID = id
	return nil
}

// GetByMerchantID fetches all services for a merchant
func (m *ServiceModel) GetByMerchantID(merchantID int64) ([]*Service, error) {
	stmt := `
        SELECT id, merchant_id, service_name, description, category,
               price_type, price_from, price_to, availability,
               service_area, is_featured, created_at, updated_at
        FROM services
        WHERE merchant_id = ?
        ORDER BY created_at DESC
    `

	rows, err := m.DB.Query(stmt, merchantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []*Service

	for rows.Next() {
		service := &Service{}
		var priceFrom, priceTo sql.NullFloat64

		err := rows.Scan(
			&service.ID, &service.MerchantID, &service.ServiceName,
			&service.Description, &service.Category, &service.PriceType,
			&priceFrom, &priceTo, &service.Availability,
			&service.ServiceArea, &service.IsFeatured,
			&service.CreatedAt, &service.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Convert nullable fields
		if priceFrom.Valid {
			value := priceFrom.Float64
			service.PriceFrom = &value
		}

		if priceTo.Valid {
			value := priceTo.Float64
			service.PriceTo = &value
		}

		services = append(services, service)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return services, nil
}

// FormattedPriceFrom returns a properly formatted string for PriceFrom
func (s *Service) FormattedPriceFrom() string {
	if s.PriceFrom == nil {
		return "Price not specified"
	}
	return fmt.Sprintf("$%.2f", *s.PriceFrom)
}

// FormattedPriceTo returns a properly formatted string for PriceTo
func (s *Service) FormattedPriceTo() string {
	if s.PriceTo == nil {
		return "Price not specified"
	}
	return fmt.Sprintf("$%.2f", *s.PriceTo)
}

// FormattedPrice returns a complete formatted price string based on price type
func (s *Service) FormattedPrice() string {
	switch s.PriceType {
	case "fixed":
		if s.PriceFrom == nil {
			return "Price not specified"
		}
		return fmt.Sprintf("$%.2f", *s.PriceFrom)
	case "hourly":
		if s.PriceFrom == nil {
			return "Rate not specified"
		}
		return fmt.Sprintf("$%.2f/hour", *s.PriceFrom)
	case "range":
		if s.PriceFrom != nil && s.PriceTo != nil {
			return fmt.Sprintf("$%.2f - $%.2f", *s.PriceFrom, *s.PriceTo)
		} else if s.PriceFrom != nil {
			return fmt.Sprintf("From $%.2f", *s.PriceFrom)
		} else if s.PriceTo != nil {
			return fmt.Sprintf("Up to $%.2f", *s.PriceTo)
		}
		return "Price range not specified"
	case "quote":
		return "Quote on request"
	case "free":
		return "Free"
	default:
		return "Price not available"
	}
}

// GetByID fetches a single service by ID
func (m *ServiceModel) GetByID(id int64) (*Service, error) {
	stmt := `
        SELECT id, merchant_id, service_name, description, category,
               price_type, price_from, price_to, availability,
               service_area, is_featured, created_at, updated_at
        FROM services
        WHERE id = ?
    `

	service := &Service{}
	var priceFrom, priceTo sql.NullFloat64

	err := m.DB.QueryRow(stmt, id).Scan(
		&service.ID, &service.MerchantID, &service.ServiceName,
		&service.Description, &service.Category, &service.PriceType,
		&priceFrom, &priceTo, &service.Availability,
		&service.ServiceArea, &service.IsFeatured,
		&service.CreatedAt, &service.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// Convert nullable fields
	if priceFrom.Valid {
		value := priceFrom.Float64
		service.PriceFrom = &value
	}

	if priceTo.Valid {
		value := priceTo.Float64
		service.PriceTo = &value
	}

	return service, nil
}

// Update method for Service model
func (m *ServiceModel) Update(s *Service) error {
	stmt := `
        UPDATE services
        SET service_name = ?, description = ?, category = ?,
            price_type = ?, price_from = ?, price_to = ?,
            availability = ?, service_area = ?, is_featured = ?,
            updated_at = NOW()
        WHERE id = ? AND merchant_id = ?
    `

	_, err := m.DB.Exec(
		stmt,
		s.ServiceName, s.Description, s.Category,
		s.PriceType, s.PriceFrom, s.PriceTo,
		s.Availability, s.ServiceArea, s.IsFeatured,
		s.ID, s.MerchantID,
	)

	return err
}
