package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ── inline models (mirrors internal/models) ──────────────────────────────────

type Organization struct {
	ID           string    `gorm:"primaryKey;type:uuid"`
	Slug         string    `gorm:"uniqueIndex;column:slug;not null"`
	Name         string    `gorm:"not null"`
	LogoURL      *string   `gorm:"column:logo_url"`
	PlatformName *string   `gorm:"column:platform_name"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
}

type OrganizationDomain struct {
	ID         string    `gorm:"primaryKey;type:uuid"`
	OrgID      string    `gorm:"column:org_id;not null"`
	Domain     string    `gorm:"uniqueIndex;not null"`
	VerifiedAt *time.Time `gorm:"column:verified_at"`
}

type User struct {
	ID           string     `gorm:"primaryKey;type:uuid"`
	Email        string     `gorm:"uniqueIndex;not null"`
	Name         string     `gorm:"not null"`
	AvatarURL    *string    `gorm:"column:avatar_url"`
	PasswordHash *string    `gorm:"column:password_hash"`
	IsSuperAdmin bool       `gorm:"column:is_super_admin;default:false"`
	CreatedAt    time.Time  `gorm:"column:created_at;autoCreateTime"`
	LastActiveAt *time.Time `gorm:"column:last_active_at"`
}

type Membership struct {
	ID     string `gorm:"primaryKey;type:uuid"`
	OrgID  string `gorm:"column:org_id;uniqueIndex:idx_org_user;not null"`
	UserID string `gorm:"column:user_id;uniqueIndex:idx_org_user;not null"`
	Role   string `gorm:"not null"`
	Status string `gorm:"default:active"`
}

type Course struct {
	ID           string    `gorm:"primaryKey;type:uuid"`
	OrgID        string    `gorm:"column:org_id;not null"`
	Title        string    `gorm:"not null"`
	Description  *string
	ThumbnailURL *string   `gorm:"column:thumbnail_url"`
	Status       string    `gorm:"default:draft"`
	CreatedByID  string    `gorm:"column:created_by;not null"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
}

type CourseInstructor struct {
	ID       string `gorm:"primaryKey;type:uuid"`
	CourseID string `gorm:"column:course_id;uniqueIndex:idx_course_instructor;not null"`
	UserID   string `gorm:"column:user_id;uniqueIndex:idx_course_instructor;not null"`
}

type CourseSection struct {
	ID       string `gorm:"primaryKey;type:uuid"`
	CourseID string `gorm:"column:course_id;not null"`
	Title    string `gorm:"not null"`
	Position int    `gorm:"not null"`
}

type Lesson struct {
	ID         string  `gorm:"primaryKey;type:uuid"`
	SectionID  string  `gorm:"column:section_id;not null"`
	Title      string  `gorm:"not null"`
	VideoURL   *string `gorm:"column:video_url"`
	Position   int     `gorm:"not null"`
	Duration   *int    `gorm:"column:duration_minutes"`
}

type Enrollment struct {
	ID       string `gorm:"primaryKey;type:uuid"`
	OrgID    string `gorm:"column:org_id;not null"`
	CourseID string `gorm:"column:course_id;uniqueIndex:idx_course_user_enroll;not null"`
	UserID   string `gorm:"column:user_id;uniqueIndex:idx_course_user_enroll;not null"`
	Status   string `gorm:"default:active"`
}

type Assignment struct {
	ID        string     `gorm:"primaryKey;type:uuid"`
	OrgID     string     `gorm:"column:org_id;not null"`
	CourseID  string     `gorm:"column:course_id;not null"`
	Title     string     `gorm:"not null"`
	MaxPoints *int       `gorm:"column:max_points"`
	Type      string     `gorm:"not null"`
	DueAt     *time.Time `gorm:"column:due_at"`
}

type Submission struct {
	ID           string    `gorm:"primaryKey;type:uuid"`
	AssignmentID string    `gorm:"column:assignment_id;not null"`
	UserID       string    `gorm:"column:user_id;not null"`
	SubmittedAt  time.Time `gorm:"column:submitted_at;autoCreateTime"`
}

type Grade struct {
	ID           string    `gorm:"primaryKey;type:uuid"`
	SubmissionID string    `gorm:"uniqueIndex;column:submission_id;not null"`
	GradedByID   string    `gorm:"column:graded_by;not null"`
	Score        int       `gorm:"not null"`
	GradedAt     time.Time `gorm:"column:graded_at;autoCreateTime"`
}

