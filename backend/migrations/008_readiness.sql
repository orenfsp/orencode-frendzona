-- Собственный сценарий «Готовность к старту»: журнал отправленных напоминаний (без дублей).
CREATE TABLE IF NOT EXISTS readiness_reminders (
  competition_id BIGINT NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
  athlete_id BIGINT NOT NULL REFERENCES athletes(id) ON DELETE CASCADE,
  bucket INT NOT NULL,
  sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (competition_id, athlete_id, bucket)
);
