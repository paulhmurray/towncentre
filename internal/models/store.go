// models/store_views.go
package models

import (
	"database/sql"
	"time"
)

type StoreView struct {
	ID         int64
	MerchantID int64
	ViewedAt   time.Time
	ViewerIP   string
}

type StoreViewModel struct {
	DB *sql.DB
}

func (m *StoreViewModel) RecordView(merchantID int64, viewerIP string) error {
	stmt := `
        INSERT INTO store_views (merchant_id, viewer_ip)
        VALUES (?, ?)
    `
	_, err := m.DB.Exec(stmt, merchantID, viewerIP)
	return err
}

func (m *StoreViewModel) GetTotalViews(merchantID int64) (int, error) {
	var count int
	stmt := `
        SELECT COUNT(*) 
        FROM store_views 
        WHERE merchant_id = ?
    `
	err := m.DB.QueryRow(stmt, merchantID).Scan(&count)
	return count, err
}
