package records

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Service { return &Service{db: db} }

func (s *Service) List(ctx context.Context, schoolID, userID, resource string, unrestricted bool, limit, offset int) ([]json.RawMessage, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, errors.New("invalid pagination")
	}
	queries := map[string]string{
		"units":         `SELECT jsonb_build_object('id',id,'name',name,'code',code,'city',city,'state',state,'timezone',timezone,'active',active) FROM units WHERE school_id=$1 ORDER BY name LIMIT $2 OFFSET $3`,
		"courses":       `SELECT jsonb_build_object('id',id,'name',name,'code',code,'description',description,'language',language,'active',active) FROM courses WHERE school_id=$1 ORDER BY name LIMIT $2 OFFSET $3`,
		"classes":       `SELECT jsonb_build_object('id',c.id,'name',c.name,'code',c.code,'courseId',c.course_id,'levelId',c.level_id,'unitId',c.unit_id,'teacherId',c.teacher_id,'startDate',c.start_date,'endDate',c.end_date,'capacity',c.capacity,'scheduleDescription',c.schedule_description,'status',c.status) FROM class_groups c WHERE c.school_id=$1 ORDER BY c.start_date DESC,c.name LIMIT $2 OFFSET $3`,
		"enrollments":   `SELECT jsonb_build_object('id',e.id,'studentId',e.student_id,'classId',e.class_id,'enrolledAt',e.enrolled_at,'status',e.status,'finalScore',e.final_score_scaled,'finalResult',e.final_result) FROM enrollments e WHERE e.school_id=$1 ORDER BY e.enrolled_at DESC LIMIT $2 OFFSET $3`,
		"lessons":       `SELECT jsonb_build_object('id',l.id,'classId',l.class_id,'moduleId',l.module_id,'teacherId',l.teacher_id,'title',l.title,'description',l.description,'scheduledStart',l.scheduled_start,'scheduledEnd',l.scheduled_end,'status',l.status,'materialsPublished',l.materials_published) FROM lessons l WHERE l.school_id=$1 ORDER BY l.scheduled_start DESC LIMIT $2 OFFSET $3`,
		"exercises":     `SELECT jsonb_build_object('id',e.id,'classId',e.class_id,'lessonId',e.lesson_id,'moduleId',e.module_id,'teacherId',e.teacher_id,'title',e.title,'description',e.description,'availableFrom',e.available_from,'dueAt',e.due_at,'maxScore',e.max_score_scaled,'attemptLimit',e.attempt_limit,'status',e.status) FROM exercises e WHERE e.school_id=$1 ORDER BY e.created_at DESC LIMIT $2 OFFSET $3`,
		"exams":         `SELECT jsonb_build_object('id',e.id,'classId',e.class_id,'moduleId',e.module_id,'teacherId',e.teacher_id,'title',e.title,'description',e.description,'opensAt',e.opens_at,'closesAt',e.closes_at,'durationMinutes',e.duration_minutes,'maxScore',e.max_score_scaled,'attemptLimit',e.attempt_limit,'status',e.status) FROM exams e WHERE e.school_id=$1 ORDER BY e.created_at DESC LIMIT $2 OFFSET $3`,
		"invoices":      `SELECT jsonb_build_object('id',i.id,'studentId',i.student_id,'enrollmentId',i.enrollment_id,'description',i.description,'amountCents',i.amount_cents,'dueDate',i.due_date,'status',i.status,'provider',i.provider,'providerReference',i.provider_reference,'issuedAt',i.issued_at,'paidAt',i.paid_at) FROM invoices i WHERE i.school_id=$1 ORDER BY i.created_at DESC LIMIT $2 OFFSET $3`,
		"support":       `SELECT jsonb_build_object('id',t.id,'ticketNumber',t.ticket_number,'requesterUserId',t.requester_user_id,'openedByUserId',t.opened_by_user_id,'category',t.category,'subject',t.subject,'priority',t.priority,'status',t.status,'assignedToUserId',t.assigned_to_user_id,'createdAt',t.created_at,'updatedAt',t.updated_at) FROM support_tickets t WHERE t.school_id=$1 ORDER BY t.updated_at DESC LIMIT $2 OFFSET $3`,
		"notifications": `SELECT jsonb_build_object('id',n.id,'userId',n.user_id,'type',n.type,'title',n.title,'message',n.message,'resourceType',n.resource_type,'resourceId',n.resource_id,'readAt',n.read_at,'createdAt',n.created_at) FROM notifications n WHERE n.school_id=$1 ORDER BY n.created_at DESC LIMIT $2 OFFSET $3`,
	}
	query, ok := queries[resource]
	if !ok {
		return nil, errors.New("unsupported resource")
	}
	args := []any{schoolID, limit, offset}
	if !unrestricted {
		scoped := map[string]string{
			"classes":       `SELECT jsonb_build_object('id',c.id,'name',c.name,'code',c.code,'courseId',c.course_id,'levelId',c.level_id,'unitId',c.unit_id,'teacherId',c.teacher_id,'startDate',c.start_date,'endDate',c.end_date,'capacity',c.capacity,'scheduleDescription',c.schedule_description,'status',c.status) FROM class_groups c WHERE c.school_id=$1 AND (c.teacher_id=$4 OR EXISTS(SELECT 1 FROM enrollments e WHERE e.class_id=c.id AND e.student_id=$4 AND e.status IN ('pending','active','suspended'))) ORDER BY c.start_date DESC,c.name LIMIT $2 OFFSET $3`,
			"enrollments":   `SELECT jsonb_build_object('id',e.id,'studentId',e.student_id,'classId',e.class_id,'enrolledAt',e.enrolled_at,'status',e.status,'finalScore',e.final_score_scaled,'finalResult',e.final_result) FROM enrollments e JOIN class_groups c ON c.id=e.class_id WHERE e.school_id=$1 AND (e.student_id=$4 OR c.teacher_id=$4) ORDER BY e.enrolled_at DESC LIMIT $2 OFFSET $3`,
			"lessons":       `SELECT jsonb_build_object('id',l.id,'classId',l.class_id,'moduleId',l.module_id,'teacherId',l.teacher_id,'title',l.title,'description',l.description,'scheduledStart',l.scheduled_start,'scheduledEnd',l.scheduled_end,'status',l.status,'materialsPublished',l.materials_published) FROM lessons l WHERE l.school_id=$1 AND (l.teacher_id=$4 OR EXISTS(SELECT 1 FROM enrollments e WHERE e.class_id=l.class_id AND e.student_id=$4 AND e.status='active')) ORDER BY l.scheduled_start DESC LIMIT $2 OFFSET $3`,
			"exercises":     `SELECT jsonb_build_object('id',e.id,'classId',e.class_id,'lessonId',e.lesson_id,'moduleId',e.module_id,'teacherId',e.teacher_id,'title',e.title,'description',e.description,'availableFrom',e.available_from,'dueAt',e.due_at,'maxScore',e.max_score_scaled,'attemptLimit',e.attempt_limit,'status',e.status) FROM exercises e WHERE e.school_id=$1 AND (e.teacher_id=$4 OR EXISTS(SELECT 1 FROM enrollments n WHERE n.class_id=e.class_id AND n.student_id=$4 AND n.status='active')) ORDER BY e.created_at DESC LIMIT $2 OFFSET $3`,
			"exams":         `SELECT jsonb_build_object('id',e.id,'classId',e.class_id,'moduleId',e.module_id,'teacherId',e.teacher_id,'title',e.title,'description',e.description,'opensAt',e.opens_at,'closesAt',e.closes_at,'durationMinutes',e.duration_minutes,'maxScore',e.max_score_scaled,'attemptLimit',e.attempt_limit,'status',e.status) FROM exams e WHERE e.school_id=$1 AND (e.teacher_id=$4 OR EXISTS(SELECT 1 FROM enrollments n WHERE n.class_id=e.class_id AND n.student_id=$4 AND n.status='active')) ORDER BY e.created_at DESC LIMIT $2 OFFSET $3`,
			"invoices":      `SELECT jsonb_build_object('id',i.id,'studentId',i.student_id,'enrollmentId',i.enrollment_id,'description',i.description,'amountCents',i.amount_cents,'dueDate',i.due_date,'status',i.status,'provider',i.provider,'providerReference',i.provider_reference,'issuedAt',i.issued_at,'paidAt',i.paid_at) FROM invoices i WHERE i.school_id=$1 AND i.student_id=$4 ORDER BY i.created_at DESC LIMIT $2 OFFSET $3`,
			"support":       `SELECT jsonb_build_object('id',t.id,'ticketNumber',t.ticket_number,'requesterUserId',t.requester_user_id,'openedByUserId',t.opened_by_user_id,'category',t.category,'subject',t.subject,'priority',t.priority,'status',t.status,'assignedToUserId',t.assigned_to_user_id,'createdAt',t.created_at,'updatedAt',t.updated_at) FROM support_tickets t WHERE t.school_id=$1 AND (t.requester_user_id=$4 OR t.opened_by_user_id=$4 OR t.assigned_to_user_id=$4) ORDER BY t.updated_at DESC LIMIT $2 OFFSET $3`,
			"notifications": `SELECT jsonb_build_object('id',n.id,'userId',n.user_id,'type',n.type,'title',n.title,'message',n.message,'resourceType',n.resource_type,'resourceId',n.resource_id,'readAt',n.read_at,'createdAt',n.created_at) FROM notifications n WHERE n.school_id=$1 AND n.user_id=$4 ORDER BY n.created_at DESC LIMIT $2 OFFSET $3`,
		}
		if scopedQuery, exists := scoped[resource]; exists {
			query = scopedQuery
			args = append(args, userID)
		}
	}
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]json.RawMessage, 0)
	for rows.Next() {
		var item []byte
		if err := rows.Scan(&item); err != nil {
			return nil, err
		}
		items = append(items, json.RawMessage(item))
	}
	return items, rows.Err()
}

