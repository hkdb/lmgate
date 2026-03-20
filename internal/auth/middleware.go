package auth

import (
	"database/sql"
	"strings"
	"sync"
	"time"

	"github.com/hkdb/lmgate/internal/models"
	"github.com/gofiber/fiber/v2"
)

type UserContext struct {
	UserID  string
	Email   string
	Role    string
	TokenID string
}

type cachedUser struct {
	user      *UserContext
	expiresAt time.Time
}

var userCache sync.Map

var userCacheTTL = 30 * time.Second

func SetUserCacheTTL(d time.Duration) {
	userCacheTTL = d
}

func Middleware(db *sql.DB, jwtSecret string, onAuthFail ...func(*fiber.Ctx, int)) fiber.Handler {
	var failCb func(*fiber.Ctx, int)
	if len(onAuthFail) > 0 {
		failCb = onAuthFail[0]
	}

	notifyFail := func(c *fiber.Ctx, status int) {
		if failCb != nil {
			failCb(c, status)
		}
	}

	return func(c *fiber.Ctx) error {
		// Try Authorization header first (supports both JWT and API tokens)
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			if !strings.HasPrefix(authHeader, "Bearer ") {
				notifyFail(c, fiber.StatusUnauthorized)
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid authorization format"})
			}
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

			// Try JWT first
			claims, err := VerifyJWT(jwtSecret, tokenStr)
			if err == nil {
				return handleJWTAuth(c, db, claims, failCb)
			}

			// Fall back to API token
			return handleAPITokenAuth(c, db, tokenStr, failCb)
		}

		// Try httpOnly cookie (JWT only — cookies never contain API tokens)
		cookieToken := c.Cookies(CookieName)
		if cookieToken != "" {
			claims, err := VerifyJWT(jwtSecret, cookieToken)
			if err != nil {
				notifyFail(c, fiber.StatusUnauthorized)
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired token"})
			}
			return handleJWTAuth(c, db, claims, failCb)
		}

		notifyFail(c, fiber.StatusUnauthorized)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authorization"})
	}
}

func handleJWTAuth(c *fiber.Ctx, db *sql.DB, claims *Claims, failCb func(*fiber.Ctx, int)) error {
	// Check cache
	if cached, ok := userCache.Load(claims.UserID); ok {
		entry := cached.(*cachedUser)
		if time.Now().Before(entry.expiresAt) {
			if entry.user == nil {
				if failCb != nil {
					failCb(c, fiber.StatusForbidden)
				}
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "user disabled"})
			}
			c.Locals("user", entry.user)
			return c.Next()
		}
		userCache.Delete(claims.UserID)
	}

	user, err := models.GetUserByID(db, claims.UserID)
	if err != nil {
		if failCb != nil {
			failCb(c, fiber.StatusUnauthorized)
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	if user.Disabled {
		userCache.Store(claims.UserID, &cachedUser{user: nil, expiresAt: time.Now().Add(userCacheTTL)})
		if failCb != nil {
			failCb(c, fiber.StatusForbidden)
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "user disabled"})
	}

	uc := &UserContext{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
	}
	userCache.Store(claims.UserID, &cachedUser{user: uc, expiresAt: time.Now().Add(userCacheTTL)})
	c.Locals("user", uc)
	return c.Next()
}

func handleAPITokenAuth(c *fiber.Ctx, db *sql.DB, raw string, failCb func(*fiber.Ctx, int)) error {
	hash := models.HashToken(raw)
	token, err := models.GetAPITokenByHash(db, hash)
	if err != nil {
		if failCb != nil {
			failCb(c, fiber.StatusUnauthorized)
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
	}

	if token.Revoked {
		if failCb != nil {
			failCb(c, fiber.StatusUnauthorized)
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "token revoked"})
	}

	if models.IsTokenExpired(token) {
		if failCb != nil {
			failCb(c, fiber.StatusUnauthorized)
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "token expired"})
	}

	user, err := models.GetUserByID(db, token.UserID)
	if err != nil {
		if failCb != nil {
			failCb(c, fiber.StatusUnauthorized)
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	if user.Disabled {
		if failCb != nil {
			failCb(c, fiber.StatusForbidden)
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "user disabled"})
	}

	uc := &UserContext{
		UserID:  user.ID,
		Email:   user.Email,
		Role:    user.Role,
		TokenID: token.ID,
	}
	c.Locals("user", uc)
	c.Locals("token_rate_limit", token.RateLimit)
	return c.Next()
}

func GetUser(c *fiber.Ctx) *UserContext {
	u, ok := c.Locals("user").(*UserContext)
	if !ok {
		return nil
	}
	return u
}

func RequireAdmin(c *fiber.Ctx) error {
	u := GetUser(c)
	if u == nil || u.Role != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "admin access required"})
	}
	return c.Next()
}

func InvalidateUserCache(userID string) {
	userCache.Delete(userID)
}
