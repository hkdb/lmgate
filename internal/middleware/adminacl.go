package middleware

import (
	"net"
	"strings"

	"github.com/hkdb/lmgate/internal/config"
	"github.com/gofiber/fiber/v2"
)

// AdminNetworkRestrict returns middleware that restricts /admin access to
// the IPs and CIDRs listed in cfg.Security.AdminAllowedNetworks.
// An empty value means no restriction (all IPs allowed).
func AdminNetworkRestrict(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		raw := cfg.Security.AdminAllowedNetworks
		if raw == "" {
			return c.Next()
		}

		clientIP := net.ParseIP(c.IP())
		if clientIP == nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "forbidden",
			})
		}

		for _, entry := range strings.Split(raw, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}

			// Try CIDR first
			if strings.Contains(entry, "/") {
				_, network, err := net.ParseCIDR(entry)
				if err != nil {
					continue
				}
				if network.Contains(clientIP) {
					return c.Next()
				}
				continue
			}

			// Bare IP
			allowed := net.ParseIP(entry)
			if allowed != nil && allowed.Equal(clientIP) {
				return c.Next()
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "forbidden",
		})
	}
}
