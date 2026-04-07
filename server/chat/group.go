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

type groupMsgInbound struct {
	Type    string `json:"type"`
	GroupID string `json:"group_id"`
	Body    string `json:"body"`
}

// HandleGroupMessage stores a group message and fans out to all online members
func HandleGroupMessage(ctx context.Context, pool *pgxpool.Pool, hub *ws.Hub, senderID uuid.UUID, senderName string, raw []byte) error {
	var msg groupMsgInbound
	if err := json.Unmarshal(raw, &msg); err != nil {
		return fmt.Errorf("invalid group_message payload: %w", err)
	}

	groupID, err := uuid.Parse(msg.GroupID)
	if err != nil {
		return fmt.Errorf("invalid group_id: %w", err)
	}

	isMember, err := db.IsGroupMember(ctx, pool, groupID, senderID)
	if err != nil {
		return err
	}
	if !isMember {
		sendError(hub, senderID, "not a member of this group")
		return nil
	}

	stored, err := db.InsertGroupMessage(ctx, pool, groupID, senderID, msg.Body)
	if err != nil {
		return fmt.Errorf("insert group message: %w", err)
	}

	outbound, _ := json.Marshal(map[string]interface{}{
		"type":       "group_message",
		"group_id":   groupID,
		"from":       senderName,
		"from_id":    senderID,
		"body":       stored.Body,
		"message_id": stored.ID,
		"timestamp":  stored.CreatedAt,
	})

	members, err := db.GetGroupMembers(ctx, pool, groupID)
	if err != nil {
		return err
	}
	for _, m := range members {
		if m.UserID != senderID {
			hub.Send(m.UserID, outbound)
		}
	}

	// ack to sender
	ack, _ := json.Marshal(map[string]interface{}{
		"type":       "group_sent",
		"message_id": stored.ID,
		"status":     "sent",
	})
	hub.Send(senderID, ack)

	return nil
}

// HandleCreateGroup creates a new group and acks the creator
func HandleCreateGroup(ctx context.Context, pool *pgxpool.Pool, hub *ws.Hub, creatorID uuid.UUID, raw []byte) error {
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}

	group, err := db.CreateGroup(ctx, pool, payload.Name, creatorID)
	if err != nil {
		sendError(hub, creatorID, "group name already taken")
		return nil
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"type":     "group_created",
		"group_id": group.ID,
		"name":     group.Name,
	})
	hub.Send(creatorID, resp)
	return nil
}

// HandleAddMember adds a user to a group (any member can add)
func HandleAddMember(ctx context.Context, pool *pgxpool.Pool, hub *ws.Hub, adderID uuid.UUID, raw []byte) error {
	var payload struct {
		GroupID string `json:"group_id"`
		UserID  string `json:"user_id"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}

	groupID, err := uuid.Parse(payload.GroupID)
	if err != nil {
		return err
	}
	userID, err := uuid.Parse(payload.UserID)
	if err != nil {
		return err
	}

	isMember, err := db.IsGroupMember(ctx, pool, groupID, adderID)
	if err != nil {
		return err
	}
	if !isMember {
		sendError(hub, adderID, "you are not a member of this group")
		return nil
	}

	if err := db.AddGroupMember(ctx, pool, groupID, userID, "member"); err != nil {
		sendError(hub, adderID, "failed to add member")
		return nil
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"type":     "member_added",
		"group_id": groupID,
		"user_id":  userID,
	})
	hub.Send(adderID, resp)
	return nil
}

// HandleLeaveGroup removes the requesting user from a group
func HandleLeaveGroup(ctx context.Context, pool *pgxpool.Pool, hub *ws.Hub, userID uuid.UUID, raw []byte) error {
	var payload struct {
		GroupID string `json:"group_id"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}

	groupID, err := uuid.Parse(payload.GroupID)
	if err != nil {
		return err
	}

	if err := db.RemoveGroupMember(ctx, pool, groupID, userID); err != nil {
		return err
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"type":     "left_group",
		"group_id": groupID,
	})
	hub.Send(userID, resp)
	return nil
}

func sendError(hub *ws.Hub, userID uuid.UUID, msg string) {
	data, _ := json.Marshal(map[string]string{"type": "error", "message": msg})
	hub.Send(userID, data)
}
