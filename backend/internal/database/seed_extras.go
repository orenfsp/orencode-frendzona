package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// SeedExtras добавляет демо-данные для Д3, Д5, Д9. Повторный запуск ничего не дублирует.
func SeedExtras(ctx context.Context, db *sql.DB) error {
	// Д9: аккаунт министерства (read-only), пароль такой же, как у остальных демо-аккаунтов
	if _, err := db.ExecContext(ctx, `INSERT INTO users(email,password_hash,full_name,municipality,organization,rusada_completed,consent_personal)
		SELECT 'ministry@fsp56.ru',password_hash,'Елена Захарова','Оренбург','Министерство физической культуры и спорта Оренбургской области',TRUE,TRUE
		FROM users WHERE email='federation@fsp56.ru' ON CONFLICT (email) DO NOTHING`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO user_roles(user_id,role) SELECT id,'ministry' FROM users WHERE email='ministry@fsp56.ru' ON CONFLICT DO NOTHING`); err != nil {
		return err
	}

	// Д3: организации
	if _, err := db.ExecContext(ctx, `INSERT INTO organizations(name,org_type,municipality,license,programs)
		SELECT DISTINCT o.name,
		  CASE WHEN o.name LIKE 'ОГУ%' OR o.name ILIKE '%университет%' THEN 'Образовательная организация высшего образования'
		       WHEN o.name ILIKE '%центр%' THEN 'Организация дополнительного образования'
		       WHEN o.name ILIKE 'Лицей%' OR o.name ILIKE 'Гимназия%' OR o.name ILIKE 'Школа%' THEN 'Общеобразовательная организация'
		       ELSE 'Спортивный клуб' END,
		  COALESCE((SELECT a.municipality FROM athletes a WHERE a.organization=o.name LIMIT 1),'Оренбург'),
		  '', 'Программа спортивной подготовки по виду спорта «спортивное программирование»: этапы НП и УТЭ'
		FROM (SELECT organization AS name FROM athletes UNION SELECT organization FROM coaches) o
		WHERE o.name<>'' AND o.name NOT LIKE 'ФСП%' AND o.name NOT ILIKE 'Без организации%'
		ON CONFLICT (name) DO NOTHING`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `UPDATE organizations SET license='Л035-01234-56/'||LPAD(id::text,6,'0')||' (демо)' WHERE license=''`); err != nil {
		return err
	}

	var groups int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM training_groups`).Scan(&groups); err != nil {
		return err
	}
	if groups == 0 {
		if err := seedGroups(ctx, db); err != nil {
			return err
		}
	}
	if err := seedNormsBonuses(ctx, db); err != nil {
		return err
	}
	return seedCases(ctx, db)
}

// Пороги нормативов и размеры премий — демонстрационные; федерация меняет их в интерфейсе.
func seedNormsBonuses(ctx context.Context, db *sql.DB) error {
	stages := []string{"Начальная подготовка", "Учебно-тренировочный этап", "Совершенствование спортивного мастерства", "Высшее спортивное мастерство"}
	typing := []float64{120, 200, 280, 340}
	pull := []float64{3, 5, 8, 10}
	run := []float64{6.4, 6.0, 5.6, 5.4}
	for i, st := range stages {
		rows := []struct {
			disc, name, unit string
			higher           bool
			th               float64
		}{
			{"", "Скорость печати на клавиатуре", "зн./мин", true, typing[i]},
			{"", "Подтягивания на перекладине", "раз", true, pull[i]},
			{"", "Бег на 30 м", "с", false, run[i]},
			{"Программирование алгоритмическое", "Решено задач на тренировочном туре", "шт.", true, float64(2 + i*2)},
		}
		for _, n := range rows {
			if _, err := db.ExecContext(ctx, `INSERT INTO norms(stage,discipline,name,unit,higher_is_better,threshold) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, st, n.disc, n.name, n.unit, n.higher, n.th); err != nil {
				return err
			}
		}
	}
	rates := map[string][3]int{"региональное": {30000, 20000, 10000}, "межрегиональное": {60000, 45000, 30000}, "всероссийское": {100000, 80000, 60000}}
	for level, a := range rates {
		for i, amount := range a {
			if _, err := db.ExecContext(ctx, `INSERT INTO bonus_rates(level,place,amount) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, level, i+1, amount); err != nil {
				return err
			}
		}
	}
	return nil
}

