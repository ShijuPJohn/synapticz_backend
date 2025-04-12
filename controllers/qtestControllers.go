package controllers

import (
	"database/sql"
	"github.com/ShijuPJohn/synapticz_backend/models"
	"github.com/ShijuPJohn/synapticz_backend/util"
	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
	"math/rand"
	"time"
)

func CreateQTest(c *fiber.Ctx) error {
	type QTestRequest struct {
		QuestionSetID      int    `json:"question_set_id"`
		TestMode           string `json:"test_mode"`
		RandomizeQuestions bool   `json:"randomize_questions"`
	}

	var req QTestRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	// Validate test mode
	validModes := map[string]bool{"practice": true, "exam": true, "timed-practice": true}
	if !validModes[req.TestMode] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid test mode"})
	}

	// Get user ID from JWT
	userID := c.Locals("user").(models.User).ID

	// Fetch question set
	var qsName string
	err := util.DB.QueryRow(`
		SELECT name FROM question_sets WHERE id = $1
	`, req.QuestionSetID).Scan(&qsName)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "question set not found"})
	}

	// Fetch questions in the set
	rows, err := util.DB.Query(`
		SELECT question_id FROM question_set_questions WHERE question_set_id = $1
	`, req.QuestionSetID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error fetching questions"})
	}
	defer rows.Close()

	var questionIDs []int
	for rows.Next() {
		var qid int
		if err := rows.Scan(&qid); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error reading questions"})
		}
		questionIDs = append(questionIDs, qid)
	}

	if len(questionIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no questions in question set"})
	}

	// Randomize if required
	if req.RandomizeQuestions {
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(questionIDs), func(i, j int) {
			questionIDs[i], questionIDs[j] = questionIDs[j], questionIDs[i]
		})
	}

	// Create new qtest
	var qtestID string
	err = util.DB.QueryRow(`
		INSERT INTO qtests (name, question_set_id, taken_by_id, n_total_questions, current_question_num, mode)
		VALUES ($1, $2, $3, $4, 0, $5)
		RETURNING id
	`, qsName, req.QuestionSetID, userID, len(questionIDs), req.TestMode).Scan(&qtestID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error creating qtest"})
	}

	// Insert into qtest_questions
	tx, err := util.DB.Begin()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "transaction begin failed"})
	}
	stmt, err := tx.Prepare("INSERT INTO qtest_questions (qtest_id, question_id) VALUES ($1, $2)")
	if err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "prep failed"})
	}
	defer stmt.Close()

	for _, qid := range questionIDs {
		if _, err := stmt.Exec(qtestID, qid); err != nil {
			tx.Rollback()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "insert qtest_questions failed"})
		}
	}

	if err := tx.Commit(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "commit failed"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":       "success",
		"qtest_id":     qtestID,
		"questions":    questionIDs,
		"randomized":   req.RandomizeQuestions,
		"question_set": qsName,
	})
}

func QTestAnswerQuestion(c *fiber.Ctx) error {
	type AnswerRequest struct {
		QuestionID        int `json:"qid"`
		SelectedAnswerIdx int `json:"selectedAnswer" validate:"required"`
	}

	var req AnswerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request",
		})
	}

	qtestID := c.Params("id")
	userID := c.Locals("user").(models.User).ID

	// Validate qtest ownership
	var dbUserID int
	err := util.DB.QueryRow(`
		SELECT taken_by_id FROM qtests WHERE id = $1
	`, qtestID).Scan(&dbUserID)
	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "test not found"})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error fetching test"})
	}
	if dbUserID != userID {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	// Update selected answer in qtest_questions
	result, err := util.DB.Exec(`
		UPDATE qtest_questions 
		SET selected_option_index = $1
		WHERE qtest_id = $2 AND question_id = $3
	`, req.SelectedAnswerIdx, qtestID, req.QuestionID)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not update answer"})
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "question not found in this test"})
	}

	// Optional: Fetch correct answer if you want to send it back
	var correctOptionIndex int
	err = util.DB.QueryRow(`
		SELECT correct_option_index FROM questions WHERE id = $1
	`, req.QuestionID).Scan(&correctOptionIndex)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not fetch correct answer"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":          "success",
		"question_id":     req.QuestionID,
		"selected_answer": req.SelectedAnswerIdx,
		"correct_answer":  correctOptionIndex,
	})
}

