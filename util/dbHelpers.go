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
    password VARCHAR(64) NOT NULL,
    role VARCHAR(50) NOT NULL CHECK(role='admin' or role='user' or role='owner') DEFAULT 'user',
    password_changed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    verified BOOLEAN DEFAULT false,
    linkedin VARCHAR(255),
    facebook VARCHAR(255),
    instagram VARCHAR(255),
    profile_pic VARCHAR(255),
    about TEXT,
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
    FOREIGN KEY (user_id) REFERENCES users(id) ,
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
    FOREIGN KEY (created_by_id) REFERENCES users(id) ON DELETE CASCADE
)`,
		`CREATE TABLE IF NOT EXISTS  user_questionsets_editors (
    user_id INT NOT NULL,
    question_set_id INT NOT NULL,
    PRIMARY KEY (user_id, question_set_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (question_set_id) REFERENCES question_sets(id) ON DELETE CASCADE
)`,
		`CREATE TABLE IF NOT EXISTS  question_set_questions (
    question_set_id INT NOT NULL,
    question_id INT,
    PRIMARY KEY (question_set_id, question_id),
    FOREIGN KEY (question_set_id) REFERENCES question_sets(id) ON DELETE CASCADE ,
    FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE SET NULL 
)`,
		`CREATE TABLE  IF NOT EXISTS qtests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finished BOOLEAN DEFAULT FALSE,
    started BOOLEAN DEFAULT FALSE,
    name VARCHAR(255) NOT NULL,
    question_set_id INT NOT NULL,
    taken_by_id INT NOT NULL,
    n_total_questions INT NOT NULL,
    current_question_num INT NOT NULL,
    n_correctly_answered INT DEFAULT 0,
    rank INT,
    taken_at_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    mode VARCHAR(50) NOT NULL CHECK (mode IN ('practice', 'exam', 'timed-practice')),
    FOREIGN KEY (question_set_id) REFERENCES question_sets(id) ON DELETE CASCADE,
    FOREIGN KEY (taken_by_id) REFERENCES users(id) ON DELETE CASCADE
)`,
		`CREATE TABLE IF NOT EXISTS qtest_questions (
    qtest_id uuid NOT NULL,
    question_id INT,
    PRIMARY KEY (qtest_id, question_id),
    FOREIGN KEY (qtest_id) REFERENCES qtests(id) ON DELETE CASCADE,
    FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE SET NULL 
)`,
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
		`CREATE TABLE IF NOT EXISTS   user_daily_activity (
  user_id INT REFERENCES users(id) ON DELETE CASCADE,
  activity_date DATE NOT NULL,
  questions_answered INT DEFAULT 0,
  tests_completed INT DEFAULT 0,
  PRIMARY KEY (user_id, activity_date)
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
		"DROP TABLE IF EXISTS questionsets_questionsettags",
		"DROP TABLE IF EXISTS user_daily_activity",
		"DROP TABLE IF EXISTS question_questiontags",
		"DROP TABLE IF EXISTS questionsettags",
		"DROP TABLE IF EXISTS questiontags",
		"DROP TABLE IF EXISTS tags",
		"DROP TABLE IF EXISTS qtest_questions",
		"DROP TABLE IF EXISTS qtests",
		"DROP TABLE IF EXISTS question_set_questions",
		"DROP TABLE IF EXISTS user_questionsets_editors",
		"DROP TABLE IF EXISTS question_sets",
		"DROP TABLE IF EXISTS user_questions_editors",
		"DROP TABLE IF EXISTS questions",
		"DROP TABLE IF EXISTS users",
	}
}
