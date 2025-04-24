package controllers

import (
	"bytes"
	"cloud.google.com/go/storage"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/ShijuPJohn/synapticz_backend/models"
	"github.com/ShijuPJohn/synapticz_backend/util"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func Index(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "page": "index page"})
}

func VerifyOAuth(c *fiber.Ctx) error {
	userFromToken, ok := c.Locals("user").(models.User)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Unauthorized",
		})
	}

	var user models.User
	err := util.DB.QueryRow(`SELECT id, name, email, role, verified FROM users WHERE id = $1 AND deleted = false`, userFromToken.ID).
		Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.Verified)

	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "User not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "DB error",
			"error":   err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"message": "Logged in successfully",
		"user_id": user.ID,
	})
}

func GoogleLogin(c *fiber.Ctx) error {
	url := util.GetGoogleConfig().AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	return c.Redirect(url)
}

func GoogleCallback(c *fiber.Ctx) error {
	var baseFrontendURI string
	var secure bool
	if os.Getenv("ENV") == "DEV" {
		baseFrontendURI = "http://localhost:3000"
		secure = false
	} else {
		baseFrontendURI = "https://synapticz-frontend-1037996227658.asia-southeast1.run.app"
		secure = true
	}
	code := c.Query("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing code"})
	}

	// Step 1: Exchange code for token
	token, err := util.GetGoogleConfig().Exchange(context.Background(), code)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Token exchange failed: " + err.Error()})
	}

	// Step 2: Fetch user info from Google
	client := util.GetGoogleConfig().Client(context.Background(), token)
	res, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get user info: " + err.Error()})
	}
	defer res.Body.Close()

	var userInfo struct {
		Email         string `json:"email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		VerifiedEmail bool   `json:"verified_email"`
	}
	if err := json.NewDecoder(res.Body).Decode(&userInfo); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Invalid response from Google"})
	}

	// Step 3: Check if user exists
	var user models.User
	err = util.DB.QueryRow(`SELECT id, name, email, role FROM users WHERE email = $1`, userInfo.Email).
		Scan(&user.ID, &user.Name, &user.Email, &user.Role)

	if err == sql.ErrNoRows {
		// Create new user
		user.Name = userInfo.Name
		user.Email = userInfo.Email
		user.Role = "user"
		user.Verified = true
		user.ProfilePic = &userInfo.Picture

		err = util.DB.QueryRow(`
			INSERT INTO users (name, email, role, verified, profile_pic)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			user.Name, user.Email, user.Role, user.Verified, user.ProfilePic).
			Scan(&user.ID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create user: " + err.Error()})
		}
		tokenString, err := util.JwtGenerate(user, strconv.Itoa(int(user.ID)))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate token"})
		}

		// Step 5: Set HTTP-only cookie
		c.Cookie(&fiber.Cookie{
			Name:     "token",
			Value:    tokenString,
			Expires:  time.Now().Add(10 * 24 * time.Hour),
			HTTPOnly: true,
			Secure:   secure, // true if you're using https
			SameSite: "Lax",
			Path:     "/",
		})
		return c.Redirect(baseFrontendURI + "/verify-oauth-newuser")
	} else if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to query user: " + err.Error()})
	} else {
		tokenString, err := util.JwtGenerate(user, strconv.Itoa(int(user.ID)))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate token"})
		}

		// Step 5: Set HTTP-only cookie
		c.Cookie(&fiber.Cookie{
			Name:     "token",
			Value:    tokenString,
			Expires:  time.Now().Add(10 * 24 * time.Hour),
			HTTPOnly: true,
			Secure:   secure, // true if you're using https
			SameSite: "Lax",
			Path:     "/",
		})
		return c.Redirect(baseFrontendURI + "/verify-oauth-login")
	}

}

