package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// JSONB is a helper type for storing JSON in PostgreSQL
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	b, err := json.Marshal(j)
	return string(b), err
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("unsupported type: %T", value)
	}
	return json.Unmarshal(bytes, j)
}

// Organization represents a tenant organization
type Organization struct {
	ID           string     `gorm:"type:uuid;primaryKey" json:"id"`
	Slug         string     `gorm:"uniqueIndex;not null" json:"slug"`
	Name         string     `gorm:"not null" json:"name"`
	LogoURL      *string    `gorm:"column:logo_url" json:"logoUrl,omitempty"`
	PlatformName *string    `gorm:"column:platform_name" json:"platformName,omitempty"`
	SettingsJSON *string    `gorm:"column:settings_json;type:text" json:"settingsJson,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	Domains      []OrganizationDomain `gorm:"foreignKey:OrgID" json:"domains,omitempty"`
	Memberships  []Membership         `gorm:"foreignKey:OrgID" json:"memberships,omitempty"`
}

func (o *Organization) BeforeCreate(tx *gorm.DB) error {
	if o.ID == "" {
		o.ID = uuid.New().String()
	}
	return nil
}

// OrganizationDomain maps domains to organizations
type OrganizationDomain struct {
	ID         string     `gorm:"type:uuid;primaryKey" json:"id"`
	OrgID      string     `gorm:"type:uuid;not null;column:org_id" json:"orgId"`
	Domain     string     `gorm:"uniqueIndex;not null" json:"domain"`
	VerifiedAt *time.Time `gorm:"column:verified_at" json:"verifiedAt,omitempty"`
	Org        *Organization `gorm:"foreignKey:OrgID" json:"org,omitempty"`
}

func (o *OrganizationDomain) BeforeCreate(tx *gorm.DB) error {
	if o.ID == "" {
		o.ID = uuid.New().String()
	}
	return nil
}

// User represents a platform user
type User struct {
	ID           string     `gorm:"type:uuid;primaryKey" json:"id"`
	Email        string     `gorm:"uniqueIndex;not null" json:"email"`
	Name         string     `gorm:"not null" json:"name"`
	AvatarURL    *string    `gorm:"column:avatar_url" json:"avatarUrl,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	LastActiveAt *time.Time `gorm:"column:last_active_at" json:"lastActiveAt,omitempty"`
	PasswordHash *string    `gorm:"column:password_hash" json:"-"`
	IsSuperAdmin bool       `gorm:"column:is_super_admin;default:false" json:"isSuperAdmin"`
	Memberships  []Membership `gorm:"foreignKey:UserID" json:"memberships,omitempty"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}

// Role enum
type Role string

const (
	RoleAdmin   Role = "admin"
	RoleTeacher Role = "teacher"
	RoleStudent Role = "student"
)

// MembershipStatus enum
type MembershipStatus string

const (
	MembershipStatusActive    MembershipStatus = "active"
	MembershipStatusInvited   MembershipStatus = "invited"
	MembershipStatusSuspended MembershipStatus = "suspended"
)

// Membership links users to organizations
type Membership struct {
	ID        string           `gorm:"type:uuid;primaryKey" json:"id"`
	OrgID     string           `gorm:"type:uuid;not null;column:org_id" json:"orgId"`
	UserID    string           `gorm:"type:uuid;not null;column:user_id" json:"userId"`
	Role      Role             `gorm:"type:varchar(20);not null" json:"role"`
	Status    MembershipStatus `gorm:"type:varchar(20);default:'active'" json:"status"`
	CreatedAt time.Time        `json:"createdAt"`
	Org       *Organization    `gorm:"foreignKey:OrgID" json:"org,omitempty"`
	User      *User            `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (m *Membership) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}

// CourseStatus enum
type CourseStatus string

const (
	CourseStatusDraft    CourseStatus = "draft"
	CourseStatusActive   CourseStatus = "active"
	CourseStatusArchived CourseStatus = "archived"
)

