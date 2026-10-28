CREATE TABLE IF NOT EXISTS prize_scale (
  sport_status TEXT NOT NULL,
  place INT NOT NULL,
  amount INT NOT NULL,
  PRIMARY KEY (sport_status, place)
);
CREATE TABLE IF NOT EXISTS prize_petitions (
  year INT PRIMARY KEY,
  status TEXT NOT NULL DEFAULT 'подготовлено',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Условные суммы для демонстрации; федерация меняет через POST /api/premiums/scale
INSERT INTO prize_scale(sport_status, place, amount) VALUES
 ('всероссийское',1,30000),('всероссийское',2,20000),('всероссийское',3,15000),
 ('межрегиональное',1,15000),('межрегиональное',2,10000),('межрегиональное',3,7000),
 ('региональное',1,5000),('региональное',2,3000),('региональное',3,2000)
ON CONFLICT DO NOTHING;
