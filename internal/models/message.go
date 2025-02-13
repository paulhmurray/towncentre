package models

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
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
	log.Printf("Getting unread count for merchant ID: %d", merchantID)
	var count int
	stmt := `
        SELECT COUNT(*) 
        FROM messages 
        WHERE merchant_id = ? AND is_read = false
    `
	err := m.DB.QueryRow(stmt, merchantID).Scan(&count)
	if err != nil {
		log.Printf("Error getting unread count: %v", err)
		return 0, err
	}
	log.Printf("Found %d unread messages", count)
	return count, err
}

func (m *MessageModel) GetByMerchantID(merchantID int64) ([]*Message, error) {
	log.Printf("Getting messages for merchant %d", merchantID)

	stmt := `
        SELECT id, merchant_id, customer_name, customer_email,
               customer_phone, message, is_read, status, created_at
        FROM messages
        WHERE merchant_id = ?
        ORDER BY created_at DESC
    `

	rows, err := m.DB.Query(stmt, merchantID)
	if err != nil {
		log.Printf("Error querying messages: %v", err)
		return nil, err
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		msg := &Message{}
		err := rows.Scan(
			&msg.ID, &msg.MerchantID, &msg.CustomerName,
			&msg.CustomerEmail, &msg.CustomerPhone, &msg.MessageText,
			&msg.IsRead, &msg.Status, &msg.CreatedAt,
		)
		if err != nil {
			log.Printf("Error scanning message: %v", err)
			return nil, err
		}
		messages = append(messages, msg)
		log.Printf("Added message: %+v", msg)
	}

	log.Printf("Returning %d messages", len(messages))
	return messages, nil
}

func (m *MessageModel) MarkAsRead(messageID, merchantID int64) error {
	log.Printf("Marking message %d as read for merchant %d", messageID, merchantID)

	stmt := `
        UPDATE messages 
        SET is_read = true 
        WHERE id = ? AND merchant_id = ?
    `
	result, err := m.DB.Exec(stmt, messageID, merchantID)
	if err != nil {
		log.Printf("Error marking message as read: %v", err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no message found with ID %d for merchant %d", messageID, merchantID)
	}

	log.Printf("Successfully marked message %d as read", messageID)
	return nil
}
func (m *MessageModel) GetByID(messageID, merchantID int64) (*Message, error) {
	log.Printf("Getting message ID: %d for merchant ID: %d", messageID, merchantID)

	stmt := `
        SELECT id, merchant_id, customer_name, customer_email,
               customer_phone, message, is_read, status, created_at
        FROM messages
        WHERE id = ? AND merchant_id = ?
    `

	msg := &Message{}
	err := m.DB.QueryRow(stmt, messageID, merchantID).Scan(
		&msg.ID,
		&msg.MerchantID,
		&msg.CustomerName,
		&msg.CustomerEmail,
		&msg.CustomerPhone,
		&msg.MessageText,
		&msg.IsRead,
		&msg.Status,
		&msg.CreatedAt,
	)

	if err != nil {
		log.Printf("Error getting message: %v", err)
		return nil, err
	}

	log.Printf("Found message: %+v", msg)
	return msg, nil
}
