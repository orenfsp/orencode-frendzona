ALTER TABLE athletes
  ADD COLUMN IF NOT EXISTS user_id BIGINT,
  ADD COLUMN IF NOT EXISTS birth_date DATE,
  ADD COLUMN IF NOT EXISTS municipality TEXT NOT NULL DEFAULT 'Оренбург',
  ADD COLUMN IF NOT EXISTS gender TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS year_started INTEGER NOT NULL DEFAULT 2023,
  ADD COLUMN IF NOT EXISTS rank_title TEXT NOT NULL DEFAULT 'Без разряда',
  ADD COLUMN IF NOT EXISTS rank_assigned_at DATE,
  ADD COLUMN IF NOT EXISTS rank_expires_at DATE,
  ADD COLUMN IF NOT EXISTS rank_order TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS fsp_id TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS disqualified BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS training_stage TEXT NOT NULL DEFAULT 'Начальная подготовка';

ALTER TABLE competitions
  ADD COLUMN IF NOT EXISTS sport_status TEXT NOT NULL DEFAULT 'региональное',
  ADD COLUMN IF NOT EXISTS event_format TEXT NOT NULL DEFAULT 'очно',
  ADD COLUMN IF NOT EXISTS calendar_plan TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS calendar_number TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS regulation TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS applications_until TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS users (
  id BIGSERIAL PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  full_name TEXT NOT NULL,
  birth_date DATE,
  municipality TEXT NOT NULL DEFAULT 'Оренбург',
  organization TEXT NOT NULL DEFAULT '',
  phone TEXT NOT NULL DEFAULT '',
  rusada_completed BOOLEAN NOT NULL DEFAULT FALSE,
  rusada_date DATE,
  rusada_url TEXT NOT NULL DEFAULT 'https://rusada.ru',
  consent_personal BOOLEAN NOT NULL DEFAULT TRUE,
  guardian_name TEXT NOT NULL DEFAULT '',
  guardian_consent BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_roles (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role TEXT NOT NULL,
  PRIMARY KEY (user_id, role)
);

CREATE TABLE IF NOT EXISTS sessions (
  token TEXT PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  active_role TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS competition_types (
  id BIGSERIAL PRIMARY KEY,
  title TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS athlete_disciplines (
  athlete_id BIGINT NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
  discipline TEXT NOT NULL,
  PRIMARY KEY (athlete_id, discipline)
);

CREATE TABLE IF NOT EXISTS coaches (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  organization TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS judges (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  category TEXT NOT NULL DEFAULT 'Судья третьей категории',
  category_expires_at DATE,
  organization TEXT NOT NULL DEFAULT 'ФСП Оренбургской области'
);

CREATE TABLE IF NOT EXISTS coach_links (
  id BIGSERIAL PRIMARY KEY,
  coach_id BIGINT NOT NULL REFERENCES coaches(id) ON DELETE CASCADE,
  athlete_id BIGINT NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
  discipline TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (coach_id, athlete_id, discipline)
);

CREATE TABLE IF NOT EXISTS applications (
  id BIGSERIAL PRIMARY KEY,
  competition_id BIGINT NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
  athlete_id BIGINT NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
  submitted_by BIGINT NOT NULL REFERENCES users(id),
  status TEXT NOT NULL DEFAULT 'pending',
  reject_reason TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (competition_id, athlete_id)
);

CREATE TABLE IF NOT EXISTS judge_assignments (
  id BIGSERIAL PRIMARY KEY,
  competition_id BIGINT NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
  judge_id BIGINT NOT NULL REFERENCES judges(id) ON DELETE CASCADE,
  position TEXT NOT NULL DEFAULT 'Судья',
  UNIQUE (competition_id, judge_id)
);

CREATE TABLE IF NOT EXISTS rank_applications (
  id BIGSERIAL PRIMARY KEY,
  athlete_id BIGINT NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
  submitted_by BIGINT NOT NULL REFERENCES users(id),
  competition_id BIGINT REFERENCES competitions(id),
  requested_rank TEXT NOT NULL,
  result_place INTEGER,
  status TEXT NOT NULL DEFAULT 'pending',
  decision_note TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS category_applications (
  id BIGSERIAL PRIMARY KEY,
  judge_id BIGINT NOT NULL REFERENCES judges(id) ON DELETE CASCADE,
  requested_category TEXT NOT NULL,
  kind TEXT NOT NULL DEFAULT 'присвоение',
  status TEXT NOT NULL DEFAULT 'pending',
  decision_note TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS team_roster (
  id BIGSERIAL PRIMARY KEY,
  season TEXT NOT NULL DEFAULT '2026',
  athlete_id BIGINT NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
  discipline TEXT NOT NULL,
  age_group TEXT NOT NULL DEFAULT '',
  UNIQUE (season, athlete_id, discipline)
);

CREATE TABLE IF NOT EXISTS inbox (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
  role_scope TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL DEFAULT 'info',
  title TEXT NOT NULL,
  message TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_log (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT,
  role TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL,
  entity TEXT NOT NULL DEFAULT '',
  details TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS certificates (
  id BIGSERIAL PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  athlete_id BIGINT NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
  competition_id BIGINT NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS enrollment_stats (
  id BIGSERIAL PRIMARY KEY,
  year INTEGER NOT NULL,
  municipality TEXT NOT NULL,
  organization TEXT NOT NULL,
  discipline TEXT NOT NULL,
  age_group TEXT NOT NULL,
  athletes_count INTEGER NOT NULL,
  UNIQUE (year, municipality, organization, discipline, age_group)
);

ALTER TABLE results
  ADD COLUMN IF NOT EXISTS score INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'финишировал';

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_applications_status ON applications(status);
CREATE INDEX IF NOT EXISTS idx_inbox_user ON inbox(user_id);
