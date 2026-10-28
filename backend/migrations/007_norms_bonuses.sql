-- Д4: нормативы ФССП и аттестация
CREATE TABLE IF NOT EXISTS norms (
  id BIGSERIAL PRIMARY KEY,
  stage TEXT NOT NULL,
  discipline TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL,
  unit TEXT NOT NULL DEFAULT '',
  higher_is_better BOOLEAN NOT NULL DEFAULT TRUE,
  threshold DOUBLE PRECISION NOT NULL,
  UNIQUE (stage, discipline, name)
);
CREATE TABLE IF NOT EXISTS norm_results (
  id BIGSERIAL PRIMARY KEY,
  athlete_id BIGINT NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
  norm_id BIGINT NOT NULL REFERENCES norms(id) ON DELETE CASCADE,
  value DOUBLE PRECISION NOT NULL,
  passed BOOLEAN NOT NULL,
  taken_at DATE NOT NULL DEFAULT CURRENT_DATE,
  entered_by BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS attestations (
  id BIGSERIAL PRIMARY KEY,
  athlete_id BIGINT NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
  from_stage TEXT NOT NULL,
  to_stage TEXT NOT NULL,
  decided_by BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Д8: премии Губернатора
CREATE TABLE IF NOT EXISTS bonus_rates (
  level TEXT NOT NULL,
  place INTEGER NOT NULL,
  amount INTEGER NOT NULL,
  PRIMARY KEY (level, place)
);
CREATE TABLE IF NOT EXISTS bonus_petitions (
  year INTEGER PRIMARY KEY,
  status TEXT NOT NULL DEFAULT 'draft',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Д6: подтверждающие документы показателей
CREATE TABLE IF NOT EXISTS indicator_docs (
  id BIGSERIAL PRIMARY KEY,
  indicator TEXT NOT NULL,
  title TEXT NOT NULL,
  url TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
