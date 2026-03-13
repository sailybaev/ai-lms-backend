package database

import (
	"log"

	"github.com/yourusername/ai-lms-backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Database connected successfully")
	return db
}

func AutoMigrate(db *gorm.DB) {
	err := db.AutoMigrate(
		&models.Organization{},
		&models.OrganizationDomain{},
		&models.User{},
		&models.Membership{},
		&models.Course{},
		&models.CourseInstructor{},
		&models.CourseSection{},
		&models.Lesson{},
		&models.Enrollment{},
		&models.Assignment{},
		&models.Submission{},
		&models.Grade{},
		&models.Group{},
		&models.GroupMember{},
		&models.ProgressEvent{},
		&models.AIChatSession{},
		&models.AIMessage{},
		&models.File{},
	)
	if err != nil {
		log.Fatalf("Failed to auto-migrate database: %v", err)
	}

	log.Println("Database migration completed")
}
