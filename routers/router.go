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

	//Auth
	auth := api.Group("/auth")
	auth.Post("/users", controllers.CreateUser)
	auth.Get("/users", middlewares.Protected(), controllers.GetUserDetails)
	auth.Get("/google-login", controllers.GoogleLogin)
	auth.Get("/verify-oauth", middlewares.Protected(), controllers.VerifyOAuth)
	auth.Get("/google-callback", controllers.GoogleCallback)
	auth.Put("/users", middlewares.Protected(), controllers.EditUserProfile)
	auth.Post("/users/verify", controllers.VerifyUserEmail)
	auth.Post("/users/resend-verification", controllers.ResendVerificationCode)
	auth.Get("/users/overview", middlewares.Protected(), controllers.GetUserActivityOverview)
	auth.Post("/login", controllers.LoginUser)
	auth.Post("/password-reset", controllers.SendPasswordResetCode)
	auth.Post("/reset-code", controllers.VerifyPasswordResetCode)
	auth.Post("/reset-password", controllers.ResetPassword)

	questions := api.Group("/questions")
	questions.Post("/", middlewares.Protected(), controllers.CreateQuestion)
	questions.Get("/", middlewares.Protected(), controllers.GetQuestions)
	questions.Get("/:id", middlewares.Protected(), controllers.GetQuestionByID)
	questions.Delete("/:id", middlewares.Protected(), controllers.DeleteQuestion)
	questions.Put("/:id", middlewares.Protected(), controllers.EditQuestion)

	questionSet := api.Group("/questionsets")
	questionSet.Post("/", middlewares.Protected(), controllers.CreateQuestionSet)
	questionSet.Get("/", controllers.GetQuestionSets)
	questionSet.Get("/:id", controllers.GetQuestionSetByID)

	testSession := api.Group("/test_session")
	testSession.Post("/", middlewares.Protected(), controllers.CreateTestSession)
	testSession.Put("/finish/:test_session_id", middlewares.Protected(), controllers.FinishTestSession)
	testSession.Get("/history", middlewares.Protected(), controllers.GetTestHistory)
	testSession.Put("/:test_session_id", middlewares.Protected(), controllers.UpdateTestSession)
	testSession.Get("/:test_session_id", middlewares.Protected(), controllers.GetTestSession)

	bookmarks := api.Group("/bookmarks")
	bookmarks.Post("/", middlewares.Protected(), controllers.CreateBookmark)
	bookmarks.Get("/", middlewares.Protected(), controllers.GetAllBookmarks)
	bookmarks.Delete("/:qid", middlewares.Protected(), controllers.RemoveBookmark)

	explanation := api.Group("/explanations")
	explanation.Post("/", middlewares.Protected(), controllers.SaveExplanation)
	explanation.Get("/", middlewares.Protected(), controllers.GetAllSavedExplanations)
	explanation.Delete("/:qid", middlewares.Protected(), controllers.RemoveExplanation)
	explanation.Put("/:qid", middlewares.Protected(), controllers.UpdateSavedExplanation)
}
