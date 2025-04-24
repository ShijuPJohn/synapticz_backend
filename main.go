package main

import (
	"fmt"
	"github.com/ShijuPJohn/synapticz_backend/routers"
	"github.com/ShijuPJohn/synapticz_backend/util"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"log"
	"os"
)

func main() {
	err := util.DBConnectAndPopulateDBVar()
	if err != nil {
		fmt.Println(err.Error())
		log.Fatal("couldn't connect to the database")
	} else {
		log.Println("Connected to the database")
	}
	if err = util.CreateTableIfNotExists(); err != nil {
		log.Fatal("Couldn't create tables", err)
	}
	log.Println("Tables Created")
	app := fiber.New()
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:3000, https://synapticz.com", // or your frontend domain
		AllowCredentials: true,
	}))
	app.Use(logger.New())

	routers.SetupRoutes(app)
	var port string
	if port = os.Getenv("PORT"); port == "" {
		port = "8080"
	}
	log.Fatal(app.Listen(":" + port))
}
