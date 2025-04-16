package controllers

import (
	"database/sql"
	"fmt"
	"github.com/ShijuPJohn/synapticz_backend/models"
	"github.com/ShijuPJohn/synapticz_backend/util"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"log"
	"math/rand"
	"strconv"
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
		SELECT question_id,mark FROM question_set_questions WHERE question_set_id = $1
	`, input.QuestionSetID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch question IDs"})
	}
	defer rows.Close()

	var questionIDs []int
	var marks []float64
	for rows.Next() {
		var id int
		var mark float64
		if err := rows.Scan(&id, &mark); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read question ID"})
		}
		questionIDs = append(questionIDs, id)
		marks = append(marks, mark)
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

	// Prepare for inserting initial answer data
	stmtAnswers, err := tx.Prepare(`
		INSERT INTO test_session_question_answers (
			test_session_id, question_id, correct_answer_list,
			selected_answer_list, questions_total_mark,
			questions_scored_mark, answered
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to prepare insert for test_session_question_answers"})
	}
	defer stmtAnswers.Close()

	for i, qid := range questionIDs {

		// Fetch correct_options from the questions table
		var correctOptions64 pq.Int64Array
		err := tx.QueryRow(`SELECT correct_options FROM questions WHERE id = $1`, qid).Scan(&correctOptions64)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to fetch correct options: " + err.Error(),
			})
		}

		// Convert to []int
		correctOptions := make([]int, len(correctOptions64))
		for i, v := range correctOptions64 {
			correctOptions[i] = int(v)
		}

		// Insert default answer data
		_, err = stmtAnswers.Exec(
			sessionID,
			qid,
			pq.Array(correctOptions),
			pq.Array([]int{}), // selected_answer_list empty
			marks[i],          // total mark per question
			0.0,               // scored mark initially 0
			false,             // not answered yet
		)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to insert into test_session_question_answers"})
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

