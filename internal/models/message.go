package models

import (
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"
)

type Message struct {
	ID            int64
	MerchantID    int64
	CustomerName  string
	CustomerEmail string
	CustomerPhone string
	MessageText   string
	IsRead        bool
	Status        string
	CreatedAt     time.Time
}

type MessageModel struct {
	DB *sql.DB
}

// Basic content filtering
var forbiddenWords = []string{
	"spam", "scam", "bitcoin", "crypto",
	// Add more as needed
}

func (m *MessageModel) validateContent(message string) error {
	// Check for minimum length
	if len(strings.TrimSpace(message)) < 10 {
		return errors.New("message too short")
	}

	// Check for forbidden words
	messageLower := strings.ToLower(message)
	for _, word := range forbiddenWords {
		if strings.Contains(messageLower, word) {
			return errors.New("message contains inappropriate content")
		}
	}

	// Check for too many links
	urlRegex := regexp.MustCompile(`https?://`)
	if len(urlRegex.FindAllString(message, -1)) > 2 {
		return errors.New("too many links in message")
	}

	return nil
}

func (m *MessageModel) Insert(msg *Message) error {
	// Validate content
	if err := m.validateContent(msg.MessageText); err != nil {
		return err
	}

	stmt := `
        INSERT INTO messages (
            merchant_id, customer_name, customer_email, 
            customer_phone, message, status
        ) VALUES (?, ?, ?, ?, ?, 'pending')
    `

	result, err := m.DB.Exec(stmt,
		msg.MerchantID,
		msg.CustomerName,
		msg.CustomerEmail,
		msg.CustomerPhone,
		msg.MessageText,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	msg.ID = id
	return nil
}

func (m *MessageModel) GetUnreadCount(merchantID int64) (int, error) {
	var count int
	stmt := `
        SELECT COUNT(*) 
        FROM messages 
        WHERE merchant_id = ? AND is_read = false
    `
	err := m.DB.QueryRow(stmt, merchantID).Scan(&count)
	return count, err
}

func (m *MessageModel) GetByMerchantID(merchantID int64) ([]*Message, error) {
	stmt := `
        SELECT id, merchant_id, customer_name, customer_email,
               customer_phone, message, is_read, status, created_at
        FROM messages
        WHERE merchant_id = ?
        ORDER BY created_at DESC
    `

	rows, err := m.DB.Query(stmt, merchantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		m := &Message{}
		err := rows.Scan(
			&m.ID, &m.MerchantID, &m.CustomerName,
			&m.CustomerEmail, &m.CustomerPhone, &m.MessageText,
			&m.IsRead, &m.Status, &m.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}

	return messages, nil
}

func (m *MessageModel) MarkAsRead(messageID, merchantID int64) error {
	stmt := `
        UPDATE messages 
        SET is_read = true 
        WHERE id = ? AND merchant_id = ?
    `
	_, err := m.DB.Exec(stmt, messageID, merchantID)
	return err
}
