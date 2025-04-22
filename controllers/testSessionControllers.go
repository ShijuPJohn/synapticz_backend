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

	// Verify ownership
	var takenByID int
	var finished bool
	var questionSetID int
	err := util.DB.QueryRow(
		`SELECT taken_by_id, finished, question_set_id 
         FROM test_sessions 
         WHERE id = $1`, testSessionID).Scan(&takenByID, &finished, &questionSetID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Test session not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch session"})
	}
	if takenByID != user.ID {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Get test session details
	var session models.TestSession
	err = util.DB.QueryRow(
		`SELECT id, finished, started, name, question_set_id, taken_by_id,
                n_total_questions, current_question_num, n_correctly_answered,
                rank, total_marks, scored_marks, started_time, finished_time, mode
         FROM test_sessions
         WHERE id = $1`, testSessionID).Scan(
		&session.ID, &session.Finished, &session.Started, &session.Name, &session.QuestionSetID,
		&session.TakenByID, &session.NTotalQuestions, &session.CurrentQuestionNum,
		&session.NCorrectlyAnswered, &session.Rank, &session.TotalMarks, &session.ScoredMarks,
		&session.StartedTime, &session.FinishedTime, &session.Mode)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch test session " + err.Error()})
	}

	// Get question set details
	var questionSet struct {
		Name        string
		Description string
		CoverImage  *string
		Subject     string
	}
	err = util.DB.QueryRow(
		`SELECT name, description, cover_image, subject 
         FROM question_sets 
         WHERE id = $1`, questionSetID).Scan(
		&questionSet.Name, &questionSet.Description, &questionSet.CoverImage, &questionSet.Subject)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch question set"})
	}

	// Get test statistics (only for finished tests)
	var testStats map[string]interface{}
	if finished {
		// Get basic test results
		var result struct {
			TotalAnswered int
			Correct       int
			Wrong         int
			Unanswered    int
		}

		err = util.DB.QueryRow(
			`SELECT 
                COUNT(CASE WHEN answered THEN 1 END) as total_answered,
                COUNT(CASE WHEN questions_scored_mark > 0 THEN 1 END) as correct,
                COUNT(CASE WHEN answered AND questions_scored_mark = 0 THEN 1 END) as wrong,
                COUNT(CASE WHEN NOT answered THEN 1 END) as unanswered
             FROM test_session_question_answers
             WHERE test_session_id = $1`, testSessionID).Scan(
			&result.TotalAnswered, &result.Correct, &result.Wrong, &result.Unanswered)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to calculate test stats"})
		}

		// Get historical stats
		var history struct {
			Attempts int
			AvgScore float64
			TopScore float64
		}
		err = util.DB.QueryRow(
			`SELECT 
                COUNT(*) as attempts,
                COALESCE(AVG(scored_marks), 0) as avg_score,
                COALESCE(MAX(scored_marks), 0) as top_score
             FROM test_sessions 
             WHERE question_set_id = $1 AND finished = true`, questionSetID).Scan(
			&history.Attempts, &history.AvgScore, &history.TopScore)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch historical stats"})
		}
		var allScores []string
		rows, err := util.DB.Query(
			`SELECT scored_marks FROM test_sessions 
             WHERE question_set_id = $1 AND finished = true 
             ORDER BY scored_marks`, questionSetID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to fetch all test scores",
			})
		}
		defer rows.Close()

		for rows.Next() {
			var score float64
			if err := rows.Scan(&score); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to scan test score",
				})
			}
			allScores = append(allScores, fmt.Sprintf("%.2f", score))
		}
		testStats = map[string]interface{}{
			"questions_answered":     result.TotalAnswered,
			"correct_answers":        result.Correct,
			"wrong_answers":          result.Wrong,
			"unanswered":             result.Unanswered,
			"percentage":             (session.ScoredMarks / session.TotalMarks) * 100,
			"total_attempts":         history.Attempts,
			"average_score":          history.AvgScore,
			"top_score":              history.TopScore,
			"all_test_takers_scores": allScores,
		}
	}

	// Get all questions with answers
	rows, err := util.DB.Query(
		`SELECT 
            q.id, q.question, q.question_type, q.options, q.correct_options, q.explanation,
            tsqa.selected_answer_list, tsqa.questions_total_mark, tsqa.questions_scored_mark, tsqa.answered
         FROM test_session_question_answers tsqa
         JOIN questions q ON tsqa.question_id = q.id
         WHERE tsqa.test_session_id = $1
         ORDER BY tsqa.question_id`, testSessionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch questions"})
	}
	defer rows.Close()

	var questions []map[string]interface{}
	for rows.Next() {
		var (
			id             int
			question       string
			questionType   string
			options        []string
			correctOptions pq.Int64Array
			explanation    *string
			selectedAns    pq.Int64Array
			totalMark      float64
			scoredMark     float64
			answered       bool
		)

		err := rows.Scan(
			&id, &question, &questionType, pq.Array(&options), &correctOptions, &explanation,
			&selectedAns, &totalMark, &scoredMark, &answered,
		)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to scan question"})
		}

		questions = append(questions, map[string]interface{}{
			"id":                    id,
			"question":              question,
			"question_type":         questionType,
			"options":               options,
			"correct_options":       convertToIntSlice(correctOptions),
			"explanation":           explanation,
			"selected_answer_list":  convertToIntSlice(selectedAns),
			"questions_total_mark":  totalMark,
			"questions_scored_mark": scoredMark,
			"answered":              answered,
			"is_correct":            scoredMark > 0,
		})
	}

	response := fiber.Map{
		"status": "success",
		"test_session": fiber.Map{
			"id":                   session.ID,
			"name":                 session.Name,
			"mode":                 session.Mode,
			"finished":             session.Finished,
			"started_time":         session.StartedTime,
			"finished_time":        session.FinishedTime,
			"total_marks":          session.TotalMarks,
			"scored_marks":         session.ScoredMarks,
			"current_question_num": session.CurrentQuestionNum,
			"rank":                 session.Rank,
		},
		"question_set": fiber.Map{
			"id":          questionSetID,
			"name":        questionSet.Name,
			"description": questionSet.Description,
			"cover_image": questionSet.CoverImage,
			"subject":     questionSet.Subject,
		},
		"questions": questions,
	}

	if finished {
		response["test_stats"] = testStats
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

// Helper function to convert pq.Int64Array to []int
func convertToIntSlice(arr pq.Int64Array) []int {
	res := make([]int, len(arr))
	for i, v := range arr {
		res[i] = int(v)
	}
	return res
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
	var questionSetID int
	err := util.DB.QueryRow(
		`SELECT taken_by_id, finished, question_set_id 
         FROM test_sessions 
         WHERE id = $1`, testSessionID).Scan(&takenByID, &finished, &questionSetID)
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

	// Check daily question limit for non-premium users
	if !user.IsPremium {
		var answeredToday int
		err := tx.QueryRow(
			`SELECT COUNT(*) 
			 FROM user_daily_questions 
			 WHERE user_id = $1 AND answered_at::date = CURRENT_DATE`, user.ID).Scan(&answeredToday)

		if err != nil && err != sql.ErrNoRows {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to check daily activity " + err.Error(),
			})
		}

		var questionLimit int
		err = tx.QueryRow(
			`SELECT questions_limit 
             FROM user_daily_activity 
             WHERE user_id = $1 AND activity_date = CURRENT_DATE`, user.ID).Scan(&questionLimit)
		if err != nil {
			if err == sql.ErrNoRows {
				questionLimit = 20
			} else {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to get question limit",
				})
			}
		}

		newAnswers := 0
		for _, val := range dto.QuestionAnswerData {
			answer, ok := val.(map[string]interface{})
			if !ok {
				continue
			}
			if answered, ok := answer["answered"].(bool); ok && answered {
				newAnswers++
			}
		}

		if answeredToday+newAnswers > questionLimit {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": fmt.Sprintf("Daily question limit reached (%d/%d)", answeredToday, questionLimit),
			})
		}
	}

	var totalScored float64
	var totalMarks float64
	var newlyAnsweredQuestions []int

	for qidStr, val := range dto.QuestionAnswerData {
		qid, err := strconv.Atoi(qidStr)
		if err != nil {
			continue
		}
		answer, ok := val.(map[string]interface{})
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Invalid answer format for question ID: %s", qidStr),
			})
		}

		selectedListRaw, ok := answer["selected_answer_list"].([]interface{})
		if !ok {
			selectedListRaw = []interface{}{}
		}

		correctListRaw, ok := answer["correct_options"].([]interface{})
		if !ok {
			correctListRaw = []interface{}{}
		}

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

		totalMarks += totalMark

		var previouslyAnswered bool
		err = tx.QueryRow(
			`SELECT answered 
			 FROM test_session_question_answers 
			 WHERE test_session_id = $1 AND question_id = $2`,
			testSessionID, qid).Scan(&previouslyAnswered)
		if err != nil && err != sql.ErrNoRows {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Failed to check previous answer status for question %d", qid),
			})
		}

		if answered && !previouslyAnswered {
			newlyAnsweredQuestions = append(newlyAnsweredQuestions, qid)
		}

		var qType string
		err = tx.QueryRow(`SELECT question_type FROM questions WHERE id = $1`, qid).Scan(&qType)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Failed to get question type for question %d: %v", qid, err),
			})
		}

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

	_, err = tx.Exec(`
		UPDATE test_sessions
		SET current_question_num = $1,
		    scored_marks = $2,
		    total_marks = $3,
		    updated_time = CURRENT_TIMESTAMP
		WHERE id = $4
	`, dto.CurrentQuestionIndex, totalScored, totalMarks, testSessionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update test session"})
	}

	// Log individual question entries (no activity_date)
	if len(newlyAnsweredQuestions) > 0 {
		for _, qid := range newlyAnsweredQuestions {
			_, err = tx.Exec(`
				INSERT INTO user_daily_questions (user_id, question_id)
				VALUES ($1, $2)
				ON CONFLICT DO NOTHING
			`, user.ID, qid)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to record answered question : " + err.Error(),
				})
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Transaction commit failed"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":               "success",
		"current_question_num": dto.CurrentQuestionIndex,
		"scored_marks":         totalScored,
		"total_marks":          totalMarks,
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

	// Verify test session ownership and status
	var takenByID int
	var finished bool
	var startedTime time.Time
	var questionSetID int
	var sessionName string
	var sessionMode string
	err := util.DB.QueryRow(
		`SELECT taken_by_id, finished, started_time, question_set_id, name, mode 
         FROM test_sessions 
         WHERE id = $1`, testSessionID).Scan(&takenByID, &finished, &startedTime, &questionSetID, &sessionName, &sessionMode)

	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Test session not found " + err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch test session " + err.Error()})
	}

	if takenByID != user.ID {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}
	if finished {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Test session is already finished"})
	}

	// Calculate test statistics in a single transaction
	tx, err := util.DB.Begin()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start transaction"})
	}
	defer tx.Rollback()

	// Get basic test results
	var testResult struct {
		TotalMarks    float64
		ScoredMarks   float64
		TotalAnswered int
		Correct       int
		Wrong         int
		Unanswered    int
	}

	err = tx.QueryRow(
		`SELECT 
            COALESCE(SUM(questions_total_mark), 0) as total_marks,
            COALESCE(SUM(questions_scored_mark), 0) as scored_marks,
            COUNT(CASE WHEN answered THEN 1 END) as total_answered,
            COUNT(CASE WHEN questions_scored_mark > 0 THEN 1 END) as correct,
            COUNT(CASE WHEN answered AND questions_scored_mark = 0 THEN 1 END) as wrong,
            COUNT(CASE WHEN NOT answered THEN 1 END) as unanswered
         FROM test_session_question_answers
         WHERE test_session_id = $1`, testSessionID).Scan(
		&testResult.TotalMarks, &testResult.ScoredMarks, &testResult.TotalAnswered,
		&testResult.Correct, &testResult.Wrong, &testResult.Unanswered)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to calculate test results"})
	}

	// Get question set details
	var questionSet struct {
		Name        string
		Description string
		CoverImage  *string
		Subject     string
		TotalQs     int
	}
	err = tx.QueryRow(
		`SELECT qs.name, qs.description, qs.cover_image, qs.subject,
            (SELECT COUNT(*) FROM question_set_questions WHERE question_set_id = qs.id) as total_questions
         FROM question_sets qs
         WHERE qs.id = $1`, questionSetID).Scan(
		&questionSet.Name, &questionSet.Description, &questionSet.CoverImage,
		&questionSet.Subject, &questionSet.TotalQs)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch question set details"})
	}

	// First mark the test session as finished
	finishedTime := time.Now().UTC()
	_, err = tx.Exec(
		`UPDATE test_sessions
         SET finished = true,
             finished_time = $1,
             total_marks = $2,
             scored_marks = $3,
             current_question_num = 0
         WHERE id = $4`,
		finishedTime, testResult.TotalMarks, testResult.ScoredMarks, testSessionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to finish test session"})
	}

	// Now get historical stats (including this test session)
	var stats struct {
		Attempts int
		AvgScore float64
		UserRank int
		TopScore float64
	}

	err = tx.QueryRow(
		`SELECT 
            COUNT(*) as attempts,
            COALESCE(AVG(scored_marks), 0) as avg_score,
            (SELECT COUNT(*) FROM test_sessions 
             WHERE question_set_id = $1 AND finished = true AND scored_marks > $2) + 1 as user_rank,
            COALESCE(MAX(scored_marks), 0) as top_score
         FROM test_sessions 
         WHERE question_set_id = $1 AND finished = true`, questionSetID, testResult.ScoredMarks).Scan(
		&stats.Attempts, &stats.AvgScore, &stats.UserRank, &stats.TopScore)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch historical stats"})
	}

	// Update the rank now that we've calculated it
	_, err = tx.Exec(
		`UPDATE test_sessions
         SET rank = $1
         WHERE id = $2`,
		stats.UserRank, testSessionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update test session rank"})
	}

	// Get all questions with answers for response
	rows, err := tx.Query(
		`SELECT 
            q.id, q.question, q.question_type, q.options, q.correct_options, q.explanation,
            tsqa.selected_answer_list, tsqa.questions_total_mark, tsqa.questions_scored_mark, tsqa.answered
         FROM test_session_question_answers tsqa
         JOIN questions q ON tsqa.question_id = q.id
         WHERE tsqa.test_session_id = $1
         ORDER BY tsqa.question_id`, testSessionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch questions"})
	}
	defer rows.Close()

	var questions []map[string]interface{}
	for rows.Next() {
		var (
			id             int
			question       string
			questionType   string
			options        []string
			correctOptions pq.Int64Array
			explanation    *string
			selectedAns    pq.Int64Array
			totalMark      float64
			scoredMark     float64
			answered       bool
		)

		err := rows.Scan(
			&id, &question, &questionType, pq.Array(&options), &correctOptions, &explanation,
			&selectedAns, &totalMark, &scoredMark, &answered,
		)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to scan question"})
		}

		questions = append(questions, map[string]interface{}{
			"id":                    id,
			"question":              question,
			"question_type":         questionType,
			"options":               options,
			"correct_options":       convertToIntSlice(correctOptions),
			"explanation":           explanation,
			"selected_answer_list":  convertToIntSlice(selectedAns),
			"questions_total_mark":  totalMark,
			"questions_scored_mark": scoredMark,
			"answered":              answered,
			"is_correct":            scoredMark > 0,
		})
	}

	// Get all scores for percentile calculation
	var allScores []float64
	rows, err = tx.Query(
		`SELECT scored_marks FROM test_sessions 
         WHERE question_set_id = $1 AND finished = true 
         ORDER BY scored_marks`, questionSetID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch all test scores",
		})
	}
	defer rows.Close()

	for rows.Next() {
		var score float64
		if err := rows.Scan(&score); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to scan test score",
			})
		}
		allScores = append(allScores, score)
	}

	// Calculate test statistics for response
	testStats := map[string]interface{}{
		"questions_answered":     testResult.TotalAnswered,
		"correct_answers":        testResult.Correct,
		"wrong_answers":          testResult.Wrong,
		"unanswered":             testResult.Unanswered,
		"percentage":             (testResult.ScoredMarks / testResult.TotalMarks) * 100,
		"total_attempts":         stats.Attempts,
		"average_score":          stats.AvgScore,
		"top_score":              stats.TopScore,
		"all_test_takers_scores": allScores,
	}

	if err = tx.Commit(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to commit transaction"})
	}

	// Return response matching GetTestSession format
	response := fiber.Map{
		"status": "success",
		"test_session": fiber.Map{
			"id":                   testSessionID,
			"name":                 sessionName,
			"mode":                 sessionMode,
			"finished":             true,
			"started_time":         startedTime,
			"finished_time":        finishedTime,
			"total_marks":          testResult.TotalMarks,
			"scored_marks":         testResult.ScoredMarks,
			"current_question_num": 0, // Reset to 0 for finished tests
			"rank":                 stats.UserRank,
		},
		"question_set": fiber.Map{
			"id":          questionSetID,
			"name":        questionSet.Name,
			"description": questionSet.Description,
			"cover_image": questionSet.CoverImage,
			"subject":     questionSet.Subject,
		},
		"questions":  questions,
		"test_stats": testStats,
	}

	return c.Status(fiber.StatusOK).JSON(response)
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
