package models

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/hkdb/lmgate/internal/database"
)

func testOpenDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestCreateUser_And_GetByID(t *testing.T) {
	db := testOpenDB(t)

	u, err := CreateUser(db, "test@example.com", "Test User", "password123", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := GetUserByID(db, u.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}

	if got.Email != "test@example.com" {
		t.Errorf("Email: got %q, want %q", got.Email, "test@example.com")
	}
	if got.DisplayName != "Test User" {
		t.Errorf("DisplayName: got %q, want %q", got.DisplayName, "Test User")
	}
	if got.Role != "user" {
		t.Errorf("Role: got %q, want %q", got.Role, "user")
	}
}

func TestCreateUser_And_GetByEmail(t *testing.T) {
	db := testOpenDB(t)

	_, err := CreateUser(db, "find@example.com", "Find Me", "pass", "admin", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := GetUserByEmail(db, "find@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if got.DisplayName != "Find Me" {
		t.Errorf("DisplayName: got %q, want %q", got.DisplayName, "Find Me")
	}
}

func TestListUsers_Multiple(t *testing.T) {
	db := testOpenDB(t)

	for i := 0; i < 2; i++ {
		email := "user" + string(rune('0'+i)) + "@example.com"
		if _, err := CreateUser(db, email, "User", "pass", "user", "local", "", false); err != nil {
			t.Fatalf("CreateUser %d: %v", i, err)
		}
	}

	users, err := ListUsers(db)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("got %d users, want 2", len(users))
	}
}

func TestUpdateUser_ChangesFields(t *testing.T) {
	db := testOpenDB(t)

	u, err := CreateUser(db, "old@example.com", "Old Name", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := UpdateUser(db, u.ID, "new@example.com", "New Name", "admin", false, false); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	got, err := GetUserByID(db, u.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.Email != "new@example.com" {
		t.Errorf("Email: got %q, want %q", got.Email, "new@example.com")
	}
	if got.Role != "admin" {
		t.Errorf("Role: got %q, want %q", got.Role, "admin")
	}
}

func TestDeleteUser_RemovesUser(t *testing.T) {
	db := testOpenDB(t)

	u, err := CreateUser(db, "del@example.com", "Delete Me", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := DeleteUser(db, u.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	_, err = GetUserByID(db, u.ID)
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestCheckPassword_Correct(t *testing.T) {
	db := testOpenDB(t)

	u, err := CreateUser(db, "pw@example.com", "PW", "correctpassword", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if !CheckPassword(u, "correctpassword") {
		t.Error("expected correct password to pass")
	}
}

func TestCheckPassword_Wrong(t *testing.T) {
	db := testOpenDB(t)

	u, err := CreateUser(db, "pw2@example.com", "PW", "correctpassword", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if CheckPassword(u, "wrongpassword") {
		t.Error("expected wrong password to fail")
	}
}

func TestRecordFailedLogin_LocksAccount(t *testing.T) {
	db := testOpenDB(t)
	email := "lockme@example.com"

	if _, err := CreateUser(db, email, "Lock", "pass", "user", "local", "", false); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := RecordFailedLogin(db, email, 3, 15*time.Minute); err != nil {
			t.Fatalf("RecordFailedLogin %d: %v", i, err)
		}
	}

	locked, err := IsAccountLocked(db, email)
	if err != nil {
		t.Fatalf("IsAccountLocked: %v", err)
	}
	if !locked {
		t.Error("expected account to be locked after max failed attempts")
	}
}

func TestClearFailedLogins_UnlocksAccount(t *testing.T) {
	db := testOpenDB(t)
	email := "unlock@example.com"

	if _, err := CreateUser(db, email, "Unlock", "pass", "user", "local", "", false); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := RecordFailedLogin(db, email, 3, 15*time.Minute); err != nil {
			t.Fatalf("RecordFailedLogin: %v", err)
		}
	}

	if err := ClearFailedLogins(db, email); err != nil {
		t.Fatalf("ClearFailedLogins: %v", err)
	}

	locked, err := IsAccountLocked(db, email)
	if err != nil {
		t.Fatalf("IsAccountLocked: %v", err)
	}
	if locked {
		t.Error("expected account to be unlocked after clearing")
	}
}

func TestHasAdminUser(t *testing.T) {
	db := testOpenDB(t)

	has, err := HasAdminUser(db)
	if err != nil {
		t.Fatalf("HasAdminUser: %v", err)
	}
	if has {
		t.Error("expected no admin user initially")
	}

	if _, err := CreateUser(db, "admin@example.com", "Admin", "pass", "admin", "local", "", false); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	has, err = HasAdminUser(db)
	if err != nil {
		t.Fatalf("HasAdminUser: %v", err)
	}
	if !has {
		t.Error("expected admin user to exist")
	}
}
