package controllers

import (
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

//	func GetAllUsers(c *fiber.Ctx) error {
//		var users []models.User
//
//		if err := utils.DB.Db.Find(&users).Error; err != nil {
//			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
//				"status":  "error",
//				"message": "Error fetching users from the database",
//				"error":   err.Error(),
//			})
//		}
//		return c.Status(fiber.StatusOK).JSON(fiber.Map{
//			"status": "success",
//			"users":  users,
//		})
//	}

func CreateUser(c *fiber.Ctx) error {
	fmt.Println("hereeee")
	u := new(models.User)
	if err := c.BodyParser(u); err != nil {
		return c.Status(400).SendString(err.Error())
	}

	if u.Role == "" {
		u.Role = "user"
	}
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
	(name, email, password, role, password_changed_at, linkedin, facebook, instagram, profile_pic, about)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	RETURNING id`

	err = util.DB.QueryRow(
		query,
		u.Name,
		u.Email,
		u.Password,
		u.Role,
		u.PasswordChangedAt,
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
