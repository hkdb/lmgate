package middleware

import (
	"net"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
)

type gatewayACL struct {
	mu       sync.RWMutex
	ips      []net.IP
	networks []*net.IPNet
	active   bool
}

var gwACL gatewayACL

// ParseGatewayACL pre-parses a comma-separated list of IPs and CIDRs into
// the package-level cache. Call at startup and whenever settings are saved.
func ParseGatewayACL(raw string) {
	gwACL.mu.Lock()
	defer gwACL.mu.Unlock()

	gwACL.ips = nil
	gwACL.networks = nil
	gwACL.active = false

	if raw == "" {
		return
	}

	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		if strings.Contains(entry, "/") {
			_, network, err := net.ParseCIDR(entry)
			if err != nil {
				continue
			}
			gwACL.networks = append(gwACL.networks, network)
			continue
		}

		ip := net.ParseIP(entry)
		if ip != nil {
			gwACL.ips = append(gwACL.ips, ip)
		}
	}

	gwACL.active = len(gwACL.ips) > 0 || len(gwACL.networks) > 0
}

// GatewayNetworkRestrict returns middleware that restricts gateway/proxy
// access to the pre-parsed IPs and CIDRs. When the allowlist is empty
// (default), this is a single bool check followed by c.Next().
func GatewayNetworkRestrict() fiber.Handler {
	return func(c *fiber.Ctx) error {
		gwACL.mu.RLock()
		active := gwACL.active
		if !active {
			gwACL.mu.RUnlock()
			return c.Next()
		}

		ips := gwACL.ips
		networks := gwACL.networks
		gwACL.mu.RUnlock()

		clientIP := net.ParseIP(c.IP())
		if clientIP == nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "forbidden",
			})
		}

		for _, ip := range ips {
			if ip.Equal(clientIP) {
				return c.Next()
			}
		}

		for _, network := range networks {
			if network.Contains(clientIP) {
				return c.Next()
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "forbidden",
		})
	}
}