func QTestNextQuestion(c *fiber.Ctx) error {
	qtestID := c.Params("id")

	// Get user ID from JWT
	userID := c.Locals("user").(models.User).ID

	// Fetch qtest and validate ownership
	var currentQNum int
	var totalQuestions int
	var takenByID int
	var finished bool

	err := util.DB.QueryRow(`
		SELECT current_question_num, n_total_questions, taken_by_id, finished
		FROM qtests
		WHERE id = $1
	`, qtestID).Scan(&currentQNum, &totalQuestions, &takenByID, &finished)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "qtest not found"})
	}
	if takenByID != userID {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	// Check if test is already finished
	if finished || currentQNum >= totalQuestions-1 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Reached end or test already finished"})
	}

	// Increment question number
	nextQNum := currentQNum + 1

	// Fetch next question ID from qtest_questions table
	var questionID int
	err = util.DB.QueryRow(`
		SELECT question_id FROM qtest_questions
		WHERE qtest_id = $1
		ORDER BY id ASC
		OFFSET $2 LIMIT 1
	`, qtestID, nextQNum).Scan(&questionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error fetching next question"})
	}

	// Fetch the full question
	var question models.Question
	err = util.DB.QueryRow(`
		SELECT id, question, options, correct_option, explanation, difficulty, question_type
		FROM questions WHERE id = $1
	`, questionID).Scan(
		&question.ID,
		&question.Question,
		pq.Array(&question.Options),
		&question.CorrectOptions,
		&question.Explanation,
		&question.Difficulty,
		&question.QuestionType,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error fetching question"})
	}

	// Optional: check if this question has already been answered
	var answered bool
	err = util.DB.QueryRow(`
		SELECT selected_option IS NOT NULL
		FROM qtest_questions
		WHERE qtest_id = $1 AND question_id = $2
	`, qtestID, questionID).Scan(&answered)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error checking answer status"})
	}

	// Update current_question_num
	_, err = util.DB.Exec(`
		UPDATE qtests SET current_question_num = $1 WHERE id = $2
	`, nextQNum, qtestID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update current question"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":           "success",
		"currentQNo":       nextQNum,
		"totalNoQuestions": totalQuestions,
		"currentQuestion":  question,
		"answered":         answered,
		"finished":         finished,
	})
}

func QTestPrevQuestion(c *fiber.Ctx) error {
	qtestID := c.Params("id")

	// Get user ID from JWT (assuming user struct stored in context)
	userID := c.Locals("user").(models.User).ID

	// Fetch current question num and taken_by_id to validate ownership
	var currentQNum int
	var totalQuestions int
	var takenByID int
	var finished bool

	err := util.DB.QueryRow(`
		SELECT current_question_num, n_total_questions, taken_by_id, finished
		FROM qtests
		WHERE id = $1
	`, qtestID).Scan(&currentQNum, &totalQuestions, &takenByID, &finished)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "qtest not found"})
	}
	if takenByID != userID {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	if currentQNum <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Already at first question"})
	}

	prevQNum := currentQNum - 1

	// Get previous question ID
	var questionID int
	err = util.DB.QueryRow(`
		SELECT question_id FROM qtest_questions
		WHERE qtest_id = $1
		ORDER BY id ASC
		OFFSET $2 LIMIT 1
	`, qtestID, prevQNum).Scan(&questionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error fetching previous question"})
	}

	// Fetch question details
	var question models.Question
	err = util.DB.QueryRow(`
		SELECT id, question, options, correct_option, explanation, difficulty, question_type
		FROM questions
		WHERE id = $1
	`, questionID).Scan(
		&question.ID,
		&question.Question,
		pq.Array(&question.Options),
		&question.CorrectOptions,
		&question.Explanation,
		&question.Difficulty,
		&question.QuestionType,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error fetching question"})
	}

	// Check if this question is already answered
	var answered bool
	err = util.DB.QueryRow(`
		SELECT selected_option IS NOT NULL
		FROM qtest_questions
		WHERE qtest_id = $1 AND question_id = $2
	`, qtestID, questionID).Scan(&answered)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error checking answer status"})
	}

	// Update current question num in qtest
	_, err = util.DB.Exec(`
		UPDATE qtests SET current_question_num = $1 WHERE id = $2
	`, prevQNum, qtestID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update question number"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":           "success",
		"currentQNo":       prevQNum,
		"totalNoQuestions": totalQuestions,
		"currentQuestion":  question,
		"answered":         answered,
		"finished":         finished,
	})
}

func QTestCurrent(c *fiber.Ctx) error {
	qtestID := c.Params("id")
	userID := c.Locals("user").(models.User).ID

	// Fetch qtest with user validation
	var currentQNum, totalQuestions, takenByID int
	var finished, started bool
	err := util.DB.QueryRow(`
		SELECT current_question_num, n_total_questions, taken_by_id, finished, started
		FROM qtests
		WHERE id = $1
	`, qtestID).Scan(&currentQNum, &totalQuestions, &takenByID, &finished, &started)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "qtest not found"})
	}
	if userID != takenByID {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	// Fetch current question id from qtest_questions using offset
	var questionID int
	err = util.DB.QueryRow(`
		SELECT question_id FROM qtest_questions
		WHERE qtest_id = $1
		ORDER BY id ASC
		OFFSET $2 LIMIT 1
	`, qtestID, currentQNum).Scan(&questionID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error fetching current question"})
	}

	// Fetch question details
	var question models.Question
	err = util.DB.QueryRow(`
		SELECT id, question, options, correct_option, explanation, difficulty, question_type
		FROM questions
		WHERE id = $1
	`, questionID).Scan(
		&question.ID,
		&question.Question,
		pq.Array(&question.Options),
		&question.CorrectOptions,
		&question.Explanation,
		&question.Difficulty,
		&question.QuestionType,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error loading question"})
	}

	// Check if answered
	var answered bool
	err = util.DB.QueryRow(`
		SELECT selected_option IS NOT NULL
		FROM qtest_questions
		WHERE qtest_id = $1 AND question_id = $2
	`, qtestID, questionID).Scan(&answered)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error checking answer status"})
	}

	// Set qtest.started = true if not already started
	if !started {
		_, err = util.DB.Exec(`UPDATE qtests SET started = true WHERE id = $1`, qtestID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update qtest started status"})
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":           "success",
		"qTestId":          qtestID,
		"currentQNo":       currentQNum,
		"totalNoQuestions": totalQuestions,
		"currentQuestion":  question,
		"answered":         answered,
		"finished":         finished,
	})
}