type createInput struct {
	Name, Code, Description, Language, City, State, Timezone                  string
	UnitID, CourseID, LevelID, ClassID, StudentID, TeacherID, RequesterUserID string
	ModuleID, LessonID, EnrollmentID                                          *string
	Title, Subject, Category, Priority, ScheduleDescription                   string
	StartDate, EndDate, ScheduledStart, ScheduledEnd, AvailableFrom, DueAt    *time.Time
	OpensAt, ClosesAt, DueDate                                                *time.Time
	Capacity, DurationMinutes, MaxScoreScaled, AttemptLimit                   int
	AmountCents                                                               int64
}

func (s *Service) Create(ctx context.Context, schoolID, actorID, resource string, unrestricted bool, raw json.RawMessage) (string, error) {
	var input createInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return "", errors.New("invalid resource payload")
	}
	id := uuid.NewString()
	if err := s.validateReferences(ctx, schoolID, actorID, resource, unrestricted, &input); err != nil {
		return "", err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var query string
	var args []any
	switch resource {
	case "units":
		if invalidText(input.Name, 2, 120) || invalidText(input.Code, 1, 40) || input.Timezone == "" {
			return "", errors.New("name, code and timezone are required")
		}
		query = `INSERT INTO units(id,school_id,name,code,city,state,timezone) VALUES($1,$2,$3,$4,$5,$6,$7)`
		args = []any{id, schoolID, strings.TrimSpace(input.Name), strings.TrimSpace(input.Code), input.City, input.State, input.Timezone}
	case "courses":
		if invalidText(input.Name, 2, 120) || invalidText(input.Code, 1, 40) || invalidText(input.Language, 2, 80) {
			return "", errors.New("name, code and language are required")
		}
		query = `INSERT INTO courses(id,school_id,name,code,description,language) VALUES($1,$2,$3,$4,$5,$6)`
		args = []any{id, schoolID, input.Name, input.Code, input.Description, input.Language}
	case "classes":
		if input.StartDate == nil || input.EndDate == nil || input.Capacity < 1 || invalidText(input.Name, 2, 120) {
			return "", errors.New("class dates, capacity and name are required")
		}
		query = `INSERT INTO class_groups(id,school_id,unit_id,course_id,level_id,name,code,teacher_id,start_date,end_date,capacity,schedule_description,status) VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9,$10,$11,$12,'planned')`
		args = []any{id, schoolID, input.UnitID, input.CourseID, input.LevelID, input.Name, input.Code, input.TeacherID, input.StartDate, input.EndDate, input.Capacity, input.ScheduleDescription}
	case "enrollments":
		if input.StudentID == "" || input.ClassID == "" {
			return "", errors.New("studentId and classId are required")
		}
		query = `INSERT INTO enrollments(id,school_id,student_id,class_id,status) VALUES($1,$2,$3,$4,'pending')`
		args = []any{id, schoolID, input.StudentID, input.ClassID}
	case "lessons":
		if input.ScheduledStart == nil || input.ScheduledEnd == nil || invalidText(input.Title, 2, 180) {
			return "", errors.New("title and schedule are required")
		}
		query = `INSERT INTO lessons(id,school_id,class_id,module_id,teacher_id,title,description,scheduled_start,scheduled_end,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'draft')`
		args = []any{id, schoolID, input.ClassID, input.ModuleID, input.TeacherID, input.Title, input.Description, input.ScheduledStart, input.ScheduledEnd}
	case "exercises":
		if input.MaxScoreScaled < 1 || invalidText(input.Title, 2, 180) {
			return "", errors.New("title and positive maxScoreScaled are required")
		}
		query = `INSERT INTO exercises(id,school_id,class_id,lesson_id,module_id,teacher_id,title,description,available_from,due_at,max_score_scaled,attempt_limit,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NULLIF($12,0),'draft')`
		args = []any{id, schoolID, input.ClassID, input.LessonID, input.ModuleID, input.TeacherID, input.Title, input.Description, input.AvailableFrom, input.DueAt, input.MaxScoreScaled, input.AttemptLimit}
	case "exams":
		if input.DurationMinutes < 1 || input.MaxScoreScaled < 1 || input.AttemptLimit < 1 || invalidText(input.Title, 2, 180) {
			return "", errors.New("title, duration, max score and attempt limit are required")
		}
		query = `INSERT INTO exams(id,school_id,class_id,module_id,teacher_id,title,description,opens_at,closes_at,duration_minutes,max_score_scaled,attempt_limit,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'draft')`
		args = []any{id, schoolID, input.ClassID, input.ModuleID, input.TeacherID, input.Title, input.Description, input.OpensAt, input.ClosesAt, input.DurationMinutes, input.MaxScoreScaled, input.AttemptLimit}
	case "support":
		if input.RequesterUserID == "" || invalidText(input.Subject, 3, 180) || invalidText(input.Description, 3, 5000) {
			return "", errors.New("requester, subject and description are required")
		}
		query = `INSERT INTO support_tickets(id,school_id,requester_user_id,opened_by_user_id,category,subject,description,priority,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'open')`
		args = []any{id, schoolID, input.RequesterUserID, actorID, input.Category, input.Subject, input.Description, input.Priority}
	default:
		return "", errors.New("resource does not support generic creation")
	}
	if _, err := tx.Exec(ctx, query, args...); err != nil {
		return "", fmt.Errorf("create %s: %w", resource, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(school_id,user_id,action,resource_type,resource_id) VALUES($1,$2,$3,$4,$5)`, schoolID, actorID, resource+".created", resource, id); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Service) validateReferences(ctx context.Context, schoolID, actorID, resource string, unrestricted bool, input *createInput) error {
	var valid bool
	switch resource {
	case "classes":
		err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM units u JOIN courses c ON c.school_id=u.school_id JOIN course_levels l ON l.course_id=c.id WHERE u.id=$1 AND c.id=$2 AND l.id=$3 AND u.school_id=$4)`, input.UnitID, input.CourseID, input.LevelID, schoolID).Scan(&valid)
		if err != nil || !valid {
			return errors.New("unit, course or level does not belong to this school")
		}
		if input.TeacherID != "" {
			if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND school_id=$2 AND active=true)`, input.TeacherID, schoolID).Scan(&valid); err != nil || !valid {
				return errors.New("teacher does not belong to this school")
			}
		}
	case "enrollments":
		err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users u JOIN class_groups c ON c.school_id=u.school_id WHERE u.id=$1 AND c.id=$2 AND u.school_id=$3 AND u.active=true)`, input.StudentID, input.ClassID, schoolID).Scan(&valid)
		if err != nil || !valid {
			return errors.New("student or class does not belong to this school")
		}
	case "lessons", "exercises", "exams":
		query := `SELECT EXISTS(SELECT 1 FROM class_groups WHERE id=$1 AND school_id=$2)`
		args := []any{input.ClassID, schoolID}
		if !unrestricted {
			query = `SELECT EXISTS(SELECT 1 FROM class_groups WHERE id=$1 AND school_id=$2 AND teacher_id=$3)`
			args = append(args, actorID)
			input.TeacherID = actorID
		} else if input.TeacherID == "" {
			return errors.New("teacherId is required")
		}
		if err := s.db.QueryRow(ctx, query, args...).Scan(&valid); err != nil || !valid {
			return errors.New("class is outside the actor scope")
		}
		if unrestricted {
			if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND school_id=$2 AND active=true)`, input.TeacherID, schoolID).Scan(&valid); err != nil || !valid {
				return errors.New("teacher does not belong to this school")
			}
		}
	case "support":
		if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND school_id=$2 AND active=true)`, input.RequesterUserID, schoolID).Scan(&valid); err != nil || !valid {
			return errors.New("requester does not belong to this school")
		}
	}
	return nil
}

