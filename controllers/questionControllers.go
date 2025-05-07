package controllers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/ShijuPJohn/synapticz_backend/models"
	"github.com/ShijuPJohn/synapticz_backend/util"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
	"strconv"
	"strings"
	"time"
)

// CreateQuestion handles the creation of a new question
func CreateQuestion(c *fiber.Ctx) error {
	validate := validator.New()
	db := util.DB

	user, ok := c.Locals("user").(models.User)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "User not found in context",
		})
	}

	body := c.Body()

	// Try parsing as an array first
	var questions []models.Question
	if err := json.Unmarshal(body, &questions); err != nil {
		// Try parsing as a single question
		var single models.Question
		if err := json.Unmarshal(body, &single); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status":  "error",
				"message": "Failed to parse request body",
				"error":   err.Error(),
			})
		}
		questions = append(questions, single)
	}

	tx, err := db.Begin()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to start transaction"})
	}
	defer tx.Rollback()

	createdQuestions := []int{}

	for i, question := range questions {
		// Validation
		q := &questions[i]

		// Normalize string fields: trim and lowercase
		q.Subject = strings.ToLower(strings.TrimSpace(q.Subject))
		q.Language = strings.ToLower(strings.TrimSpace(q.Language))
		q.QuestionType = strings.ToLower(strings.TrimSpace(q.QuestionType))
		if q.Exam != nil {
			trimmed := strings.ToLower(strings.TrimSpace(*q.Exam))
			q.Exam = &trimmed
		}

		for ti := range q.Tags {
			q.Tags[ti] = strings.ToLower(strings.TrimSpace(q.Tags[ti]))
		}
		err := validate.Struct(struct {
			Question       string   `json:"question" validate:"required"`
			Subject        string   `json:"subject" validate:"required"`
			Tags           []string `json:"tags"`
			Exam           *string  `json:"exam"`
			Language       string   `json:"language" validate:"required"`
			Difficulty     int      `json:"difficulty" validate:"oneof=1 2 3 4 5 6 7 8 9 10"`
			QuestionType   string   `json:"question_type" validate:"oneof=m-choice m-select numeric"`
			Options        []string `json:"options" validate:"required"`
			CorrectOptions []int    `json:"correct_options" validate:"required"`
			Explanation    *string  `json:"explanation"`
		}{
			Question:       question.Question,
			Subject:        question.Subject,
			Tags:           question.Tags,
			Exam:           question.Exam,
			Language:       question.Language,
			Difficulty:     question.Difficulty,
			QuestionType:   question.QuestionType,
			Options:        question.Options,
			CorrectOptions: question.CorrectOptions,
			Explanation:    question.Explanation,
		})
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status":  "error",
				"message": "Validation failed",
				"error":   err.Error(),
			})
		}

		question.CreatedByID = user.ID
		question.CreatedAt = time.Now()
		question.UpdatedAt = time.Now()

		// Insert into questions table
		insertQuery := `INSERT INTO questions (
			question, subject, exam, language, difficulty,
			question_type, options, correct_options, explanation,
			created_by_id, created_at, updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id`

		var questionID string
		err = tx.QueryRow(
			insertQuery,
			question.Question,
			question.Subject,
			question.Exam,
			question.Language,
			question.Difficulty,
			question.QuestionType,
			pq.Array(question.Options),
			pq.Array(question.CorrectOptions),
			question.Explanation,
			question.CreatedByID,
			question.CreatedAt,
			question.UpdatedAt,
		).Scan(&questionID)

		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to insert question", "details": err.Error()})
		}

		// Handle tags
		for _, tagName := range question.Tags {
			var tagID string
			err = tx.QueryRow("SELECT id FROM questiontags WHERE name = $1", tagName).Scan(&tagID)
			if err == sql.ErrNoRows {
				err = tx.QueryRow("INSERT INTO questiontags (name) VALUES ($1) RETURNING id", tagName).Scan(&tagID)
				if err != nil {
					return c.Status(500).JSON(fiber.Map{"error": "Failed to insert tag", "details": err.Error()})
				}
			} else if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch tag", "details": err.Error()})
			}

			_, err = tx.Exec("INSERT INTO question_questiontags (question_id, questiontags_id) VALUES ($1, $2)", questionID, tagID)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to link question and tag", "details": err.Error()})
			}
		}

		question.ID, _ = strconv.Atoi(questionID)
		createdQuestions = append(createdQuestions, question.ID)
	}

	if err = tx.Commit(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to commit transaction", "details": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":    "success",
		"message":   "Question(s) created successfully",
		"questions": createdQuestions,
	})
}

