-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    user_id UUID PRIMARY KEY DEFAULT uuidv7(),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL,
    first_name TEXT NOT NULL, 
    last_name TEXT NOT NULL,

    CONSTRAINT valid_role CHECK (
        role IN (
            'judge', 
            'admin'
        )
    )
);

CREATE TABLE IF NOT EXISTS candidate (
    candidate_id UUID PRIMARY KEY DEFAULT uuidv7(),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    first_name TEXT NOT NULL, 
    last_name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS rubric (
    rubric_id UUID PRIMARY KEY DEFAULT uuidv7(),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS criterion (
    criterion_id UUID PRIMARY KEY DEFAULT uuidv7(),
    rubric_id UUID NOT NULL REFERENCES rubric(rubric_id),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    max_points SMALLINT NOT NULL CHECK (max_points > 0),
    position SMALLINT NOT NULL
);

CREATE TABLE IF NOT EXISTS interview (
    interview_id UUID PRIMARY KEY DEFAULT uuidv7(),
    rubric_id UUID NOT NULL REFERENCES rubric(rubric_id),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    mode TEXT NOT NULL,
    position SMALLINT NOT NULL,

    CONSTRAINT valid_mode CHECK (
        mode IN (
            'one_on_one', 
            'panel'
        )
    )
);

CREATE TABLE IF NOT EXISTS judge (
    judge_id UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id UUID NOT NULL REFERENCES users(user_id),
    interview_id UUID NOT NULL REFERENCES interview(interview_id),

    UNIQUE (user_id, interview_id)
);

CREATE TABLE IF NOT EXISTS score (
    score_id UUID PRIMARY KEY DEFAULT uuidv7(),
    judge_id UUID NOT NULL REFERENCES judge(judge_id),
    candidate_id UUID NOT NULL REFERENCES candidate(candidate_id),
    criterion_id UUID NOT NULL REFERENCES criterion(criterion_id),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    value SMALLINT NOT NULL CHECK (value >= 0),

    UNIQUE (judge_id, candidate_id, criterion_id)
);

-- +goose Down
DROP TABLE IF EXISTS score;
DROP TABLE IF EXISTS judge;
DROP TABLE IF EXISTS interview;
DROP TABLE IF EXISTS criterion;
DROP TABLE IF EXISTS rubric;
DROP TABLE IF EXISTS candidate;
DROP TABLE IF EXISTS users;
