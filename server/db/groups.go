package db

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Group struct {
	ID        uuid.UUID
	Name      string
	CreatedBy uuid.UUID
	CreatedAt time.Time
}

type GroupMessage struct {
	ID        uuid.UUID
	GroupID   uuid.UUID
	SenderID  uuid.UUID
	Body      string
	CreatedAt time.Time
}

type GroupMemberInfo struct {
	UserID   uuid.UUID
	Username string
	Role     string
	JoinedAt time.Time
}

func CreateGroup(ctx context.Context, pool *pgxpool.Pool, name string, creatorID uuid.UUID) (Group, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Group{}, err
	}
	defer tx.Rollback(ctx)

	var g Group
	if err := tx.QueryRow(ctx,
		`INSERT INTO groups (name, created_by) VALUES ($1, $2)
		 RETURNING id, name, created_by, created_at`,
		name, creatorID,
	).Scan(&g.ID, &g.Name, &g.CreatedBy, &g.CreatedAt); err != nil {
		return Group{}, err
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, 'admin')`,
		g.ID, creatorID,
	); err != nil {
		return Group{}, err
	}

	return g, tx.Commit(ctx)
}

func GetGroupByName(ctx context.Context, pool *pgxpool.Pool, name string) (Group, error) {
	var g Group
	err := pool.QueryRow(ctx,
		`SELECT id, name, created_by, created_at FROM groups WHERE name = $1`, name,
	).Scan(&g.ID, &g.Name, &g.CreatedBy, &g.CreatedAt)
	if err == pgx.ErrNoRows {
		return Group{}, ErrNotFound
	}
	return g, err
}

func IsGroupMember(ctx context.Context, pool *pgxpool.Pool, groupID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id = $1 AND user_id = $2)`,
		groupID, userID,
	).Scan(&exists)
	return exists, err
}

func AddGroupMember(ctx context.Context, pool *pgxpool.Pool, groupID, userID uuid.UUID, role string) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, $3)
		 ON CONFLICT (group_id, user_id) DO NOTHING`,
		groupID, userID, role,
	)
	return err
}

func RemoveGroupMember(ctx context.Context, pool *pgxpool.Pool, groupID, userID uuid.UUID) error {
	_, err := pool.Exec(ctx,
		`DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`,
		groupID, userID,
	)
	return err
}

func GetGroupMembers(ctx context.Context, pool *pgxpool.Pool, groupID uuid.UUID) ([]GroupMemberInfo, error) {
	rows, err := pool.Query(ctx, `
		SELECT gm.user_id, u.username, gm.role, gm.joined_at
		FROM group_members gm
		JOIN users u ON gm.user_id = u.id
		WHERE gm.group_id = $1
		ORDER BY gm.joined_at ASC
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []GroupMemberInfo
	for rows.Next() {
		var m GroupMemberInfo
		if err := rows.Scan(&m.UserID, &m.Username, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func InsertGroupMessage(ctx context.Context, pool *pgxpool.Pool, groupID, senderID uuid.UUID, body string) (GroupMessage, error) {
	var m GroupMessage
	err := pool.QueryRow(ctx, `
		INSERT INTO group_messages (group_id, sender_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, group_id, sender_id, body, created_at
	`, groupID, senderID, body).Scan(
		&m.ID, &m.GroupID, &m.SenderID, &m.Body, &m.CreatedAt,
	)
	return m, err
}

func GetGroupMessages(ctx context.Context, pool *pgxpool.Pool, groupID uuid.UUID, limit, offset int) ([]GroupMessage, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, group_id, sender_id, body, created_at
		FROM group_messages
		WHERE group_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, groupID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []GroupMessage
	for rows.Next() {
		var m GroupMessage
		if err := rows.Scan(&m.ID, &m.GroupID, &m.SenderID, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func ListUserGroups(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) ([]Group, error) {
	rows, err := pool.Query(ctx, `
		SELECT g.id, g.name, g.created_by, g.created_at
		FROM groups g
		JOIN group_members gm ON g.id = gm.group_id
		WHERE gm.user_id = $1
		ORDER BY g.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatedBy, &g.CreatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}
