ALTER TABLE athletes ADD COLUMN IF NOT EXISTS stage TEXT NOT NULL DEFAULT 'начальная подготовка';

CREATE TABLE IF NOT EXISTS norms (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  stage TEXT NOT NULL,
  discipline TEXT NOT NULL DEFAULT '',
  unit TEXT NOT NULL DEFAULT '',
  threshold NUMERIC NOT NULL,
  higher_is_better BOOLEAN NOT NULL DEFAULT TRUE,
  UNIQUE (name, stage, discipline)
);

CREATE TABLE IF NOT EXISTS norm_results (
  id BIGSERIAL PRIMARY KEY,
  athlete_id BIGINT NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
  norm_id BIGINT NOT NULL REFERENCES norms(id) ON DELETE CASCADE,
  value NUMERIC NOT NULL,
  passed BOOLEAN NOT NULL,
  entered_by BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_norm_results_athlete ON norm_results(athlete_id);

-- Демонстрационные пороги (условные, федерация может изменять через API)
INSERT INTO norms(name, stage, discipline, unit, threshold, higher_is_better) VALUES
 ('Скорость печати', 'начальная подготовка', '', 'зн/мин', 150, TRUE),
 ('Подтягивания (ОФП)', 'начальная подготовка', '', 'раз', 4, TRUE),
 ('Бег 30 м (ОФП)', 'начальная подготовка', '', 'с', 6.6, FALSE),
 ('Скорость печати', 'учебно-тренировочный этап', '', 'зн/мин', 220, TRUE),
 ('Подтягивания (ОФП)', 'учебно-тренировочный этап', '', 'раз', 7, TRUE),
 ('Бег 30 м (ОФП)', 'учебно-тренировочный этап', '', 'с', 6.0, FALSE)
ON CONFLICT DO NOTHING;
