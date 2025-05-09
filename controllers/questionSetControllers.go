package controllers

import (
	"database/sql"
	"fmt"
	"github.com/ShijuPJohn/synapticz_backend/models"
	"github.com/ShijuPJohn/synapticz_backend/util"
	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
	"math"
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
	CoverImage         *string    `json:"cover_image"`
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
			time_duration, description, associated_resource, created_by_id, created_at, cover_image
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10, $11)
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
		input.CoverImage,
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
	uid := c.Query("uid")
	search := c.Query("search")
	resource := c.Query("resource")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	// Base query for fetching question sets
	baseQuery := `
		SELECT 
			qs.id, qs.name, qs.mode, qs.subject, qs.exam, qs.language,
			qs.time_duration, qs.description, qs.associated_resource,
			qs.created_by_id, u.name, qs.cover_image, qs.created_at,
			(
				SELECT COUNT(*) 
				FROM question_set_questions qq 
				WHERE qq.question_set_id = qs.id
			) AS total_questions
		FROM question_sets qs
		JOIN users u ON qs.created_by_id = u.id
		WHERE qs.deleted <> true
	`

	// Count query for total items
	countQuery := `
		SELECT COUNT(DISTINCT qs.id)
		FROM question_sets qs
		JOIN users u ON qs.created_by_id = u.id
		WHERE qs.deleted <> true
	`

	args := []interface{}{}
	argID := 1

	// If UID is present, check user role and apply role-based filter
	if uid != "" {
		var userRole string
		uidInt, _ := strconv.Atoi(uid)
		err := util.DB.QueryRow("SELECT role FROM users WHERE id = $1", uidInt).Scan(&userRole)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid user ID or user not found",
			})
		}
		if userRole == "user" {
			baseQuery += fmt.Sprintf(" AND qs.created_by_id = $%d", argID)
			countQuery += fmt.Sprintf(" AND qs.created_by_id = $%d", argID)
			args = append(args, uid)
			argID++
		}
	}

	// Apply filters to both queries
	if subject != "" {
		baseQuery += fmt.Sprintf(" AND qs.subject = $%d", argID)
		countQuery += fmt.Sprintf(" AND qs.subject = $%d", argID)
		args = append(args, subject)
		argID++
	}
	if exam != "" {
		baseQuery += fmt.Sprintf(" AND qs.exam = $%d", argID)
		countQuery += fmt.Sprintf(" AND qs.exam = $%d", argID)
		args = append(args, exam)
		argID++
	}
	if language != "" {
		baseQuery += fmt.Sprintf(" AND qs.language = $%d", argID)
		countQuery += fmt.Sprintf(" AND qs.language = $%d", argID)
		args = append(args, language)
		argID++
	}
	if resource != "" {
		baseQuery += fmt.Sprintf(" AND qs.associated_resource = $%d", argID)
		countQuery += fmt.Sprintf(" AND qs.associated_resource = $%d", argID)
		args = append(args, resource)
		argID++
	}
	if tags != "" {
		tagList := strings.Split(tags, ",")
		tagCount := len(tagList)

		tagFilter := fmt.Sprintf(`
			AND qs.id IN (
				SELECT questionset_id
				FROM questionsets_questionsettags qst
				JOIN questionsettags t ON qst.questionsettags_id = t.id
				WHERE t.name = ANY($%d)
				GROUP BY questionset_id
				HAVING COUNT(DISTINCT t.name) = %d
			)
		`, argID, tagCount)

		baseQuery += tagFilter
		countQuery += tagFilter
		args = append(args, pq.Array(tagList))
		argID++
	}
	if search != "" {
		searchTerm := "%" + search + "%"
		searchFilter := fmt.Sprintf(`
        AND (
            LOWER(qs.name) ILIKE $%d OR
            LOWER(qs.subject) ILIKE $%d OR
            LOWER(qs.description) ILIKE $%d OR
            LOWER(qs.associated_resource) ILIKE $%d OR
            EXISTS (
                SELECT 1 FROM questionsets_questionsettags qst2
                JOIN questionsettags t2 ON qst2.questionsettags_id = t2.id
                WHERE qst2.questionset_id = qs.id AND LOWER(t2.name) ILIKE $%d
            )
        )
    `, argID, argID, argID, argID, argID)

		baseQuery += searchFilter
		countQuery += searchFilter
		args = append(args, strings.ToLower(searchTerm))
		argID++
	}

	// Get total count
	var totalCount int
	err := util.DB.QueryRow(countQuery, args...).Scan(&totalCount)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to count question sets: " + err.Error(),
		})
	}

	// Add sorting and pagination to the base query
	baseQuery += " ORDER BY qs.created_at DESC"
	baseQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argID, argID+1)
	args = append(args, limit, offset)

	// Execute main query
	rows, err := util.DB.Query(baseQuery, args...)
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
		CreatedByID        int       `json:"created_by_id"`
		CreatedByName      string    `json:"created_by_name"`
		CoverImage         string    `json:"coverImage"`
		CreatedAt          time.Time `json:"created_at"`
		TotalQuestions     int       `json:"total_questions"`
		QuestionIDs        []int     `json:"question_ids"`
	}

	var results []QuestionSetResponse

	// Pre-allocate slice with capacity for the expected number of results
	results = make([]QuestionSetResponse, 0, limit)

	for rows.Next() {
		var qs QuestionSetResponse
		var coverImage sql.NullString

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
			&coverImage,
			&qs.CreatedAt,
			&qs.TotalQuestions,
		)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to scan question set: " + err.Error(),
			})
		}

		qs.CoverImage = ""
		if coverImage.Valid {
			qs.CoverImage = coverImage.String
		}

		// Get question IDs in a single batch for all sets (optimization)
		// We'll do this after collecting all set IDs to reduce DB queries
		results = append(results, qs)
	}

	// Batch fetch question IDs for all sets
	if len(results) > 0 {
		setIDs := make([]int, len(results))
		for i, qs := range results {
			setIDs[i] = qs.ID
		}

		questionIDsQuery := `
			SELECT question_set_id, question_id 
			FROM question_set_questions 
			WHERE question_set_id = ANY($1)
			ORDER BY question_set_id, question_id
		`
		questionRows, err := util.DB.Query(questionIDsQuery, pq.Array(setIDs))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to fetch question IDs: " + err.Error(),
			})
		}
		defer questionRows.Close()

		// Create a map of set ID to question IDs
		questionMap := make(map[int][]int)
		for questionRows.Next() {
			var setID, questionID int
			if err := questionRows.Scan(&setID, &questionID); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to scan question ID: " + err.Error(),
				})
			}
			questionMap[setID] = append(questionMap[setID], questionID)
		}

		// Assign question IDs to each set
		for i := range results {
			results[i].QuestionIDs = questionMap[results[i].ID]
		}
	}

	// Calculate pagination info
	totalPages := int(math.Ceil(float64(totalCount) / float64(limit)))
	pagination := fiber.Map{
		"total":       totalCount,
		"total_pages": totalPages,
		"per_page":    limit,
		"current":     page,
		"next":        nil,
		"prev":        nil,
	}

	if page < totalPages {
		pagination["next"] = page + 1
	}
	if page > 1 {
		pagination["prev"] = page - 1
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data":       results,
		"pagination": pagination,
	})
}

