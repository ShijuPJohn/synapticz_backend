package controllers

import (
	"database/sql"
	"fmt"
	"github.com/ShijuPJohn/synapticz_backend/models"
	"github.com/ShijuPJohn/synapticz_backend/util"
	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
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

	for _, qid := range questionIDs {

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
			1.0,               // total mark per question
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
	fmt.Println(session.ScoredMarks)
	fmt.Println(session.TotalMarks)
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
		TotalMarksScored     float64                `json:"total_marks_scored"`
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

	// Validate ownership
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

	// Begin updating question-level data
	tx, err := util.DB.Begin()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Transaction begin failed"})
	}
	defer tx.Rollback()

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

		//totalMark := answer["questions_total_mark"].(float64)
		scoredMark := answer["questions_scored_mark"].(float64)
		answered := answer["answered"].(bool)

		_, err = tx.Exec(`
			UPDATE test_session_question_answers
			SET selected_answer_list = $1,
			    questions_scored_mark = $2,
			    answered = $3
			WHERE test_session_id = $4 AND question_id = $5
		`, pq.Array(selectedList), scoredMark, answered, testSessionID, qid)

		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Failed to update answer for question %d: %v", qid, err),
			})
		}
	}

	// Update test session current index and score
	_, err = tx.Exec(`
		UPDATE test_sessions
		SET current_question_num = $1, scored_marks = $2
		WHERE id = $3
	`, dto.CurrentQuestionIndex, dto.TotalMarksScored, testSessionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update test session"})
	}

	if err := tx.Commit(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Transaction commit failed"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":               "success",
		"current_question_num": dto.CurrentQuestionIndex,
		"scored_marks":         dto.TotalMarksScored,
	})
}
