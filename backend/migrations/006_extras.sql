-- Д3: организации спортивной подготовки, группы, журнал занятий
CREATE TABLE IF NOT EXISTS organizations (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  org_type TEXT NOT NULL DEFAULT 'Школа',
  municipality TEXT NOT NULL DEFAULT 'Оренбург',
  license TEXT NOT NULL DEFAULT '',
  programs TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS training_groups (
  id BIGSERIAL PRIMARY KEY,
  organization_id BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  stage TEXT NOT NULL,
  discipline TEXT NOT NULL,
  coach_id BIGINT REFERENCES coaches(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (organization_id, name)
);
CREATE TABLE IF NOT EXISTS group_members (
  id BIGSERIAL PRIMARY KEY,
  group_id BIGINT NOT NULL REFERENCES training_groups(id) ON DELETE CASCADE,
  athlete_id BIGINT NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
  joined_at DATE NOT NULL DEFAULT CURRENT_DATE,
  left_at DATE,
  note TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_group_members_active ON group_members(group_id, athlete_id) WHERE left_at IS NULL;
CREATE TABLE IF NOT EXISTS training_sessions (
  id BIGSERIAL PRIMARY KEY,
  group_id BIGINT NOT NULL REFERENCES training_groups(id) ON DELETE CASCADE,
  held_at DATE NOT NULL,
  topic TEXT NOT NULL,
  format TEXT NOT NULL DEFAULT 'очно',
  proof_url TEXT NOT NULL DEFAULT '',
  created_by BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS session_attendance (
  session_id BIGINT NOT NULL REFERENCES training_sessions(id) ON DELETE CASCADE,
  athlete_id BIGINT NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
  PRIMARY KEY (session_id, athlete_id)
);

-- Д5: апелляции и фиксация нарушений
CREATE TABLE IF NOT EXISTS appeals (
  id BIGSERIAL PRIMARY KEY,
  competition_id BIGINT NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
  athlete_id BIGINT NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
  submitted_by BIGINT NOT NULL REFERENCES users(id),
  subject TEXT NOT NULL,
  description TEXT NOT NULL,
  evidence_url TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'submitted',
  decision TEXT NOT NULL DEFAULT '',
  new_place INTEGER,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  decided_at TIMESTAMPTZ
);
CREATE TABLE IF NOT EXISTS violations (
  id BIGSERIAL PRIMARY KEY,
  competition_id BIGINT NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
  athlete_id BIGINT NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
  reported_by BIGINT NOT NULL REFERENCES users(id),
  kind TEXT NOT NULL,
  report TEXT NOT NULL,
  evidence_url TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'reported',
  sanction TEXT NOT NULL DEFAULT '',
  decision TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  decided_at TIMESTAMPTZ
);
CREATE TABLE IF NOT EXISTS case_history (
  id BIGSERIAL PRIMARY KEY,
  case_type TEXT NOT NULL,
  case_id BIGINT NOT NULL,
  status TEXT NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  user_id BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_case_history ON case_history(case_type, case_id);
