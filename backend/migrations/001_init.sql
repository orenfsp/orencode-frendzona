CREATE TABLE IF NOT EXISTS athletes (
  id BIGSERIAL PRIMARY KEY,
  full_name TEXT NOT NULL,
  organization TEXT NOT NULL,
  discipline TEXT NOT NULL,
  age_group TEXT NOT NULL,
  points INTEGER NOT NULL DEFAULT 0,
  coach_name TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'Активен',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (full_name, organization)
);

CREATE TABLE IF NOT EXISTS competitions (
  id BIGSERIAL PRIMARY KEY,
  title TEXT NOT NULL UNIQUE,
  discipline TEXT NOT NULL,
  competition_type TEXT NOT NULL DEFAULT 'Региональный',
  age_group TEXT NOT NULL DEFAULT 'Все группы',
  starts_at TIMESTAMPTZ NOT NULL,
  location TEXT NOT NULL DEFAULT 'Оренбург',
  status TEXT NOT NULL DEFAULT 'Черновик',
  participants INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS results (
  id BIGSERIAL PRIMARY KEY,
  athlete_id BIGINT NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
  competition_id BIGINT NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
  place INTEGER NOT NULL CHECK (place > 0),
  points INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (athlete_id, competition_id)
);

CREATE INDEX IF NOT EXISTS idx_athletes_points ON athletes(points DESC);
CREATE INDEX IF NOT EXISTS idx_competitions_starts_at ON competitions(starts_at);