func GetQuestions(c *fiber.Ctx) error {
	db := util.DB

	// Query params
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("noQs", "10"))
	offset := (page - 1) * limit
	sort := c.Query("sort", "desc")
	subject := c.Query("subject")
	exam := c.Query("exam")
	language := c.Query("language")
	tags := c.Query("tags")
	fields := c.Query("fields")
	hoursAgo, _ := strconv.Atoi(c.Query("hours", "0"))
	createdBy := c.Query("createdBy")
	createdBySelf := c.Query("createdBySelf")
	qidsParam := c.Query("qids") // New parameter for question IDs array

	// Simulated current user ID (replace with actual extraction logic)
	currentUserID := c.Locals("user").(models.User).ID

	// Parse qids if provided
	var qids []int
	if qidsParam != "" {
		for _, idStr := range strings.Split(qidsParam, ",") {
			id, err := strconv.Atoi(strings.TrimSpace(idStr))
			if err == nil {
				qids = append(qids, id)
			}
		}
	}

	// Build base query
	selectedFields := "q.id, q.question, q.subject, q.exam, q.language, q.difficulty, q.question_type, q.options, q.correct_options, q.explanation, q.created_by_id, q.created_at, q.updated_at, u.name"
	if fields != "" {
		selectedFields = "q.id, u.name"
		for _, field := range strings.Split(fields, ",") {
			selectedFields += ", q." + strings.TrimSpace(field)
		}
	}

	baseQuery := `
		SELECT ` + selectedFields + `,
			COALESCE(json_agg(DISTINCT qt.name) FILTER (WHERE qt.name IS NOT NULL), '[]') AS tags
		FROM questions q
		LEFT JOIN question_questiontags qqt ON q.id = qqt.question_id
		LEFT JOIN questiontags qt ON qt.id = qqt.questiontags_id
		LEFT JOIN users u ON u.id = q.created_by_id
	`

	// Filters
	var conditions []string
	var args []interface{}
	argID := 1

	// If qids are provided, make them the primary filter
	if len(qids) > 0 {
		conditions = append(conditions, fmt.Sprintf("q.id = ANY($%d)", argID))
		args = append(args, pq.Array(qids))
		argID++
	}

	if subject != "" {
		conditions = append(conditions, fmt.Sprintf("q.subject = $%d", argID))
		args = append(args, subject)
		argID++
	}
	if exam != "" {
		conditions = append(conditions, fmt.Sprintf("q.exam = $%d", argID))
		args = append(args, exam)
		argID++
	}
	if language != "" {
		conditions = append(conditions, fmt.Sprintf("q.language = $%d", argID))
		args = append(args, language)
		argID++
	}
	if tags != "" {
		tagList := strings.Split(tags, ",")
		tagPlaceholders := []string{}
		for _, tag := range tagList {
			tagPlaceholders = append(tagPlaceholders, fmt.Sprintf("$%d", argID))
			args = append(args, strings.TrimSpace(tag))
			argID++
		}
		conditions = append(conditions, fmt.Sprintf(`qt.name IN (%s)`, strings.Join(tagPlaceholders, ", ")))
	}
	if hoursAgo > 0 {
		conditions = append(conditions, fmt.Sprintf("q.created_at >= NOW() - INTERVAL '%d hours'", hoursAgo))
	}
	if createdBy != "" {
		conditions = append(conditions, fmt.Sprintf("u.name ILIKE $%d", argID))
		args = append(args, "%"+createdBy+"%")
		argID++
	}
	if createdBySelf == "true" && currentUserID > 0 {
		conditions = append(conditions, fmt.Sprintf("q.created_by_id = $%d", argID))
		args = append(args, currentUserID)
		argID++
	}
	conditions = append(conditions, "q.deleted=false")

	if len(conditions) > 0 {
		baseQuery += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Grouping and sorting
	baseQuery += " GROUP BY q.id, u.name"
	if sort == "asc" {
		baseQuery += " ORDER BY q.created_at ASC"
	} else {
		baseQuery += " ORDER BY q.created_at DESC"
	}

	// Only apply pagination if we're not fetching specific IDs
	if len(qids) == 0 {
		baseQuery += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
	}

	// Execute query
	rows, err := db.Query(baseQuery, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to retrieve questions",
			"error":   err.Error(),
		})
	}
	defer rows.Close()

	// Parse results
	type QuestionResponse struct {
		ID             int       `json:"id"`
		Question       string    `json:"question,omitempty"`
		Subject        string    `json:"subject,omitempty"`
		Exam           *string   `json:"exam,omitempty"`
		Language       string    `json:"language,omitempty"`
		Difficulty     int       `json:"difficulty,omitempty"`
		QuestionType   string    `json:"question_type,omitempty"`
		Options        []string  `json:"options,omitempty"`
		CorrectOptions []string  `json:"correct_options,omitempty"`
		Explanation    *string   `json:"explanation,omitempty"`
		CreatedByID    int       `json:"created_by_id,omitempty"`
		CreatedByName  string    `json:"created_by_name,omitempty"`
		CreatedAt      time.Time `json:"created_at,omitempty"`
		UpdatedAt      time.Time `json:"updated_at,omitempty"`
		Tags           []string  `json:"tags"`
	}

	var questions []QuestionResponse
	var tagsJSON []byte
	for rows.Next() {
		var q QuestionResponse
		err := rows.Scan(
			&q.ID, &q.Question, &q.Subject, &q.Exam, &q.Language, &q.Difficulty,
			&q.QuestionType, pq.Array(&q.Options), pq.Array(&q.CorrectOptions),
			&q.Explanation, &q.CreatedByID, &q.CreatedAt, &q.UpdatedAt, &q.CreatedByName, &tagsJSON,
		)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": "Failed to parse results",
				"error":   err.Error(),
			})
		}
		if err := json.Unmarshal(tagsJSON, &q.Tags); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": "Failed to parse tags JSON",
				"error":   err.Error(),
			})
		}
		questions = append(questions, q)
	}

	// If specific IDs were requested, return them all (no pagination)
	if len(qids) > 0 {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":    "success",
			"questions": questions,
		})
	}

	// Otherwise return paginated response
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":    "success",
		"questions": questions,
		"page":      page,
		"limit":     limit,
	})
}

