package db

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Message struct {
	ID        uuid.UUID
	ChatID    uuid.UUID
	SenderID  uuid.UUID
	Body      string
	Status    string
	CreatedAt time.Time
}

// FindOrCreateChat returns an existing DM chat between two users, or creates one
func FindOrCreateChat(ctx context.Context, pool *pgxpool.Pool, userA, userB uuid.UUID) (uuid.UUID, error) {
	var chatID uuid.UUID
	err := pool.QueryRow(ctx, `
		SELECT cm1.chat_id
		FROM chat_members cm1
		JOIN chat_members cm2 ON cm1.chat_id = cm2.chat_id
		WHERE cm1.user_id = $1 AND cm2.user_id = $2
		LIMIT 1
	`, userA, userB).Scan(&chatID)

	if err == nil {
		return chatID, nil
	}
	if err != pgx.ErrNoRows {
		return uuid.Nil, err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	if err := tx.QueryRow(ctx,
		`INSERT INTO chats DEFAULT VALUES RETURNING id`,
	).Scan(&chatID); err != nil {
		return uuid.Nil, err
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO chat_members (chat_id, user_id) VALUES ($1, $2), ($1, $3)`,
		chatID, userA, userB,
	); err != nil {
		return uuid.Nil, err
	}

	return chatID, tx.Commit(ctx)
}

func InsertMessage(ctx context.Context, pool *pgxpool.Pool, chatID, senderID uuid.UUID, body string) (Message, error) {
	var m Message
	err := pool.QueryRow(ctx, `
		INSERT INTO messages (chat_id, sender_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, chat_id, sender_id, body, status, created_at
	`, chatID, senderID, body).Scan(
		&m.ID, &m.ChatID, &m.SenderID, &m.Body, &m.Status, &m.CreatedAt,
	)
	return m, err
}

type PendingMessage struct {
	Message
	SenderUsername string
}

func GetPendingMessages(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) ([]PendingMessage, error) {
	rows, err := pool.Query(ctx, `
		SELECT m.id, m.chat_id, m.sender_id, m.body, m.status, m.created_at, u.username
		FROM messages m
		JOIN chat_members cm ON cm.chat_id = m.chat_id
		JOIN users u ON u.id = m.sender_id
		WHERE cm.user_id = $1 AND m.sender_id != $1 AND m.status = 'sent'
		ORDER BY m.created_at ASC
		LIMIT 100
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []PendingMessage
	for rows.Next() {
		var m PendingMessage
		if err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Body, &m.Status, &m.CreatedAt, &m.SenderUsername); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func GetUndeliveredMessages(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) ([]Message, error) {
	rows, err := pool.Query(ctx, `
		SELECT m.id, m.chat_id, m.sender_id, m.body, m.status, m.created_at
		FROM messages m
		JOIN chat_members cm ON cm.chat_id = m.chat_id
		WHERE cm.user_id = $1 AND m.sender_id != $1 AND m.status = 'sent'
		ORDER BY m.created_at ASC
		LIMIT 100
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Body, &m.Status, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func UpdateMessageStatus(ctx context.Context, pool *pgxpool.Pool, messageID uuid.UUID, status string) error {
	_, err := pool.Exec(ctx, `UPDATE messages SET status = $1 WHERE id = $2`, status, messageID)
	return err
}

func GetChatHistory(ctx context.Context, pool *pgxpool.Pool, chatID uuid.UUID, limit, offset int) ([]Message, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, chat_id, sender_id, body, status, created_at
		FROM messages
		WHERE chat_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, chatID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Body, &m.Status, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}
