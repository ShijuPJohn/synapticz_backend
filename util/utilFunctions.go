package util

import (
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"log"
	"os"
	"strings"
)

var DB *sql.DB
var JWTSecret string
var MailAPIKey string

func getDBCredentialsandPopulateJWTSecret() (string, error) {
	if env := os.Getenv("ENV"); env == "DEV" {
		err := godotenv.Load()
		if err != nil {
			return "", errors.New("couldn't get environment variables")
		}
		dbUser := os.Getenv("DB_USER")
		dbPass := os.Getenv("DB_PASS")
		dbHost := os.Getenv("DB_HOST")
		dbPort := os.Getenv("DB_PORT")
		dbName := os.Getenv("DB_NAME")
		sslMode := os.Getenv("SSL_MODE")
		JWTSecret = os.Getenv("JWT_SECRET")
		MailAPIKey = os.Getenv("MAIL_API_KEY")
		str, err := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", dbHost, dbPort, dbUser, dbPass, dbName, sslMode), nil
		return str, nil
	} else {
		name := "projects/1037996227658/secrets/synapticz2pg/versions/6"
		ctx := context.Background()
		client, err := secretmanager.NewClient(ctx)
		if err != nil {
			return "", errors.New("couldn't get cloud secret")
		}
		defer client.Close()
		req := &secretmanagerpb.AccessSecretVersionRequest{
			Name: name,
		}
		result, err := client.AccessSecretVersion(ctx, req)
		if err != nil {
			log.Fatal("failed to access secret version: %w", err)
		}
		stringVal := string(result.Payload.Data)
		words := strings.Fields(stringVal)
		MailAPIKey = words[0]
		JWTSecret = words[1]
		return strings.Join(words[2:], " "), nil
	}
}
func DBConnectAndPopulateDBVar() error {
	connectString, err := getDBCredentialsandPopulateJWTSecret()
	if err != nil {
		return errors.New("couldn't get credentials")
	}
	DB, err = sql.Open("postgres", connectString)
	if err != nil {
		return err
	}
	if err = DB.Ping(); err != nil {
		return err
	}
	return nil
}
