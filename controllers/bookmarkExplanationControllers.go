package controllers

import (
	"github.com/ShijuPJohn/synapticz_backend/models"
	"github.com/ShijuPJohn/synapticz_backend/util"
	"github.com/gofiber/fiber/v2"
)

type BookmarkRequest struct {
	QuestionID int `json:"question_id"`
}

func CreateBookmark(c *fiber.Ctx) error {
	userID := c.Locals("user").(models.User).ID

	var req BookmarkRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body " + err.Error(),
		})
	}

	if req.QuestionID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid question ID",
		})
	}

	query := `
		INSERT INTO bookmarked_questions (user_id, question_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, question_id) DO NOTHING;
	`

	_, err := util.DB.Exec(query, userID, req.QuestionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Could not bookmark question" + err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Question bookmarked successfully",
	})
}
