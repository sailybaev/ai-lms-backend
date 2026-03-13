package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── JSONB ──────────────────────────────────────────────────────────────────

func TestJSONB_Value_nil(t *testing.T) {
	var j JSONB
	v, err := j.Value()
	require.NoError(t, err)
	assert.Nil(t, v)
}

func TestJSONB_Value_non_nil(t *testing.T) {
	j := JSONB{"key": "value", "num": float64(42)}
	v, err := j.Value()
	require.NoError(t, err)
	str, ok := v.(string)
	require.True(t, ok, "expected string driver.Value")
	assert.Contains(t, str, `"key"`)
	assert.Contains(t, str, `"value"`)
}

func TestJSONB_Scan_nil(t *testing.T) {
	var j JSONB
	err := j.Scan(nil)
	require.NoError(t, err)
	assert.Nil(t, j)
}

func TestJSONB_Scan_bytes(t *testing.T) {
	var j JSONB
	err := j.Scan([]byte(`{"hello":"world"}`))
	require.NoError(t, err)
	assert.Equal(t, "world", j["hello"])
}

func TestJSONB_Scan_string(t *testing.T) {
	var j JSONB
	err := j.Scan(`{"answer":42}`)
	require.NoError(t, err)
	assert.Equal(t, float64(42), j["answer"])
}

func TestJSONB_Scan_unsupported_type(t *testing.T) {
	var j JSONB
	err := j.Scan(12345)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported type")
}

func TestJSONB_Scan_invalid_json(t *testing.T) {
	var j JSONB
	err := j.Scan([]byte(`not-json`))
	assert.Error(t, err)
}

// ── BeforeCreate UUID hooks ─────────────────────────────────────────────────
// We test the hooks by calling them directly with a nil *gorm.DB (safe because
// the hooks only inspect o.ID and set it — they never use the db argument).

func TestOrganization_BeforeCreate_assigns_uuid(t *testing.T) {
	o := &Organization{}
	require.NoError(t, o.BeforeCreate(nil))
	assert.NotEmpty(t, o.ID)
}

func TestOrganization_BeforeCreate_preserves_existing_id(t *testing.T) {
	o := &Organization{ID: "existing-id"}
	require.NoError(t, o.BeforeCreate(nil))
	assert.Equal(t, "existing-id", o.ID)
}

func TestUser_BeforeCreate_assigns_uuid(t *testing.T) {
	u := &User{}
	require.NoError(t, u.BeforeCreate(nil))
	assert.NotEmpty(t, u.ID)
}

func TestMembership_BeforeCreate_assigns_uuid(t *testing.T) {
	m := &Membership{}
	require.NoError(t, m.BeforeCreate(nil))
	assert.NotEmpty(t, m.ID)
}

func TestCourse_BeforeCreate_assigns_uuid(t *testing.T) {
	c := &Course{}
	require.NoError(t, c.BeforeCreate(nil))
	assert.NotEmpty(t, c.ID)
}

func TestEnrollment_BeforeCreate_assigns_uuid(t *testing.T) {
	e := &Enrollment{}
	require.NoError(t, e.BeforeCreate(nil))
	assert.NotEmpty(t, e.ID)
}

func TestFile_BeforeCreate_assigns_uuid(t *testing.T) {
	f := &File{}
	require.NoError(t, f.BeforeCreate(nil))
	assert.NotEmpty(t, f.ID)
}

func TestAIChatSession_BeforeCreate_assigns_uuid(t *testing.T) {
	s := &AIChatSession{}
	require.NoError(t, s.BeforeCreate(nil))
	assert.NotEmpty(t, s.ID)
}

func TestAIMessage_BeforeCreate_assigns_uuid(t *testing.T) {
	m := &AIMessage{}
	require.NoError(t, m.BeforeCreate(nil))
	assert.NotEmpty(t, m.ID)
}

func TestGroup_BeforeCreate_assigns_uuid(t *testing.T) {
	g := &Group{}
	require.NoError(t, g.BeforeCreate(nil))
	assert.NotEmpty(t, g.ID)
}

// ── Enum constants ──────────────────────────────────────────────────────────

func TestRoleConstants(t *testing.T) {
	assert.Equal(t, Role("admin"), RoleAdmin)
	assert.Equal(t, Role("teacher"), RoleTeacher)
	assert.Equal(t, Role("student"), RoleStudent)
}

func TestMembershipStatusConstants(t *testing.T) {
	assert.Equal(t, MembershipStatus("active"), MembershipStatusActive)
	assert.Equal(t, MembershipStatus("invited"), MembershipStatusInvited)
	assert.Equal(t, MembershipStatus("suspended"), MembershipStatusSuspended)
}

func TestCourseStatusConstants(t *testing.T) {
	assert.Equal(t, CourseStatus("draft"), CourseStatusDraft)
	assert.Equal(t, CourseStatus("active"), CourseStatusActive)
	assert.Equal(t, CourseStatus("archived"), CourseStatusArchived)
}

func TestEnrollmentStatusConstants(t *testing.T) {
	assert.Equal(t, EnrollmentStatus("active"), EnrollmentStatusActive)
	assert.Equal(t, EnrollmentStatus("completed"), EnrollmentStatusCompleted)
	assert.Equal(t, EnrollmentStatus("dropped"), EnrollmentStatusDropped)
}

func TestAssignmentTypeConstants(t *testing.T) {
	assert.Equal(t, AssignmentType("essay"), AssignmentTypeEssay)
	assert.Equal(t, AssignmentType("quiz"), AssignmentTypeQuiz)
	assert.Equal(t, AssignmentType("project"), AssignmentTypeProject)
}

func TestSenderConstants(t *testing.T) {
	assert.Equal(t, Sender("user"), SenderUser)
	assert.Equal(t, Sender("assistant"), SenderAssistant)
	assert.Equal(t, Sender("system"), SenderSystem)
}
