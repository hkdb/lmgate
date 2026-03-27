package admin

import (
	"github.com/gofiber/fiber/v2"

	"github.com/hkdb/lmgate/internal/auth"
	"github.com/hkdb/lmgate/internal/models"
)

func (a *Admin) ListGroups(c *fiber.Ctx) error {
	groups, err := models.ListGroups(a.DB)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list groups"})
	}
	if groups == nil {
		groups = []models.GroupWithCount{}
	}
	return c.JSON(groups)
}

func (a *Admin) CreateGroup(c *fiber.Ctx) error {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		AdminRole   bool   `json:"admin_role"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}

	group, err := models.CreateGroup(a.DB, req.Name, req.Description, "local", "", req.AdminRole)
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "group already exists or creation failed"})
	}

	return c.Status(fiber.StatusCreated).JSON(group)
}

func (a *Admin) GetGroup(c *fiber.Ctx) error {
	group, err := models.GetGroupByID(a.DB, c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "group not found"})
	}

	members, err := models.GetGroupMembers(a.DB, group.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get members"})
	}
	if members == nil {
		members = []models.GroupMember{}
	}

	return c.JSON(fiber.Map{
		"group":   group,
		"members": members,
	})
}

func (a *Admin) UpdateGroup(c *fiber.Ctx) error {
	id := c.Params("id")

	group, err := models.GetGroupByID(a.DB, id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "group not found"})
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		AdminRole   *bool  `json:"admin_role"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	if req.Name == "" {
		req.Name = group.Name
	}

	adminRole := group.AdminRole
	if req.AdminRole != nil {
		adminRole = *req.AdminRole
	}

	if err := models.UpdateGroup(a.DB, id, req.Name, req.Description, adminRole); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update group"})
	}

	// Re-sync role for all group members when admin_role changes
	members, err := models.GetGroupMembers(a.DB, id)
	if err == nil {
		for _, m := range members {
			_ = models.SyncUserRoleFromGroups(a.DB, m.UserID)
			auth.InvalidateUserCache(m.UserID)
		}
	}

	updated, _ := models.GetGroupByID(a.DB, id)
	return c.JSON(updated)
}

func (a *Admin) DeleteGroup(c *fiber.Ctx) error {
	id := c.Params("id")

	// Also clean up model_acls referencing this group
	_, _ = a.DB.Exec(`DELETE FROM model_acls WHERE group_id = ?`, id)

	if err := models.DeleteGroup(a.DB, id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete group"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (a *Admin) AddGroupMember(c *fiber.Ctx) error {
	groupID := c.Params("id")

	var req struct {
		Email string `json:"email"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	if req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email is required"})
	}

	user, err := models.GetUserByEmail(a.DB, req.Email)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	if err := models.AddUserToGroup(a.DB, user.ID, groupID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to add member"})
	}

	auth.InvalidateUserCache(user.ID)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "added"})
}

func (a *Admin) RemoveGroupMember(c *fiber.Ctx) error {
	groupID := c.Params("id")
	userID := c.Params("userId")

	if err := models.RemoveUserFromGroup(a.DB, userID, groupID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to remove member"})
	}

	auth.InvalidateUserCache(userID)
	return c.SendStatus(fiber.StatusNoContent)
}
