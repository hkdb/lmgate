package models

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID                  string `json:"id"`
	Email               string `json:"email"`
	DisplayName         string `json:"display_name"`
	PasswordHash        string `json:"-"`
	Role                string `json:"role"`
	AuthProvider        string `json:"auth_provider"`
	ProviderSub         string `json:"provider_sub,omitempty"`
	Disabled            bool   `json:"disabled"`
	ForcePasswordChange bool   `json:"force_password_change"`
	TOTPEnabled         bool   `json:"totp_enabled"`
	PasswordChangedAt   string `json:"password_changed_at"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
}

func CreateUser(db *sql.DB, email, displayName, password, role, authProvider, providerSub string, forcePasswordChange bool) (*User, error) {
	var passwordHash string
	if password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hashing password: %w", err)
		}
		passwordHash = string(hash)
	}

	now := time.Now().UTC().Format(time.DateTime)
	u := &User{
		ID:                  uuid.New().String(),
		Email:               email,
		DisplayName:         displayName,
		PasswordHash:        passwordHash,
		Role:                role,
		AuthProvider:        authProvider,
		ProviderSub:         providerSub,
		ForcePasswordChange: forcePasswordChange,
		PasswordChangedAt:   now,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	_, err := db.Exec(
		`INSERT INTO users (id, email, display_name, password_hash, role, auth_provider, provider_sub, force_password_change, password_changed_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Email, u.DisplayName, u.PasswordHash, u.Role, u.AuthProvider, u.ProviderSub, u.ForcePasswordChange, u.PasswordChangedAt, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting user: %w", err)
	}

	return u, nil
}

func GetUserByID(db *sql.DB, id string) (*User, error) {
	u := &User{}
	var providerSub sql.NullString
	err := db.QueryRow(
		`SELECT id, email, display_name, password_hash, role, auth_provider, provider_sub, disabled, force_password_change, totp_enabled, password_changed_at, created_at, updated_at
		 FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.Role, &u.AuthProvider, &providerSub, &u.Disabled, &u.ForcePasswordChange, &u.TOTPEnabled, &u.PasswordChangedAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	u.ProviderSub = providerSub.String
	return u, nil
}

func GetUserByEmail(db *sql.DB, email string) (*User, error) {
	u := &User{}
	var providerSub sql.NullString
	err := db.QueryRow(
		`SELECT id, email, display_name, password_hash, role, auth_provider, provider_sub, disabled, force_password_change, totp_enabled, password_changed_at, created_at, updated_at
		 FROM users WHERE email = ?`, email,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.Role, &u.AuthProvider, &providerSub, &u.Disabled, &u.ForcePasswordChange, &u.TOTPEnabled, &u.PasswordChangedAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	u.ProviderSub = providerSub.String
	return u, nil
}

func GetUserByProviderSub(db *sql.DB, provider, sub string) (*User, error) {
	u := &User{}
	var providerSub sql.NullString
	err := db.QueryRow(
		`SELECT id, email, display_name, password_hash, role, auth_provider, provider_sub, disabled, force_password_change, totp_enabled, password_changed_at, created_at, updated_at
		 FROM users WHERE auth_provider = ? AND provider_sub = ?`, provider, sub,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.Role, &u.AuthProvider, &providerSub, &u.Disabled, &u.ForcePasswordChange, &u.TOTPEnabled, &u.PasswordChangedAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	u.ProviderSub = providerSub.String
	return u, nil
}

func ListUsers(db *sql.DB) ([]User, error) {
	rows, err := db.Query(
		`SELECT id, email, display_name, role, auth_provider, provider_sub, disabled, force_password_change, totp_enabled, password_changed_at, created_at, updated_at
		 FROM users ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var providerSub sql.NullString
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Role, &u.AuthProvider, &providerSub, &u.Disabled, &u.ForcePasswordChange, &u.TOTPEnabled, &u.PasswordChangedAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		u.ProviderSub = providerSub.String
		users = append(users, u)
	}
	return users, rows.Err()
}

func UpdateUser(db *sql.DB, id, email, displayName, role string, disabled, forcePasswordChange bool) error {
	_, err := db.Exec(
		`UPDATE users SET email = ?, display_name = ?, role = ?, disabled = ?, force_password_change = ?, updated_at = datetime('now')
		 WHERE id = ?`,
		email, displayName, role, disabled, forcePasswordChange, id,
	)
	return err
}

func ClearForcePasswordChange(db *sql.DB, id string) error {
	_, err := db.Exec(`UPDATE users SET force_password_change = 0, updated_at = datetime('now') WHERE id = ?`, id)
	return err
}

func UpdateUserPassword(db *sql.DB, id, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}
	_, err = db.Exec(`UPDATE users SET password_hash = ?, password_changed_at = datetime('now'), updated_at = datetime('now') WHERE id = ?`, string(hash), id)
	return err
}

func IsPasswordExpired(user *User, expiryDays int) bool {
	if expiryDays <= 0 {
		return false
	}
	if user.PasswordChangedAt == "" {
		return false
	}
	changed, err := time.Parse(time.DateTime, user.PasswordChangedAt)
	if err != nil {
		return false
	}
	return time.Now().UTC().After(changed.Add(time.Duration(expiryDays) * 24 * time.Hour))
}

func DeleteUser(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func CheckPassword(user *User, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) == nil
}

func EnableTOTP(db *sql.DB, userID string) error {
	_, err := db.Exec(`UPDATE users SET totp_enabled = 1, updated_at = datetime('now') WHERE id = ?`, userID)
	return err
}

func DisableTOTP(db *sql.DB, userID string) error {
	_, err := db.Exec(`UPDATE users SET totp_enabled = 0, updated_at = datetime('now') WHERE id = ?`, userID)
	return err
}

func RecordFailedLogin(db *sql.DB, email string, maxAttempts int, lockoutDuration time.Duration) error {
	_, err := db.Exec(`UPDATE users SET failed_login_count = failed_login_count + 1, updated_at = datetime('now') WHERE email = ?`, email)
	if err != nil {
		return err
	}

	var count int
	err = db.QueryRow(`SELECT failed_login_count FROM users WHERE email = ?`, email).Scan(&count)
	if err != nil {
		return err
	}

	if count < maxAttempts {
		return nil
	}

	lockedUntil := time.Now().UTC().Add(lockoutDuration).Format(time.DateTime)
	_, err = db.Exec(`UPDATE users SET locked_until = ?, failed_login_count = 0, updated_at = datetime('now') WHERE email = ?`, lockedUntil, email)
	if err != nil {
		return err
	}

	log.Printf("account locked due to repeated failed attempts: email=%s", email)
	return nil
}

func ClearFailedLogins(db *sql.DB, email string) error {
	_, err := db.Exec(`UPDATE users SET failed_login_count = 0, locked_until = '', updated_at = datetime('now') WHERE email = ?`, email)
	return err
}

func IsAccountLocked(db *sql.DB, email string) (bool, error) {
	var lockedUntil string
	err := db.QueryRow(`SELECT locked_until FROM users WHERE email = ?`, email).Scan(&lockedUntil)
	if err != nil {
		return false, err
	}

	if lockedUntil == "" {
		return false, nil
	}

	t, err := time.Parse(time.DateTime, lockedUntil)
	if err != nil {
		return false, nil
	}

	return time.Now().UTC().Before(t), nil
}

func HasAdminUser(db *sql.DB) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin' AND disabled = 0`).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
