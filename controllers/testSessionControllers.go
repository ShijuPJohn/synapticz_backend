package controllers

import (
	"github.com/ShijuPJohn/synapticz_backend/models"
	"github.com/ShijuPJohn/synapticz_backend/util"
	"github.com/gofiber/fiber/v2"
	"math/rand"
	"time"
)

func CreateTestSession(c *fiber.Ctx) error {
	type CreateTestSessionInput struct {
		QuestionSetID      int    `json:"question_set_id"`
		Mode               string `json:"mode"` // practice, exam, timed-practice
		RandomizeQuestions bool   `json:"randomize_questions"`
	}

	var input CreateTestSessionInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	user := c.Locals("user").(models.User)

	// Get question IDs for the set
	rows, err := util.DB.Query(`
		SELECT question_id FROM question_set_questions WHERE question_set_id = $1
	`, input.QuestionSetID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch question IDs"})
	}
	defer rows.Close()

	var questionIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read question ID"})
		}
		questionIDs = append(questionIDs, id)
	}

	if len(questionIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No questions found in the set"})
	}

	if input.RandomizeQuestions {
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(questionIDs), func(i, j int) {
			questionIDs[i], questionIDs[j] = questionIDs[j], questionIDs[i]
		})
	}

	// Get question set name
	var qsetName string
	err = util.DB.QueryRow("SELECT name FROM question_sets WHERE id = $1", input.QuestionSetID).Scan(&qsetName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch question set name"})
	}

	// Start a transaction
	tx, err := util.DB.Begin()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to begin transaction"})
	}
	defer tx.Rollback()

	var sessionID string
	err = tx.QueryRow(`
		INSERT INTO test_sessions (name, question_set_id, taken_by_id, n_total_questions, current_question_num, mode)
		VALUES ($1, $2, $3, $4, 0, $5)
		RETURNING id
	`, qsetName, input.QuestionSetID, user.ID, len(questionIDs), input.Mode).Scan(&sessionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create test session " + err.Error()})
	}

	// Save ordered questions in testsession_questions
	stmt, err := tx.Prepare("INSERT INTO testsession_questions (test_sessions_id, question_id) VALUES ($1, $2)")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to prepare insert for testsession_questions"})
	}
	defer stmt.Close()

	for _, qid := range questionIDs {
		if _, err := stmt.Exec(sessionID, qid); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to insert question into session"})
		}
	}

	if err := tx.Commit(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to commit transaction"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":       "success",
		"test_session": sessionID,
		"question_ids": questionIDs,
		"randomized":   input.RandomizeQuestions,
		"question_set": qsetName,
	})
}