type Group struct {
	ID                 string  `gorm:"primaryKey;type:uuid"`
	OrgID              string  `gorm:"column:org_id;not null"`
	CourseID           *string `gorm:"column:course_id"`
	AssignedTeacherID  *string `gorm:"column:assigned_teacher_id"`
	Name               string  `gorm:"not null"`
	Description        *string
}

type GroupMember struct {
	ID      string `gorm:"primaryKey;type:uuid"`
	GroupID string `gorm:"column:group_id;uniqueIndex:idx_group_member;not null"`
	UserID  string `gorm:"column:user_id;uniqueIndex:idx_group_member;not null"`
}

type ProgressEvent struct {
	ID         string    `gorm:"primaryKey;type:uuid"`
	OrgID      string    `gorm:"column:org_id;not null"`
	UserID     string    `gorm:"column:user_id;not null"`
	CourseID   *string   `gorm:"column:course_id"`
	LessonID   *string   `gorm:"column:lesson_id"`
	Type       string    `gorm:"not null"`
	OccurredAt time.Time `gorm:"column:occurred_at;autoCreateTime"`
}

// ─────────────────────────────────────────────────────────────────────────────

func newID() string { return uuid.NewString() }

func hashPassword(pw string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), 10)
	if err != nil {
		log.Fatalf("bcrypt: %v", err)
	}
	return string(h)
}

func ptr[T any](v T) *T { return &v }

func daysAgo(n int) time.Time {
	return time.Now().AddDate(0, 0, -n)
}

func daysFromNow(n int) time.Time {
	return time.Now().AddDate(0, 0, n)
}

func upsertUser(db *gorm.DB, email, name, password string, isSuperAdmin bool) User {
	hash := hashPassword(password)
	u := User{}
	db.Where("email = ?", email).First(&u)
	if u.ID == "" {
		u = User{
			ID:           newID(),
			Email:        email,
			Name:         name,
			PasswordHash: &hash,
			IsSuperAdmin: isSuperAdmin,
		}
		db.Create(&u)
	} else {
		db.Model(&u).Updates(map[string]any{"password_hash": hash, "is_super_admin": isSuperAdmin})
	}
	return u
}

func upsertMembership(db *gorm.DB, orgID, userID, role string) {
	m := Membership{}
	db.Where("org_id = ? AND user_id = ?", orgID, userID).First(&m)
	if m.ID == "" {
		db.Create(&Membership{ID: newID(), OrgID: orgID, UserID: userID, Role: role, Status: "active"})
	} else {
		db.Model(&m).Update("role", role)
	}
}

