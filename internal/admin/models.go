package admin

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gofiber/fiber/v2"
)

type modelResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Status   string `json:"status"`
}

type modelACLResponse struct {
	ID           string `json:"id"`
	ModelPattern string `json:"model_pattern"`
	UserEmail    string `json:"user_email"`
	Permission   string `json:"permission"`
	GroupID      string `json:"group_id,omitempty"`
	GroupName    string `json:"group_name,omitempty"`
	CreatedAt    string `json:"created_at"`
}

func (a *Admin) ListModels(c *fiber.Ctx) error {
	url := fmt.Sprintf("%s/v1/models", a.Config.Upstream.URL)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "failed to reach upstream"})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "upstream returned error"})
	}

	var upstream struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to parse upstream response"})
	}

	models := make([]modelResponse, 0, len(upstream.Data))
	for _, m := range upstream.Data {
		models = append(models, modelResponse{
			ID:       m.ID,
			Name:     m.ID,
			Provider: "upstream",
			Status:   "available",
		})
	}

	return c.JSON(models)
}

func (a *Admin) ListModelACLs(c *fiber.Ctx) error {
	rows, err := a.DB.Query(
		`SELECT ma.id, ma.model, COALESCE(u.email, ''), ma.allowed, COALESCE(ma.group_id, ''), COALESCE(g.name, '')
		 FROM model_acls ma
		 LEFT JOIN users u ON u.id = ma.user_id
		 LEFT JOIN groups g ON g.id = ma.group_id
		 ORDER BY COALESCE(u.email, g.name), ma.model`,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list model acls"})
	}
	defer rows.Close()

	acls := make([]modelACLResponse, 0)
	for rows.Next() {
		var id, model, email, groupID, groupName string
		var allowed bool
		if err := rows.Scan(&id, &model, &email, &allowed, &groupID, &groupName); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to scan model acl"})
		}
		permission := "deny"
		if allowed {
			permission = "allow"
		}
		acls = append(acls, modelACLResponse{
			ID:           id,
			ModelPattern: model,
			UserEmail:    email,
			Permission:   permission,
			GroupID:      groupID,
			GroupName:    groupName,
			CreatedAt:    "",
		})
	}

	return c.JSON(acls)
}

func (a *Admin) CreateModelACL(c *fiber.Ctx) error {
	var req struct {
		ModelPattern string `json:"model_pattern"`
		UserEmail    string `json:"user_email"`
		GroupID      string `json:"group_id"`
		Permission   string `json:"permission"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	if req.ModelPattern == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "model_pattern is required"})
	}

	if req.UserEmail == "" && req.GroupID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_email or group_id is required"})
	}

	if req.UserEmail != "" && req.GroupID != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "specify either user_email or group_id, not both"})
	}

	allowed := req.Permission == "allow"
	id := uuid.New().String()

	if req.GroupID != "" {
		_, err := a.DB.Exec(
			`INSERT INTO model_acls (id, user_id, model, allowed, group_id) VALUES (?, '', ?, ?, ?)`,
			id, req.ModelPattern, allowed, req.GroupID,
		)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create model acl"})
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
	}

	var userID string
	err := a.DB.QueryRow(`SELECT id FROM users WHERE email = ?`, req.UserEmail).Scan(&userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	_, err = a.DB.Exec(
		`INSERT INTO model_acls (id, user_id, model, allowed) VALUES (?, ?, ?, ?)
		 ON CONFLICT(user_id, model) DO UPDATE SET allowed = excluded.allowed`,
		id, userID, req.ModelPattern, allowed,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create model acl"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

func (a *Admin) DeleteModelACL(c *fiber.Ctx) error {
	if _, err := a.DB.Exec(`DELETE FROM model_acls WHERE id = ?`, c.Params("id")); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete model acl"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (a *Admin) GetUpstreamType(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"type": a.Config.Upstream.Type})
}

func (a *Admin) PullModel(c *fiber.Ctx) error {
	if a.Config.Upstream.Type != "ollama" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "model pull is only supported for ollama upstream"})
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := c.BodyParser(&req); err != nil || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}

	body, _ := json.Marshal(map[string]string{"name": req.Name})
	upstream := fmt.Sprintf("%s/api/pull", a.Config.Upstream.URL)
	resp, err := http.Post(upstream, "application/json", bytes.NewReader(body))
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "failed to reach upstream"})
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if _, err := fmt.Fprintf(w, "data: %s\n\n", line); err != nil {
				return
			}
			if err := w.Flush(); err != nil {
				return
			}
		}
	})

	return nil
}

func (a *Admin) DeleteModel(c *fiber.Ctx) error {
	if a.Config.Upstream.Type != "ollama" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "model delete is only supported for ollama upstream"})
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := c.BodyParser(&req); err != nil || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}

	body, _ := json.Marshal(map[string]string{"name": req.Name})
	upstream := fmt.Sprintf("%s/api/delete", a.Config.Upstream.URL)

	httpReq, err := http.NewRequest(http.MethodDelete, upstream, bytes.NewReader(body))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to build request"})
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "failed to reach upstream"})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "upstream returned error"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}