func GetQuestionByID(c *fiber.Ctx) error {
	db := util.DB

	// Parse question ID from route
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Question ID is required",
		})
	}

	// Query to fetch the question and its tags
	query := `
		SELECT 
			q.id, q.question, q.subject, q.exam, q.language, q.difficulty, q.question_type,
			q.options, q.correct_options, q.explanation, q.created_by_id, q.created_at, q.updated_at,
			COALESCE(json_agg(DISTINCT qt.name) FILTER (WHERE qt.name IS NOT NULL), '[]') AS tags
		FROM questions q
		LEFT JOIN question_questiontags qqt ON q.id = qqt.question_id
		LEFT JOIN questiontags qt ON qt.id = qqt.questiontags_id
		WHERE q.id = $1
		GROUP BY q.id
	`

	row := db.QueryRow(query, id)

	// Struct for response
	type QuestionResponse struct {
		ID             int       `json:"id"`
		Question       string    `json:"question,omitempty"`
		Subject        string    `json:"subject,omitempty"`
		Exam           *string   `json:"exam,omitempty"`
		Language       string    `json:"language,omitempty"`
		Difficulty     int       `json:"difficulty,omitempty"`
		QuestionType   string    `json:"question_type,omitempty"`
		Options        []string  `json:"options,omitempty"`
		CorrectOptions []string  `json:"correct_options,omitempty"`
		Explanation    *string   `json:"explanation,omitempty"`
		CreatedByID    int       `json:"created_by_id,omitempty"`
		CreatedAt      time.Time `json:"created_at,omitempty"`
		UpdatedAt      time.Time `json:"updated_at,omitempty"`
		Tags           []string  `json:"tags"`
	}

	var q QuestionResponse
	var tagsJSON []byte

	err := row.Scan(
		&q.ID, &q.Question, &q.Subject, &q.Exam, &q.Language, &q.Difficulty,
		&q.QuestionType, pq.Array(&q.Options), pq.Array(&q.CorrectOptions),
		&q.Explanation, &q.CreatedByID, &q.CreatedAt, &q.UpdatedAt, &tagsJSON,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"status":  "error",
				"message": "Question not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to fetch question",
			"error":   err.Error(),
		})
	}

	if err := json.Unmarshal(tagsJSON, &q.Tags); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to parse tags JSON",
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":   "success",
		"question": q,
	})
}

