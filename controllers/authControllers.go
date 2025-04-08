package controllers

import (
	"github.com/gofiber/fiber/v2"
)

func Index(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "page": "index page"})
}

//func GetAllUsers(c *fiber.Ctx) error {
//	var users []models.User
//
//	if err := utils.DB.Db.Find(&users).Error; err != nil {
//		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
//			"status":  "error",
//			"message": "Error fetching users from the database",
//			"error":   err.Error(),
//		})
//	}
//	return c.Status(fiber.StatusOK).JSON(fiber.Map{
//		"status": "success",
//		"users":  users,
//	})
//}
//
//func CreateUser(c *fiber.Ctx) error {
//	u := new(models.User)
//
//	if err := c.BodyParser(u); err != nil {
//		return c.Status(400).SendString(err.Error())
//	}
//	if u.Role == "" {
//		u.Role = "user"
//	}
//	u.PasswordChangedAt = time.Now()
//	validate := validator.New()
//	err := validate.Struct(u)
//	if err != nil {
//		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
//			"status":  "error",
//			"message": err.Error(),
//		})
//	}
//	hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
//	if err != nil {
//		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
//			"status":  "Error",
//			"message": err.Error(),
//		})
//	}
//	u.Password = string(hash)
//
//	if err := utils.DB.Db.Create(&u).Error; err != nil {
//		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
//			"status":  "error",
//			"message": "Error inserting user into database",
//			"error":   err.Error(),
//		})
//	}
//	token, err := utils.JwtGenerate(*u, strconv.Itoa(int(u.ID)))
//	if err != nil {
//		fmt.Println(err.Error())
//		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
//	}
//	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
//		"status":  "success",
//		"message": "User Created",
//		"token":   token,
//		"user_id": u.ID})
//}
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
