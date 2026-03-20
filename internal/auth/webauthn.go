package auth

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/hkdb/lmgate/internal/config"
	"github.com/hkdb/lmgate/internal/models"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

var (
	webAuthnInstance *webauthn.WebAuthn
	sessionStore     sync.Map // map[string]*webauthn.SessionData
)

func InitWebAuthn(cfg *config.Config) error {
	rpID := cfg.WebAuthn.RPID
	if rpID == "" {
		return nil // WebAuthn not configured
	}

	origins := cfg.WebAuthn.RPOrigins
	if len(origins) == 0 {
		scheme := "https"
		if cfg.Server.TLS.Disabled {
			scheme = "http"
		}
		origins = []string{fmt.Sprintf("%s://%s", scheme, rpID)}
	}

	displayName := cfg.WebAuthn.RPDisplayName
	if displayName == "" {
		displayName = "LM Gate"
	}

	wconfig := &webauthn.Config{
		RPDisplayName: displayName,
		RPID:          rpID,
		RPOrigins:     origins,
	}

	var err error
	webAuthnInstance, err = webauthn.New(wconfig)
	if err != nil {
		return fmt.Errorf("initializing webauthn: %w", err)
	}

	return nil
}

func GetWebAuthn() *webauthn.WebAuthn {
	return webAuthnInstance
}

func StoreWebAuthnSession(key string, session *webauthn.SessionData) {
	sessionStore.Store(key, &sessionEntry{
		data:      session,
		expiresAt: time.Now().Add(5 * time.Minute),
	})
}

func LoadWebAuthnSession(key string) (*webauthn.SessionData, bool) {
	val, ok := sessionStore.Load(key)
	if !ok {
		return nil, false
	}

	entry := val.(*sessionEntry)
	if time.Now().After(entry.expiresAt) {
		sessionStore.Delete(key)
		return nil, false
	}

	sessionStore.Delete(key)
	return entry.data, true
}

type sessionEntry struct {
	data      *webauthn.SessionData
	expiresAt time.Time
}

// WebAuthnUser implements the webauthn.User interface
type WebAuthnUser struct {
	ID          string
	Email       string
	DisplayName string
	Credentials []webauthn.Credential
}

func NewWebAuthnUser(user *models.User, db *sql.DB) (*WebAuthnUser, error) {
	creds, err := models.ListWebAuthnCredentials(db, user.ID)
	if err != nil {
		return nil, err
	}

	waCreds := make([]webauthn.Credential, len(creds))
	for i, c := range creds {
		waCreds[i] = webauthn.Credential{
			ID:              c.CredentialID,
			PublicKey:       c.PublicKey,
			AttestationType: c.AttestationType,
			Authenticator: webauthn.Authenticator{
				AAGUID:    c.AAGUID,
				SignCount: c.SignCount,
			},
		}
	}

	return &WebAuthnUser{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Credentials: waCreds,
	}, nil
}

func (u *WebAuthnUser) WebAuthnID() []byte {
	return []byte(u.ID)
}

func (u *WebAuthnUser) WebAuthnName() string {
	return u.Email
}

func (u *WebAuthnUser) WebAuthnDisplayName() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Email
}

func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.Credentials
}

func (u *WebAuthnUser) WebAuthnIcon() string {
	return ""
}

func (u *WebAuthnUser) CredentialExcludeList() []protocol.CredentialDescriptor {
	excludeList := make([]protocol.CredentialDescriptor, len(u.Credentials))
	for i, cred := range u.Credentials {
		excludeList[i] = protocol.CredentialDescriptor{
			Type:            protocol.PublicKeyCredentialType,
			CredentialID:    cred.ID,
		}
	}
	return excludeList
}
