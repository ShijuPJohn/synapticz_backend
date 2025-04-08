package util

import (
	"errors"
	"fmt"
	"github.com/ShijuPJohn/synapticz_backend/models"
	"github.com/golang-jwt/jwt/v4"
	"time"
)

func JwtGenerate(user models.User, id string) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["email"] = user.Email
	claims["role"] = user.Role
	claims["id"] = id
	claims["exp"] = time.Now().Add(time.Hour * 72).Unix()
	claims["issued"] = time.Now().Unix()
	t, err := token.SignedString([]byte(JWTSecret))
	return t, err

}

// Function to verify and parse the JWT token
func VerifyJwtToken(tokenString string) (jwt.MapClaims, error) {
	// Remove "Bearer " prefix from the token, if present
	if len(tokenString) > 6 && tokenString[:7] == "Bearer " {
		tokenString = tokenString[7:]
	}
	fmt.Println("tokenString:", tokenString)

	// Parse the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(JWTSecret), nil
	})
	fmt.Println("token parsed")
	fmt.Println(token)

	if err != nil {
		return nil, err
	}

	// Extract claims if the token is valid
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		fmt.Println("claims", claims)
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// Function to check if the token was issued before the password was changed
func IsTokenValid(claims jwt.MapClaims, user models.User) error {
	// Extract issued_at claim from token (assuming it's a Unix timestamp)
	issuedAt, ok := claims["issued"].(float64)
	if !ok {
		return errors.New("invalid token: missing issued_at claim")
	}

	// Convert issued_at to time.Time
	issuedAtTime := time.Unix(int64(issuedAt), 0)

	// Compare with PasswordChangedAt
	if user.PasswordChangedAt.After(issuedAtTime) {
		return errors.New("token invalid: password was changed after the token was issued")
	}

	return nil
}