func sendVerificationEmail(to, subject, htmlBody string) error {
	payload := map[string]interface{}{
		"from":    "Synapticz <no-reply@synapticz.com>",
		"to":      []string{to},
		"subject": subject,
		"html":    htmlBody,
	}

	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+util.MailAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("Email failed: %s", string(bodyBytes))
	}

	return nil
}

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

	hash, err := bcrypt.GenerateFromPassword([]byte(*u.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": err.Error(),
		})
	}
	*u.Password = string(hash)

	// Step 1: Create user with verified = false
	query := `INSERT INTO users 
	(name, email, password, role, linkedin, facebook, instagram, profile_pic, about, verified)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, false)
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

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Error inserting user into database",
			"error":   err.Error(),
		})
	}

	// Step 2: Generate 6-digit verification code
	code := fmt.Sprintf("%06d", rand.Intn(1000000))

	// Step 3: Store the code in verification table
	_, err = util.DB.Exec(`
		INSERT INTO email_verification_codes (user_id, code, expires_at,email)
		VALUES ($1, $2, $3, $4)
	`, u.ID, code, time.Now().Add(10*time.Minute), u.Email)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status": "error",
			"error":  "Failed to save verification code",
		})
	}

	// Step 4: Send code via email using Resend
	emailBody := fmt.Sprintf(`
    <div style="font-family: Arial, sans-serif; font-size: 16px; color: #333;">
        <p>Hello,</p>
        <p>Thank you for signing up for <strong>Synapticz</strong>.</p>
        <p>Your verification code is:</p>
        <p style="font-size: 24px; font-weight: bold; color: #0ea5e9;">%s</p>
        <p>This code is valid for <strong>10 minutes</strong>.</p>
        <p>If you didn't request this, please ignore this email.</p>
        <br>
        <p>Best regards,<br>Team Synapticz</p>
    </div>
`, code)
	err = sendVerificationEmail(u.Email, "Verify your Synapticz account", emailBody)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status": "error",
			"error":  "Failed to send verification email",
		})
	}

	// Step 5: Respond with success (login token can be generated after verification)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":     "pending",
		"message":    "User created. Verification email sent.",
		"user_id":    u.ID,
		"user_email": u.Email,
	})
}

func VerifyUserEmail(c *fiber.Ctx) error {
	type VerifyDTO struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}

	var dto VerifyDTO
	if err := c.BodyParser(&dto); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid input",
		})
	}

	var user models.User
	query := `SELECT id, name, email, password, role, linkedin, facebook, instagram, profile_pic, about 
			  FROM users WHERE email = $1 AND deleted = false`

	err := util.DB.QueryRow(query, dto.Email).Scan(
		&user.ID, &user.Name, &user.Email, &user.Password, &user.Role,
		&user.LinkedIn, &user.Facebook, &user.Instagram, &user.ProfilePic, &user.About,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	// Step 2: Check if code matches and is not expired
	var expiresAt time.Time
	err = util.DB.QueryRow(`
		SELECT expires_at FROM email_verification_codes 
		WHERE user_id = $1 AND code = $2
	`, user.ID, dto.Code).Scan(&expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid or expired code"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	if time.Now().After(expiresAt) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Verification code has expired"})
	}

	// Step 3: Mark user as verified
	_, err = util.DB.Exec(`UPDATE users SET verified = true WHERE id = $1`, user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to verify user"})
	}

	// Step 4: Delete the used verification code
	_, _ = util.DB.Exec(`DELETE FROM email_verification_codes WHERE user_id = $1`, user.ID)

	// Step 5: Generate JWT token
	token, err := util.JwtGenerate(user, strconv.Itoa(int(user.ID)))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate token"})
	}
	c.Cookie(&fiber.Cookie{
		Name:     "token",
		Value:    token,
		Expires:  time.Now().Add(10 * 24 * time.Hour),
		HTTPOnly: true,
		Secure:   false, // true if you're using https
		SameSite: "Lax",
		Path:     "/",
	})

	// Step 6: Redirect to frontend
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"message": "Logged in successfully",
		"user_id": user.ID,
	})
}

func ResendVerificationCode(c *fiber.Ctx) error {
	type Request struct {
		ID    int    `json:"ID"`
		Email string `json:"email"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil || req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid email " + err.Error()})
	}

	// Check if a code already exists and how recently it was sent
	var lastSent time.Time
	err := util.DB.QueryRow(`
		SELECT created_at FROM email_verification_codes WHERE email = $1
	`, req.Email).Scan(&lastSent)

	if err != nil && err != sql.ErrNoRows {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to check timestamp " + err.Error()})
	}

	if err == nil && time.Since(lastSent) < 45*time.Second {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "Please wait before resending"})
	}

	// Generate new 6-digit code
	code := fmt.Sprintf("%06d", rand.Intn(1000000))

	// Upsert the verification code
	_, err = util.DB.Exec(`
		INSERT INTO email_verification_codes (user_id, email, code, created_at, expires_at)
		VALUES ($1, $2,$3, $4,$5)
		ON CONFLICT (email)
		DO UPDATE SET code = EXCLUDED.code, created_at = EXCLUDED.created_at
	`, req.ID, req.Email, code, time.Now(), time.Now().Add(10*time.Minute))

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to store code " + err.Error()})
	}

	// Send email via Resend
	emailBody := fmt.Sprintf(`
    <div style="font-family: Arial, sans-serif; font-size: 16px; color: #333;">
        <p>Hello,</p>
        <p>Thank you for signing up for <strong>Synapticz</strong>.</p>
        <p>Your verification code is:</p>
        <p style="font-size: 24px; font-weight: bold; color: #0ea5e9;">%s</p>
        <p>This code is valid for <strong>10 minutes</strong>.</p>
        <p>If you didn't request this, please ignore this email.</p>
        <br>
        <p>Best regards,<br>Team Synapticz</p>
    </div>
`, code)
	err = sendVerificationEmail(req.Email, "Verify your Synapticz account", emailBody)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to send email  " + err.Error()})
	}

	return c.JSON(fiber.Map{"message": "Verification code resent"})
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

	if err := bcrypt.CompareHashAndPassword([]byte(*user.Password), []byte(input.Password)); err != nil {
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
	c.Cookie(&fiber.Cookie{
		Name:     "token",
		Value:    token,
		Expires:  time.Now().Add(10 * 24 * time.Hour),
		HTTPOnly: true,
		Secure:   false, // true if you're using https
		SameSite: "Lax",
		Path:     "/",
	})

	// Step 6: Redirect to frontend
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"message": "Logged in successfully",
		"user_id": user.ID,
	})
}

