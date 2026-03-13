package handlers

import (
	"errors"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/ai-lms-backend/internal/models"
	"github.com/yourusername/ai-lms-backend/internal/services"
	"gorm.io/gorm"
)

type TeacherHandler struct {
	DB *gorm.DB
}

func NewTeacherHandler(db *gorm.DB) *TeacherHandler {
	return &TeacherHandler{DB: db}
}

// GetTeacherCourses handles GET /api/org/:org/teacher/courses
func (h *TeacherHandler) GetTeacherCourses(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	org, user, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "teacher", "admin")
	if err != nil {
		if errors.Is(err, services.ErrOrgNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Teacher access required"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Courses created by this teacher
	var createdCourses []models.Course
	h.DB.Where("org_id = ? AND created_by = ?", org.ID, user.ID).Find(&createdCourses)

	// Courses where this user is an instructor
	var instructorEntries []models.CourseInstructor
	h.DB.Where("user_id = ?", user.ID).Find(&instructorEntries)

	courseIDSet := make(map[string]struct{})
	for _, c := range createdCourses {
		courseIDSet[c.ID] = struct{}{}
	}
	for _, ci := range instructorEntries {
		courseIDSet[ci.CourseID] = struct{}{}
	}

	if len(courseIDSet) == 0 {
		c.JSON(http.StatusOK, gin.H{"courses": []interface{}{}})
		return
	}

	courseIDs := make([]string, 0, len(courseIDSet))
	for id := range courseIDSet {
		courseIDs = append(courseIDs, id)
	}

	var courses []models.Course
	h.DB.Where("id IN ? AND org_id = ?", courseIDs, org.ID).
		Preload("Instructors").
		Preload("Instructors.User").
		Preload("Sections").
		Preload("Sections.Lessons").
		Find(&courses)

	result := make([]gin.H, 0, len(courses))
	for _, course := range courses {
		var enrollmentCount int64
		h.DB.Model(&models.Enrollment{}).Where("course_id = ? AND status = 'active'", course.ID).Count(&enrollmentCount)

		var completedCount int64
		h.DB.Model(&models.Enrollment{}).Where("course_id = ? AND status = 'completed'", course.ID).Count(&completedCount)

		lessonCount := 0
		for _, sec := range course.Sections {
			lessonCount += len(sec.Lessons)
		}

		result = append(result, gin.H{
			"id":              course.ID,
			"title":           course.Title,
			"description":     course.Description,
			"thumbnailUrl":    course.ThumbnailURL,
			"status":          course.Status,
			"createdAt":       course.CreatedAt,
			"enrollmentCount": enrollmentCount,
			"completedCount":  completedCount,
			"lessonCount":     lessonCount,
		})
	}

	c.JSON(http.StatusOK, gin.H{"courses": result})
}

// GetTeacherGroups handles GET /api/org/:org/teacher/groups?courseId=...
func (h *TeacherHandler) GetTeacherGroups(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")
	courseID := c.Query("courseId")

	org, user, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "teacher", "admin")
	if err != nil {
		if errors.Is(err, services.ErrOrgNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Teacher access required"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	query := h.DB.Where("org_id = ? AND assigned_teacher_id = ?", org.ID, user.ID).
		Preload("Members").
		Preload("Members.User").
		Preload("Course")

	if courseID != "" {
		query = query.Where("course_id = ?", courseID)
	}

	var groups []models.Group
	if err := query.Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch groups"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"groups": groups})
}

// GetTeacherStudents handles GET /api/org/:org/teacher/students?courseId=...
func (h *TeacherHandler) GetTeacherStudents(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")
	courseIDFilter := c.Query("courseId")

	org, user, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string), "teacher", "admin")
	if err != nil {
		if errors.Is(err, services.ErrOrgNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		if errors.Is(err, services.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Teacher access required"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Step 1: Find teacher's courses
	var createdCourses []models.Course
	h.DB.Where("org_id = ? AND created_by = ?", org.ID, user.ID).Find(&createdCourses)

	var instructorEntries []models.CourseInstructor
	h.DB.Where("user_id = ?", user.ID).Find(&instructorEntries)

	courseIDSet := make(map[string]struct{})
	for _, c := range createdCourses {
		courseIDSet[c.ID] = struct{}{}
	}
	for _, ci := range instructorEntries {
		courseIDSet[ci.CourseID] = struct{}{}
	}

	if courseIDFilter != "" {
		if _, ok := courseIDSet[courseIDFilter]; !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access to course denied"})
			return
		}
		courseIDSet = map[string]struct{}{courseIDFilter: {}}
	}

	if len(courseIDSet) == 0 {
		c.JSON(http.StatusOK, gin.H{"students": []interface{}{}})
		return
	}

	courseIDs := make([]string, 0, len(courseIDSet))
	for id := range courseIDSet {
		courseIDs = append(courseIDs, id)
	}

	// Step 2: Get active enrollments
	var enrollments []models.Enrollment
	h.DB.Where("course_id IN ? AND org_id = ? AND status = 'active'", courseIDs, org.ID).
		Preload("User").
		Preload("Course").
		Find(&enrollments)

	// Step 3: Get progress events (viewed_lesson) for those students/courses
	studentIDs := make([]string, 0)
	for _, e := range enrollments {
		studentIDs = append(studentIDs, e.UserID)
	}

	var progressEvents []models.ProgressEvent
	if len(studentIDs) > 0 {
		h.DB.Where("org_id = ? AND user_id IN ? AND course_id IN ? AND type = ?",
			org.ID, studentIDs, courseIDs, models.ProgressEventTypeViewedLesson).
			Find(&progressEvents)
	}

	// Step 4: Get course sections with lessons to compute total lessons per course
	var sections []models.CourseSection
	h.DB.Where("course_id IN ?", courseIDs).Preload("Lessons").Find(&sections)

	lessonCountByCourse := make(map[string]int)
	for _, sec := range sections {
		lessonCountByCourse[sec.CourseID] += len(sec.Lessons)
	}

	// Count lesson views by user+course
	type viewKey struct{ UserID, CourseID string }
	viewCounts := make(map[viewKey]int)
	for _, pe := range progressEvents {
		if pe.CourseID != nil {
			viewCounts[viewKey{pe.UserID, *pe.CourseID}]++
		}
	}

	// Step 5: Get submissions with grades
	var submissions []models.Submission
	if len(studentIDs) > 0 {
		h.DB.Where("user_id IN ?", studentIDs).Preload("Grade").Preload("Assignment").Find(&submissions)
	}

	type gradeKey struct{ UserID, CourseID string }
	scoresByCourse := make(map[gradeKey][]float64)
	for _, sub := range submissions {
		if sub.Grade != nil && sub.Assignment != nil {
			key := gradeKey{sub.UserID, sub.Assignment.CourseID}
			scoresByCourse[key] = append(scoresByCourse[key], sub.Grade.Score)
		}
	}

	// Step 6: Deduplicate by student, compute progress + grade, sort by name
	type StudentKey struct{ UserID string }
	studentMap := make(map[string]gin.H)

	for _, enrollment := range enrollments {
		if enrollment.User == nil {
			continue
		}
		uid := enrollment.UserID
		cid := enrollment.CourseID

		// Progress %
		totalLessons := lessonCountByCourse[cid]
		viewedLessons := viewCounts[viewKey{uid, cid}]
		progress := 0.0
		if totalLessons > 0 {
			progress = float64(viewedLessons) / float64(totalLessons) * 100
			if progress > 100 {
				progress = 100
			}
		}

		// Average score -> letter grade
		scores := scoresByCourse[gradeKey{uid, cid}]
		letterGrade := "N/A"
		if len(scores) > 0 {
			sum := 0.0
			for _, s := range scores {
				sum += s
			}
			avg := sum / float64(len(scores))
			switch {
			case avg >= 90:
				letterGrade = "A"
			case avg >= 80:
				letterGrade = "B"
			case avg >= 70:
				letterGrade = "C"
			case avg >= 60:
				letterGrade = "D"
			default:
				letterGrade = "F"
			}
		}

		entry, exists := studentMap[uid]
		if !exists {
			entry = gin.H{
				"id":        uid,
				"name":      enrollment.User.Name,
				"email":     enrollment.User.Email,
				"avatarUrl": enrollment.User.AvatarURL,
				"courses":   []gin.H{},
			}
		}

		courses := entry["courses"].([]gin.H)
		courseName := ""
		if enrollment.Course != nil {
			courseName = enrollment.Course.Title
		}
		courses = append(courses, gin.H{
			"courseId":   cid,
			"courseName": courseName,
			"progress":   progress,
			"grade":      letterGrade,
			"status":     enrollment.Status,
		})
		entry["courses"] = courses
		studentMap[uid] = entry
	}

	studentList := make([]gin.H, 0, len(studentMap))
	for _, s := range studentMap {
		studentList = append(studentList, s)
	}

	sort.Slice(studentList, func(i, j int) bool {
		ni, _ := studentList[i]["name"].(string)
		nj, _ := studentList[j]["name"].(string)
		return ni < nj
	})

	c.JSON(http.StatusOK, gin.H{"students": studentList})
}
