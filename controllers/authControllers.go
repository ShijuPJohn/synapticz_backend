package controllers

import (
	"cloud.google.com/go/storage"
	"context"
	"database/sql"
	"fmt"
	"github.com/ShijuPJohn/synapticz_backend/models"
	"github.com/ShijuPJohn/synapticz_backend/util"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"io"
	"strconv"
	"strings"
	"time"
)

func Index(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "page": "index page"})
}

// creating a new user
func CreateUser(c *fiber.Ctx) error {
	u := new(models.User)
	if err := c.BodyParser(u); err != nil {
		return c.Status(400).SendString(err.Error())
	}

	u.Role = "user"
	u.PasswordChangedAt = time.Now()

	validate := validator.New()
	if err := validate.Struct(u); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": err.Error(),
		})
	}
	fmt.Println("here1")
	hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Println("here2", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": err.Error(),
		})
	}
	u.Password = string(hash)
	fmt.Println("here3")
	query := `INSERT INTO users 
	(name, email, password, role, linkedin, facebook, instagram, profile_pic, about)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	RETURNING id`

	err = util.DB.QueryRow(
		query,
		u.Name,
		u.Email,
		u.Password,
		u.Role,
		u.LinkedIn,
		u.Facebook,
		u.Instagram,
		u.ProfilePic,
		u.About,
	).Scan(&u.ID)
	fmt.Println("here4")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Error inserting user into database",
			"error":   err.Error(),
		})
	}
	fmt.Println("here5")
	token, err := util.JwtGenerate(*u, strconv.Itoa(int(u.ID)))
	if err != nil {
		fmt.Println("Error Here.....", err.Error())
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":  "success",
		"message": "User Created",
		"token":   token,
		"user_id": u.ID,
	})
}

func LoginUser(c *fiber.Ctx) error {
	type LoginInput struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required"`
	}
	var input LoginInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid request body",
		})
	}

	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": err.Error(),
		})
	}

	var user models.User
	query := `
	SELECT id, name, email, password, role, password_changed_at, verified, linkedin, facebook, instagram, profile_pic, about 
	FROM users 
	WHERE email = $1 AND deleted = false
	LIMIT 1
	`

	err := util.DB.QueryRow(query, input.Email).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Password,
		&user.Role,
		&user.PasswordChangedAt,
		&user.Verified,
		&user.LinkedIn,
		&user.Facebook,
		&user.Instagram,
		&user.ProfilePic,
		&user.About,
	)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid email or password",
		})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid email or password",
		})
	}

	token, err := util.JwtGenerate(user, strconv.Itoa(int(user.ID)))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Could not generate token",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"message": "Logged in successfully",
		"token":   token,
		"user_id": user.ID,
	})
}

func GetUserDetails(c *fiber.Ctx) error {
	// Get user ID from URL params
	userId := c.Locals("user").(models.User).ID

	// Fetch user details from DB manually
	var user models.User
	query := `SELECT id, name, email, role, password_changed_at, verified, linkedin, facebook, instagram, profile_pic, about, deleted, created_at, updated_at ,goal
			  FROM users WHERE id = $1 AND deleted = false`

	row := util.DB.QueryRow(query, userId)
	err := row.Scan(
		&user.ID, &user.Name, &user.Email, &user.Role, &user.PasswordChangedAt,
		&user.Verified, &user.LinkedIn, &user.Facebook, &user.Instagram,
		&user.ProfilePic, &user.About, &user.Deleted, &user.CreatedAt, &user.UpdatedAt, &user.Goal,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"status":  "error",
				"message": "User not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Database error",
			"error":   err.Error(),
		})
	}

	// Remove sensitive fields like password
	user.Password = ""

	return c.JSON(fiber.Map{
		"status": "success",
		"user":   user,
	})
}

func EditUserProfile(c *fiber.Ctx) error {
	user := c.Locals("user").(models.User)

	// Define a struct for updatable fields only
	type UpdatePayload struct {
		Name       *string `json:"name"`
		About      *string `json:"about"`
		Goal       *string `json:"goal"`
		LinkedIn   *string `json:"linkedin"`
		Facebook   *string `json:"facebook"`
		Instagram  *string `json:"instagram"`
		ProfilePic *string `json:"profile_pic"`
	}

	var payload UpdatePayload
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid input: " + err.Error(),
		})
	}
	fmt.Println(*payload.Goal)

	query := `
		UPDATE users
		SET
			name = COALESCE($1, name),
			about = COALESCE($2, about),
			goal = COALESCE($3, goal),
			linkedin = COALESCE($4, linkedin),
			facebook = COALESCE($5, facebook),
			instagram = COALESCE($6, instagram),
			profile_pic = COALESCE($7, profile_pic),
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $8 AND deleted = false
		RETURNING id
	`

	var updatedID int
	err := util.DB.QueryRow(
		query,
		payload.Name,
		payload.About,
		payload.Goal,
		payload.LinkedIn,
		payload.Facebook,
		payload.Instagram,
		payload.ProfilePic,
		user.ID,
	).Scan(&updatedID)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":  "Failed to update user profile",
			"detail": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Profile updated successfully",
	})
}

