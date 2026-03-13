package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/ai-lms-backend/internal/models"
	"github.com/yourusername/ai-lms-backend/internal/services"
	"gorm.io/gorm"
)

type AnalyticsHandler struct {
	DB *gorm.DB
}

func NewAnalyticsHandler(db *gorm.DB) *AnalyticsHandler {
	return &AnalyticsHandler{DB: db}
}

func parseRange(rangeParam string) (startDate, previousStartDate time.Time, days int) {
	now := time.Now()
	switch rangeParam {
	case "30d":
		days = 30
	case "90d":
		days = 90
	case "1y":
		days = 365
	default: // 7d
		days = 7
	}
	startDate = now.AddDate(0, 0, -days)
	previousStartDate = startDate.AddDate(0, 0, -days)
	return
}

type roleCount struct {
	Role  string `json:"role"`
	Count int64  `json:"count"`
}

type courseEnrollment struct {
	ID    string
	Title string
	Count int64
}

// GetAnalytics handles GET /api/org/:org/analytics?range=7d|30d|90d|1y
func (h *AnalyticsHandler) GetAnalytics(c *gin.Context) {
	orgSlug := c.Param("org")
	userEmail, _ := c.Get("userEmail")

	org, _, err := services.GetOrgAndVerifyRole(h.DB, orgSlug, userEmail.(string))
	if err != nil {
		if errors.Is(err, services.ErrOrgNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	rangeParam := c.DefaultQuery("range", "7d")
	startDate, previousStartDate, days := parseRange(rangeParam)
	now := time.Now()

	// --- totalUsers ---
	var totalUsers int64
	h.DB.Model(&models.Membership{}).Where("org_id = ?", org.ID).Count(&totalUsers)

	// --- activeUsers (users with lastActiveAt >= startDate) ---
	var activeUsers int64
	h.DB.Model(&models.User{}).
		Joins("JOIN memberships ON memberships.user_id = users.id").
		Where("memberships.org_id = ? AND users.last_active_at >= ?", org.ID, startDate).
		Count(&activeUsers)

	// --- previousActiveUsers ---
	var previousActiveUsers int64
	h.DB.Model(&models.User{}).
		Joins("JOIN memberships ON memberships.user_id = users.id").
		Where("memberships.org_id = ? AND users.last_active_at >= ? AND users.last_active_at < ?", org.ID, previousStartDate, startDate).
		Count(&previousActiveUsers)

	// --- totalCourses and activeCourses ---
	var totalCourses int64
	h.DB.Model(&models.Course{}).Where("org_id = ?", org.ID).Count(&totalCourses)

	var activeCourses int64
	h.DB.Model(&models.Course{}).Where("org_id = ? AND status = 'active'", org.ID).Count(&activeCourses)

	// --- totalEnrollments and activeEnrollments ---
	var totalEnrollments int64
	h.DB.Model(&models.Enrollment{}).Where("org_id = ?", org.ID).Count(&totalEnrollments)

	var activeEnrollments int64
	h.DB.Model(&models.Enrollment{}).Where("org_id = ? AND status = 'active'", org.ID).Count(&activeEnrollments)

	// --- previousEnrollments (courses created in previous period) ---
	var previousEnrollments int64
	h.DB.Model(&models.Enrollment{}).
		Joins("JOIN courses ON courses.id = enrollments.course_id").
		Where("enrollments.org_id = ? AND courses.created_at >= ? AND courses.created_at < ?", org.ID, previousStartDate, startDate).
		Count(&previousEnrollments)

	// --- completedEnrollments ---
	var completedEnrollments int64
	h.DB.Model(&models.Enrollment{}).Where("org_id = ? AND status = 'completed'", org.ID).Count(&completedEnrollments)

	// --- previousCompletedEnrollments ---
	var previousCompletedEnrollments int64
	h.DB.Model(&models.Enrollment{}).
		Joins("JOIN courses ON courses.id = enrollments.course_id").
		Where("enrollments.org_id = ? AND enrollments.status = 'completed' AND courses.created_at >= ? AND courses.created_at < ?",
			org.ID, previousStartDate, startDate).
		Count(&previousCompletedEnrollments)

	// --- recentLogins ---
	var recentLogins []models.ProgressEvent
	h.DB.Where("org_id = ? AND type = ? AND occurred_at >= ?", org.ID, models.ProgressEventTypeLogin, startDate).
		Order("occurred_at DESC").
		Limit(100).
		Preload("User").
		Find(&recentLogins)

	// --- usersByRole ---
	type RoleCountRow struct {
		Role  string
		Count int64
	}
	var roleRows []RoleCountRow
	h.DB.Model(&models.Membership{}).
		Select("role, count(*) as count").
		Where("org_id = ?", org.ID).
		Group("role").
		Scan(&roleRows)

	usersByRole := make([]gin.H, 0, len(roleRows))
	for _, r := range roleRows {
		usersByRole = append(usersByRole, gin.H{"role": r.Role, "count": r.Count})
	}

	// --- topCourses (top 5 by enrollment count) ---
	type CourseRow struct {
		ID    string
		Title string
		Count int64
	}
	var courseRows []CourseRow
	h.DB.Model(&models.Enrollment{}).
		Select("enrollments.course_id as id, courses.title as title, count(*) as count").
		Joins("JOIN courses ON courses.id = enrollments.course_id").
		Where("enrollments.org_id = ?", org.ID).
		Group("enrollments.course_id, courses.title").
		Order("count DESC").
		Limit(5).
		Scan(&courseRows)

	topCourses := make([]gin.H, 0, len(courseRows))
	for _, cr := range courseRows {
		var lessonViews int64
		h.DB.Model(&models.ProgressEvent{}).
			Where("org_id = ? AND course_id = ? AND type = ?", org.ID, cr.ID, models.ProgressEventTypeViewedLesson).
			Count(&lessonViews)

		topCourses = append(topCourses, gin.H{
			"id":           cr.ID,
			"title":        cr.Title,
			"enrollments":  cr.Count,
			"lessonViews":  lessonViews,
		})
	}

	// --- activityEvents (all events in period) ---
	var activityEvents []models.ProgressEvent
	h.DB.Where("org_id = ? AND occurred_at >= ?", org.ID, startDate).
		Order("occurred_at ASC").
		Find(&activityEvents)

	// --- Activity Trend: group by date ---
	// Initialize all days in range
	type DayTrend struct {
		Date        string `json:"date"`
		Logins      int    `json:"logins"`
		LessonViews int    `json:"lessonViews"`
		Assignments int    `json:"assignments"`
	}

	dayMap := make(map[string]*DayTrend)
	for i := 0; i < days; i++ {
		d := startDate.AddDate(0, 0, i)
		key := d.Format("Jan 2")
		dayMap[key] = &DayTrend{Date: key}
	}

	for _, event := range activityEvents {
		key := event.OccurredAt.Format("Jan 2")
		if _, ok := dayMap[key]; !ok {
			dayMap[key] = &DayTrend{Date: key}
		}
		switch event.Type {
		case models.ProgressEventTypeLogin:
			dayMap[key].Logins++
		case models.ProgressEventTypeViewedLesson:
			dayMap[key].LessonViews++
		case models.ProgressEventTypeCompletedAssignment:
			dayMap[key].Assignments++
		}
	}

	// Sort activity trend by date
	activityTrend := make([]DayTrend, 0, len(dayMap))
	for _, v := range dayMap {
		activityTrend = append(activityTrend, *v)
	}
	sort.Slice(activityTrend, func(i, j int) bool {
		ti, _ := time.Parse("Jan 2", activityTrend[i].Date)
		tj, _ := time.Parse("Jan 2", activityTrend[j].Date)
		return ti.Before(tj)
	})

	// --- Peak Hours ---
	hourCounts := make(map[int]int)
	for _, login := range recentLogins {
		hour := login.OccurredAt.Hour()
		hourCounts[hour]++
	}

	peakHourLabels := []int{6, 9, 12, 15, 18, 21}
	hourLabelMap := map[int]string{
		6:  "6 AM",
		9:  "9 AM",
		12: "12 PM",
		15: "3 PM",
		18: "6 PM",
		21: "9 PM",
	}

	type PeakHour struct {
		Hour  string `json:"hour"`
		Count int    `json:"count"`
	}
	peakHours := make([]PeakHour, 0, len(peakHourLabels))
	for _, h := range peakHourLabels {
		count := 0
		// sum hours in [h, h+3)
		for hr := h; hr < h+3; hr++ {
			count += hourCounts[hr%24]
		}
		peakHours = append(peakHours, PeakHour{
			Hour:  hourLabelMap[h],
			Count: count,
		})
	}

	// --- Top Performers ---
	type StudentPerf struct {
		UserID    string
		Name      string
		Email     string
		AvatarURL *string
		Total     int
		Completed int
	}

	var studentMemberships []models.Membership
	h.DB.Where("org_id = ? AND role = ?", org.ID, models.RoleStudent).
		Preload("User").
		Find(&studentMemberships)

	var studentsPerf []StudentPerf
	for _, m := range studentMemberships {
		if m.User == nil {
			continue
		}
		var total, completed int64
		h.DB.Model(&models.Enrollment{}).Where("org_id = ? AND user_id = ?", org.ID, m.UserID).Count(&total)
		h.DB.Model(&models.Enrollment{}).Where("org_id = ? AND user_id = ? AND status = 'completed'", org.ID, m.UserID).Count(&completed)
		studentsPerf = append(studentsPerf, StudentPerf{
			UserID:    m.UserID,
			Name:      m.User.Name,
			Email:     m.User.Email,
			AvatarURL: m.User.AvatarURL,
			Total:     int(total),
			Completed: int(completed),
		})
	}

	sort.Slice(studentsPerf, func(i, j int) bool {
		ci := 0.0
		cj := 0.0
		if studentsPerf[i].Total > 0 {
			ci = float64(studentsPerf[i].Completed) / float64(studentsPerf[i].Total) * 100
		}
		if studentsPerf[j].Total > 0 {
			cj = float64(studentsPerf[j].Completed) / float64(studentsPerf[j].Total) * 100
		}
		return ci > cj
	})

	topCount := 5
	if len(studentsPerf) < topCount {
		topCount = len(studentsPerf)
	}

	type TopPerformer struct {
		ID           string  `json:"id"`
		Name         string  `json:"name"`
		Email        string  `json:"email"`
		AvatarURL    *string `json:"avatarUrl"`
		Completion   float64 `json:"completion"`
		CoursesCount int     `json:"coursesCount"`
	}

	topPerformers := make([]TopPerformer, 0, topCount)
	for _, sp := range studentsPerf[:topCount] {
		completion := 0.0
		if sp.Total > 0 {
			completion = float64(sp.Completed) / float64(sp.Total) * 100
		}
		topPerformers = append(topPerformers, TopPerformer{
			ID:           sp.UserID,
			Name:         sp.Name,
			Email:        sp.Email,
			AvatarURL:    sp.AvatarURL,
			Completion:   completion,
			CoursesCount: sp.Total,
		})
	}

	// --- Recent Activity (last 5 active users with descriptions) ---
	activityDescriptions := []string{
		"completed a lesson",
		"submitted an assignment",
		"logged in",
		"started a new course",
		"viewed course materials",
	}

	var recentActiveUsers []models.User
	h.DB.Joins("JOIN memberships ON memberships.user_id = users.id").
		Where("memberships.org_id = ? AND users.last_active_at IS NOT NULL", org.ID).
		Order("users.last_active_at DESC").
		Limit(5).
		Find(&recentActiveUsers)

	type RecentActivity struct {
		UserID    string  `json:"userId"`
		Name      string  `json:"name"`
		AvatarURL *string `json:"avatarUrl"`
		Action    string  `json:"action"`
		TimeAgo   string  `json:"timeAgo"`
	}

	recentActivity := make([]RecentActivity, 0, len(recentActiveUsers))
	for i, u := range recentActiveUsers {
		desc := activityDescriptions[i%len(activityDescriptions)]
		timeAgo := ""
		if u.LastActiveAt != nil {
			diff := now.Sub(*u.LastActiveAt)
			switch {
			case diff < time.Minute:
				timeAgo = "just now"
			case diff < time.Hour:
				timeAgo = fmt.Sprintf("%d minutes ago", int(diff.Minutes()))
			case diff < 24*time.Hour:
				timeAgo = fmt.Sprintf("%d hours ago", int(diff.Hours()))
			default:
				timeAgo = fmt.Sprintf("%d days ago", int(diff.Hours()/24))
			}
		}
		recentActivity = append(recentActivity, RecentActivity{
			UserID:    u.ID,
			Name:      u.Name,
			AvatarURL: u.AvatarURL,
			Action:    desc,
			TimeAgo:   timeAgo,
		})
	}

	// --- avgCompletionRate ---
	avgCompletionRate := 0.0
	if totalEnrollments > 0 {
		avgCompletionRate = float64(completedEnrollments) / float64(totalEnrollments) * 100
	}

	// --- Computed growth rates ---
	userGrowth := 0.0
	if previousActiveUsers > 0 {
		userGrowth = float64(activeUsers-previousActiveUsers) / float64(previousActiveUsers) * 100
	}

	enrollmentGrowth := 0.0
	if previousEnrollments > 0 {
		enrollmentGrowth = float64(activeEnrollments-previousEnrollments) / float64(previousEnrollments) * 100
	}

	completionGrowth := 0.0
	if previousCompletedEnrollments > 0 {
		completionGrowth = float64(completedEnrollments-previousCompletedEnrollments) / float64(previousCompletedEnrollments) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"overview": gin.H{
			"totalUsers":          totalUsers,
			"activeUsers":         activeUsers,
			"userGrowth":          userGrowth,
			"totalCourses":        totalCourses,
			"activeCourses":       activeCourses,
			"totalEnrollments":    totalEnrollments,
			"activeEnrollments":   activeEnrollments,
			"enrollmentGrowth":    enrollmentGrowth,
			"completedEnrollments": completedEnrollments,
			"avgCompletionRate":   avgCompletionRate,
			"completionGrowth":    completionGrowth,
		},
		"usersByRole":     usersByRole,
		"topCourses":      topCourses,
		"activityTrend":   activityTrend,
		"peakHours":       peakHours,
		"topPerformers":   topPerformers,
		"recentActivity":  recentActivity,
		"range":           rangeParam,
		"startDate":       startDate,
		"endDate":         now,
	})
}
