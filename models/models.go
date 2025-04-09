package models

import "time"

type User struct {
	ID                int       `json:"id" db:"id"`
	Name              string    `json:"name" db:"name"`
	Email             string    `json:"email" db:"email"`
	Password          string    `json:"password" db:"password"`
	Role              string    `json:"role" db:"role"`
	PasswordChangedAt time.Time `json:"password_changed_at" db:"password_changed_at"`
	Verified          bool      `json:"verified" db:"verified"`
	LinkedIn          *string   `json:"linkedin,omitempty" db:"linkedin"`
	Facebook          *string   `json:"facebook,omitempty" db:"facebook"`
	Instagram         *string   `json:"instagram,omitempty" db:"instagram"`
	ProfilePic        *string   `json:"profile_pic,omitempty" db:"profile_pic"`
	About             *string   `json:"about,omitempty" db:"about"`
	Deleted           bool      `json:"deleted" db:"deleted"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}

type Question struct {
	ID             int       `json:"id" db:"id"`
	Question       string    `json:"question" db:"question"`
	Subject        string    `json:"subject" db:"subject"`
	Exam           *string   `json:"exam,omitempty" db:"exam"`
	Language       string    `json:"language" db:"language"`
	Tags           []string  `json:"tags" db:"tags"`
	Difficulty     int       `json:"difficulty" db:"difficulty"`
	QuestionType   string    `json:"question_type" db:"question_type"`
	Options        []string  `json:"options" db:"options"`
	CorrectOptions []int     `json:"correct_options" db:"correct_options"`
	Explanation    *string   `json:"explanation,omitempty" db:"explanation"`
	CreatedByID    int       `json:"created_by_id" db:"created_by_id"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

type UserQuestionEditor struct {
	UserID     int `json:"user_id" db:"user_id"`
	QuestionID int `json:"question_id" db:"question_id"`
}

type QuestionSet struct {
	ID                 int       `json:"id" db:"id"`
	Name               string    `json:"name" db:"name"`
	Mode               string    `json:"mode" db:"mode"`
	Subject            string    `json:"subject" db:"subject"`
	Exam               *string   `json:"exam,omitempty" db:"exam"`
	Language           string    `json:"language" db:"language"`
	AssociatedResource *string   `json:"associatedResource" db:"associatedResource"`
	TimeDuration       *string   `json:"time_duration,omitempty" db:"time_duration"`
	Difficulty         *int      `json:"difficulty,omitempty" db:"difficulty"`
	Description        *string   `json:"description,omitempty" db:"description"`
	CreatedByID        int       `json:"created_by_id" db:"created_by_id"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time `json:"updated_at" db:"updated_at"`
}

type UserQuestionSetEditor struct {
	UserID        int `json:"user_id" db:"user_id"`
	QuestionSetID int `json:"question_set_id" db:"question_set_id"`
}

type QuestionSetQuestion struct {
	QuestionSetID int `json:"question_set_id" db:"question_set_id"`
	QuestionID    int `json:"question_id" db:"question_id"`
}

type QTest struct {
	ID                 string    `json:"id" db:"id"` // UUID
	Finished           bool      `json:"finished" db:"finished"`
	Started            bool      `json:"started" db:"started"`
	Name               string    `json:"name" db:"name"`
	Tags               []string  `json:"tags" db:"tags"`
	QuestionSetID      int       `json:"question_set_id" db:"question_set_id"`
	TakenByID          int       `json:"taken_by_id" db:"taken_by_id"`
	NTotalQuestions    int       `json:"n_total_questions" db:"n_total_questions"`
	CurrentQuestionNum int       `json:"current_question_num" db:"current_question_num"`
	NCorrectlyAnswered int       `json:"n_correctly_answered" db:"n_correctly_answered"`
	Rank               *int      `json:"rank,omitempty" db:"rank"`
	TakenAtTime        time.Time `json:"taken_at_time" db:"taken_at_time"`
	Mode               string    `json:"mode" db:"mode"`
}

type QTestQuestion struct {
	QTestID    string `json:"qtest_id" db:"qtest_id"`
	QuestionID int    `json:"question_id" db:"question_id"`
}

type QuestionTag struct {
	ID   int    `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

type QuestionSetTag struct {
	ID   int    `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

type QuestionToTag struct {
	QuestionID    int `json:"question_id" db:"question_id"`
	QuestionTagID int `json:"questiontags_id" db:"questiontags_id"`
}

type QuestionSetToTag struct {
	QuestionSetID    int `json:"questionset_id" db:"questionset_id"`
	QuestionSetTagID int `json:"questionsettags_id" db:"questionsettags_id"`
}

type UserDailyActivity struct {
	UserID            int       `json:"user_id" db:"user_id"`
	ActivityDate      time.Time `json:"activity_date" db:"activity_date"`
	QuestionsAnswered int       `json:"questions_answered" db:"questions_answered"`
	TestsCompleted    int       `json:"tests_completed" db:"tests_completed"`
}
