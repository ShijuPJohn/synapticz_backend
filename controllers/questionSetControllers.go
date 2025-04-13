package controllers

import (
	"database/sql"
	"fmt"
	"github.com/ShijuPJohn/synapticz_backend/models"
	"github.com/ShijuPJohn/synapticz_backend/util"
	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
	"strconv"
	"strings"
	"time"
)

type CreateQuestionSetInput struct {
	Name               string     `json:"name"`
	Mode               string     `json:"mode"`
	Subject            string     `json:"subject"`
	Exam               string     `json:"exam"`
	Language           string     `json:"language"`
	TimeDuration       int        `json:"time_duration"`
	Description        string     `json:"description"`
	AssociatedResource string     `json:"associated_resource"`
	QuestionIDs        []int      `json:"question_ids"`
	Tags               []string   `json:"tags"`
	Marks              *[]float64 `json:"marks"`
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
	markSlice := []float64{}
	if input.Marks != nil && len(*input.Marks) == len(input.QuestionIDs) {
		markSlice = *input.Marks
		insertQ := `
		INSERT INTO question_set_questions (question_set_id, question_id, mark)
		VALUES ($1, $2, $3)
	`
		for i, qid := range input.QuestionIDs {
			_, err := tx.Exec(insertQ, questionSetID, qid, markSlice[i])
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to associate question: " + err.Error(),
				})
			}
		}
	} else {
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
	}

	// Insert questions

	// Handle tags
	getOrInsertTag := `
		INSERT INTO questionsettags (name)
		VALUES ($1)
		ON CONFLICT (name) DO NOTHING
		RETURNING id
	`

	getTagID := `
		SELECT id FROM questionsettags WHERE name = $1
	`

	insertTagRelation := `
		INSERT INTO questionsets_questionsettags (questionset_id, questionsettags_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`

	for _, tag := range input.Tags {
		var tagID int
		// First try to insert tag and get ID
		err = tx.QueryRow(getOrInsertTag, tag).Scan(&tagID)

		if err == sql.ErrNoRows {
			// Tag existed, so get ID
			err = tx.QueryRow(getTagID, tag).Scan(&tagID)
		}
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to insert/retrieve tag: " + err.Error(),
			})
		}

		_, err = tx.Exec(insertTagRelation, questionSetID, tagID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to link tag to question set: " + err.Error(),
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
func GetQuestionSets(c *fiber.Ctx) error {
	subject := c.Query("subject")
	exam := c.Query("exam")
	language := c.Query("language")
	tags := c.Query("tags")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	query := `
		SELECT DISTINCT 
			qs.id, qs.name, qs.mode, qs.subject, qs.exam, qs.language,
			qs.time_duration, qs.description, qs.associated_resource,
			qs.created_by_id, u.name as created_by_name, qs.created_at,
			COUNT(qq.question_id) AS total_questions
		FROM question_sets qs
		JOIN users u ON qs.created_by_id = u.id
		LEFT JOIN question_set_questions qq ON qs.id = qq.question_set_id
		LEFT JOIN questionsets_questionsettags qst ON qs.id = qst.questionset_id
		LEFT JOIN questionsettags t ON qst.questionsettags_id = t.id
		WHERE 1=1
	`
	args := []interface{}{}
	argID := 1

	if subject != "" {
		query += fmt.Sprintf(" AND qs.subject = $%d", argID)
		args = append(args, subject)
		argID++
	}
	if exam != "" {
		query += fmt.Sprintf(" AND qs.exam = $%d", argID)
		args = append(args, exam)
		argID++
	}
	if language != "" {
		query += fmt.Sprintf(" AND qs.language = $%d", argID)
		args = append(args, language)
		argID++
	}
	if tags != "" {
		tagList := strings.Split(tags, ",")
		tagCount := len(tagList)

		query += fmt.Sprintf(`
			AND qs.id IN (
				SELECT questionset_id
				FROM questionsets_questionsettags qst
				JOIN questionsettags t ON qst.questionsettags_id = t.id
				WHERE t.name = ANY($%d)
				GROUP BY questionset_id
				HAVING COUNT(DISTINCT t.name) = %d
			)
		`, argID, tagCount)

		args = append(args, pq.Array(tagList))
		argID++
	}

	query += `
		GROUP BY 
			qs.id, qs.name, qs.mode, qs.subject, qs.exam, qs.language,
			qs.time_duration, qs.description, qs.associated_resource,
			qs.created_by_id, u.name, qs.created_at
	`
	query += fmt.Sprintf(" ORDER BY qs.created_at DESC LIMIT $%d OFFSET $%d", argID, argID+1)
	args = append(args, limit, offset)

	rows, err := util.DB.Query(query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch question sets: " + err.Error(),
		})
	}
	defer rows.Close()

	type QuestionSetResponse struct {
		ID                 int       `json:"id"`
		Name               string    `json:"name"`
		Mode               string    `json:"mode"`
		Subject            string    `json:"subject"`
		Exam               string    `json:"exam"`
		Language           string    `json:"language"`
		TimeDuration       int       `json:"time_duration"`
		Description        string    `json:"description"`
		AssociatedResource string    `json:"associated_resource"`
		CreatedByID        string    `json:"created_by_id"`
		CreatedByName      string    `json:"created_by_name"`
		CreatedAt          time.Time `json:"created_at"`
		TotalQuestions     int       `json:"total_questions"`
	}

	var results []QuestionSetResponse

	for rows.Next() {
		var qs QuestionSetResponse
		err := rows.Scan(
			&qs.ID,
			&qs.Name,
			&qs.Mode,
			&qs.Subject,
			&qs.Exam,
			&qs.Language,
			&qs.TimeDuration,
			&qs.Description,
			&qs.AssociatedResource,
			&qs.CreatedByID,
			&qs.CreatedByName,
			&qs.CreatedAt,
			&qs.TotalQuestions,
		)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to scan question set: " + err.Error(),
			})
		}
		results = append(results, qs)
	}

	return c.Status(fiber.StatusOK).JSON(results)
}

