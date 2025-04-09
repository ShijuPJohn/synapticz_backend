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

	var question models.Question
	if err := c.BodyParser(&question); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to parse request body",
			"error":   err.Error(),
		})
	}

	// Validate fields (excluding tags relationship)
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
	fmt.Println(question.CorrectOptions)
	tx, err := db.Begin()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to start transaction"})
	}
	defer tx.Rollback()

	// Step 1: Insert into questions
	insertQuery := `
		INSERT INTO questions (
			question, subject, exam, language, difficulty,
			question_type, options, correct_options, explanation,
			created_by_id, created_at, updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id
	`

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

	// Step 2: Handle Tags
	for _, tagName := range question.Tags {
		var tagID string

		// Check if tag exists
		err = tx.QueryRow("SELECT id FROM questiontags WHERE name = $1", tagName).Scan(&tagID)
		if err == sql.ErrNoRows {
			// Insert new tag
			err = tx.QueryRow("INSERT INTO questiontags (name) VALUES ($1) RETURNING id", tagName).Scan(&tagID)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to insert tag", "details": err.Error()})
			}
		} else if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch tag", "details": err.Error()})
		}

		// Insert into question_tags junction
		_, err = tx.Exec("INSERT INTO question_questiontags (question_id, questiontags_id) VALUES ($1, $2)", questionID, tagID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to link question and tag", "details": err.Error()})
		}
	}

	if err = tx.Commit(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to commit transaction", "details": err.Error()})
	}

	question.ID, _ = strconv.Atoi(questionID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":   "success",
		"message":  "Question created successfully",
		"question": question,
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
	tags := c.Query("tags") // comma-separated tag names
	fields := c.Query("fields")

	// Start building base query
	selectedFields := "q.id, q.question, q.subject, q.exam, q.language, q.difficulty, q.question_type, q.options, q.correct_options, q.explanation, q.created_by_id, q.created_at, q.updated_at"
	if fields != "" {
		selectedFields = "q.id" // always include id
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
	`

	// Filters
	var conditions []string
	var args []interface{}
	argID := 1

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
		tagFilter := fmt.Sprintf(`qt.name IN (%s)`, strings.Join(tagPlaceholders, ", "))
		conditions = append(conditions, tagFilter)
	}

	// WHERE clause
	if len(conditions) > 0 {
		baseQuery += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Grouping and sorting
	baseQuery += " GROUP BY q.id"
	if sort == "asc" {
		baseQuery += " ORDER BY q.created_at ASC"
	} else {
		baseQuery += " ORDER BY q.created_at DESC"
	}

	// Pagination
	baseQuery += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)

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
			&q.Explanation, &q.CreatedByID, &q.CreatedAt, &q.UpdatedAt, &tagsJSON,
		)
		if err := json.Unmarshal(tagsJSON, &q.Tags); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": "Failed to parse tags JSON",
				"error":   err.Error(),
			})
		}
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": "Failed to parse results",
				"error":   err.Error(),
			})
		}
		questions = append(questions, q)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":    "success",
		"questions": questions,
	})
}

func GetQuestionByID(c *fiber.Ctx) error {
	fmt.Println("reached the controller function")
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

//func GetQuestionByID(c *fiber.Ctx) error {
//	// Get the question ID from the request parameters
//	idParam := c.Params("id")
//	qID, err := primitive.ObjectIDFromHex(idParam)
//	if err != nil {
//		return handleError(c, fiber.StatusBadRequest, err.Error())
//	}
//
//	// Define a filter to find the question by its ID
//	filter := bson.M{"_id": qID}
//
//	// Find the question in the database
//	var question models.Question
//	err = utils.Mg.Db.Collection("questions").FindOne(c.Context(), filter).Decode(&question)
//	if err != nil {
//		if err.Error() == "mongo: no documents in result" {
//			return handleError(c, fiber.StatusNotFound, "Question not found")
//		}
//		return handleError(c, fiber.StatusInternalServerError, err.Error())
//	}
//
//	// Return the question
//	return c.Status(fiber.StatusOK).JSON(fiber.Map{"question": question})
//}
//func handleError(c *fiber.Ctx, statusCode int, errorMessage string) error {
//	return c.Status(statusCode).JSON(fiber.Map{
//		"status":  "error",
//		"message": errorMessage,
//	})
//}
