package util

import (
	"fmt"
)

func ddlStrings() []string {
	sqlStrings := []string{}
	sqlStrings = append(sqlStrings,
		`CREATE TABLE IF NOT EXISTS  users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    email VARCHAR(128) UNIQUE NOT NULL,
    password VARCHAR(512),
    role VARCHAR(50) NOT NULL CHECK(role='admin' or role='user' or role='owner') DEFAULT 'user',
    password_changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    country VARCHAR(128),
    country_code VARCHAR(8),
    mobile_number VARCHAR(16),
    mobile_number_verified BOOLEAN DEFAULT false,
    verified BOOLEAN DEFAULT false,
    linkedin VARCHAR(255),
    facebook VARCHAR(255),
    instagram VARCHAR(255),
    profile_pic VARCHAR(255),
    is_premium BOOLEAN DEFAULT FALSE,
    premium_since TIMESTAMP,
    premium_expiry TIMESTAMP,
    about TEXT,
    goal TEXT,
    deleted BOOLEAN DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`,
		`CREATE TABLE IF NOT EXISTS  questions (
    id SERIAL PRIMARY KEY,
    question TEXT NOT NULL,
    subject VARCHAR(255) NOT NULL,
    exam VARCHAR(255),
    language VARCHAR(255) NOT NULL,
    difficulty INT CHECK (difficulty BETWEEN 1 AND 10),
    question_type VARCHAR(50) NOT NULL CHECK (question_type IN ('m-choice', 'm-select', 'numeric')),
    options TEXT[] NOT NULL,
    correct_options INT[] NOT NULL,
    explanation TEXT,
    created_by_id INT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by_id) REFERENCES users(id) ON DELETE CASCADE
)`,
		`CREATE TABLE IF NOT EXISTS  user_questions_editors (
    user_id INT NOT NULL,
    question_id INT NOT NULL,
    PRIMARY KEY (user_id, question_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE CASCADE
);`,
		`CREATE TABLE IF NOT EXISTS  question_sets (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    mode VARCHAR(50) NOT NULL CHECK (mode IN ('practice', 'exam', 'timed')),
    subject VARCHAR(255) NOT NULL,
    exam VARCHAR(255),
    language VARCHAR(255) NOT NULL,
    time_duration VARCHAR(50),
    difficulty INT CHECK (difficulty BETWEEN 1 AND 10),
    description TEXT,
    associated_resource TEXT,
    created_by_id INT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    cover_image VARCHAR(512),
    FOREIGN KEY (created_by_id) REFERENCES users(id) ON DELETE CASCADE
)`,
		`CREATE TABLE IF NOT EXISTS  user_questionsets_editors (
    user_id INT NOT NULL,
    question_set_id INT,
    PRIMARY KEY (user_id, question_set_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (question_set_id) REFERENCES question_sets(id) ON DELETE CASCADE
)`,
		`CREATE TABLE IF NOT EXISTS  question_set_questions (
    question_set_id INT NOT NULL,
    question_id INT,
    mark FLOAT default 1,
    PRIMARY KEY (question_set_id, question_id),
    FOREIGN KEY (question_set_id) REFERENCES question_sets(id) ON DELETE CASCADE ,
    FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE CASCADE 
)`,
		`CREATE TABLE  IF NOT EXISTS test_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finished BOOLEAN DEFAULT FALSE,
    started BOOLEAN DEFAULT FALSE,
    name VARCHAR(255) NOT NULL,
    question_set_id INT NOT NULL,
    taken_by_id INT NOT NULL,
    seconds_per_question INT,
    time_cap_seconds INT,
    remaining_time_seconds INT,
    n_total_questions INT NOT NULL,
    current_question_num INT NOT NULL,
    n_correctly_answered INT DEFAULT 0,
    rank INT,
    total_marks float default 0,
    scored_marks float default 0,
    updated_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    started_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    finished_time TIMESTAMP,
    mode VARCHAR(50) NOT NULL CHECK (mode IN ('q_timed', 't_timed', 'untimed')) DEFAULT 'untimed',
    FOREIGN KEY (question_set_id) REFERENCES question_sets(id) ON DELETE CASCADE,
    FOREIGN KEY (taken_by_id) REFERENCES users(id) ON DELETE CASCADE
)`,
		`CREATE TABLE IF NOT EXISTS test_session_question_answers(
  test_session_id UUID REFERENCES test_sessions(id) ON DELETE CASCADE,
  question_id INT REFERENCES questions(id) ON DELETE CASCADE ,
  order_list INT[] DEFAULT '{0,1,2,3}', 
  correct_answer_list INT[],
  selected_answer_list INT[],
  questions_total_mark FLOAT,
  questions_scored_mark FLOAT,
  index_num INT,
  answered BOOLEAN DEFAULT FALSE,
  PRIMARY KEY (test_session_id, question_id)
);`,
		`CREATE TABLE  IF NOT EXISTS  questiontags (
  id SERIAL PRIMARY KEY,
  name TEXT UNIQUE NOT NULL
);`,
		`CREATE TABLE IF NOT EXISTS questionsettags (
  id SERIAL PRIMARY KEY,
  name TEXT UNIQUE NOT NULL
);`,
		`CREATE TABLE IF NOT EXISTS question_questiontags (
  question_id INT REFERENCES questions(id) ON DELETE CASCADE,
  questiontags_id INT REFERENCES questiontags(id) ON DELETE CASCADE,
  PRIMARY KEY (question_id, questiontags_id)
);`,

		`CREATE TABLE IF NOT EXISTS questionsets_questionsettags (
  questionset_id INT REFERENCES question_sets(id) ON DELETE CASCADE,
  questionsettags_id INT REFERENCES questionsettags(id) ON DELETE CASCADE,
  PRIMARY KEY (questionset_id, questionsettags_id)
);`,
		`CREATE TABLE IF NOT EXISTS user_daily_activity (
  user_id INT REFERENCES users(id) ON DELETE CASCADE,
  activity_date DATE NOT NULL,
  tests_completed INT DEFAULT 0,
  
  questions_limit INT DEFAULT 20,
  PRIMARY KEY (user_id, activity_date)
);
`, `
CREATE TABLE IF NOT EXISTS payments (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    amount NUMERIC(10,2) NOT NULL,
    currency VARCHAR(10) DEFAULT 'INR',
    payment_provider VARCHAR(50), -- e.g., "stripe", "razorpay"
    payment_status VARCHAR(20), -- e.g., "success", "failed"
    transaction_id TEXT,
    paid_at TIMESTAMP DEFAULT now()
);`, `
CREATE TABLE IF NOT EXISTS subscription_plans (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50),
    price NUMERIC(10,2),
    duration_days INT,
    question_limit_per_day INT,
    created_at TIMESTAMP DEFAULT now()
);`, `
CREATE TABLE IF NOT EXISTS bookmarked_questions (
    user_id INT NOT NULL,
    question_id INT NOT NULL,
    bookmarked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, question_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE CASCADE
);
`, `
CREATE TABLE IF NOT EXISTS saved_explanations (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    question_id INT NOT NULL,
    explanation TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, question_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE CASCADE
);
`, `
CREATE TABLE IF NOT EXISTS question_error_reports (
    id SERIAL PRIMARY KEY,
    question_id INT NOT NULL,
    reported_by_id INT NOT NULL,
    reported_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    report_type VARCHAR(50) NOT NULL CHECK (report_type IN ('question', 'option', 'explanation')),
    option_index INT,
    report_text TEXT NOT NULL,
    status VARCHAR(50) DEFAULT 'open' CHECK (status IN ('open', 'reviewed', 'resolved', 'rejected')),
    FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE CASCADE,
    FOREIGN KEY (reported_by_id) REFERENCES users(id) ON DELETE CASCADE
);
`, `
CREATE TABLE IF NOT EXISTS user_daily_questions (
    user_id INT NOT NULL,
    question_id INT NOT NULL,
    answered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    answered_correct BOOLEAN DEFAULT FALSE,
    taken_duration_seconds INT,
    PRIMARY KEY (user_id, question_id,answered_at),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE CASCADE
);
`, `CREATE TABLE IF NOT EXISTS email_verification_codes (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email VARCHAR(512) NOT NULL,
    code VARCHAR(6) NOT NULL,
    purpose VARCHAR(64) DEFAULT 'email_verification',
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                                                    
);`, `
CREATE TABLE IF NOT EXISTS question_set_reviews (
    id SERIAL PRIMARY KEY,
    question_set_id INT NOT NULL,
    user_id INT NOT NULL,
    rating INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    review TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (question_set_id) REFERENCES question_sets(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
`)
	return sqlStrings
}
func CreateTableIfNotExists() error {
	sqlStrings := ddlStrings()
	for i, sql := range sqlStrings {
		_, err := DB.Exec(sql)
		if err != nil {
			return fmt.Errorf("error creating table %d: %w", i, err)
		}
	}
	return nil
}
func dropTables() []string {
	return []string{
		"DROP TABLE IF EXISTS email_verification_codes",
		"DROP TABLE IF EXISTS user_daily_questions",
		"DROP TABLE IF EXISTS question_error_reports",
		"DROP TABLE IF EXISTS saved_explanations",
		"DROP TABLE IF EXISTS bookmarked_questions",
		"DROP TABLE IF EXISTS payments",
		"DROP TABLE IF EXISTS subscription_plans",
		"DROP TABLE IF EXISTS user_daily_activity",
		"DROP TABLE IF EXISTS questionsets_questionsettags",
		"DROP TABLE IF EXISTS question_questiontags",
		"DROP TABLE IF EXISTS questionsettags",
		"DROP TABLE IF EXISTS questiontags",
		"DROP TABLE IF EXISTS test_session_question_answers",
		"DROP TABLE IF EXISTS test_sessions",
		"DROP TABLE IF EXISTS question_set_reviews",
		"DROP TABLE IF EXISTS question_set_questions",
		"DROP TABLE IF EXISTS user_questionsets_editors",
		"DROP TABLE IF EXISTS question_sets",
		"DROP TABLE IF EXISTS user_questions_editors",
		"DROP TABLE IF EXISTS questions",
		"DROP TABLE IF EXISTS users",
	}
}
