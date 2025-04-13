// models/service_category.go
package models

import (
	"database/sql"
)

type ServiceCategory struct {
	ID           int64
	Name         string
	Slug         string
	Icon         string
	DisplayOrder int
}

type ServiceCategoryModel struct {
	DB *sql.DB
}

// GetAll fetches all service categories ordered by display_order
func (m *ServiceCategoryModel) GetAll() ([]*ServiceCategory, error) {
	stmt := `
        SELECT id, name, slug, icon, display_order
        FROM service_categories
        ORDER BY display_order ASC
    `

	rows, err := m.DB.Query(stmt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []*ServiceCategory

	for rows.Next() {
		category := &ServiceCategory{}

		err := rows.Scan(
			&category.ID, &category.Name, &category.Slug,
			&category.Icon, &category.DisplayOrder,
		)

		if err != nil {
			return nil, err
		}

		categories = append(categories, category)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return categories, nil
}

// GetBySlug fetches a service category by its slug
func (m *ServiceCategoryModel) GetBySlug(slug string) (*ServiceCategory, error) {
	stmt := `
        SELECT id, name, slug, icon, display_order
        FROM service_categories
        WHERE slug = ?
    `

	category := &ServiceCategory{}

	err := m.DB.QueryRow(stmt, slug).Scan(
		&category.ID, &category.Name, &category.Slug,
		&category.Icon, &category.DisplayOrder,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return category, nil
}