//
////func GetQTestByID(c *fiber.Ctx) error {
////	qTestId := c.Params("id")
////	idObject, err := primitive.ObjectIDFromHex(qTestId)
////	if err != nil {
////		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": err.Error()})
////	}
////	qTest := new(models.QTest)
////	err = utils.Mg.Db.Collection("q_test").FindOne(c.Context(), bson.M{"_id": idObject}).Decode(&qTest)
////	if err != nil {
////		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": err.Error()})
////	}
////	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "qTest": qTest})
////}
////func TakeTest(c *fiber.Ctx) error {
////	qTestId := c.Params("id")
////	type Temp struct {
////		Question string `json:"question" validate:"required"`
////		Answer   string `json:"answer" validate:"required"`
////	}
////
////	t := new(Temp)
////	if err := c.BodyParser(&t); err != nil {
////		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
////			"status":  "error",
////			"message": "Bad Request",
////			"error":   err.Error(),
////		})
////	}
////	selectedAnswer, err := strconv.Atoi(t.Answer)
////	idObject, err := primitive.ObjectIDFromHex(qTestId)
////	if err != nil {
////		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": err.Error()})
////	}
////	qTest := new(models.QTest)
////	qIdObject, err := primitive.ObjectIDFromHex(t.Question)
////
////	if err != nil {
////		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": err.Error()})
////	}
////	question := new(models.Question)
////	err = utils.Mg.Db.Collection("q_test").FindOne(c.Context(), bson.M{"_id": idObject}).Decode(&qTest)
////	if err != nil {
////		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": err.Error()})
////	}
////	if qTest.Finished {
////		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": "Test already finished"})
////	}
////	err = utils.Mg.Db.Collection("questions").FindOne(c.Context(), bson.M{"_id": qIdObject}).Decode(&question)
////	if err != nil {
////		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": err.Error()})
////	}
////	var answerSlice []int
////	answerSlice = append(answerSlice, selectedAnswer)
////	answerSlice = append(answerSlice, question.CorrectOptions)
////	user := c.Locals("user").(*jwt.Token)
////	claims := user.Claims.(jwt.MapClaims)
////	if qTest.TakenById != claims["id"].(string) {
////		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "unauthorized"})
////	}
////
////	qTest.AllQuestionsIDs[t.Question] = answerSlice
////
////	update := bson.M{"allQuestionsId": qTest.AllQuestionsIDs}
////	filter := bson.M{"_id": idObject}
////	if selectedAnswer == question.CorrectOptions {
////		update["nCorrectlyAnswered"] = qTest.NCorrectlyAnswered + 1
////	}
////	updateQuery := bson.M{"$set": update}
////	result, err := utils.Mg.Db.Collection("q_test").UpdateOne(c.Context(), filter, updateQuery)
////	if err != nil {
////		return handleError(c, fiber.StatusBadRequest, err.Error())
////	}
////	if qTest.Mode != "exam" {
////		totalScoreSoFar := 0
////		for question := range qTest.AllQuestionsIDs {
////			if qTest.AllQuestionsIDs[question][0] == qTest.AllQuestionsIDs[question][1] && qTest.AllQuestionsIDs[question][0] != 0 {
////				totalScoreSoFar++
////			}
////		}
////		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "result": result, "totalScoreSoFar": totalScoreSoFar})
////
////	}
////
////	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "result": result})
////
////}
//
////	func finishQTest(c *fiber.Ctx) error {
////		qTestId := c.Params("id")
////		qTest := new(models.QTest)
////		qTestIdObject,err := primitive.ObjectIDFromHex(qTestId)
////		if err != nil {
////			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": err.Error()})
////		}
////	}
//func shuffleQuestionIDs(input *[]string) []string {
//	rand.Seed(time.Now().UnixNano()) // Seed the random number generator
//
//	for i := range *input {
//		j := rand.Intn(i + 1)
//		(*input)[i], (*input)[j] = (*input)[j], (*input)[i]
//	}
//	return *input
//}