const (
	bucketName   = "synapticz-storage"               // ⬅️ Change this
	storageURL   = "https://storage.googleapis.com/" // Base URL
	uploadFolder = "profile_pics/"                   // Optional: for organization
)

func UploadProfilePic(c *fiber.Ctx) error {
	// Get the file from the form
	fileHeader, err := c.FormFile("file")
	userName := c.Locals("user").(models.User).Name
	cleanedName := SlugifyUsername(userName)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "File is required : " + err.Error(),
		})
	}

	// Open the uploaded file
	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to open file : " + err.Error(),
		})
	}
	defer file.Close()

	// Generate a unique filename with extension
	uniqueFilename := fmt.Sprintf("%s-%s%s", cleanedName, uuid.New().String(), getFileExtension(fileHeader.Filename))
	objectName := uploadFolder + uniqueFilename

	// Upload to GCS
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create GCS client : " + err.Error(),
		})
	}
	defer client.Close()

	writer := client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	writer.ContentType = fileHeader.Header.Get("Content-Type")

	if _, err := io.Copy(writer, file); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to upload image : " + err.Error(),
		})
	}

	if err := writer.Close(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to finalize upload : " + err.Error(),
		})
	}

	publicURL := fmt.Sprintf("%s%s/%s", storageURL, bucketName, objectName)

	return c.JSON(fiber.Map{
		"url": publicURL,
	})
}