func GetQuestionSetByID(c *fiber.Ctx) error {
	id := c.Params("id")

	var qs struct {
		ID                   int       `json:"id"`
		Name                 string    `json:"name"`
		Mode                 string    `json:"mode"`
		Subject              string    `json:"subject"`
		Exam                 string    `json:"exam"`
		Language             string    `json:"language"`
		TimeDuration         int       `json:"time_duration"`
		Description          string    `json:"description"`
		AssociatedResource   string    `json:"associated_resource"`
		CoverImage           *string   `json:"cover_image"`
		CreatedAt            time.Time `json:"created_at"`
		CreatedByName        string    `json:"created_by_name"`
		TestSessionsTakenCnt int       `json:"test_sessions_taken_count"`
	}

	query := `
		SELECT 
			qs.id, qs.name, qs.mode, qs.subject, qs.exam, qs.language,
			qs.time_duration, qs.description, qs.associated_resource,
			qs.cover_image, qs.created_at, u.name AS created_by_name,
			(SELECT COUNT(*) FROM test_sessions ts WHERE ts.question_set_id = qs.id) AS test_sessions_taken_count
		FROM question_sets qs
		JOIN users u ON qs.created_by_id = u.id
		WHERE qs.id = $1
	`

	err := util.DB.QueryRow(query, id).Scan(
		&qs.ID, &qs.Name, &qs.Mode, &qs.Subject, &qs.Exam, &qs.Language,
		&qs.TimeDuration, &qs.Description, &qs.AssociatedResource,
		&qs.CoverImage, &qs.CreatedAt, &qs.CreatedByName, &qs.TestSessionsTakenCnt,
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

	// Get question IDs
	questionRows, err := util.DB.Query(`
		SELECT question_id
		FROM question_set_questions
		WHERE question_set_id = $1
	`, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve question IDs: " + err.Error(),
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
		"cover_image":         qs.CoverImage,
		"created_at":          qs.CreatedAt,
		"created_by_name":     qs.CreatedByName,
		"tags":                tags,
		"test_sessions_taken": qs.TestSessionsTakenCnt,
		"question_ids":        questionIDs,
		"can_start_test":      true,
	})
}

