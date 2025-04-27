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
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"log"
	"os"
	"strings"
)

var DB *sql.DB
var JWTSecret string
var MailAPIKey string
var ClientID string
var ClientSecret string

func getDBCredentialsandPopulateJWTSecret() (string, error) {
	if env := os.Getenv("ENV"); env == "DEV" || env == "DEV_DB" {
		var dbUser string
		var dbPass string
		var dbHost string
		var dbPort string
		var dbName string
		var sslMode string
		err := godotenv.Load()
		if err != nil {
			return "", errors.New("couldn't get environment variables")
		}
		if os.Getenv("ENV") == "DEV_DB" {
			dbUser = os.Getenv("LOCAL_DB_USER")
			dbPass = os.Getenv("LOCAL_DB_PASS")
			dbHost = os.Getenv("LOCAL_DB_HOST")
			dbPort = os.Getenv("LOCAL_DB_PORT")
			dbName = os.Getenv("LOCAL_DB_NAME")
			sslMode = os.Getenv("SSL_MODE")
		} else {
			dbUser = os.Getenv("DB_USER")
			dbPass = os.Getenv("DB_PASS")
			dbHost = os.Getenv("DB_HOST")
			dbPort = os.Getenv("DB_PORT")
			dbName = os.Getenv("DB_NAME")
			sslMode = os.Getenv("SSL_MODE")
		}

		JWTSecret = os.Getenv("JWT_SECRET")
		MailAPIKey = os.Getenv("MAIL_API_KEY")
		ClientID = os.Getenv("GOOGLE_CLIENT_ID")
		ClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
		str, err := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", dbHost, dbPort, dbUser, dbPass, dbName, sslMode), nil
		return str, nil
	} else {
		name := "projects/1037996227658/secrets/synapticz2pg/versions/7"
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
		ClientID = words[0]
		ClientSecret = words[1]
		MailAPIKey = words[2]
		JWTSecret = words[3]
		return strings.Join(words[4:], " "), nil
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
func GetGoogleConfig() *oauth2.Config {
	var uri string
	if os.Getenv("ENV") == "DEV" {
		uri = "http://localhost:8080/api/auth/google-callback"
	} else {
		uri = "https://synapticz-backend-go-1037996227658.asia-southeast1.run.app/api/auth/google-callback"
	}
	return &oauth2.Config{
		RedirectURL:  uri,
		ClientID:     ClientID,
		ClientSecret: ClientSecret,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
		Endpoint:     google.Endpoint,
	}
}
