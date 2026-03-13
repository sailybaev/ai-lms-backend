package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/yourusername/ai-lms-backend/internal/config"
	"github.com/yourusername/ai-lms-backend/internal/handlers"
	"github.com/yourusername/ai-lms-backend/internal/middleware"
	"gorm.io/gorm"
)

func Setup(router *gin.Engine, db *gorm.DB, cfg *config.Config) {
	// Static file serving for uploaded files
	router.Static("/uploads", "./public/uploads")

	// --- Instantiate handlers ---
	authHandler := handlers.NewAuthHandler(db, cfg.JWTSecret)
	orgResolveHandler := handlers.NewOrgResolveHandler(db)
	brandingHandler := handlers.NewBrandingHandler(db)
	coursesHandler := handlers.NewCoursesHandler(db)
	groupsHandler := handlers.NewGroupsHandler(db)
	studentsHandler := handlers.NewStudentsHandler(db)
	teachersHandler := handlers.NewTeachersHandler(db)
	profileHandler := handlers.NewProfileHandler(db, cfg.UploadDir)
	uploadHandler := handlers.NewUploadHandler(cfg.UploadDir)
	analyticsHandler := handlers.NewAnalyticsHandler(db)
	teacherHandler := handlers.NewTeacherHandler(db)
	superadminHandler := handlers.NewSuperadminHandler(db)
	platformOrgsHandler := handlers.NewPlatformOrgsHandler(db)
	platformUsersHandler := handlers.NewPlatformUsersHandler(db)
	aiChatHandler := handlers.NewAIChatHandler(db, cfg.AIBaseURL)

	api := router.Group("/api")

	// --- Auth routes (no middleware) ---
	authGroup := api.Group("/auth")
	{
		authGroup.POST("/login", authHandler.Login)
	}

	// --- Org resolution (no auth middleware) ---
	api.GET("/org/resolve", orgResolveHandler.Resolve)

	// --- Org-scoped routes (require JWT) ---
	orgGroup := api.Group("/org/:org")
	orgGroup.Use(middleware.JWTMiddleware(cfg.JWTSecret))
	{
		// Branding
		orgGroup.GET("/branding", brandingHandler.GetBranding)
		orgGroup.PATCH("/branding", brandingHandler.UpdateBranding)

		// Courses (admin only — enforced in handler)
		orgGroup.GET("/courses", coursesHandler.ListCourses)
		orgGroup.POST("/courses", coursesHandler.CreateCourse)
		orgGroup.PUT("/courses", coursesHandler.UpdateCourse)
		orgGroup.DELETE("/courses", coursesHandler.DeleteCourse)

		// Admin Groups
		orgGroup.GET("/admin/groups", groupsHandler.ListGroups)
		orgGroup.POST("/admin/groups", groupsHandler.CreateGroup)
		orgGroup.PUT("/admin/groups", groupsHandler.UpdateGroup)
		orgGroup.DELETE("/admin/groups", groupsHandler.DeleteGroup)

		// Admin photo upload
		orgGroup.POST("/admin/photo", uploadHandler.UploadPhoto)

		// Students
		orgGroup.GET("/students", studentsHandler.ListStudents)
		orgGroup.POST("/students", studentsHandler.CreateStudent)
		orgGroup.PUT("/students", studentsHandler.UpdateStudent)
		orgGroup.PATCH("/students", studentsHandler.PatchStudentStatus)
		orgGroup.DELETE("/students", studentsHandler.DeleteStudent)

		// Teachers
		orgGroup.GET("/teachers", teachersHandler.ListTeachers)
		orgGroup.POST("/teachers", teachersHandler.CreateTeacher)
		orgGroup.PUT("/teachers", teachersHandler.UpdateTeacher)
		orgGroup.PATCH("/teachers", teachersHandler.PatchTeacherStatus)
		orgGroup.DELETE("/teachers", teachersHandler.DeleteTeacher)

		// Profile
		orgGroup.GET("/profile", profileHandler.GetProfile)
		orgGroup.PATCH("/profile", profileHandler.UpdateProfile)
		orgGroup.DELETE("/profile", profileHandler.DeleteProfile)
		orgGroup.PATCH("/profile/password", profileHandler.ChangePassword)
		orgGroup.POST("/profile/photo", uploadHandler.UploadPhoto)

		// Analytics
		orgGroup.GET("/analytics", analyticsHandler.GetAnalytics)

		// Teacher-scoped routes
		orgGroup.GET("/teacher/courses", teacherHandler.GetTeacherCourses)
		orgGroup.GET("/teacher/groups", teacherHandler.GetTeacherGroups)
		orgGroup.GET("/teacher/students", teacherHandler.GetTeacherStudents)

		// AI Chat
		orgGroup.POST("/ai/sessions", aiChatHandler.CreateSession)
		orgGroup.GET("/ai/sessions", aiChatHandler.ListSessions)
		orgGroup.GET("/ai/sessions/:sessionId", aiChatHandler.GetSession)
		orgGroup.POST("/ai/sessions/:sessionId/messages", aiChatHandler.SendMessage)
	}

	// --- Superadmin routes (require JWT + superadmin) ---
	superadminGroup := api.Group("/superadmin")
	superadminGroup.Use(middleware.JWTMiddleware(cfg.JWTSecret))
	superadminGroup.Use(middleware.SuperadminMiddleware())
	{
		superadminGroup.GET("/stats", superadminHandler.GetStats)
		superadminGroup.GET("/users", superadminHandler.ListUsers)
		superadminGroup.GET("/admins", superadminHandler.ListAdmins)
		superadminGroup.POST("/admins", superadminHandler.CreateAdmin)
		superadminGroup.PATCH("/admins/:id", superadminHandler.PatchAdmin)
		superadminGroup.GET("/organizations/:id", superadminHandler.GetOrganization)
		superadminGroup.GET("/organizations/:id/members", superadminHandler.GetOrgMembers)
		superadminGroup.POST("/organizations/:id/members", superadminHandler.CreateOrgMember)
		superadminGroup.PATCH("/organizations/:id/members/:membershipId", superadminHandler.PatchOrgMember)
		superadminGroup.DELETE("/organizations/:id/members/:membershipId", superadminHandler.DeleteOrgMember)
	}

	// --- Platform admin routes (JWT protected for security) ---
	adminGroup := api.Group("/admin")
	adminGroup.Use(middleware.JWTMiddleware(cfg.JWTSecret))
	{
		adminGroup.GET("/orgs", platformOrgsHandler.ListOrgs)
		adminGroup.POST("/orgs", platformOrgsHandler.CreateOrg)
		adminGroup.GET("/users", platformUsersHandler.ListUsers)
		adminGroup.POST("/users", platformUsersHandler.CreateUser)
		adminGroup.GET("/users/:id", platformUsersHandler.GetUser)
		adminGroup.PATCH("/users/:id", platformUsersHandler.UpdateUser)
		adminGroup.DELETE("/users/:id", platformUsersHandler.DeleteUser)
	}
}