func (s *Service) Transition(ctx context.Context, schoolID, actorID, resource, id, status string, unrestricted bool) error {
	allowed := map[string]map[string]bool{
		"classes":   {"planned": true, "open": true, "active": true, "completed": true, "cancelled": true, "archived": true},
		"exercises": {"draft": true, "published": true, "closed": true, "archived": true},
		"exams":     {"draft": true, "scheduled": true, "open": true, "closed": true, "grading": true, "published": true, "archived": true},
		"support":   {"open": true, "assigned": true, "in_progress": true, "waiting_user": true, "resolved": true, "closed": true},
	}
	if !allowed[resource][status] {
		return errors.New("invalid status transition target")
	}
	tables := map[string]string{"classes": "class_groups", "exercises": "exercises", "exams": "exams", "support": "support_tickets"}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	command := fmt.Sprintf("UPDATE %s SET status=$1,updated_at=now() WHERE id=$2 AND school_id=$3", tables[resource])
	args := []any{status, id, schoolID}
	if !unrestricted && (resource == "exercises" || resource == "exams") {
		command += " AND teacher_id=$4"
		args = append(args, actorID)
	}
	if !unrestricted && resource == "support" {
		command += " AND assigned_to_user_id=$4"
		args = append(args, actorID)
	}
	result, err := tx.Exec(ctx, command, args...)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return errors.New("resource not found")
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_events(school_id,user_id,action,resource_type,resource_id,metadata) VALUES($1,$2,$3,$4,$5,jsonb_build_object('status',$6))`, schoolID, actorID, resource+".status_changed", resource, id, status)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func invalidText(value string, minimum, maximum int) bool {
	length := len(strings.TrimSpace(value))
	return length < minimum || length > maximum
}
