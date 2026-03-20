package admin

import (
	"github.com/gofiber/fiber/v2"
)

type dashboardMetrics struct {
	TotalUsers      int     `json:"total_users"`
	ActiveTokens    int     `json:"active_tokens"`
	AvailableModels int     `json:"available_models"`
	RequestsToday   int     `json:"requests_today"`
	RequestsWeek    int     `json:"requests_this_week"`
	ErrorRate       float64 `json:"error_rate"`
}

func (a *Admin) GetDashboard(c *fiber.Ctx) error {
	var m dashboardMetrics

	if err := a.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&m.TotalUsers); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to query users"})
	}

	if err := a.DB.QueryRow(`SELECT COUNT(*) FROM api_tokens WHERE revoked = 0`).Scan(&m.ActiveTokens); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to query tokens"})
	}

	if err := a.DB.QueryRow(`SELECT COUNT(DISTINCT model) FROM usage_metrics`).Scan(&m.AvailableModels); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to query models"})
	}

	if err := a.DB.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE created_at >= date('now') AND log_type = 'api'`).Scan(&m.RequestsToday); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to query today's requests"})
	}

	if err := a.DB.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE created_at >= date('now', '-7 days') AND log_type = 'api'`).Scan(&m.RequestsWeek); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to query weekly requests"})
	}

	var totalToday, errorsToday int
	if err := a.DB.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE created_at >= date('now') AND log_type = 'api'`).Scan(&totalToday); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to query error rate"})
	}
	if totalToday > 0 {
		if err := a.DB.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE created_at >= date('now') AND status_code >= 400 AND log_type = 'api'`).Scan(&errorsToday); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to query error rate"})
		}
		m.ErrorRate = float64(errorsToday) / float64(totalToday)
	}

	return c.JSON(m)
}
