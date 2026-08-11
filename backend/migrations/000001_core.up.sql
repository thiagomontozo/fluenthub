CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE schools (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), legal_name text NOT NULL, display_name text NOT NULL,
  slug text NOT NULL UNIQUE, email text NOT NULL, phone text NOT NULL DEFAULT '', website text NOT NULL DEFAULT '',
  timezone text NOT NULL, locale text NOT NULL, active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), school_id uuid NOT NULL REFERENCES schools(id), name text NOT NULL,
  email text NOT NULL, password_hash text NOT NULL, phone text, active boolean NOT NULL DEFAULT true,
  last_login_at timestamptz, disabled_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (school_id,email)
);
CREATE UNIQUE INDEX users_email_ci_idx ON users(school_id,lower(email));
CREATE TABLE school_branding (
  school_id uuid PRIMARY KEY REFERENCES schools(id), system_title text NOT NULL DEFAULT 'FluentHub', school_display_name text NOT NULL,
  logo_light_storage_key text, logo_dark_storage_key text, favicon_storage_key text, login_background_storage_key text,
  primary_color text NOT NULL DEFAULT '#4f46e5', secondary_color text NOT NULL DEFAULT '#0f172a', accent_color text NOT NULL DEFAULT '#f59e0b',
  welcome_text text NOT NULL DEFAULT '', certificate_title text NOT NULL DEFAULT 'Certificate of Completion', certificate_footer text NOT NULL DEFAULT '',
  certificate_logo_storage_key text, updated_at timestamptz NOT NULL DEFAULT now(), updated_by uuid REFERENCES users(id)
);
CREATE TABLE roles (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), school_id uuid NOT NULL REFERENCES schools(id), name text NOT NULL, code text NOT NULL, description text NOT NULL DEFAULT '', system boolean NOT NULL DEFAULT false, UNIQUE(school_id,code));
CREATE TABLE permissions (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), code text NOT NULL UNIQUE, description text NOT NULL DEFAULT '');
CREATE TABLE role_permissions (role_id uuid NOT NULL REFERENCES roles(id), permission_id uuid NOT NULL REFERENCES permissions(id), PRIMARY KEY(role_id,permission_id));
CREATE TABLE user_roles (user_id uuid NOT NULL REFERENCES users(id), role_id uuid NOT NULL REFERENCES roles(id), assigned_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(user_id,role_id));
CREATE TABLE user_sessions (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid NOT NULL REFERENCES users(id), token_hash text NOT NULL UNIQUE, expires_at timestamptz NOT NULL, revoked_at timestamptz, created_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX user_sessions_user_idx ON user_sessions(user_id,expires_at);
CREATE TABLE units (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), school_id uuid NOT NULL REFERENCES schools(id), name text NOT NULL, code text NOT NULL, address text NOT NULL DEFAULT '', city text NOT NULL DEFAULT '', state text NOT NULL DEFAULT '', postal_code text NOT NULL DEFAULT '', phone text NOT NULL DEFAULT '', email text NOT NULL DEFAULT '', timezone text NOT NULL, active boolean NOT NULL DEFAULT true, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(school_id,code));
CREATE INDEX units_school_idx ON units(school_id,active);
CREATE TABLE courses (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), school_id uuid NOT NULL REFERENCES schools(id), name text NOT NULL, code text NOT NULL, description text NOT NULL DEFAULT '', language text NOT NULL, active boolean NOT NULL DEFAULT true, archived_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(school_id,code));
CREATE INDEX courses_school_idx ON courses(school_id,active);
CREATE TABLE course_levels (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), course_id uuid NOT NULL REFERENCES courses(id), name text NOT NULL, code text NOT NULL, display_order integer NOT NULL, description text NOT NULL DEFAULT '', active boolean NOT NULL DEFAULT true, UNIQUE(course_id,code));
CREATE TABLE course_modules (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), level_id uuid NOT NULL REFERENCES course_levels(id), name text NOT NULL, description text NOT NULL DEFAULT '', display_order integer NOT NULL, estimated_hours integer NOT NULL DEFAULT 0 CHECK(estimated_hours>=0), active boolean NOT NULL DEFAULT true);
CREATE TABLE class_groups (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), school_id uuid NOT NULL REFERENCES schools(id), unit_id uuid NOT NULL REFERENCES units(id), course_id uuid NOT NULL REFERENCES courses(id), level_id uuid NOT NULL REFERENCES course_levels(id), name text NOT NULL, code text NOT NULL, teacher_id uuid REFERENCES users(id), start_date date NOT NULL, end_date date NOT NULL, capacity integer NOT NULL CHECK(capacity>0), schedule_description text NOT NULL DEFAULT '', status text NOT NULL CHECK(status IN ('planned','open','active','completed','cancelled','archived')), archived_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(school_id,code), CHECK(end_date>=start_date));
CREATE INDEX class_groups_school_status_idx ON class_groups(school_id,status);
CREATE INDEX class_groups_unit_idx ON class_groups(unit_id,status);
CREATE TABLE enrollments (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), school_id uuid NOT NULL REFERENCES schools(id), student_id uuid NOT NULL REFERENCES users(id), class_id uuid NOT NULL REFERENCES class_groups(id), enrolled_at timestamptz NOT NULL DEFAULT now(), status text NOT NULL CHECK(status IN ('pending','active','suspended','completed','failed','cancelled')), completed_at timestamptz, final_score_scaled integer, final_result text, certificate_id uuid, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(student_id,class_id));
CREATE INDEX enrollments_student_idx ON enrollments(school_id,student_id,status);
CREATE INDEX enrollments_class_idx ON enrollments(class_id,status);

INSERT INTO permissions(code) VALUES
('school.manage'),('branding.manage'),('admin.create'),('users.read'),('users.manage'),('units.read'),('units.manage'),
('courses.read'),('courses.manage'),('classes.read'),('classes.manage'),('teachers.manage'),('students.manage'),('operators.manage'),
('lessons.read'),('lessons.manage'),('exercises.read'),('exercises.manage'),('exercises.grade'),('exams.read'),('exams.manage'),('exams.grade'),
('attendance.read'),('attendance.manage'),('billing.read'),('billing.manage'),('notifications.send'),('support.open'),('support.assign'),
('support.handle'),('academic.read'),('academic.manage'),('academic.override'),('certificates.read'),('certificates.issue'),('audit.read');