// Course represents a learning course
type Course struct {
	ID           string       `gorm:"type:uuid;primaryKey" json:"id"`
	OrgID        string       `gorm:"type:uuid;not null;column:org_id" json:"orgId"`
	Title        string       `gorm:"not null" json:"title"`
	Description  *string      `json:"description,omitempty"`
	ThumbnailURL *string      `gorm:"column:thumbnail_url" json:"thumbnailUrl,omitempty"`
	Status       CourseStatus `gorm:"type:varchar(20);default:'draft'" json:"status"`
	CreatedBy    string       `gorm:"type:uuid;not null;column:created_by" json:"createdBy"`
	CreatedAt    time.Time    `json:"createdAt"`
	Org          *Organization      `gorm:"foreignKey:OrgID" json:"org,omitempty"`
	Creator      *User              `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Instructors  []CourseInstructor `gorm:"foreignKey:CourseID" json:"instructors,omitempty"`
	Sections     []CourseSection    `gorm:"foreignKey:CourseID" json:"sections,omitempty"`
	Enrollments  []Enrollment       `gorm:"foreignKey:CourseID" json:"enrollments,omitempty"`
}

func (c *Course) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}

// CourseInstructor links instructors to courses
type CourseInstructor struct {
	ID       string `gorm:"type:uuid;primaryKey" json:"id"`
	CourseID string `gorm:"type:uuid;not null;column:course_id;uniqueIndex:idx_course_instructor" json:"courseId"`
	UserID   string `gorm:"type:uuid;not null;column:user_id;uniqueIndex:idx_course_instructor" json:"userId"`
	Course   *Course `gorm:"foreignKey:CourseID" json:"course,omitempty"`
	User     *User   `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (ci *CourseInstructor) BeforeCreate(tx *gorm.DB) error {
	if ci.ID == "" {
		ci.ID = uuid.New().String()
	}
	return nil
}

// CourseSection represents a section within a course
type CourseSection struct {
	ID       string   `gorm:"type:uuid;primaryKey" json:"id"`
	CourseID string   `gorm:"type:uuid;not null;column:course_id" json:"courseId"`
	Title    string   `gorm:"not null" json:"title"`
	Position int      `gorm:"not null;default:0" json:"position"`
	Course   *Course  `gorm:"foreignKey:CourseID" json:"course,omitempty"`
	Lessons  []Lesson `gorm:"foreignKey:SectionID" json:"lessons,omitempty"`
}

func (cs *CourseSection) BeforeCreate(tx *gorm.DB) error {
	if cs.ID == "" {
		cs.ID = uuid.New().String()
	}
	return nil
}

// Lesson represents a lesson within a section
type Lesson struct {
	ID              string    `gorm:"type:uuid;primaryKey" json:"id"`
	SectionID       string    `gorm:"type:uuid;not null;column:section_id" json:"sectionId"`
	Title           string    `gorm:"not null" json:"title"`
	ContentRichtext *JSONB    `gorm:"type:jsonb;column:content_richtext" json:"contentRichtext,omitempty"`
	VideoURL        *string   `gorm:"column:video_url" json:"videoUrl,omitempty"`
	Position        int       `gorm:"not null;default:0" json:"position"`
	DurationMinutes *int      `gorm:"column:duration_minutes" json:"durationMinutes,omitempty"`
	Section         *CourseSection `gorm:"foreignKey:SectionID" json:"section,omitempty"`
}

func (l *Lesson) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return nil
}

// EnrollmentStatus enum
type EnrollmentStatus string

const (
	EnrollmentStatusActive    EnrollmentStatus = "active"
	EnrollmentStatusCompleted EnrollmentStatus = "completed"
	EnrollmentStatusDropped   EnrollmentStatus = "dropped"
)

