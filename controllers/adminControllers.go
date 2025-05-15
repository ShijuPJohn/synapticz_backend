package controllers

import (
	"database/sql"
	"fmt"
	"github.com/ShijuPJohn/synapticz_backend/models"
	"github.com/ShijuPJohn/synapticz_backend/util"
	"github.com/gofiber/fiber/v2"
	"math"
	"strconv"
	"strings"
	"time"
)

func GetAllUsers(c *fiber.Ctx) error {
	// Get current user from JWT middleware
	currentUser := c.Locals("user").(models.User)

	// Only allow admin access
	if currentUser.Role != "admin" && currentUser.Role != "owner" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  "error",
			"message": "Only admins can access this endpoint",
		})
	}

	// Pagination parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	offset := (page - 1) * limit

	// Filtering parameters
	search := c.Query("search")
	role := c.Query("role")
	country := c.Query("country")
	isPremium := c.Query("is_premium")
	verified := c.Query("verified")
	sortBy := c.Query("sort_by", "created_at")
	sortOrder := c.Query("sort_order", "desc")

	// Build base query
	query := `
		SELECT 
			id, name, email, role, profile_pic, 
			is_premium, country, country_code,
			verified, created_at
		FROM users 
		WHERE deleted = false
	`

	// Add filters
	var args []interface{}
	var whereClauses []string
	argCount := 1

	if search != "" {
		whereClauses = append(whereClauses, fmt.Sprintf(`
			(name ILIKE $%d OR email ILIKE $%d)
		`, argCount, argCount))
		args = append(args, "%"+search+"%")
		argCount++
	}

	if role != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("role = $%d", argCount))
		args = append(args, role)
		argCount++
	}

	if country != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("country = $%d", argCount))
		args = append(args, country)
		argCount++
	}

	if isPremium != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("is_premium = $%d", argCount))
		args = append(args, isPremium == "true")
		argCount++
	}

	if verified != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("verified = $%d", argCount))
		args = append(args, verified == "true")
		argCount++
	}

	if len(whereClauses) > 0 {
		query += " AND " + strings.Join(whereClauses, " AND ")
	}

	// Add sorting
	query += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)

	// Add pagination
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argCount, argCount+1)
	args = append(args, limit, offset)

	// Execute query
	rows, err := util.DB.Query(query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to fetch users",
			"error":   err.Error(),
		})
	}
	defer rows.Close()

	// Get total count for pagination
	var totalCount int
	countQuery := "SELECT COUNT(*) FROM users WHERE deleted = false"
	if len(whereClauses) > 0 {
		countQuery += " AND " + strings.Join(whereClauses, " AND ")
	}
	err = util.DB.QueryRow(countQuery, args[:len(args)-2]...).Scan(&totalCount)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to get total count",
			"error":   err.Error(),
		})
	}

	// Process results
	type UserResponse struct {
		ID          int       `json:"id"`
		Name        string    `json:"name"`
		Email       string    `json:"email"`
		Role        string    `json:"role"`
		ProfilePic  *string   `json:"profile_pic"`
		IsPremium   bool      `json:"is_premium"`
		Country     *string   `json:"country"`
		CountryCode *string   `json:"country_code"`
		Verified    bool      `json:"verified"`
		CreatedAt   time.Time `json:"created_at"`
	}

	var users []UserResponse
	for rows.Next() {
		var u UserResponse
		err := rows.Scan(
			&u.ID,
			&u.Name,
			&u.Email,
			&u.Role,
			&u.ProfilePic,
			&u.IsPremium,
			&u.Country,
			&u.CountryCode,
			&u.Verified,
			&u.CreatedAt,
		)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": "Failed to scan user data",
				"error":   err.Error(),
			})
		}
		users = append(users, u)
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(totalCount) / float64(limit)))

	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"users": users,
			"pagination": fiber.Map{
				"total":        totalCount,
				"count":        len(users),
				"per_page":     limit,
				"current_page": page,
				"total_pages":  totalPages,
			},
		},
	})
}
func GetUserDetailsAdmin(c *fiber.Ctx) error {
	// Get requested user ID from path params
	requestedUserID, err := c.ParamsInt("uid")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid user ID",
		})
	}

	// Get current user from JWT middleware
	currentUser := c.Locals("user").(models.User)

	// Authorization check - only admin/owner or the user themselves can access
	if currentUser.Role != "admin" && currentUser.Role != "owner" && currentUser.ID != requestedUserID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  "error",
			"message": "Unauthorized: Only admins, owners or the account owner can access this information",
		})
	}

	// Fetch user details from DB
	var user models.User
	query := `SELECT id, name, email, role, password_changed_at, verified, linkedin, 
              facebook, instagram, profile_pic, about, deleted, created_at, updated_at,
              goal, country, country_code, mobile_number, is_premium
              FROM users WHERE id = $1 AND deleted = false`

	row := util.DB.QueryRow(query, requestedUserID)
	err = row.Scan(
		&user.ID, &user.Name, &user.Email, &user.Role, &user.PasswordChangedAt,
		&user.Verified, &user.LinkedIn, &user.Facebook, &user.Instagram,
		&user.ProfilePic, &user.About, &user.Deleted, &user.CreatedAt,
		&user.UpdatedAt, &user.Goal, &user.Country, &user.CountryCode,
		&user.MobileNumber, &user.IsPremium,
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

	// Create response object with only non-sensitive fields
	response := fiber.Map{
		"id":           user.ID,
		"name":         user.Name,
		"email":        user.Email,
		"role":         user.Role,
		"verified":     user.Verified,
		"profile_pic":  user.ProfilePic,
		"about":        user.About,
		"goal":         user.Goal,
		"country":      user.Country,
		"country_code": user.CountryCode,
		"is_premium":   user.IsPremium,
		"created_at":   user.CreatedAt,
		"social_links": fiber.Map{
			"linkedin":  user.LinkedIn,
			"facebook":  user.Facebook,
			"instagram": user.Instagram,
		},
	}

	// Include mobile number only if admin/owner or the user themselves
	if currentUser.Role == "admin" || currentUser.Role == "owner" || currentUser.ID == requestedUserID {
		response["mobile_number"] = user.MobileNumber
		response["password_changed_at"] = user.PasswordChangedAt
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   response,
	})
}
