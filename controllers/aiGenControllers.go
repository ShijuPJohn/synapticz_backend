package controllers

import (
	"bytes"
	"encoding/json"
	"github.com/ShijuPJohn/synapticz_backend/models"
	"github.com/ShijuPJohn/synapticz_backend/util"
	"github.com/gofiber/fiber/v2"
	"io/ioutil"
	"net/http"
	"os"
)

type QuizRequest struct {
	Prompt         string      `json:"prompt"`
	NOfQ           int         `json:"n_of_q"`
	QuestionFormat interface{} `json:"question_format"`
	QuizFormat     interface{} `json:"quiz_format"`
	Language       string      `json:"language"`
	Difficulty     int         `json:"difficulty"`
	QuestionType   string      `json:"question_type"`
}

func GenerateQuizFromPrompt(c *fiber.Ctx) error {
	// Parse request body
	var req struct {
		Prompt        string `json:"prompt"`
		Language      string `json:"language"`
		Difficulty    int    `json:"difficulty"`
		QuestionType  string `json:"question_type"`
		QuestionCount int    `json:"question_count"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}
	userFromToken := c.Locals("user").(models.User)
	userID := userFromToken.ID

	// Fetch user from DBqSetOwnerID
	var user models.User
	err := util.DB.QueryRow("SELECT id, role FROM users WHERE id = $1", userID).Scan(&user.ID, &user.Role)
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "User not found")
	}
	if !(user.Role == "admin" || user.Role == "owner") && (req.QuestionCount > 20) {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "A user can generate a maximum of 20 questions at once",
		})
	}

	var questionFormat string = `[
  {
    "question": "question statement",
    "subject": "subject name",
    "exam": "general",
    "language": "english",
    "difficulty": number(1-10),
    "question_type": "m-choice" or "m-select",
    "options": [answer options here],
    "correct_options": [zero based index],
    "explanation": "explanation here",
    "tags": ["tag1", "tag2"]
  },`
	var quizFormat string = `{
  "name": "title",
  "mode": "practice",
  "subject": "subject",
  "exam": "na",
  "language": "language",
  "time_duration": 40,
  "description": a description of what it is about,
  "associated_resource": "the input url",
  "question_ids": [1,2],
  "tags":  ["tag1", "tag2"]
}`

	payload := QuizRequest{
		Prompt:         req.Prompt,
		NOfQ:           req.QuestionCount,
		QuestionType:   req.QuestionType,
		QuestionFormat: questionFormat,
		QuizFormat:     quizFormat,
		Language:       req.Language,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to marshal request data",
			"error":   err.Error(),
		})
	}
	pyserverURL := os.Getenv("py_url")
	resp, err := http.Post(
		pyserverURL+"/quiz",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to connect to quiz service",
			"error":   err.Error(),
		})
	}
	defer resp.Body.Close()

	// Read response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to read service response",
			"error":   err.Error(),
		})
	}

	// Check for error response from Python service
	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]interface{}
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return c.Status(resp.StatusCode).JSON(fiber.Map{
				"status":  "error",
				"message": "Quiz generation failed",
				"error":   string(body),
			})
		}
		return c.Status(resp.StatusCode).JSON(errorResp)
	}

	// Parse successful response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to parse service response",
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status": "success",
		"data":   result,
	})
}