func GetUserDetails(c *fiber.Ctx) error {
	// Get user ID from URL params
	userId := c.Locals("user").(models.User).ID

	// Fetch user details from DB manually
	var user models.User
	query := `SELECT id, name, email, role, password_changed_at, verified, linkedin, facebook, instagram, profile_pic, about, deleted, created_at, updated_at ,goal, country, country_code, mobile_number
			  FROM users WHERE id = $1 AND deleted = false`

	row := util.DB.QueryRow(query, userId)
	err := row.Scan(
		&user.ID, &user.Name, &user.Email, &user.Role, &user.PasswordChangedAt,
		&user.Verified, &user.LinkedIn, &user.Facebook, &user.Instagram,
		&user.ProfilePic, &user.About, &user.Deleted, &user.CreatedAt, &user.UpdatedAt, &user.Goal, &user.Country, &user.CountryCode, &user.MobileNumber,
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
	if user.Password != nil {
		*user.Password = ""
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"user":   user,
	})
}

func EditUserProfile(c *fiber.Ctx) error {
	user := c.Locals("user").(models.User)

	// Define a struct for updatable fields only
	type UpdatePayload struct {
		Name         *string `json:"name"`
		About        *string `json:"about"`
		Goal         *string `json:"goal"`
		LinkedIn     *string `json:"linkedin"`
		Facebook     *string `json:"facebook"`
		Instagram    *string `json:"instagram"`
		ProfilePic   *string `json:"profile_pic"`
		Country      *string `json:"country"`
		CountryCode  *string `json:"country_code"`
		MobileNumber *string `json:"mobile_number"`
	}

	var payload UpdatePayload
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid input: " + err.Error(),
		})
	}

	query := `
		UPDATE users
		SET
			name = COALESCE($1, name),
			about = COALESCE($2, about),
			goal = COALESCE($3, goal),
			linkedin = COALESCE($4, linkedin),
			facebook = COALESCE($5, facebook),
			instagram = COALESCE($6, instagram),
			profile_pic = COALESCE($7, country),
			country =  COALESCE($8, profile_pic),
			country_code =  COALESCE($9, country_code),
			mobile_number =  COALESCE($10, mobile_number),
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $11 AND deleted = false
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
		payload.Country,
		payload.CountryCode,
		payload.MobileNumber,
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