func getFileExtension(filename string) string {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			return filename[i:]
		}
	}
	return ""
}
func SlugifyUsername(name string) string {
	// Trim leading/trailing spaces and replace all internal spaces with "-"
	return strings.ReplaceAll(strings.TrimSpace(name), " ", "-")
}
func GetUserActivityOverview(c *fiber.Ctx) error {
	user := c.Locals("user").(models.User)

	// Step 0: Get timezone from query param
	timezone := c.Query("tz", "UTC") // fallback to UTC

	// Step 1: Calculate local "today" and last 6 days in UTC for filtering
	now := time.Now().UTC()
	tzLoc, err := time.LoadLocation(timezone)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid timezone"})
	}

	localToday := now.In(tzLoc).Truncate(24 * time.Hour)
	sevenDaysAgo := localToday.AddDate(0, 0, -6)
	startOfYear := time.Date(localToday.Year(), 1, 1, 0, 0, 0, 0, time.UTC)

	type DayActivity struct {
		Date              string   `json:"date"`
		QuestionsAnswered int      `json:"questions_answered"`
		TestsCreated      []string `json:"tests_created"`
		TestsCompleted    []string `json:"tests_completed"`
	}

	dayMap := make(map[string]*DayActivity)

	// Step 2: Daily answered questions (converted by timezone)
	rows, err := util.DB.Query(`
		SELECT TO_CHAR(answered_at AT TIME ZONE 'UTC' AT TIME ZONE $2, 'YYYY-MM-DD') AS local_date, COUNT(*) 
		FROM user_daily_questions 
		WHERE user_id = $1 AND answered_at >= $3
		GROUP BY local_date
	`, user.ID, timezone, sevenDaysAgo)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch questions answered"})
	}
	defer rows.Close()

	for rows.Next() {
		var date string
		var count int
		if err := rows.Scan(&date, &count); err == nil {
			dayMap[date] = &DayActivity{Date: date, QuestionsAnswered: count}
		}
	}

	// Step 3: Test sessions created
	rows, err = util.DB.Query(`
		SELECT TO_CHAR(started_time AT TIME ZONE 'UTC' AT TIME ZONE $2, 'YYYY-MM-DD'), name 
		FROM test_sessions 
		WHERE taken_by_id = $1 AND started_time >= $3
	`, user.ID, timezone, sevenDaysAgo)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch test sessions created"})
	}
	defer rows.Close()

	for rows.Next() {
		var date, name string
		if err := rows.Scan(&date, &name); err == nil {
			if _, exists := dayMap[date]; !exists {
				dayMap[date] = &DayActivity{Date: date}
			}
			dayMap[date].TestsCreated = append(dayMap[date].TestsCreated, name)
		}
	}

	// Step 4: Test sessions completed
	rows, err = util.DB.Query(`
		SELECT TO_CHAR(finished_time AT TIME ZONE 'UTC' AT TIME ZONE $2, 'YYYY-MM-DD'), name 
		FROM test_sessions 
		WHERE taken_by_id = $1 AND finished = true AND finished_time IS NOT NULL AND finished_time >= $3
	`, user.ID, timezone, sevenDaysAgo)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch test sessions completed"})
	}
	defer rows.Close()

	for rows.Next() {
		var date, name string
		if err := rows.Scan(&date, &name); err == nil {
			if _, exists := dayMap[date]; !exists {
				dayMap[date] = &DayActivity{Date: date}
			}
			dayMap[date].TestsCompleted = append(dayMap[date].TestsCompleted, name)
		}
	}

	// Step 5: Normalize output for the past 7 days
	dailyActivities := []DayActivity{}
	for i := 0; i < 7; i++ {
		date := localToday.AddDate(0, 0, -i).Format("2006-01-02")
		if dayMap[date] != nil {
			if dayMap[date].TestsCreated == nil {
				dayMap[date].TestsCreated = []string{}
			}
			if dayMap[date].TestsCompleted == nil {
				dayMap[date].TestsCompleted = []string{}
			}
			dailyActivities = append(dailyActivities, *dayMap[date])
		} else {
			dailyActivities = append(dailyActivities, DayActivity{
				Date:              date,
				QuestionsAnswered: 0,
				TestsCreated:      []string{},
				TestsCompleted:    []string{},
			})
		}
	}

	// Step 6: Build year summary
	summaryMap := map[string]int{}
	rows, err = util.DB.Query(`
  WITH all_activities AS (
      SELECT (answered_at AT TIME ZONE $1)::date AS activity_date FROM user_daily_questions WHERE user_id = $2
      UNION ALL
      SELECT (started_time AT TIME ZONE $1)::date AS activity_date FROM test_sessions WHERE taken_by_id = $2
      UNION ALL
      SELECT (finished_time AT TIME ZONE $1)::date AS activity_date FROM test_sessions WHERE taken_by_id = $2 AND finished = true
  )
  SELECT activity_date, COUNT(*) 
  FROM all_activities
  WHERE activity_date >= $3
  GROUP BY activity_date
`, timezone, user.ID, startOfYear)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get yearly summary"})
	}
	defer rows.Close()

	for rows.Next() {
		var date string
		var count int
		if err := rows.Scan(&date, &count); err == nil {
			summaryMap[date] = count
		}
	}

	// Step 7: Profile
	var profile struct {
		Name       string    `json:"name"`
		Email      string    `json:"email"`
		ProfilePic *string   `json:"profile_pic,omitempty"`
		About      *string   `json:"about,omitempty"`
		IsPremium  bool      `json:"isPremium"`
		JoinedAt   time.Time `json:"joinedAt"`
		Goal       *string   `json:"goal"`
		Facebook   *string   `json:"facebook"`
		Linkedin   *string   `json:"linkedin"`
		Instagram  *string   `json:"instagram"`
	}
	err = util.DB.QueryRow(`
		SELECT name, email, profile_pic, about, is_premium, created_at, goal, facebook, linkedin, instagram
		FROM users WHERE id = $1
	`, user.ID).Scan(&profile.Name, &profile.Email, &profile.ProfilePic, &profile.About, &profile.IsPremium, &profile.JoinedAt, &profile.Goal, &profile.Facebook, &profile.Linkedin, &profile.Instagram)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch user profile"})
	}

	return c.JSON(fiber.Map{
		"profile":        profile,
		"daily_activity": dailyActivities,
		"year_summary":   summaryMap,
	})
}

//
//func LoginUser(c *fiber.Ctx) error {
//	type loginModel struct {
//		Email    string `json:"email"`
//		Password string `json:"password"`
//	}
//	loginObject := new(loginModel)
//	err := c.BodyParser(&loginObject)
//	if err != nil {
//		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "error": err.Error()})
//	}
//
//	// Find the user from the PostgreSQL database
//	var userFromDB models.User
//	if err := utils.DB.Db.Where("email = ?", loginObject.Email).First(&userFromDB).Error; err != nil {
//		// Check if the error is because the user is not found
//		if errors.Is(err, gorm.ErrRecordNotFound) {
//			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "invalid credentials"})
//		}
//		// Handle other database errors
//		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "error": err.Error()})
//	}
//
//	// Compare the hashed password stored in the database with the provided password
//	err = bcrypt.CompareHashAndPassword([]byte(userFromDB.Password), []byte(loginObject.Password))
//	if err != nil {
//		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "invalid credentials"})
//	}
//
//	// Generate JWT token using user ID
//	token, err := utils.JwtGenerate(userFromDB, strconv.Itoa(int(userFromDB.ID)))
//	if err != nil {
//		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "invalid credentials"})
//	}
//	if token == "" {
//		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "internal server error"})
//	}
//
//	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "token": token})
//}
