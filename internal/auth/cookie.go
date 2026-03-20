package auth

import "github.com/gofiber/fiber/v2"

const CookieName = "lmgate_token"

func SetAuthCookie(c *fiber.Ctx, token string, maxAge int, tlsDisabled bool) {
	c.Cookie(&fiber.Cookie{
		Name:     CookieName,
		Value:    token,
		HTTPOnly: true,
		Secure:   !tlsDisabled,
		SameSite: "Lax",
		MaxAge:   maxAge,
		Path:     "/",
	})
}

func ClearAuthCookie(c *fiber.Ctx, tlsDisabled bool) {
	c.Cookie(&fiber.Cookie{
		Name:     CookieName,
		Value:    "",
		HTTPOnly: true,
		Secure:   !tlsDisabled,
		SameSite: "Lax",
		MaxAge:   -1,
		Path:     "/",
	})
}