func GetQuestionSetByID(c *fiber.Ctx) error {
	id := c.Params("id")

	var qs struct {
		ID                 int       `json:"id"`
		Name               string    `json:"name"`
		Mode               string    `json:"mode"`
		Subject            string    `json:"subject"`
		Exam               string    `json:"exam"`
		Language           string    `json:"language"`
		TimeDuration       int       `json:"time_duration"`
		Description        string    `json:"description"`
		AssociatedResource string    `json:"associated_resource"`
		CreatedAt          time.Time `json:"created_at"`
		CreatedByName      string    `json:"created_by_name"`
	}

	query := `
		SELECT 
			qs.id, qs.name, qs.mode, qs.subject, qs.exam, qs.language,
			qs.time_duration, qs.description, qs.associated_resource,
			qs.created_at, u.name as created_by_name
		FROM question_sets qs
		JOIN users u ON qs.created_by_id = u.id
		WHERE qs.id = $1
	`

	err := util.DB.QueryRow(query, id).Scan(
		&qs.ID, &qs.Name, &qs.Mode, &qs.Subject, &qs.Exam, &qs.Language,
		&qs.TimeDuration, &qs.Description, &qs.AssociatedResource,
		&qs.CreatedAt, &qs.CreatedByName,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Question set not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve question set: " + err.Error(),
		})
	}

	// Get question IDs
	questionRows, err := util.DB.Query(`
		SELECT question_id FROM question_set_questions
		WHERE question_set_id = $1
	`, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve questions: " + err.Error(),
		})
	}
	defer questionRows.Close()

	var questionIDs []int
	for questionRows.Next() {
		var qid int
		if err := questionRows.Scan(&qid); err == nil {
			questionIDs = append(questionIDs, qid)
		}
	}

	// Get tags
	tagRows, err := util.DB.Query(`
		SELECT t.name
		FROM questionsets_questionsettags qst
		JOIN questionsettags t ON qst.questionsettags_id = t.id
		WHERE qst.questionset_id = $1
	`, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve tags: " + err.Error(),
		})
	}
	defer tagRows.Close()

	var tags []string
	for tagRows.Next() {
		var tag string
		if err := tagRows.Scan(&tag); err == nil {
			tags = append(tags, tag)
		}
	}

	return c.JSON(fiber.Map{
		"id":                  qs.ID,
		"name":                qs.Name,
		"mode":                qs.Mode,
		"subject":             qs.Subject,
		"exam":                qs.Exam,
		"language":            qs.Language,
		"time_duration":       qs.TimeDuration,
		"description":         qs.Description,
		"associated_resource": qs.AssociatedResource,
		"created_at":          qs.CreatedAt,
		"created_by_name":     qs.CreatedByName,
		"question_ids":        questionIDs,
		"tags":                tags,
	})
}
