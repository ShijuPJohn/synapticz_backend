package controllers

import (
	"github.com/ShijuPJohn/synapticz_backend/models"
	"github.com/ShijuPJohn/synapticz_backend/util"
	"github.com/gofiber/fiber/v2"
	"time"
)

type CreateQuestionSetInput struct {
	Name               string   `json:"name"`
	Mode               string   `json:"mode"`
	Subject            string   `json:"subject"`
	Exam               string   `json:"exam"`
	Language           string   `json:"language"`
	TimeDuration       int      `json:"time_duration"`
	Description        string   `json:"description"`
	AssociatedResource string   `json:"associated_resource"`
	CreatedByID        string   `json:"created_by_id"`
	QuestionIDs        []string `json:"question_ids"`
}

func CreateQuestionSet(c *fiber.Ctx) error {
	var input CreateQuestionSetInput

	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid input: " + err.Error(),
		})
	}

	user := c.Locals("user").(models.User)

	tx, err := util.DB.Begin()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to start transaction",
		})
	}
	defer tx.Rollback()

	// Insert and return the auto-generated ID
	var questionSetID int
	insertQS := `
		INSERT INTO question_sets (
			name, mode, subject, exam, language,
			time_duration, description, associated_resource, created_by_id, created_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id
	`
	err = tx.QueryRow(
		insertQS,
		input.Name,
		input.Mode,
		input.Subject,
		input.Exam,
		input.Language,
		input.TimeDuration,
		input.Description,
		input.AssociatedResource,
		user.ID,
		time.Now(),
	).Scan(&questionSetID)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to insert question set: " + err.Error(),
		})
	}

	insertQ := `
		INSERT INTO question_set_questions (question_set_id, question_id)
		VALUES ($1, $2)
	`
	for _, qid := range input.QuestionIDs {
		_, err := tx.Exec(insertQ, questionSetID, qid)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to associate question: " + err.Error(),
			})
		}
	}

	if err := tx.Commit(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Transaction commit failed: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":      questionSetID,
		"message": "Question set created successfully",
	})
}

//
//func GetQuestionSets(c *fiber.Ctx) error {
//	filterClauses := []string{"1=1"}
//	params := []interface{}{}
//	paramIndex := 1
//
//	if subject := c.Query("subject"); subject != "" {
//		filterClauses = append(filterClauses, "subject = $"+strconv.Itoa(paramIndex))
//		params = append(params, subject)
//		paramIndex++
//	}
//	if category := c.Query("category"); category != "" {
//		filterClauses = append(filterClauses, "category = $"+strconv.Itoa(paramIndex))
//		params = append(params, category)
//		paramIndex++
//	}
//	if language := c.Query("language"); language != "" {
//		filterClauses = append(filterClauses, "language = $"+strconv.Itoa(paramIndex))
//		params = append(params, language)
//		paramIndex++
//	}
//
//	filter := strings.Join(filterClauses, " AND ")
//	query := "SELECT id, title, description, subject, category, tags, language, created_by_id FROM question_sets WHERE " + filter
//
//	count, _ := strconv.Atoi(c.Query("count", "0"))
//	if count > 0 {
//		query += " LIMIT " + strconv.Itoa(count)
//	}
//
//	rows, err := util.DB.Query(query, params...)
//	if err != nil {
//		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
//	}
//	defer rows.Close()
//
//	questionSets := []models.QuestionSet{}
//	for rows.Next() {
//		var q models.QuestionSet
//		var tagStr string
//		if err := rows.Scan(&q.ID, &q.Title, &q.Description, &q.Subject, &q.Category, &tagStr, &q.Language, &q.CreatedByID); err != nil {
//			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
//		}
//		q.Tags = strings.Split(tagStr, ",")
//		questionSets = append(questionSets, q)
//	}
//
//	return c.JSON(fiber.Map{"question_sets": questionSets})
//}
//
//func GetQuestionSetByID(c *fiber.Ctx) error {
//	id := c.Params("id")
//	query := `SELECT id, title, description, subject, category, tags, language, created_by_id FROM question_sets WHERE id = $1`
//	var q models.QuestionSet
//	var tagStr string
//	err := util.DB.QueryRow(query, id).Scan(&q.ID, &q.Title, &q.Description, &q.Subject, &q.Category, &tagStr, &q.Language, &q.CreatedByID)
//	if err != nil {
//		return c.Status(404).JSON(fiber.Map{"error": "Question set not found"})
//	}
//
//	q.Tags = strings.Split(tagStr, ",")
//	return c.JSON(fiber.Map{"question_set": q})
//}
//
//func EditQuestionSet(c *fiber.Ctx) error {
//	id := c.Params("id")
//	userID := c.Locals("user_id").(int)
//	userRole := c.Locals("user_role").(string)
//
//	// Only the creator or admin/owner can edit
//	var creatorID int
//	err := util.DB.QueryRow("SELECT created_by_id FROM question_sets WHERE id = $1", id).Scan(&creatorID)
//	if err != nil {
//		return c.Status(404).JSON(fiber.Map{"error": "Question set not found"})
//	}
//	if creatorID != userID && userRole != "admin" && userRole != "owner" {
//		return c.Status(403).JSON(fiber.Map{"error": "Unauthorized"})
//	}
//
//	var q models.QuestionSet
//	if err := c.BodyParser(&q); err != nil {
//		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
//	}
//
//	tagStr := strings.Join(q.Tags, ",")
//	query := `UPDATE question_sets SET title = $1, description = $2, subject = $3, category = $4, tags = $5, language = $6 WHERE id = $7`
//	_, err = util.DB.Exec(query, q.Title, q.Description, q.Subject, q.Category, tagStr, q.Language, id)
//	if err != nil {
//		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
//	}
//
//	return c.JSON(fiber.Map{"status": "updated"})
//}
//
//func DeleteQuestionSet(c *fiber.Ctx) error {
//	id := c.Params("id")
//	userID := c.Locals("user_id").(int)
//	userRole := c.Locals("user_role").(string)
//
//	// Only the creator or admin/owner can delete
//	var creatorID int
//	err := util.DB.QueryRow("SELECT created_by_id FROM question_sets WHERE id = $1", id).Scan(&creatorID)
//	if err != nil {
//		return c.Status(404).JSON(fiber.Map{"error": "Question set not found"})
//	}
//	if creatorID != userID && userRole != "admin" && userRole != "owner" {
//		return c.Status(403).JSON(fiber.Map{"error": "Unauthorized"})
//	}
//
//	_, err = util.DB.Exec("DELETE FROM question_sets WHERE id = $1", id)
//	if err != nil {
//		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
//	}
//
//	return c.JSON(fiber.Map{"status": "deleted"})
//}
