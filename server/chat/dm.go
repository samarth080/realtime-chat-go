package chat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samarth080/peer-chat/db"
	"github.com/samarth080/peer-chat/ws"
)

type dmInbound struct {
	Type string `json:"type"`
	To   string `json:"to"`
	Body string `json:"body"`
	ID   string `json:"id"` // client-generated correlation ID
}

// Router abstracts pub/sub delivery for cross-instance routing (implemented in Plan 2)
type Router interface {
	Publish(ctx context.Context, userID uuid.UUID, data []byte) error
}

// HandleDM processes a direct message — inserts to DB, delivers locally or via pub/sub
func HandleDM(ctx context.Context, pool *pgxpool.Pool, hub *ws.Hub, router Router, senderID uuid.UUID, senderName string, raw []byte) error {
	var msg dmInbound
	if err := json.Unmarshal(raw, &msg); err != nil {
		return fmt.Errorf("invalid dm payload: %w", err)
	}

	receiverID, err := uuid.Parse(msg.To)
	if err != nil {
		return fmt.Errorf("invalid receiver id: %w", err)
	}

	chatID, err := db.FindOrCreateChat(ctx, pool, senderID, receiverID)
	if err != nil {
		return fmt.Errorf("find/create chat: %w", err)
	}

	stored, err := db.InsertMessage(ctx, pool, chatID, senderID, msg.Body)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	outbound, _ := json.Marshal(map[string]interface{}{
		"type":       "message",
		"from":       senderName,
		"from_id":    senderID,
		"body":       stored.Body,
		"message_id": stored.ID,
		"timestamp":  stored.CreatedAt,
	})

	// Try local delivery first; fall back to pub/sub for cross-instance
	delivered := hub.Send(receiverID, outbound)
	if !delivered && router != nil {
		if err := router.Publish(ctx, receiverID, outbound); err != nil {
			return fmt.Errorf("pub/sub delivery failed: %w", err)
		}
	}
	// Mark delivered immediately so reconnect flow doesn't re-send
	if delivered {
		db.UpdateMessageStatus(ctx, pool, stored.ID, "delivered")
	}

	ack, _ := json.Marshal(map[string]interface{}{
		"type":       "sent",
		"message_id": stored.ID,
		"id":         msg.ID,
		"status":     "sent",
	})
	hub.Send(senderID, ack)

	return nil
}

// HandleAck processes a read receipt from receiver — marks message as read and notifies sender
func HandleAck(ctx context.Context, pool *pgxpool.Pool, hub *ws.Hub, router Router, readerID uuid.UUID, raw []byte) error {
	var payload struct {
		MessageID string `json:"message_id"`
		SenderID  string `json:"sender_id"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	msgID, err := uuid.Parse(payload.MessageID)
	if err != nil {
		return err
	}
	if err := db.UpdateMessageStatus(ctx, pool, msgID, "read"); err != nil {
		return err
	}
	// Notify the original sender that their message was read
	if payload.SenderID != "" {
		senderID, err := uuid.Parse(payload.SenderID)
		if err == nil {
			notify, _ := json.Marshal(map[string]interface{}{
				"type":       "read",
				"message_id": msgID,
			})
			if !hub.Send(senderID, notify) && router != nil {
				router.Publish(ctx, senderID, notify)
			}
		}
	}
	return nil
}
