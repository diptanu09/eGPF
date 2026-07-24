package db

import (
	"fmt"
	"time"
)

// ChatMessage represents an individual communication record
type ChatMessage struct {
	ID               int64     `json:"id"`
	SenderUsername   string    `json:"sender_username"`
	ReceiverUsername string    `json:"receiver_username"`
	MessageText      string    `json:"message_text"`
	IsRead           bool      `json:"is_read"`
	CreatedAt        time.Time `json:"created_at"`
}

// UserChatSummary represents active conversation threads for the Admin view
type UserChatSummary struct {
	Username    string    `json:"username"`
	UnreadCount int       `json:"unread_count"`
	LastMessage string    `json:"last_message"`
	LastActive  time.Time `json:"last_active"`
}

// ExecuteSendMessage inserts a new chat message into the database
func ExecuteSendMessage(sender, receiver, messageText string) error {
	if db == nil {
		return fmt.Errorf("database handle uninitialized")
	}

	if messageText == "" {
		return fmt.Errorf("message content cannot be empty")
	}

	query := `
		INSERT INTO agartala.chat_messages (sender_username, receiver_username, message_text, is_read, created_at)
		VALUES ($1, $2, $3, FALSE, NOW());
	`
	_, err := db.Exec(query, sender, receiver, messageText)
	if err != nil {
		return fmt.Errorf("failed to dispatch chat message: %w", err)
	}

	return nil
}

// FetchChatHistory retrieves conversation history between two users sorted by timestamp
func FetchChatHistory(user1, user2 string, limit int) ([]ChatMessage, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle uninitialized")
	}

	if limit <= 0 {
		limit = 100 // Default safety cap
	}

	query := `
		SELECT message_id, sender_username, receiver_username, message_text, is_read, created_at
		FROM agartala.chat_messages
		WHERE (sender_username = $1 AND receiver_username = $2)
		   OR (sender_username = $2 AND receiver_username = $1)
		ORDER BY created_at ASC
		LIMIT $3;
	`

	rows, err := db.Query(query, user1, user2, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch conversation history: %w", err)
	}
	defer rows.Close()

	var history []ChatMessage
	for rows.Next() {
		var msg ChatMessage
		if err := rows.Scan(&msg.ID, &msg.SenderUsername, &msg.ReceiverUsername, &msg.MessageText, &msg.IsRead, &msg.CreatedAt); err != nil {
			return nil, err
		}
		history = append(history, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return history, nil
}

// MarkMessagesAsRead updates the read state when a user opens a conversation thread
func MarkMessagesAsRead(reader, sender string) error {
	if db == nil {
		return fmt.Errorf("database handle uninitialized")
	}

	query := `
		UPDATE agartala.chat_messages
		SET is_read = TRUE
		WHERE receiver_username = $1 AND sender_username = $2 AND is_read = FALSE;
	`
	_, err := db.Exec(query, reader, sender)
	return err
}

// FetchAdminList retrieves all active administrators available for chat
func FetchAdminList() ([]string, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle uninitialized")
	}

	query := `
		SELECT username 
		FROM agartala.users 
		WHERE role = 'admin' AND status = 'approved' 
		ORDER BY username ASC;
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var admins []string
	for rows.Next() {
		var adminName string
		if err := rows.Scan(&adminName); err == nil {
			admins = append(admins, adminName)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return admins, nil
}

// FetchActiveUserThreads retrieves a list of user conversations for Administrators
func FetchActiveUserThreads() ([]UserChatSummary, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle uninitialized")
	}

	query := `
		WITH LatestMessages AS (
			SELECT 
				CASE WHEN sender_username = 'admin' THEN receiver_username ELSE sender_username END AS user_handle,
				message_text,
				created_at,
				is_read,
				receiver_username,
				ROW_NUMBER() OVER (
					PARTITION BY CASE WHEN sender_username = 'admin' THEN receiver_username ELSE sender_username END 
					ORDER BY created_at DESC
				) as rn
			FROM agartala.chat_messages
		)
		SELECT 
			u.username,
			COALESCE(lm.message_text, 'No messages yet') AS last_message,
			COALESCE(lm.created_at, NOW()) AS last_active,
			(
				SELECT COUNT(*) 
				FROM agartala.chat_messages 
				WHERE receiver_username = 'admin' AND sender_username = u.username AND is_read = FALSE
			) AS unread_count
		FROM agartala.users u
		LEFT JOIN LatestMessages lm ON u.username = lm.user_handle AND lm.rn = 1
		WHERE u.role != 'admin' AND u.status = 'approved'
		ORDER BY last_active DESC;
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []UserChatSummary
	for rows.Next() {
		var s UserChatSummary
		if err := rows.Scan(&s.Username, &s.LastMessage, &s.LastActive, &s.UnreadCount); err == nil {
			summaries = append(summaries, s)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return summaries, nil
}

// FetchTotalUnreadCount gives a global unread message count badge for the top bar
func FetchTotalUnreadCount(username string) (int, error) {
	if db == nil {
		return 0, nil
	}

	var count int
	query := `SELECT COUNT(*) FROM agartala.chat_messages WHERE receiver_username = $1 AND is_read = FALSE;`
	err := db.QueryRow(query, username).Scan(&count)
	return count, err
}