// Enrollment links students to courses
type Enrollment struct {
	ID        string           `gorm:"type:uuid;primaryKey" json:"id"`
	OrgID     string           `gorm:"type:uuid;not null;column:org_id" json:"orgId"`
	CourseID  string           `gorm:"type:uuid;not null;column:course_id;uniqueIndex:idx_enrollment" json:"courseId"`
	UserID    string           `gorm:"type:uuid;not null;column:user_id;uniqueIndex:idx_enrollment" json:"userId"`
	Status    EnrollmentStatus `gorm:"type:varchar(20);default:'active'" json:"status"`
	CreatedAt time.Time        `json:"createdAt"`
	Org       *Organization `gorm:"foreignKey:OrgID" json:"org,omitempty"`
	Course    *Course       `gorm:"foreignKey:CourseID" json:"course,omitempty"`
	User      *User         `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (e *Enrollment) BeforeCreate(tx *gorm.DB) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	return nil
}

// AssignmentType enum
type AssignmentType string

const (
	AssignmentTypeEssay   AssignmentType = "essay"
	AssignmentTypeQuiz    AssignmentType = "quiz"
	AssignmentTypeProject AssignmentType = "project"
)

// Assignment represents a course assignment
type Assignment struct {
	ID                  string         `gorm:"type:uuid;primaryKey" json:"id"`
	OrgID               string         `gorm:"type:uuid;not null;column:org_id" json:"orgId"`
	CourseID            string         `gorm:"type:uuid;not null;column:course_id" json:"courseId"`
	Title               string         `gorm:"not null" json:"title"`
	DescriptionRichtext *JSONB         `gorm:"type:jsonb;column:description_richtext" json:"descriptionRichtext,omitempty"`
	DueAt               *time.Time     `gorm:"column:due_at" json:"dueAt,omitempty"`
	MaxPoints           *float64       `gorm:"column:max_points" json:"maxPoints,omitempty"`
	Type                AssignmentType `gorm:"type:varchar(20);not null" json:"type"`
	Org                 *Organization `gorm:"foreignKey:OrgID" json:"org,omitempty"`
	Course              *Course       `gorm:"foreignKey:CourseID" json:"course,omitempty"`
	Submissions         []Submission  `gorm:"foreignKey:AssignmentID" json:"submissions,omitempty"`
}

func (a *Assignment) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

// Submission represents a student submission
type Submission struct {
	ID            string     `gorm:"type:uuid;primaryKey" json:"id"`
	AssignmentID  string     `gorm:"type:uuid;not null;column:assignment_id" json:"assignmentId"`
	UserID        string     `gorm:"type:uuid;not null;column:user_id" json:"userId"`
	SubmittedAt   time.Time  `gorm:"column:submitted_at" json:"submittedAt"`
	ContentJSON   *JSONB     `gorm:"type:jsonb;column:content_json" json:"contentJson,omitempty"`
	AttachmentURL *string    `gorm:"column:attachment_url" json:"attachmentUrl,omitempty"`
	Assignment    *Assignment `gorm:"foreignKey:AssignmentID" json:"assignment,omitempty"`
	User          *User       `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Grade         *Grade      `gorm:"foreignKey:SubmissionID" json:"grade,omitempty"`
}

func (s *Submission) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}

// Grade represents a grade for a submission
type Grade struct {
	ID              string     `gorm:"type:uuid;primaryKey" json:"id"`
	SubmissionID    string     `gorm:"type:uuid;not null;uniqueIndex;column:submission_id" json:"submissionId"`
	GradedBy        string     `gorm:"type:uuid;not null;column:graded_by" json:"gradedBy"`
	Score           float64    `gorm:"not null" json:"score"`
	FeedbackRichtext *JSONB    `gorm:"type:jsonb;column:feedback_richtext" json:"feedbackRichtext,omitempty"`
	GradedAt        time.Time  `gorm:"column:graded_at" json:"gradedAt"`
	Submission      *Submission `gorm:"foreignKey:SubmissionID" json:"submission,omitempty"`
	Grader          *User       `gorm:"foreignKey:GradedBy" json:"grader,omitempty"`
}

func (g *Grade) BeforeCreate(tx *gorm.DB) error {
	if g.ID == "" {
		g.ID = uuid.New().String()
	}
	return nil
}

// Group represents a student group
type Group struct {
	ID                string  `gorm:"type:uuid;primaryKey" json:"id"`
	OrgID             string  `gorm:"type:uuid;not null;column:org_id" json:"orgId"`
	CourseID          *string `gorm:"type:uuid;column:course_id" json:"courseId,omitempty"`
	AssignedTeacherID *string `gorm:"type:uuid;column:assigned_teacher_id" json:"assignedTeacherId,omitempty"`
	Name              string  `gorm:"not null" json:"name"`
	Description       *string `json:"description,omitempty"`
	Org             *Organization `gorm:"foreignKey:OrgID" json:"org,omitempty"`
	Course          *Course       `gorm:"foreignKey:CourseID" json:"course,omitempty"`
	AssignedTeacher *User         `gorm:"foreignKey:AssignedTeacherID" json:"assignedTeacher,omitempty"`
	Members         []GroupMember `gorm:"foreignKey:GroupID" json:"members,omitempty"`
}

func (g *Group) BeforeCreate(tx *gorm.DB) error {
	if g.ID == "" {
		g.ID = uuid.New().String()
	}
	return nil
}

