package routers

import (
	"github.com/ShijuPJohn/synapticz_backend/controllers"
	"github.com/ShijuPJohn/synapticz_backend/middlewares"
	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App) {

	//Index
	api := app.Group("/api")
	api.Post("/image-upload", middlewares.Protected(), controllers.UploadProfilePic)
	//api.Get("/home", middlewares.Protected(), handlers.Home)

	//Auth
	auth := api.Group("/auth")
	//auth.Get("/users", controllers.GetAllUsers)
	auth.Post("/users", controllers.CreateUser)
	auth.Get("/users", middlewares.Protected(), controllers.GetUserDetails)
	auth.Put("/users", middlewares.Protected(), controllers.EditUserProfile)
	auth.Post("/users/verify", controllers.VerifyUserEmail)
	auth.Post("/users/resend-verification", controllers.ResendVerificationCode)
	auth.Get("/users/overview", middlewares.Protected(), controllers.GetUserActivityOverview)
	auth.Post("/login", controllers.LoginUser)

	//
	// Questions
	questions := api.Group("/questions")
	questions.Post("/", middlewares.Protected(), controllers.CreateQuestion)
	questions.Get("/", controllers.GetQuestions)

	//questions.Patch("/:id", middlewares.Protected(), controllers.EditQuestion)
	questions.Get("/:id", middlewares.Protected(), controllers.GetQuestionByID)
	questions.Delete("/:id", middlewares.Protected(), controllers.DeleteQuestion)

	questionSet := api.Group("/questionsets")
	questionSet.Post("/", middlewares.Protected(), controllers.CreateQuestionSet)
	questionSet.Get("/", controllers.GetQuestionSets)
	//questionSet.Patch("/:id", middlewares.Protected(), controllers.EditQuestionSet)
	questionSet.Get("/:id", controllers.GetQuestionSetByID)
	//questionSet.Delete("/:id", middlewares.Protected(), controllers.DeleteQuestionSet)

	testSession := api.Group("/test_session")
	testSession.Post("/", middlewares.Protected(), controllers.CreateTestSession)
	//testSession.Get("/", middlewares.Protected(), controllers.GetTestSessionByUserID)
	testSession.Put("/finish/:test_session_id", middlewares.Protected(), controllers.FinishTestSession)
	testSession.Get("/history", middlewares.Protected(), controllers.GetTestHistory)
	testSession.Put("/:test_session_id", middlewares.Protected(), controllers.UpdateTestSession)
	testSession.Get("/:test_session_id", middlewares.Protected(), controllers.GetTestSession)

}
