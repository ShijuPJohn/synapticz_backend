package controllers

import (
	"database/sql"
	"fmt"
	"github.com/ShijuPJohn/synapticz_backend/models"
	"github.com/ShijuPJohn/synapticz_backend/util"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"strconv"
	"time"
)

func Index(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "page": "index page"})
}

// creating a user
func CreateUser(c *fiber.Ctx) error {
	fmt.Println("hereeee")
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

	hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": err.Error(),
		})
	}
	u.Password = string(hash)

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

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Error inserting user into database",
			"error":   err.Error(),
		})
	}

	token, err := util.JwtGenerate(*u, strconv.Itoa(int(u.ID)))
	if err != nil {
		fmt.Println(err.Error())
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
	paramID := c.Params("id")
	requestedID, err := strconv.Atoi(paramID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid user ID in URL",
		})
	}

	// Get user from context (set by middleware)
	authenticatedUser := c.Locals("user").(models.User)

	// Check if the user is requesting their own details
	if int(authenticatedUser.ID) != requestedID {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "You are not authorized to view this user's details",
		})
	}

	// Fetch user details from DB manually
	var user models.User
	query := `SELECT id, name, email, role, password_changed_at, verified, linkedin, facebook, instagram, profile_pic, about, deleted, created_at, updated_at 
			  FROM users WHERE id = $1 AND deleted = false`

	row := util.DB.QueryRow(query, requestedID)
	err = row.Scan(
		&user.ID, &user.Name, &user.Email, &user.Role, &user.PasswordChangedAt,
		&user.Verified, &user.LinkedIn, &user.Facebook, &user.Instagram,
		&user.ProfilePic, &user.About, &user.Deleted, &user.CreatedAt, &user.UpdatedAt,
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