func main() {
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}

	fmt.Println("🌱 Seeding database...")

	// ── Super admin ─────────────────────────────────────────────────────────
	superAdmin := upsertUser(db, "sailybaevvv@gmail.com", "Alikhan Sailybaev", "Admin123!", true)
	fmt.Println("✅ Super admin:", superAdmin.Email)

	// ── Organization ────────────────────────────────────────────────────────
	org := Organization{}
	db.Where("slug = ?", "demo").First(&org)
	if org.ID == "" {
		org = Organization{
			ID:           newID(),
			Slug:         "demo",
			Name:         "Demo University",
			PlatformName: ptr("Demo Learning Platform"),
		}
		db.Create(&org)
		db.Create(&OrganizationDomain{ID: newID(), OrgID: org.ID, Domain: "demo.localhost"})
	}
	fmt.Println("✅ Organization:", org.Slug)

	upsertMembership(db, org.ID, superAdmin.ID, "admin")
	fmt.Println("✅ Super admin added as org admin")

	// ── Teachers ─────────────────────────────────────────────────────────────
	teacher := upsertUser(db, "teacher@demo.edu", "Sarah Johnson", "Teacher123!", false)
	upsertMembership(db, org.ID, teacher.ID, "teacher")

	teacher2 := upsertUser(db, "teacher2@demo.edu", "Michael Brown", "Teacher123!", false)
	upsertMembership(db, org.ID, teacher2.ID, "teacher")
	fmt.Println("✅ Teachers created")

	// ── Students ─────────────────────────────────────────────────────────────
	type studentDef struct{ email, name string }
	studentDefs := []studentDef{
		{"alice@demo.edu", "Alice Chen"},
		{"bob@demo.edu", "Bob Martinez"},
		{"carol@demo.edu", "Carol White"},
		{"david@demo.edu", "David Kim"},
		{"emma@demo.edu", "Emma Wilson"},
	}
	students := make([]User, len(studentDefs))
	for i, s := range studentDefs {
		students[i] = upsertUser(db, s.email, s.name, "Student123!", false)
		upsertMembership(db, org.ID, students[i].ID, "student")
	}
	fmt.Println("✅ Students created")

	// ── Courses ───────────────────────────────────────────────────────────────
	makeOrSkipCourse := func(title string) Course {
		c := Course{}
		db.Where("org_id = ? AND title = ?", org.ID, title).First(&c)
		return c
	}

	course1 := makeOrSkipCourse("Introduction to Web Development")
	if course1.ID == "" {
		course1 = Course{ID: newID(), OrgID: org.ID, Title: "Introduction to Web Development", Description: ptr("Learn the fundamentals of HTML, CSS, and JavaScript."), Status: "active", CreatedByID: teacher.ID}
		db.Create(&course1)
		db.Create(&CourseInstructor{ID: newID(), CourseID: course1.ID, UserID: teacher.ID})
	}

	course2 := makeOrSkipCourse("Advanced React & TypeScript")
	if course2.ID == "" {
		course2 = Course{ID: newID(), OrgID: org.ID, Title: "Advanced React & TypeScript", Description: ptr("Master React hooks, context API, and TypeScript."), Status: "active", CreatedByID: teacher.ID}
		db.Create(&course2)
		db.Create(&CourseInstructor{ID: newID(), CourseID: course2.ID, UserID: teacher.ID})
		db.Create(&CourseInstructor{ID: newID(), CourseID: course2.ID, UserID: teacher2.ID})
	}

	course3 := makeOrSkipCourse("Database Design & SQL")
	if course3.ID == "" {
		course3 = Course{ID: newID(), OrgID: org.ID, Title: "Database Design & SQL", Description: ptr("Relational databases, SQL queries, and schema design."), Status: "active", CreatedByID: teacher2.ID}
		db.Create(&course3)
		db.Create(&CourseInstructor{ID: newID(), CourseID: course3.ID, UserID: teacher2.ID})
	}

	course4 := makeOrSkipCourse("Python for Data Science")
	if course4.ID == "" {
		course4 = Course{ID: newID(), OrgID: org.ID, Title: "Python for Data Science", Description: ptr("Python, NumPy, Pandas, and data visualisation."), Status: "draft", CreatedByID: teacher2.ID}
		db.Create(&course4)
	}
	fmt.Println("✅ Courses created")

	// ── Sections & Lessons ────────────────────────────────────────────────────
	makeSection := func(courseID, title string, pos int) CourseSection {
		s := CourseSection{}
		db.Where("course_id = ? AND title = ?", courseID, title).First(&s)
		if s.ID == "" {
			s = CourseSection{ID: newID(), CourseID: courseID, Title: title, Position: pos}
			db.Create(&s)
		}
		return s
	}
	makeLesson := func(sectionID, title string, pos, dur int) Lesson {
		l := Lesson{}
		db.Where("section_id = ? AND title = ?", sectionID, title).First(&l)
		if l.ID == "" {
			l = Lesson{ID: newID(), SectionID: sectionID, Title: title, Position: pos, Duration: ptr(dur)}
			db.Create(&l)
		}
		return l
	}

	c1s1 := makeSection(course1.ID, "Getting Started with HTML", 1)
	c1s2 := makeSection(course1.ID, "Styling with CSS", 2)
	c1s3 := makeSection(course1.ID, "JavaScript Basics", 3)
	l1_1 := makeLesson(c1s1.ID, "What is HTML?", 1, 15)
	l1_2 := makeLesson(c1s1.ID, "HTML Tags & Structure", 2, 30)
	l1_3 := makeLesson(c1s1.ID, "Forms & Inputs", 3, 25)
	l1_4 := makeLesson(c1s2.ID, "CSS Selectors", 1, 20)
	l1_5 := makeLesson(c1s2.ID, "Box Model & Layout", 2, 35)
	makeLesson(c1s2.ID, "Flexbox & Grid", 3, 40)
	l1_7 := makeLesson(c1s3.ID, "Variables & Data Types", 1, 25)
	makeLesson(c1s3.ID, "Functions & Scope", 2, 30)

	c2s1 := makeSection(course2.ID, "React Fundamentals", 1)
	c2s2 := makeSection(course2.ID, "Hooks Deep Dive", 2)
	l2_1 := makeLesson(c2s1.ID, "Components & Props", 1, 30)
	l2_2 := makeLesson(c2s1.ID, "State Management", 2, 40)
	l2_3 := makeLesson(c2s2.ID, "useState & useEffect", 1, 45)
	makeLesson(c2s2.ID, "Custom Hooks", 2, 35)

	c3s1 := makeSection(course3.ID, "SQL Fundamentals", 1)
	l3_1 := makeLesson(c3s1.ID, "SELECT Queries", 1, 25)
	l3_2 := makeLesson(c3s1.ID, "JOINs Explained", 2, 40)
	makeLesson(c3s1.ID, "Indexes & Performance", 3, 35)

	fmt.Println("✅ Sections & lessons created")

	// ── Enrollments ───────────────────────────────────────────────────────────
	upsertEnrollment := func(courseID, userID, status string) {
		e := Enrollment{}
		db.Where("course_id = ? AND user_id = ?", courseID, userID).First(&e)
		if e.ID == "" {
			db.Create(&Enrollment{ID: newID(), OrgID: org.ID, CourseID: courseID, UserID: userID, Status: status})
		}
	}
	upsertEnrollment(course1.ID, students[0].ID, "active")
	upsertEnrollment(course2.ID, students[0].ID, "active")
	upsertEnrollment(course3.ID, students[0].ID, "completed")
	upsertEnrollment(course1.ID, students[1].ID, "active")
	upsertEnrollment(course3.ID, students[1].ID, "active")
	upsertEnrollment(course1.ID, students[2].ID, "completed")
	upsertEnrollment(course2.ID, students[2].ID, "active")
	upsertEnrollment(course2.ID, students[3].ID, "active")
	upsertEnrollment(course3.ID, students[3].ID, "active")
	upsertEnrollment(course1.ID, students[4].ID, "active")
	upsertEnrollment(course3.ID, students[4].ID, "active")
	fmt.Println("✅ Enrollments created")

	// ── Assignments ───────────────────────────────────────────────────────────
	makeAssignment := func(courseID, title, atype string, maxPts int, daysUntilDue int) Assignment {
		a := Assignment{}
		db.Where("course_id = ? AND title = ?", courseID, title).First(&a)
		if a.ID == "" {
			due := daysFromNow(daysUntilDue)
			a = Assignment{ID: newID(), OrgID: org.ID, CourseID: courseID, Title: title, Type: atype, MaxPoints: ptr(maxPts), DueAt: &due}
			db.Create(&a)
		}
		return a
	}
	assign1 := makeAssignment(course1.ID, "Build a Personal Portfolio Page", "project", 100, 7)
	assign2 := makeAssignment(course1.ID, "JavaScript Calculator", "project", 100, 14)
	assign3 := makeAssignment(course2.ID, "React Todo App", "project", 100, 10)
	makeAssignment(course3.ID, "SQL Query Challenge", "essay", 80, 5)
	fmt.Println("✅ Assignments created")

	// ── Submissions & Grades ──────────────────────────────────────────────────
	makeSubmission := func(assignmentID, userID string) Submission {
		s := Submission{}
		db.Where("assignment_id = ? AND user_id = ?", assignmentID, userID).First(&s)
		if s.ID == "" {
			s = Submission{ID: newID(), AssignmentID: assignmentID, UserID: userID, SubmittedAt: daysAgo(2)}
			db.Create(&s)
		}
		return s
	}
	makeGrade := func(submissionID, gradedByID string, score int) {
		g := Grade{}
		db.Where("submission_id = ?", submissionID).First(&g)
		if g.ID == "" {
			db.Create(&Grade{ID: newID(), SubmissionID: submissionID, GradedByID: gradedByID, Score: score})
		}
	}

	sub1 := makeSubmission(assign1.ID, students[0].ID)
	makeGrade(sub1.ID, teacher.ID, 92)
	sub2 := makeSubmission(assign1.ID, students[1].ID)
	makeGrade(sub2.ID, teacher.ID, 78)
	sub3 := makeSubmission(assign2.ID, students[0].ID)
	makeGrade(sub3.ID, teacher.ID, 85)
	sub4 := makeSubmission(assign3.ID, students[0].ID)
	makeGrade(sub4.ID, teacher.ID, 95)
	sub5 := makeSubmission(assign3.ID, students[2].ID)
	makeGrade(sub5.ID, teacher.ID, 88)
	fmt.Println("✅ Submissions & grades created")

	// ── Progress Events ───────────────────────────────────────────────────────
	addProgress := func(userID string, courseID, lessonID *string, eventType string) {
		offset := rand.Intn(7)
		db.Create(&ProgressEvent{
			ID: newID(), OrgID: org.ID, UserID: userID,
			CourseID: courseID, LessonID: lessonID,
			Type: eventType, OccurredAt: daysAgo(offset),
		})
	}
	addProgress(students[0].ID, &course1.ID, &l1_1.ID, "viewed_lesson")
	addProgress(students[0].ID, &course1.ID, &l1_2.ID, "viewed_lesson")
	addProgress(students[0].ID, &course1.ID, &l1_3.ID, "viewed_lesson")
	addProgress(students[0].ID, &course1.ID, &l1_4.ID, "viewed_lesson")
	addProgress(students[0].ID, &course1.ID, &l1_5.ID, "viewed_lesson")
	addProgress(students[0].ID, &course2.ID, &l2_1.ID, "viewed_lesson")
	addProgress(students[0].ID, &course2.ID, &l2_2.ID, "viewed_lesson")
	addProgress(students[0].ID, &course1.ID, nil, "completed_assignment")

	addProgress(students[1].ID, &course1.ID, &l1_1.ID, "viewed_lesson")
	addProgress(students[1].ID, &course1.ID, &l1_2.ID, "viewed_lesson")
	addProgress(students[1].ID, &course3.ID, &l3_1.ID, "viewed_lesson")
	addProgress(students[1].ID, &course1.ID, nil, "completed_assignment")

	addProgress(students[2].ID, &course1.ID, &l1_1.ID, "viewed_lesson")
	addProgress(students[2].ID, &course2.ID, &l2_1.ID, "viewed_lesson")
	addProgress(students[2].ID, &course2.ID, &l2_3.ID, "viewed_lesson")

	addProgress(students[3].ID, &course2.ID, &l2_1.ID, "viewed_lesson")
	addProgress(students[3].ID, &course3.ID, &l3_1.ID, "viewed_lesson")
	addProgress(students[3].ID, &course3.ID, &l3_2.ID, "viewed_lesson")

	addProgress(students[4].ID, &course1.ID, &l1_7.ID, "viewed_lesson")
	addProgress(students[4].ID, &course3.ID, &l3_1.ID, "viewed_lesson")

	for _, s := range append(students, teacher, teacher2, superAdmin) {
		addProgress(s.ID, nil, nil, "login")
	}
	fmt.Println("✅ Progress events created")

	// ── Groups ────────────────────────────────────────────────────────────────
	makeGroup := func(name, description string, courseID, teacherID *string, memberIDs []string) {
		g := Group{}
		db.Where("org_id = ? AND name = ?", org.ID, name).First(&g)
		if g.ID == "" {
			g = Group{ID: newID(), OrgID: org.ID, Name: name, Description: ptr(description), CourseID: courseID, AssignedTeacherID: teacherID}
			db.Create(&g)
			for _, uid := range memberIDs {
				db.Create(&GroupMember{ID: newID(), GroupID: g.ID, UserID: uid})
			}
		}
	}
	makeGroup("Web Dev - Group A", "Morning cohort", &course1.ID, &teacher.ID, []string{students[0].ID, students[1].ID, students[4].ID})
	makeGroup("React Advanced - Group A", "Advanced React students", &course2.ID, &teacher.ID, []string{students[0].ID, students[2].ID, students[3].ID})
	makeGroup("SQL Fundamentals - Group A", "Database course group", &course3.ID, &teacher2.ID, []string{students[1].ID, students[3].ID, students[4].ID})
	fmt.Println("✅ Groups created")

	fmt.Println(`
╔══════════════════════════════════════════════════════════════╗
║                    🎉 Seed Complete!                         ║
╠══════════════════════════════════════════════════════════════╣
║  Organization:  Demo University  (slug: demo)                ║
╠══════════════════════════════════════════════════════════════╣
║  SUPER ADMIN                                                 ║
║    sailybaevvv@gmail.com  /  Admin123!                       ║
╠══════════════════════════════════════════════════════════════╣
║  TEACHERS                                                    ║
║    teacher@demo.edu   /  Teacher123!  (Sarah Johnson)        ║
║    teacher2@demo.edu  /  Teacher123!  (Michael Brown)        ║
╠══════════════════════════════════════════════════════════════╣
║  STUDENTS  (password: Student123!)                           ║
║    alice@demo.edu  ·  bob@demo.edu  ·  carol@demo.edu        ║
║    david@demo.edu  ·  emma@demo.edu                          ║
╠══════════════════════════════════════════════════════════════╣
║  4 courses  ·  4 assignments  ·  3 groups  ·  15 lessons     ║
╚══════════════════════════════════════════════════════════════╝`)
}
