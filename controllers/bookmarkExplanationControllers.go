package controllers

import (
	"github.com/ShijuPJohn/synapticz_backend/models"
	"github.com/ShijuPJohn/synapticz_backend/util"
	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
	"strconv"
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
func RemoveBookmark(c *fiber.Ctx) error {
	userID := c.Locals("user").(models.User).ID

	questionId := c.Params("qid")
	query := `
		DELETE FROM bookmarked_questions
		WHERE user_id = $1 AND question_id = $2;
	`

	_, err := util.DB.Exec(query, userID, questionId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Could not remove bookmark " + err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Bookmark removed successfully",
	})
}
func SaveExplanation(c *fiber.Ctx) error {
	userID := c.Locals("user").(models.User).ID

	type ExplanationRequest struct {
		QuestionID  int    `json:"question_id"`
		Explanation string `json:"explanation"`
	}

	var req ExplanationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body: " + err.Error(),
		})
	}

	if req.QuestionID <= 0 || req.Explanation == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Question ID and explanation are required",
		})
	}

	query := `
		INSERT INTO saved_explanations (user_id, question_id, explanation)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, question_id)
		DO UPDATE SET explanation = EXCLUDED.explanation, updated_at = CURRENT_TIMESTAMP;
	`

	_, err := util.DB.Exec(query, userID, req.QuestionID, req.Explanation)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Could not save explanation: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Explanation saved successfully",
	})
}

func RemoveExplanation(c *fiber.Ctx) error {
	userID := c.Locals("user").(models.User).ID
	questionID := c.Params("qid")

	query := `
		DELETE FROM saved_explanations
		WHERE user_id = $1 AND question_id = $2;
	`

	_, err := util.DB.Exec(query, userID, questionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Could not remove explanation: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Explanation removed successfully",
	})
}
func GetAllBookmarks(c *fiber.Ctx) error {
	user := c.Locals("user").(models.User)

	rows, err := util.DB.Query(`
		SELECT 
			q.id, q.question, q.question_type, q.options, q.correct_options, q.explanation
		FROM bookmarked_questions bq
		JOIN questions q ON bq.question_id = q.id
		WHERE bq.user_id = $1
		ORDER BY q.id DESC`, user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch bookmarked questions",
		})
	}
	defer rows.Close()

	bookmarks := []map[string]interface{}{}

	for rows.Next() {
		var (
			id           int
			question     string
			questionType string
			options      []string
			correctOpts  pq.Int64Array
			explanation  *string
		)
		err := rows.Scan(&id, &question, &questionType, pq.Array(&options), &correctOpts, &explanation)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to parse bookmarked question",
			})
		}

		bookmarks = append(bookmarks, map[string]interface{}{
			"id":              id,
			"question":        question,
			"question_type":   questionType,
			"options":         options,
			"correct_options": convertToIntSlice(correctOpts),
			"explanation":     explanation,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":    "success",
		"bookmarks": bookmarks,
	})
}
func GetAllSavedExplanations(c *fiber.Ctx) error {
	user := c.Locals("user").(models.User)

	rows, err := util.DB.Query(`
		SELECT 
			q.id, q.question, q.question_type, q.options, q.correct_options, se.explanation
		FROM saved_explanations se
		JOIN questions q ON se.question_id = q.id
		WHERE se.user_id = $1
		ORDER BY q.id DESC`, user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch saved explanations",
		})
	}
	defer rows.Close()

	saved := []map[string]interface{}{}

	for rows.Next() {
		var (
			id           int
			question     string
			questionType string
			options      []string
			correctOpts  pq.Int64Array
			explanation  *string
		)
		err := rows.Scan(&id, &question, &questionType, pq.Array(&options), &correctOpts, &explanation)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to parse saved explanation",
			})
		}

		saved = append(saved, map[string]interface{}{
			"id":              id,
			"question":        question,
			"question_type":   questionType,
			"options":         options,
			"correct_options": convertToIntSlice(correctOpts),
			"explanation":     explanation,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":             "success",
		"saved_explanations": saved,
	})
}
func UpdateSavedExplanation(c *fiber.Ctx) error {
	user := c.Locals("user").(models.User)
	idParam := c.Params("qid")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid explanation ID",
		})
	}
	var body struct {
		Explanation string `json:"explanation"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	result, err := util.DB.Exec(`
		UPDATE saved_explanations 
		SET explanation = $1 
		WHERE question_id = $2 AND user_id = $3
	`, body.Explanation, id, user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update explanation",
		})
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Explanation not found or not owned by user",
		})
	}

	return c.JSON(fiber.Map{
		"status":      "success",
		"message":     "Explanation updated successfully",
		"question_id": id,
	})
}
