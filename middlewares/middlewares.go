package middlewares

import (
	"database/sql"
	"fmt"
	"github.com/ShijuPJohn/synapticz_backend/models"
	"github.com/ShijuPJohn/synapticz_backend/util"
	"github.com/gofiber/fiber/v2"
	"strconv"
)

func NotFound(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
		"Status":  "Error",
		"Message": "Not Found",
	}) // => 404 "Not Found"
}

func Protected() fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := c.Cookies("token")
		fmt.Println("Token from request", token)
		claims, err := util.ParseJWT(token)
		fmt.Println("Claims from token", claims)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token " + err.Error()})
		}
		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"status":  "error",
				"message": "No token provided",
			})
		}

		// 3. Get user ID from claims
		userID, err := strconv.Atoi(claims["id"].(string))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"status":  "error",
				"message": "Invalid token payload",
			})
		}

		// 4. Fetch user manually using SQL
		var user models.User
		query := `SELECT id, name, email, password, role, password_changed_at, verified, linkedin, facebook, instagram, profile_pic, about, deleted, created_at, updated_at
		          FROM users WHERE id = $1 AND deleted = false`

		row := util.DB.QueryRow(query, userID)
		err = row.Scan(
			&user.ID, &user.Name, &user.Email, &user.Password, &user.Role,
			&user.PasswordChangedAt, &user.Verified, &user.LinkedIn, &user.Facebook,
			&user.Instagram, &user.ProfilePic, &user.About, &user.Deleted,
			&user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			if err == sql.ErrNoRows {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
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

		// 5. Validate token with password change timestamp
		if err := util.IsTokenValid(claims, user); err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"status":  "error",
				"message": err.Error(),
			})
		}

		// 6. Store user in request context
		c.Locals("user", user)

		// 7. Continue
		return c.Next()
	}
}

func jwtError(c *fiber.Ctx, err error) error {
	if err.Error() == "Missing or malformed JWT" {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"status": "error", "message": "Missing or malformed JWT", "data": nil})

	} else {
		c.Status(fiber.StatusUnauthorized)
		return c.JSON(fiber.Map{"status": "error", "message": "Invalid or expired JWT", "data": nil})
	}
}