// GroupMember links users to groups
type GroupMember struct {
	ID      string `gorm:"type:uuid;primaryKey" json:"id"`
	GroupID string `gorm:"type:uuid;not null;column:group_id;uniqueIndex:idx_group_member" json:"groupId"`
	UserID  string `gorm:"type:uuid;not null;column:user_id;uniqueIndex:idx_group_member" json:"userId"`
	Group   *Group `gorm:"foreignKey:GroupID" json:"group,omitempty"`
	User    *User  `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (gm *GroupMember) BeforeCreate(tx *gorm.DB) error {
	if gm.ID == "" {
		gm.ID = uuid.New().String()
	}
	return nil
}

// ProgressEventType enum
type ProgressEventType string

const (
	ProgressEventTypeViewedLesson        ProgressEventType = "viewed_lesson"
	ProgressEventTypeCompletedAssignment ProgressEventType = "completed_assignment"
	ProgressEventTypeLogin               ProgressEventType = "login"
	ProgressEventTypeAIUsage             ProgressEventType = "ai_usage"
)

// ProgressEvent tracks user activity
type ProgressEvent struct {
	ID           string            `gorm:"type:uuid;primaryKey" json:"id"`
	OrgID        string            `gorm:"type:uuid;not null;column:org_id" json:"orgId"`
	UserID       string            `gorm:"type:uuid;not null;column:user_id" json:"userId"`
	CourseID     *string           `gorm:"type:uuid;column:course_id" json:"courseId,omitempty"`
	LessonID     *string           `gorm:"type:uuid;column:lesson_id" json:"lessonId,omitempty"`
	Type         ProgressEventType `gorm:"type:varchar(50);not null" json:"type"`
	MetadataJSON *JSONB            `gorm:"type:jsonb;column:metadata_json" json:"metadataJson,omitempty"`
	OccurredAt   time.Time         `gorm:"column:occurred_at" json:"occurredAt"`
	Org    *Organization `gorm:"foreignKey:OrgID" json:"org,omitempty"`
	User   *User         `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Course *Course       `gorm:"foreignKey:CourseID" json:"course,omitempty"`
	Lesson *Lesson       `gorm:"foreignKey:LessonID" json:"lesson,omitempty"`
}

func (pe *ProgressEvent) BeforeCreate(tx *gorm.DB) error {
	if pe.ID == "" {
		pe.ID = uuid.New().String()
	}
	return nil
}

// Sender enum for AI chat
type Sender string

const (
	SenderUser      Sender = "user"
	SenderAssistant Sender = "assistant"
	SenderSystem    Sender = "system"
)

// AIChatSession represents an AI chat session
type AIChatSession struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	OrgID     string    `gorm:"type:uuid;not null;column:org_id" json:"orgId"`
	UserID    string    `gorm:"type:uuid;not null;column:user_id" json:"userId"`
	CourseID  *string   `gorm:"type:uuid;column:course_id" json:"courseId,omitempty"`
	LessonID  *string   `gorm:"type:uuid;column:lesson_id" json:"lessonId,omitempty"`
	Title     *string   `json:"title,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	Org      *Organization `gorm:"foreignKey:OrgID" json:"org,omitempty"`
	User     *User         `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Messages []AIMessage   `gorm:"foreignKey:SessionID" json:"messages,omitempty"`
}

func (s *AIChatSession) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}

// AIMessage represents a message in an AI chat session
type AIMessage struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	SessionID string    `gorm:"type:uuid;not null;column:session_id" json:"sessionId"`
	Sender    Sender    `gorm:"type:varchar(20);not null" json:"sender"`
	Content   string    `gorm:"not null;type:text" json:"content"`
	Tokens    *int      `json:"tokens,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	Session   *AIChatSession `gorm:"foreignKey:SessionID" json:"session,omitempty"`
}

func (m *AIMessage) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}

// FileKind enum
type FileKind string

const (
	FileKindAssignmentAttachment FileKind = "assignment_attachment"
	FileKindSubmissionAttachment FileKind = "submission_attachment"
	FileKindLessonAsset          FileKind = "lesson_asset"
)

// File represents an uploaded file
type File struct {
	ID         string    `gorm:"type:uuid;primaryKey" json:"id"`
	OrgID      string    `gorm:"type:uuid;not null;column:org_id" json:"orgId"`
	UploaderID string    `gorm:"type:uuid;not null;column:uploader_id" json:"uploaderId"`
	URL        string    `gorm:"not null" json:"url"`
	Kind       FileKind  `gorm:"type:varchar(50);not null" json:"kind"`
	CreatedAt  time.Time `json:"createdAt"`
	Org      *Organization `gorm:"foreignKey:OrgID" json:"org,omitempty"`
	Uploader *User         `gorm:"foreignKey:UploaderID" json:"uploader,omitempty"`
}

func (f *File) BeforeCreate(tx *gorm.DB) error {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	return nil
}