func DeleteQuestion(c *fiber.Ctx) error {
	db := util.DB

	// Get question ID from URL
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Question ID is required",
		})
	}

	user := c.Locals("user").(models.User)

	// Check if question exists and get creator
	var createdByID int
	err := db.QueryRow("SELECT created_by_id FROM questions WHERE id = $1 AND deleted=false", id).Scan(&createdByID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"status":  "error",
				"message": "Question not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Database error",
			"error":   err.Error(),
		})
	}

	// Authorization check
	if user.ID != createdByID && user.Role != "admin" && user.Role != "owner" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  "error",
			"message": "You are not authorized to delete this question",
		})
	}

	// Perform soft delete by updating deleted_at
	_, err = db.Exec(`UPDATE questions SET deleted = true WHERE id = $1`, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to soft delete question",
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"message": "Question deleted successfully",
	})
}

func EditQuestion(c *fiber.Ctx) error {
	db := util.DB
	validate := validator.New()

	// Get question ID from URL
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Question ID is required",
		})
	}

	user := c.Locals("user").(models.User)

	// Check if question exists and get creator
	var createdByID int
	err := db.QueryRow("SELECT created_by_id FROM questions WHERE id = $1", id).Scan(&createdByID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"status":  "error",
				"message": "Question not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Database error",
			"error":   err.Error(),
		})
	}

	// Authorization check
	if user.ID != createdByID && user.Role != "admin" && user.Role != "owner" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  "error",
			"message": "You are not authorized to edit this question",
		})
	}

	// Parse and validate body
	var updated models.Question
	if err := c.BodyParser(&updated); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	err = validate.Struct(struct {
		Question       string   `json:"question" validate:"required"`
		Subject        string   `json:"subject" validate:"required"`
		Tags           []string `json:"tags"`
		Exam           *string  `json:"exam"`
		Language       string   `json:"language" validate:"required"`
		Difficulty     int      `json:"difficulty" validate:"oneof=1 2 3 4 5 6 7 8 9 10"`
		QuestionType   string   `json:"question_type" validate:"oneof=m-choice m-select numeric"`
		Options        []string `json:"options" validate:"required"`
		CorrectOptions []int    `json:"correct_options" validate:"required"`
		Explanation    *string  `json:"explanation"`
	}{
		Question:       updated.Question,
		Subject:        updated.Subject,
		Tags:           updated.Tags,
		Exam:           updated.Exam,
		Language:       updated.Language,
		Difficulty:     updated.Difficulty,
		QuestionType:   updated.QuestionType,
		Options:        updated.Options,
		CorrectOptions: updated.CorrectOptions,
		Explanation:    updated.Explanation,
	})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Validation failed",
			"error":   err.Error(),
		})
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to start transaction",
			"error":   err.Error(),
		})
	}
	defer tx.Rollback()

	// Update question
	_, err = tx.Exec(`
		UPDATE questions SET
			question = $1,
			subject = $2,
			exam = $3,
			language = $4,
			difficulty = $5,
			question_type = $6,
			options = $7,
			correct_options = $8,
			explanation = $9,
			updated_at = $10
		WHERE id = $11`,
		updated.Question,
		updated.Subject,
		updated.Exam,
		updated.Language,
		updated.Difficulty,
		updated.QuestionType,
		pq.Array(updated.Options),
		pq.Array(updated.CorrectOptions),
		updated.Explanation,
		time.Now(),
		id,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to update question",
			"error":   err.Error(),
		})
	}

	// Delete existing tag links
	_, err = tx.Exec("DELETE FROM question_questiontags WHERE question_id = $1", id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to clear old tags",
			"error":   err.Error(),
		})
	}

	// Insert new tags and links
	for _, tagName := range updated.Tags {
		var tagID string
		err = tx.QueryRow("SELECT id FROM questiontags WHERE name = $1", tagName).Scan(&tagID)
		if err == sql.ErrNoRows {
			err = tx.QueryRow("INSERT INTO questiontags (name) VALUES ($1) RETURNING id", tagName).Scan(&tagID)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to insert tag", "details": err.Error()})
			}
		} else if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch tag", "details": err.Error()})
		}

		_, err = tx.Exec("INSERT INTO question_questiontags (question_id, questiontags_id) VALUES ($1, $2)", id, tagID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to link tag", "details": err.Error()})
		}
	}

	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to commit transaction", "details": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"message": "Question updated successfully",
	})
}
