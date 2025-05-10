package controllers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"github.com/ShijuPJohn/synapticz_backend/util"
	"github.com/gofiber/fiber/v2"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

func SearchResourceURL(c *fiber.Ctx) error {
	// Parse the YouTube url from the request body
	type Request struct {
		URL      string `json:"resource_url"`
		Language string `json:"language"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Validate the url contains youtube.com
	if !strings.Contains(req.URL, "youtube.com") && !strings.Contains(req.URL, "youtu.be") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid YouTube url",
		})
	}

	// Extract the video ID from the url
	videoID := extractVideoID(req.URL)
	if videoID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Could not extract video ID from url",
		})
	}

	// Check for all question sets with this url
	rows, err := util.DB.Query(`
        SELECT id, name, created_by_id, created_at 
        FROM question_sets 
        WHERE associated_resource = $1 AND deleted = false AND language=$2
        ORDER BY created_at DESC
    `, req.URL, req.Language)

	if err != nil && err != sql.ErrNoRows {
		// Database error
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Database error",
			"error":   err.Error(),
		})
	}
	defer rows.Close()

	type QuestionSetInfo struct {
		ID          int       `json:"id"`
		Name        string    `json:"name"`
		CreatedByID int       `json:"created_by_id"`
		CreatedAt   time.Time `json:"created_at"`
	}

	var existingSets []QuestionSetInfo
	for rows.Next() {
		var qs QuestionSetInfo
		if err := rows.Scan(&qs.ID, &qs.Name, &qs.CreatedByID, &qs.CreatedAt); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": "Failed to scan question set",
				"error":   err.Error(),
			})
		}
		existingSets = append(existingSets, qs)
	}

	if len(existingSets) > 0 {
		// Found existing question sets - return them all
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":       "success",
			"exists":       true,
			"message":      "Question sets already exist",
			"questionSets": existingSets,
			"videoID":      videoID,
		})
	}

	// No existing question sets found - proceed with creation
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"exists":  false,
		"message": "No existing question sets found, proceed with creation",
		"videoID": videoID,
	})
}

// Helper function to extract YouTube video ID from url
func extractVideoID(url string) string {
	// Handle youtu.be/ID format
	if strings.Contains(url, "youtu.be/") {
		parts := strings.Split(url, "youtu.be/")
		if len(parts) > 1 {
			return strings.Split(parts[1], "?")[0]
		}
	}

	// Handle youtube.com/watch?v=ID format
	if strings.Contains(url, "v=") {
		parts := strings.Split(url, "v=")
		if len(parts) > 1 {
			return strings.Split(parts[1], "&")[0]
		}
	}

	// Handle youtube.com/embed/ID format
	if strings.Contains(url, "embed/") {
		parts := strings.Split(url, "embed/")
		if len(parts) > 1 {
			return strings.Split(parts[1], "?")[0]
		}
	}

	// Handle youtube.com/shorts/ID format
	if strings.Contains(url, "shorts/") {
		parts := strings.Split(url, "shorts/")
		if len(parts) > 1 {
			return strings.Split(parts[1], "?")[0]
		}
	}

	return ""
}

type QuizRequest struct {
	URL            string      `json:"url"`
	Transcript     string      `json:"transcript"`
	NOfQ           int         `json:"n_of_q"`
	QuestionFormat interface{} `json:"question_format"`
	QuizFormat     interface{} `json:"quiz_format"`
	Language       string      `json:"language"`
}

func GenerateQuizFromYTVideo(c *fiber.Ctx) error {
	// Parse request body
	var req struct {
		URL      string `json:"url"`
		Language string `json:"language"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}
	var nOfQ int = 20
	var questionFormat string = `[
  {
    "question": "question statement",
    "subject": "subject name",
    "exam": "general",
    "language": "english",
    "difficulty": number 1-10,
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
	if !strings.Contains(req.URL, "youtube.com") && !strings.Contains(req.URL, "youtu.be") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid YouTube url",
		})
	}

	// Prepare request to Python service
	payload := QuizRequest{
		URL:            req.URL,
		NOfQ:           nOfQ,
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