func seedGroups(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `SELECT o.id,o.name,a.discipline,BOOL_OR(a.rank_title<>'Без разряда')
		FROM organizations o JOIN athletes a ON a.organization=o.name GROUP BY o.id,o.name,a.discipline ORDER BY o.name,a.discipline`)
	if err != nil {
		return err
	}
	type spec struct {
		orgID      int64
		org, disc  string
		ranked     bool
	}
	var specs []spec
	for rows.Next() {
		var s spec
		if err := rows.Scan(&s.orgID, &s.org, &s.disc, &s.ranked); err != nil {
			rows.Close()
			return err
		}
		specs = append(specs, s)
	}
	rows.Close()
	for i, s := range specs {
		stage := "Начальная подготовка"
		if s.ranked {
			stage = "Учебно-тренировочный этап"
		}
		short := strings.TrimPrefix(s.disc, "Программирование ")
		var gid int64
		err := db.QueryRowContext(ctx, `INSERT INTO training_groups(organization_id,name,stage,discipline,coach_id)
			VALUES($1,$2,$3,$4,(SELECT c.id FROM coaches c WHERE c.organization=$5 ORDER BY c.id LIMIT 1))
			ON CONFLICT (organization_id,name) DO UPDATE SET stage=EXCLUDED.stage RETURNING id`,
			s.orgID, fmt.Sprintf("Группа %d — %s", i+1, short), stage, s.disc, s.org).Scan(&gid)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO group_members(group_id,athlete_id,joined_at)
			SELECT $1,a.id,make_date(LEAST(a.year_started,EXTRACT(YEAR FROM CURRENT_DATE)::int),9,1) FROM athletes a WHERE a.organization=$2 AND a.discipline=$3`, gid, s.org, s.disc); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `UPDATE athletes SET training_stage=$1 WHERE organization=$2 AND discipline=$3`, stage, s.org, s.disc); err != nil {
			return err
		}
		for n := 0; n < 6; n++ {
			format, proof := "очно", ""
			if n%3 == 2 {
				format, proof = "онлайн", fmt.Sprintf("https://example.org/records/group-%d/session-%d", gid, n+1)
			}
			var sid int64
			if err := db.QueryRowContext(ctx, `INSERT INTO training_sessions(group_id,held_at,topic,format,proof_url) VALUES($1,CURRENT_DATE-$2::int,$3,$4,$5) RETURNING id`,
				gid, (6-n)*7, fmt.Sprintf("Занятие %d: разбор задач и практика", n+1), format, proof).Scan(&sid); err != nil {
				return err
			}
			if _, err := db.ExecContext(ctx, `INSERT INTO session_attendance(session_id,athlete_id)
				SELECT $1,athlete_id FROM group_members WHERE group_id=$2 AND (athlete_id+$3::int)%5<>0`, sid, gid, n); err != nil {
				return err
			}
		}
	}
	return nil
}

// seedCases создаёт по одному примеру апелляции и нарушения, чтобы раздел Д5 не был пустым.
func seedCases(ctx context.Context, db *sql.DB) error {
	var n int
	if err := db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM appeals)+(SELECT COUNT(*) FROM violations)`).Scan(&n); err != nil || n > 0 {
		return err
	}
	var compID, athleteID, userID int64
	err := db.QueryRowContext(ctx, `SELECT r.competition_id,r.athlete_id,a.user_id FROM results r JOIN athletes a ON a.id=r.athlete_id
		WHERE a.user_id IS NOT NULL AND r.place>1 ORDER BY r.competition_id DESC,r.place LIMIT 1`).Scan(&compID, &athleteID, &userID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	var id int64
	if err := db.QueryRowContext(ctx, `INSERT INTO appeals(competition_id,athlete_id,submitted_by,subject,description,evidence_url)
		VALUES($1,$2,$3,'Несогласие с оценкой задачи C','Решение прошло все тесты в тестирующей системе, но баллы за задачу C не засчитаны. Прикладываю лог посылок.','https://example.org/logs/submission-1042') RETURNING id`, compID, athleteID, userID).Scan(&id); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO case_history(case_type,case_id,status,note,user_id) VALUES('appeal',$1,'submitted','Апелляция подана',$2)`, id, userID); err != nil {
		return err
	}
	var fedID int64
	if err := db.QueryRowContext(ctx, `SELECT user_id FROM user_roles WHERE role='federation' LIMIT 1`).Scan(&fedID); err != nil {
		return nil
	}
	var otherAthlete int64
	if err := db.QueryRowContext(ctx, `SELECT r.athlete_id FROM results r WHERE r.competition_id=$1 AND r.athlete_id<>$2 ORDER BY r.place DESC LIMIT 1`, compID, athleteID).Scan(&otherAthlete); err != nil {
		return nil
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO violations(competition_id,athlete_id,reported_by,kind,report,evidence_url,status,sanction,decision,decided_at)
		VALUES($1,$2,$3,'Нарушение правил соревнования','Во время тура зафиксировано использование постороннего ресурса с готовыми решениями.','https://example.org/evidence/proctoring-77','decided','warning','Вынесено предупреждение, повторное нарушение влечёт дисквалификацию.',NOW()) RETURNING id`,
		compID, otherAthlete, fedID).Scan(&id); err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO case_history(case_type,case_id,status,note,user_id) VALUES('violation',$1,'reported','Рапорт подан: нарушение правил соревнования',$2),('violation',$1,'decided','Вынесено предупреждение',$2)`, id, fedID)
	return err
}
