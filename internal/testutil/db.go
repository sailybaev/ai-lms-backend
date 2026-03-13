package testutil

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/yourusername/ai-lms-backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewTestDB creates an in-memory SQLite database with all tables migrated.
// It is automatically cleaned up when the test finishes.
func NewTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	err = db.AutoMigrate(
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
		t.Fatalf("failed to migrate test database: %v", err)
	}

	return db
}
