package presence

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/samarth080/peer-chat/ws"
)

const typingTTL = 5 * time.Second

func typingKey(chatID, userID uuid.UUID) string {
	return fmt.Sprintf("typing:%s:%s", chatID, userID)
}

// SetTyping records that userID is typing in chatID and fans out to online members
func SetTyping(ctx context.Context, rdb *redis.Client, hub *ws.Hub, chatID, senderID uuid.UUID, senderName string, memberIDs []uuid.UUID) error {
	if err := rdb.Set(ctx, typingKey(chatID, senderID), "1", typingTTL).Err(); err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]interface{}{
		"type":    "typing",
		"from":    senderName,
		"from_id": senderID,
		"chat_id": chatID,
	})
	if err != nil {
		return err
	}

	for _, memberID := range memberIDs {
		if memberID != senderID {
			hub.Send(memberID, payload)
		}
	}
	return nil
}
