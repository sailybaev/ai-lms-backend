package services

import (
	"errors"
	"fmt"

	"github.com/yourusername/ai-lms-backend/internal/models"
	"gorm.io/gorm"
)

var (
	ErrOrgNotFound  = errors.New("organization not found")
	ErrUserNotFound = errors.New("user not found")
	ErrForbidden    = errors.New("insufficient role")
)

// GetOrgAndVerifyRole finds the org by slug, finds the user by email,
// and checks that the user has at least one of the allowed roles in that org.
func GetOrgAndVerifyRole(db *gorm.DB, orgSlug, userEmail string, roles ...string) (*models.Organization, *models.User, error) {
	var org models.Organization
	if err := db.Where("slug = ?", orgSlug).First(&org).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrOrgNotFound
		}
		return nil, nil, fmt.Errorf("database error: %w", err)
	}

	var user models.User
	if err := db.Where("email = ?", userEmail).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrUserNotFound
		}
		return nil, nil, fmt.Errorf("database error: %w", err)
	}

	if len(roles) == 0 {
		return &org, &user, nil
	}

	var membership models.Membership
	if err := db.Where("org_id = ? AND user_id = ?", org.ID, user.ID).First(&membership).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrForbidden
		}
		return nil, nil, fmt.Errorf("database error: %w", err)
	}

	for _, role := range roles {
		if string(membership.Role) == role {
			return &org, &user, nil
		}
	}

	return nil, nil, ErrForbidden
}

// GetOrgAndMembership returns org, user, and membership — no role check.
func GetOrgAndMembership(db *gorm.DB, orgSlug, userEmail string) (*models.Organization, *models.User, *models.Membership, error) {
	var org models.Organization
	if err := db.Where("slug = ?", orgSlug).First(&org).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, ErrOrgNotFound
		}
		return nil, nil, nil, fmt.Errorf("database error: %w", err)
	}

	var user models.User
	if err := db.Where("email = ?", userEmail).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, ErrUserNotFound
		}
		return nil, nil, nil, fmt.Errorf("database error: %w", err)
	}

	var membership models.Membership
	if err := db.Where("org_id = ? AND user_id = ?", org.ID, user.ID).First(&membership).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, ErrForbidden
		}
		return nil, nil, nil, fmt.Errorf("database error: %w", err)
	}

	return &org, &user, &membership, nil
}
