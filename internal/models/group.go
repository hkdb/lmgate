package models

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
	SourceID    string `json:"source_id"`
	AdminRole   bool   `json:"admin_role"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func CreateGroup(db *sql.DB, name, description, source, sourceID string, adminRole bool) (*Group, error) {
	now := time.Now().UTC().Format(time.DateTime)
	g := &Group{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Source:      source,
		SourceID:    sourceID,
		AdminRole:   adminRole,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	_, err := db.Exec(
		`INSERT INTO groups (id, name, description, source, source_id, admin_role, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.Name, g.Description, g.Source, g.SourceID, g.AdminRole, g.CreatedAt, g.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting group: %w", err)
	}

	return g, nil
}

func GetGroupByID(db *sql.DB, id string) (*Group, error) {
	g := &Group{}
	err := db.QueryRow(
		`SELECT id, name, description, source, source_id, admin_role, created_at, updated_at
		 FROM groups WHERE id = ?`, id,
	).Scan(&g.ID, &g.Name, &g.Description, &g.Source, &g.SourceID, &g.AdminRole, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return g, nil
}

func GetGroupByNameAndSource(db *sql.DB, name, source string) (*Group, error) {
	g := &Group{}
	err := db.QueryRow(
		`SELECT id, name, description, source, source_id, admin_role, created_at, updated_at
		 FROM groups WHERE name = ? AND source = ?`, name, source,
	).Scan(&g.ID, &g.Name, &g.Description, &g.Source, &g.SourceID, &g.AdminRole, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return g, nil
}

type GroupWithCount struct {
	Group
	MemberCount int `json:"member_count"`
}

func ListGroups(db *sql.DB) ([]GroupWithCount, error) {
	rows, err := db.Query(
		`SELECT g.id, g.name, g.description, g.source, g.source_id, g.admin_role, g.created_at, g.updated_at,
		        COUNT(ug.user_id) as member_count
		 FROM groups g
		 LEFT JOIN user_groups ug ON ug.group_id = g.id
		 GROUP BY g.id
		 ORDER BY g.created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []GroupWithCount
	for rows.Next() {
		var g GroupWithCount
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.Source, &g.SourceID, &g.AdminRole, &g.CreatedAt, &g.UpdatedAt, &g.MemberCount); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func UpdateGroup(db *sql.DB, id, name, description string, adminRole bool) error {
	_, err := db.Exec(
		`UPDATE groups SET name = ?, description = ?, admin_role = ?, updated_at = datetime('now') WHERE id = ?`,
		name, description, adminRole, id,
	)
	return err
}

func DeleteGroup(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM groups WHERE id = ?`, id)
	return err
}

func AddUserToGroup(db *sql.DB, userID, groupID string) error {
	_, err := db.Exec(
		`INSERT OR IGNORE INTO user_groups (user_id, group_id) VALUES (?, ?)`,
		userID, groupID,
	)
	if err != nil {
		return err
	}
	return SyncUserRoleFromGroups(db, userID)
}

func RemoveUserFromGroup(db *sql.DB, userID, groupID string) error {
	_, err := db.Exec(
		`DELETE FROM user_groups WHERE user_id = ? AND group_id = ?`,
		userID, groupID,
	)
	if err != nil {
		return err
	}
	return SyncUserRoleFromGroups(db, userID)
}

func GetUserGroups(db *sql.DB, userID string) ([]Group, error) {
	rows, err := db.Query(
		`SELECT g.id, g.name, g.description, g.source, g.source_id, g.admin_role, g.created_at, g.updated_at
		 FROM groups g
		 JOIN user_groups ug ON ug.group_id = g.id
		 WHERE ug.user_id = ?
		 ORDER BY g.name`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.Source, &g.SourceID, &g.AdminRole, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

type GroupMember struct {
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

func GetGroupMembers(db *sql.DB, groupID string) ([]GroupMember, error) {
	rows, err := db.Query(
		`SELECT u.id, u.email, u.display_name
		 FROM users u
		 JOIN user_groups ug ON ug.user_id = u.id
		 WHERE ug.group_id = ?
		 ORDER BY u.email`, groupID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []GroupMember
	for rows.Next() {
		var m GroupMember
		if err := rows.Scan(&m.UserID, &m.Email, &m.DisplayName); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func GetGroupIDsForUser(db *sql.DB, userID string) ([]string, error) {
	rows, err := db.Query(
		`SELECT group_id FROM user_groups WHERE user_id = ?`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func SyncUserRoleFromGroups(db *sql.DB, userID string) error {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM user_groups ug
		 JOIN groups g ON g.id = ug.group_id
		 WHERE ug.user_id = ? AND g.admin_role = 1`, userID,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("checking admin group membership: %w", err)
	}

	role := "user"
	if count > 0 {
		role = "admin"
	}

	_, err = db.Exec(`UPDATE users SET role = ? WHERE id = ?`, role, userID)
	return err
}