func GetTestSession(c *fiber.Ctx) error {
	testSessionID := c.Params("test_session_id")
	if testSessionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Test session ID is required",
		})
	}

	user := c.Locals("user").(models.User)

	// Check ownership
	var takenByID int
	err := util.DB.QueryRow("SELECT taken_by_id FROM test_sessions WHERE id = $1", testSessionID).Scan(&takenByID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Test session not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch session owner"})
	}
	if takenByID != user.ID {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Fetch test session metadata
	var session models.TestSession
	err = util.DB.QueryRow(`
		SELECT id, finished, started, name, question_set_id, taken_by_id,
		       n_total_questions, current_question_num, n_correctly_answered,
		       rank, total_marks, scored_marks, started_time, finished_time, mode
		FROM test_sessions
		WHERE id = $1
	`, testSessionID).Scan(
		&session.ID, &session.Finished, &session.Started, &session.Name, &session.QuestionSetID,
		&session.TakenByID, &session.NTotalQuestions, &session.CurrentQuestionNum,
		&session.NCorrectlyAnswered, &session.Rank, &session.TotalMarks, &session.ScoredMarks,
		&session.StartedTime, &session.FinishedTime, &session.Mode,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch test session"})
	}
	// Fetch full question + answer data
	rows, err := util.DB.Query(`
		SELECT 
			tsqa.question_id,
			q.question,
			q.question_type,
			q.options,
			q.correct_options,
			q.explanation,
			tsqa.selected_answer_list,
			tsqa.correct_answer_list,
			tsqa.questions_total_mark,
			tsqa.questions_scored_mark,
			tsqa.answered
		FROM test_session_question_answers tsqa
		JOIN questions q ON tsqa.question_id = q.id
		WHERE tsqa.test_session_id = $1
		ORDER BY tsqa.question_id
	`, testSessionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch questions"})
	}
	defer rows.Close()

	var questions []map[string]interface{}

	for rows.Next() {
		var (
			id              int
			question        string
			questionType    string
			options         []string
			correctOptions  pq.Int64Array
			explanation     *string
			selectedAnswers pq.Int64Array
			correctAnswers  pq.Int64Array
			totalMark       float64
			scoredMark      float64
			answered        bool
		)

		err := rows.Scan(
			&id, &question, &questionType, pq.Array(&options), &correctOptions, &explanation,
			&selectedAnswers, &correctAnswers, &totalMark, &scoredMark, &answered,
		)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Scan error: " + err.Error()})
		}

		// Convert pq.Int64Array → []int
		toIntSlice := func(arr pq.Int64Array) []int {
			res := make([]int, len(arr))
			for i, v := range arr {
				res[i] = int(v)
			}
			return res
		}

		q := map[string]interface{}{
			"id":                    id,
			"question":              question,
			"question_type":         questionType,
			"options":               options,
			"correct_options":       toIntSlice(correctOptions),
			"explanation":           explanation,
			"selected_answer_list":  toIntSlice(selectedAnswers),
			"correct_answer_list":   toIntSlice(correctAnswers),
			"questions_total_mark":  totalMark,
			"questions_scored_mark": scoredMark,
			"answered":              answered,
		}

		questions = append(questions, q)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":                 "success",
		"test_session":           session,
		"questions":              questions,
		"current_question_index": session.CurrentQuestionNum,
		"current_question_id":    questions[session.CurrentQuestionNum]["id"],
	})
}
func UpdateTestSession(c *fiber.Ctx) error {
	type answerDTO struct {
		QuestionAnswerData   map[string]interface{} `json:"question_answer_data"`
		CurrentQuestionIndex int                    `json:"current_question_index"`
	}

	testSessionID := c.Params("test_session_id")
	if testSessionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Test session ID is required",
		})
	}

	var dto answerDTO
	if err := c.BodyParser(&dto); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid input",
		})
	}

	user := c.Locals("user").(models.User)

	var takenByID int
	var finished bool
	err := util.DB.QueryRow(`SELECT taken_by_id, finished FROM test_sessions WHERE id = $1`, testSessionID).Scan(&takenByID, &finished)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Test session not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch session metadata"})
	}
	if takenByID != user.ID {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}
	if finished {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "finished", "message": "Test session already finished"})
	}

	tx, err := util.DB.Begin()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Transaction begin failed"})
	}
	defer tx.Rollback()

	var totalScored float64

	for qidStr, val := range dto.QuestionAnswerData {
		qid, err := strconv.Atoi(qidStr)
		if err != nil {
			continue
		}
		answer := val.(map[string]interface{})
		selectedListRaw := answer["selected_answer_list"].([]interface{})
		correctListRaw := answer["correct_answer_list"].([]interface{})

		selectedList := make([]int64, len(selectedListRaw))
		for i, v := range selectedListRaw {
			selectedList[i] = int64(v.(float64))
		}
		correctList := make([]int64, len(correctListRaw))
		for i, v := range correctListRaw {
			correctList[i] = int64(v.(float64))
		}

		totalMark := answer["questions_total_mark"].(float64)
		answered := answer["answered"].(bool)

		// Get question type
		var qType string
		err = tx.QueryRow(`SELECT question_type FROM questions WHERE id = $1`, qid).Scan(&qType)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Failed to get question type for question %d: %v", qid, err),
			})
		}

		// Scoring logic
		var scored float64 = 0

		if qType == "m-choice" {
			if len(selectedList) == 1 && len(correctList) == 1 && selectedList[0] == correctList[0] {
				scored = totalMark
			}
		} else if qType == "m-select" {
			if len(selectedList) > 0 {
				hasWrong := false
				correctMap := make(map[int64]bool)
				for _, c := range correctList {
					correctMap[c] = true
				}
				for _, selected := range selectedList {
					if !correctMap[selected] {
						hasWrong = true
						break
					}
				}
				if !hasWrong {
					fraction := float64(len(selectedList)) / float64(len(correctList))
					scored = totalMark * fraction
				}
			}
		}

		totalScored += scored

		_, err = tx.Exec(`
			UPDATE test_session_question_answers
			SET selected_answer_list = $1,
			    questions_scored_mark = $2,
			    answered = $3
			WHERE test_session_id = $4 AND question_id = $5
		`, pq.Array(selectedList), scored, answered, testSessionID, qid)

		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Failed to update answer for question %d: %v", qid, err),
			})
		}
	}

	// Update test session progress
	_, err = tx.Exec(`
		UPDATE test_sessions
		SET current_question_num = $1, scored_marks = $2
		WHERE id = $3
	`, dto.CurrentQuestionIndex, totalScored, testSessionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update test session"})
	}

	if err := tx.Commit(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Transaction commit failed"})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":               "success",
		"current_question_num": dto.CurrentQuestionIndex,
		"scored_marks":         totalScored,
	})
}
func FinishTestSession(c *fiber.Ctx) error {
	testSessionID := c.Params("test_session_id")
	if testSessionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Test session ID is required",
		})
	}

	user := c.Locals("user").(models.User)

	var takenByID int
	var finished bool
	var startedTime time.Time
	err := util.DB.QueryRow(`
		SELECT taken_by_id, finished, started_time 
		FROM test_sessions 
		WHERE id = $1
	`, testSessionID).Scan(&takenByID, &finished, &startedTime)

	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Test session not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch test session"})
	}

	if takenByID != user.ID {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}
	if finished {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Test session is already finished"})
	}

	// Aggregate marks
	var totalMarks, scoredMarks float64
	err = util.DB.QueryRow(`
		SELECT 
			COALESCE(SUM(questions_total_mark), 0), 
			COALESCE(SUM(questions_scored_mark), 0)
		FROM test_session_question_answers
		WHERE test_session_id = $1
	`, testSessionID).Scan(&totalMarks, &scoredMarks)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to calculate marks"})
	}

	finishedTime := time.Now()
	currentQuestionNum := 0

	// Update session
	_, err = util.DB.Exec(`
	UPDATE test_sessions
	SET finished = true,
	    finished_time = $1,
	    total_marks = $2,
	    scored_marks = $3,
	    current_question_num = $4
	WHERE id = $5
`, finishedTime, totalMarks, scoredMarks, currentQuestionNum, testSessionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to finish test session: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":          "success",
		"started_time":    startedTime,
		"finished_time":   finishedTime,
		"total_marks":     totalMarks,
		"scored_marks":    scoredMarks,
		"test_session_id": testSessionID,
		"finished_status": true,
	})
}
func GetTestHistory(c *fiber.Ctx) error {
	db := util.DB // *sql.DB connection

	userID := c.Locals("user").(models.User).ID // assuming set as int

	// Pagination
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	offset := (page - 1) * limit

	// Filters
	subject := c.Query("subject", "")
	exam := c.Query("exam", "")
	date := c.Query("date", "") // format: YYYY-MM-DD

	// Base query
	query := `
	SELECT ts.id,ts.name,ts.finished, ts.started, ts.started_time, ts.finished_time,
	       ts.mode,ts.total_marks, ts.scored_marks, qs.subject, qs.exam, qs.language, qs.cover_image,ts.updated_time
	FROM test_sessions ts join question_sets qs on ts.question_set_id = qs.id 
	WHERE ts.taken_by_id = $1
	`
	args := []interface{}{userID}
	argIdx := 2

	// Add filters dynamically
	if subject != "" {
		query += fmt.Sprintf(" AND qs.subject ILIKE $%d", argIdx)
		args = append(args, "%"+subject+"%")
		argIdx++
	}

	if exam != "" {
		query += fmt.Sprintf(" AND qs.exam ILIKE $%d", argIdx)
		args = append(args, "%"+exam+"%")
		argIdx++
	}

	if date != "" {
		query += fmt.Sprintf(" AND DATE(ts.started_time) = $%d", argIdx)
		args = append(args, date)
		argIdx++
	}

	// Add pagination
	query += fmt.Sprintf(" ORDER BY ts.started_time DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	// Execute query
	rows, err := db.Query(query, args...)
	if err != nil {
		log.Println("Query error:", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch test history"})
	}
	defer rows.Close()

	// Result structure
	type TestHistory struct {
		ID           uuid.UUID `json:"id"`
		Name         string    `json:"qSetName"`
		Finished     bool      `json:"finished"`
		Started      bool      `json:"started"`
		StartedTime  time.Time `json:"startedTime"`
		FinishedTime time.Time `json:"finishedTime"`
		Mode         string    `json:"mode"`
		TotalMarks   float64   `json:"totalMarks"`
		ScoredMarks  float64   `json:"scoredMarks"`
		Subject      string    `json:"subject"`
		Exam         string    `json:"exam"`
		Language     string    `json:"language"`
		CoverImage   string    `json:"coverImage"`
		UpdatedTime  time.Time `json:"updatedTime"`
	}

	history := []TestHistory{}
	for rows.Next() {
		var h TestHistory
		if err := rows.Scan(
			&h.ID, &h.Name, &h.Finished, &h.Started, &h.StartedTime, &h.FinishedTime, &h.Mode,
			&h.TotalMarks, &h.ScoredMarks, &h.Subject, &h.Exam, &h.Language, &h.CoverImage, &h.UpdatedTime,
		); err != nil {
			log.Println("Row scan error:", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to scan test history"})
		}
		history = append(history, h)
	}

	return c.JSON(fiber.Map{
		"page":    page,
		"limit":   limit,
		"history": history,
		"count":   len(history),
		"hasMore": len(history) == limit,
	})
}