func SoftDeleteQuestionSet(c *fiber.Ctx) error {
	// Get question set ID from path params
	qSetID := c.Params("id")

	// Get user ID from JWT
	userFromToken := c.Locals("user").(models.User)
	userID := userFromToken.ID

	// Fetch user from DBqSetOwnerID
	var user models.User
	err := util.DB.QueryRow("SELECT id, role FROM users WHERE id = $1", userID).Scan(&user.ID, &user.Role)
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "User not found")
	}

	// Fetch the question set from DB
	var qSetOwnerID int
	err = util.DB.QueryRow("SELECT created_by_id FROM question_sets WHERE id = $1", qSetID).Scan(&qSetOwnerID)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "Question set not found")
	}

	// Authorization check
	if user.Role != "admin" && user.Role != "owner" && qSetOwnerID != user.ID {
		return fiber.NewError(fiber.StatusForbidden, "You are not allowed to delete this question set")
	}

	// Perform soft delete (update the 'deleted' column)
	_, err = util.DB.Exec("UPDATE question_sets SET deleted = true WHERE id = $1", qSetID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to delete question set")
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Question set deleted successfully",
	})
}

type UpdateQuestionSetInput struct {
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
	CoverImage         *string    `json:"cover_image"`
}

func UpdateQuestionSet(c *fiber.Ctx) error {
	// Get question set ID from path params
	qSetID := c.Params("id")

	// Get user from JWT context
	user := c.Locals("user").(models.User)

	// Parse input
	var input UpdateQuestionSetInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid input: " + err.Error(),
		})
	}

	// Start transaction
	tx, err := util.DB.Begin()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to start transaction",
		})
	}
	defer tx.Rollback()

	// Check if question set exists and verify ownership
	var createdByID int
	err = tx.QueryRow(
		"SELECT created_by_id FROM question_sets WHERE id = $1 AND deleted <> true",
		qSetID,
	).Scan(&createdByID)

	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Question set not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to verify question set: " + err.Error(),
		})
	}

	// Authorization check - only admin, owner or creator can update
	if user.Role != "admin" && user.Role != "owner" && createdByID != user.ID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You are not authorized to update this question set",
		})
	}

	// Update question set details
	updateQuery := `
		UPDATE question_sets
		SET 
			name = $1,
			mode = $2,
			subject = $3,
			exam = $4,
			language = $5,
			time_duration = $6,
			description = $7,
			associated_resource = $8,
			cover_image = COALESCE($9, cover_image)
		WHERE id = $10
	`

	_, err = tx.Exec(
		updateQuery,
		input.Name,
		input.Mode,
		input.Subject,
		input.Exam,
		input.Language,
		input.TimeDuration,
		input.Description,
		input.AssociatedResource,
		input.CoverImage,
		qSetID,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update question set: " + err.Error(),
		})
	}

	// Handle questions update
	// First, delete existing questions
	_, err = tx.Exec("DELETE FROM question_set_questions WHERE question_set_id = $1", qSetID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to remove existing questions: " + err.Error(),
		})
	}

	// Then add new questions
	if len(input.QuestionIDs) > 0 {
		markSlice := []float64{}
		if input.Marks != nil && len(*input.Marks) == len(input.QuestionIDs) {
			markSlice = *input.Marks
			insertQ := `
				INSERT INTO question_set_questions (question_set_id, question_id, mark)
				VALUES ($1, $2, $3)
			`
			for i, qid := range input.QuestionIDs {
				_, err := tx.Exec(insertQ, qSetID, qid, markSlice[i])
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
				_, err := tx.Exec(insertQ, qSetID, qid)
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": "Failed to associate question: " + err.Error(),
					})
				}
			}
		}
	}

	// Handle tags update
	// First, delete existing tags
	_, err = tx.Exec("DELETE FROM questionsets_questionsettags WHERE questionset_id = $1", qSetID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to remove existing tags: " + err.Error(),
		})
	}

	// Then add new tags
	if len(input.Tags) > 0 {
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

			_, err = tx.Exec(insertTagRelation, qSetID, tagID)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to link tag to question set: " + err.Error(),
				})
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Transaction commit failed: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Question set updated successfully",
	})
}
