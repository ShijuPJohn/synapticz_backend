package models

import (
	"github.com/google/uuid"
	"github.com/lib/pq"
	"time"
)

// User model
type User struct {
	ID                int       `json:"id"`
	Name              string    `json:"name"`
	Email             string    `json:"email"`
	Password          string    `json:"password"`
	Role              string    `json:"role"`
	PasswordChangedAt time.Time `json:"passwordChangedAt"`
	Verified          *bool     `json:"verified"`
	LinkedIn          *string   `json:"linkedIn"`
	Facebook          *string   `json:"facebook"`
	Instagram         *string   `json:"instagram"`
	ProfilePic        *string   `json:"profilePic"`
	About             *string   `json:"about"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// Question model
type Question struct {
	ID             int            `json:"id"`
	Question       string         `json:"question"`
	Subject        string         `json:"subject"`
	Tags           pq.StringArray `json:"tags"`
	Exam           *string        `json:"exam"`
	Language       string         `json:"language"`
	Difficulty     int            `json:"difficulty"`
	QuestionType   string         `json:"questionType"`
	Options        pq.StringArray `json:"options"`
	CorrectOptions int            `json:"correctOptions"`
	Explanation    *string        `json:"explanation"`
	CreatedByID    int            `json:"createdById"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

// QuestionSet model
type QuestionSet struct {
	ID           int            `json:"id"`
	Name         string         `json:"name"`
	Mode         string         `json:"mode"`
	Subject      string         `json:"subject"`
	Tags         pq.StringArray `json:"tags"`
	Exam         *string        `json:"exam"`
	Language     string         `json:"language"`
	TimeDuration *string        `json:"timeDuration"`
	Difficulty   int            `json:"difficulty"`
	Description  *string        `json:"description"`
	CreatedByID  int            `json:"createdById"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
}

// QTest model
type QTest struct {
	ID                 uuid.UUID        `json:"id"`
	Finished           bool             `json:"finished"`
	Started            bool             `json:"started"`
	Name               string           `json:"name"`
	Tags               pq.StringArray   `json:"tags"`
	QuestionSetID      uuid.UUID        `json:"questionSetId"`
	TakenByID          uuid.UUID        `json:"takenById"`
	NTotalQuestions    int              `json:"nTotalQuestions"`
	AllQuestionsIDs    map[string][]int `json:"allQuestionsIds"`
	CurrentQuestionNum int              `json:"currentQuestionNum"`
	QuestionIDsOrdered pq.StringArray   `json:"questionIdsOrdered"`
	NCorrectlyAnswered int              `json:"nCorrectlyAnswered"`
	Rank               *int             `json:"rank"`
	TakenAtTime        time.Time        `json:"takenAtTime"`
	Mode               string           `json:"mode"`
}

// Many-to-many relationships
type UserQuestionEditor struct {
	UserID     int `json:"userId"`
	QuestionID int `json:"questionId"`
}

type UserQuestionSetEditor struct {
	UserID        int `json:"userId"`
	QuestionSetID int `json:"questionSetId"`
}

type QuestionSetQuestion struct {
	QuestionSetID int `json:"questionSetId"`
	QuestionID    int `json:"questionId"`
}
