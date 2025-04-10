package controllers

//
//import (
//	"github.com/gofiber/fiber/v2"
//	"github.com/golang-jwt/jwt/v4"
//	"go.mongodb.org/mongo-driver/bson"
//	"go.mongodb.org/mongo-driver/bson/primitive"
//	"math/rand"
//	"neocognito-backend/models"
//	"neocognito-backend/utils"
//	"time"
//)
//
//func CreateQTest(c *fiber.Ctx) error {
//	type Temp struct {
//		QuestionSet        string `json:"question_set" validate:"required"`
//		TestMode           string `json:"test_mode" `
//		RandomizeQuestions bool   `json:"randomize_questions" validate:"required"`
//	}
//	t := new(Temp)
//	if err := c.BodyParser(&t); err != nil {
//		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
//			"status":  "error",
//			"message": "Bad Request",
//		})
//	}
//	idObject, err := primitive.ObjectIDFromHex(t.QuestionSet)
//	if err != nil {
//		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "error": err.Error()})
//	}
//	questionSetVar := new(models.QuestionSet)
//	err = utils.Mg.Db.Collection("question_set").FindOne(c.Context(), bson.M{"_id": idObject}).Decode(&questionSetVar)
//	qTest := new(models.QTest)
//	qTest.CurrentQuestionNum = 0
//	qTest.Name = questionSetVar.Name
//
//	qmap := map[string][]int{}
//
//	for _, qId := range questionSetVar.Questions {
//		qmap[qId] = []int{-1, -1}
//	}
//	if t.RandomizeQuestions {
//		qTest.QuestionIDsOrdered = shuffleQuestionIDs(&questionSetVar.Questions)
//	} else {
//		qTest.QuestionIDsOrdered = questionSetVar.Questions
//	}
//	qTest.AllQuestionsIDs = qmap
//	qTest.Mode = t.TestMode
//	qTest.Started = false
//	qTest.Finished = false
//	user := c.Locals("user").(*jwt.Token)
//	claims := user.Claims.(jwt.MapClaims)
//	qTest.TakenById = claims["id"].(string)
//	qTest.TakenAtTime = time.Now()
//	qTest.Tags = questionSetVar.Tags
//	qTest.QuestionSetId = questionSetVar.ID
//	insertionResult, err := utils.Mg.Db.Collection("q_test").InsertOne(c.Context(), qTest)
//	if err != nil {
//		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "error": err.Error()})
//	}
//	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
//		"status":    "success",
//		"id":        insertionResult.InsertedID,
//		"questions": qTest.QuestionIDsOrdered,
//	})
//}
//func QTestAnswerQuestion(c *fiber.Ctx) error {
//	qTestId := c.Params("id")
//	idObject, err := primitive.ObjectIDFromHex(qTestId)
//	if err != nil {
//		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": ""})
//	}
//	qTest := new(models.QTest)
//	err = utils.Mg.Db.Collection("q_test").FindOne(c.Context(), bson.M{"_id": idObject}).Decode(&qTest)
//	if err != nil {
//		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": err.Error()})
//	}
//	user := c.Locals("user").(*jwt.Token)
//	claims := user.Claims.(jwt.MapClaims)
//	userIdFromToken := claims["id"].(string)
//	if userIdFromToken != qTest.TakenById {
//		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "error": "unauthorized"})
//	}
//	if qTest.Finished || !qTest.Started {
//		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": "finished test"})
//	}
//	type Temp struct {
//		QId            string `json:"qid"`
//		SelectedAnswer int    `json:"selectedAnswer" validate:"required"`
//		CorrectAnswer  int    `json:"correctAnswer" `
//	}
//	t := new(Temp)
//	if err := c.BodyParser(&t); err != nil {
//		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
//			"status":  "error",
//			"message": "Bad Request",
//		})
//	}
//	filter := bson.M{"_id": idObject}
//	update := bson.M{"allQuestionsId." + t.QId: []int{t.CorrectAnswer, t.SelectedAnswer}}
//	updateQuery := bson.M{"$set": update}
//	result, err := utils.Mg.Db.Collection("q_test").UpdateOne(c.Context(), filter, updateQuery)
//	if err != nil {
//		return handleError(c, fiber.StatusBadRequest, err.Error())
//	}
//
//	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "result": result})
//}
//
//func QTestNextQuestion(c *fiber.Ctx) error {
//	qTestIdString := c.Params("id")
//	qTestIdHex, err := primitive.ObjectIDFromHex(qTestIdString)
//	if err != nil {
//		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": ""})
//	}
//	qTest := new(models.QTest)
//	err = utils.Mg.Db.Collection("q_test").FindOne(c.Context(), bson.M{"_id": qTestIdHex}).Decode(&qTest)
//	if err != nil {
//		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": err.Error()})
//	}
//
//	user := c.Locals("user").(*jwt.Token)
//	claims := user.Claims.(jwt.MapClaims)
//	userIdFromToken := claims["id"].(string)
//	if userIdFromToken != qTest.TakenById {
//		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "error": "unauthorized"})
//	}
//	question := new(models.Question)
//	currentQuestionNum :=
//		qTest.CurrentQuestionNum + 1
//	if qTest.CurrentQuestionNum >= (len(qTest.QuestionIDsOrdered) - 1) {
//		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Reached End"})
//	}
//	currentQuestionId := qTest.QuestionIDsOrdered[currentQuestionNum]
//	questionIdHex, err := primitive.ObjectIDFromHex(currentQuestionId)
//	if err != nil {
//		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": ""})
//	}
//	err = utils.Mg.Db.Collection("questions").FindOne(c.Context(), bson.M{"_id": questionIdHex}).Decode(&question)
//	if err != nil {
//		return handleError(c, fiber.StatusBadRequest, err.Error())
//	}
//	isCurrentQuestionAnswered := qTest.AllQuestionsIDs[currentQuestionId][1] != -1
//	filter := bson.M{"_id": qTestIdHex}
//	update := bson.M{"currentQuestionNum": currentQuestionNum}
//	//update["started"] = true
//	updateQuery := bson.M{"$set": update}
//	result, err := utils.Mg.Db.Collection("q_test").UpdateOne(c.Context(), filter, updateQuery)
//	if err != nil {
//		return handleError(c, fiber.StatusBadRequest, err.Error())
//	}
//	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success",
//		"currentQNo":       currentQuestionNum,
//		"totalNoQuestions": len(qTest.QuestionIDsOrdered),
//		"currentQuestion":  question,
//		"answered":         isCurrentQuestionAnswered,
//		"result":           result,
//		"finished":         qTest.Finished,
//	})
//}
//
//func QTestPrevQuestion(c *fiber.Ctx) error {
//	qTestIdString := c.Params("id")
//	qTestIdHex, err := primitive.ObjectIDFromHex(qTestIdString)
//	if err != nil {
//		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": ""})
//	}
//	qTest := new(models.QTest)
//	err = utils.Mg.Db.Collection("q_test").FindOne(c.Context(), bson.M{"_id": qTestIdHex}).Decode(&qTest)
//	if err != nil {
//		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": err.Error()})
//	}
//	user := c.Locals("user").(*jwt.Token)
//	claims := user.Claims.(jwt.MapClaims)
//	userIdFromToken := claims["id"].(string)
//	if userIdFromToken != qTest.TakenById {
//		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "error": "unauthorized"})
//	}
//	question := new(models.Question)
//
//	if qTest.CurrentQuestionNum <= 0 {
//		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Reached 0"})
//	}
//	currentQuestionNum :=
//		qTest.CurrentQuestionNum - 1
//	currentQuestionId := qTest.QuestionIDsOrdered[currentQuestionNum]
//	questionIdHex, err := primitive.ObjectIDFromHex(currentQuestionId)
//	if err != nil {
//		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": ""})
//	}
//	err = utils.Mg.Db.Collection("questions").FindOne(c.Context(), bson.M{"_id": questionIdHex}).Decode(&question)
//	if err != nil {
//		return handleError(c, fiber.StatusBadRequest, err.Error())
//	}
//	isCurrentQuestionAnswered := qTest.AllQuestionsIDs[currentQuestionId][1] != -1
//
//	filter := bson.M{"_id": qTestIdHex}
//	update := bson.M{"currentQuestionNum": currentQuestionNum}
//	//update["started"] = true
//	updateQuery := bson.M{"$set": update}
//	result, err := utils.Mg.Db.Collection("q_test").UpdateOne(c.Context(), filter, updateQuery)
//	if err != nil {
//		return handleError(c, fiber.StatusBadRequest, err.Error())
//	}
//	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success",
//		"currentQNo":       currentQuestionNum,
//		"totalNoQuestions": len(qTest.QuestionIDsOrdered),
//		"currentQuestion":  question, "answered": isCurrentQuestionAnswered,
//		"result":   result,
//		"finished": qTest.Finished,
//	})
//}
//
//func QTestCurrent(c *fiber.Ctx) error {
//	qTestIdString := c.Params("id")
//	qTestIdHex, err := primitive.ObjectIDFromHex(qTestIdString)
//	if err != nil {
//		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": ""})
//	}
//	qTest := new(models.QTest)
//	err = utils.Mg.Db.Collection("q_test").FindOne(c.Context(), bson.M{"_id": qTestIdHex}).Decode(&qTest)
//	if err != nil {
//		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": err.Error()})
//	}
//	user := c.Locals("user").(*jwt.Token)
//	claims := user.Claims.(jwt.MapClaims)
//	userIdFromToken := claims["id"].(string)
//	if userIdFromToken != qTest.TakenById {
//		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "error": "unauthorized"})
//	}
//	question := new(models.Question)
//	currentQuestionNum :=
//		qTest.CurrentQuestionNum
//	currentQuestionId := qTest.QuestionIDsOrdered[currentQuestionNum]
//	questionIdHex, err := primitive.ObjectIDFromHex(currentQuestionId)
//	if err != nil {
//		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": ""})
//	}
//	err = utils.Mg.Db.Collection("questions").FindOne(c.Context(), bson.M{"_id": questionIdHex}).Decode(&question)
//	if err != nil {
//		return handleError(c, fiber.StatusBadRequest, err.Error())
//	}
//	isCurrentQuestionAnswered := qTest.AllQuestionsIDs[currentQuestionId][1] != -1
//	filter := bson.M{"_id": qTestIdHex}
//	update := bson.M{"started": true}
//	updateQuery := bson.M{"$set": update}
//	result, err := utils.Mg.Db.Collection("q_test").UpdateOne(c.Context(), filter, updateQuery)
//	if err != nil {
//		return handleError(c, fiber.StatusBadRequest, err.Error())
//	}
//	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success",
//		"qTest":            qTest,
//		"currentQNo":       currentQuestionNum,
//		"totalNoQuestions": len(qTest.QuestionIDsOrdered),
//		"currentQuestion":  question,
//		"answered":         isCurrentQuestionAnswered, "result": result,
//		"finished": qTest.Finished,
//	})
//}
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
