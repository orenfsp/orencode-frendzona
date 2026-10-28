package database

import (
	"context"
	"database/sql"
	"time"

	"golang.org/x/crypto/bcrypt"

	"orenkod/backend/internal/rules"
)

func Seed(ctx context.Context, db *sql.DB) error {
	hash, err := bcrypt.GenerateFromPassword([]byte("demo"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	password := string(hash)

	types := []string{"Чемпионат", "Первенство", "Кубок", "Чемпионат России", "Первенство России", "Всероссийские соревнования", "Межрегиональные соревнования", "Региональные соревнования"}
	for _, title := range types {
		if _, err := db.ExecContext(ctx, `INSERT INTO competition_types(title) VALUES($1) ON CONFLICT (title) DO NOTHING`, title); err != nil {
			return err
		}
	}

	type userSpec struct {
		email, name, birth, mun, org, roles string
		athlete                             bool
		coach, judge                        bool
	}
	users := []userSpec{
		{"federation@fsp56.ru", "Анна Петрова", "1988-04-12", "Оренбург", "ФСП Оренбургской области", "federation", false, false, false},
		{"morozov@fsp56.ru", "Алексей Морозов", "2008-03-01", "Оренбург", "Лицей № 1", "athlete,coach", true, true, false},
		{"krylova@fsp56.ru", "Мария Крылова", "1991-06-18", "Оренбург", "Лицей № 1", "coach", false, true, false},
		{"volkov@fsp56.ru", "Илья Волков", "1987-11-02", "Оренбург", "ОГУ", "coach,judge", false, true, true},
		{"judge@fsp56.ru", "Сергей Новиков", "1984-01-20", "Оренбург", "ФСП Оренбургской области", "judge", false, false, true},
		{"sokolova@fsp56.ru", "Дарья Соколова", "2002-09-14", "Оренбург", "ОГУ", "athlete", true, false, false},
	}
	for _, u := range users {
		var id int64
		err := db.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,full_name,birth_date,municipality,organization,rusada_completed,rusada_date,consent_personal)
			VALUES($1,$2,$3,$4,$5,$6,TRUE,'2026-02-10',TRUE) ON CONFLICT (email) DO UPDATE SET full_name=EXCLUDED.full_name RETURNING id`,
			u.email, password, u.name, u.birth, u.mun, u.org).Scan(&id)
		if err != nil {
			return err
		}
		for _, role := range splitCSV(u.roles) {
			if _, err := db.ExecContext(ctx, `INSERT INTO user_roles(user_id,role) VALUES($1,$2) ON CONFLICT DO NOTHING`, id, role); err != nil {
				return err
			}
		}
		if u.coach {
			if _, err := db.ExecContext(ctx, `INSERT INTO coaches(user_id,organization) VALUES($1,$2) ON CONFLICT (user_id) DO NOTHING`, id, u.org); err != nil {
				return err
			}
		}
		if u.judge {
			if _, err := db.ExecContext(ctx, `INSERT INTO judges(user_id,category,category_expires_at,organization) VALUES($1,'Судья первой категории','2027-03-01',$2) ON CONFLICT (user_id) DO NOTHING`, id, u.org); err != nil {
				return err
			}
		}
	}

	competitions := []struct {
		title, discipline, kind, age, starts, location, status, sportStatus, format, plan, num string
		participants                                                                           int
	}{
		{"Кубок Оренбуржья по спортивному программированию", rules.Disciplines[0], "Кубок", "Юниоры и юниорки 18–20", "2026-11-08T10:00:00+05:00", "Оренбург · ОГУ", "Регистрация", "региональное", "смешанный", "Календарный план Оренбургской области", "56-2026-14", 0},
		{"Осенний алгоритмический спринт", rules.Disciplines[0], "Региональные соревнования", "Все группы", "2026-11-16T11:00:00+05:00", "Онлайн", "Регистрация", "региональное", "онлайн", "Календарный план Оренбургской области", "56-2026-15", 0},
		{"Отбор в сборную области", rules.Disciplines[1], "Первенство", "Юниоры и юниорки 18–20", "2026-11-29T09:30:00+05:00", "Оренбург · Точка кипения", "Планируется", "региональное", "очно", "Календарный план Оренбургской области", "56-2026-18", 0},
		{"Практикум по робототехнике", rules.Disciplines[4], "Региональные соревнования", "Юноши и девушки 14–17", "2026-12-06T12:00:00+05:00", "Орск · IT-центр", "Планируется", "региональное", "очно", "Календарный план Оренбургской области", "56-2026-19", 0},
		{"Региональная олимпиада по ИБ", rules.Disciplines[2], "Чемпионат", "Мужчины и женщины 21+", "2026-10-10T10:00:00+05:00", "Оренбург · ОГУ", "Завершено", "региональное", "очно", "Календарный план Оренбургской области", "56-2026-09", 3},
		{"Летняя школа разработки", rules.Disciplines[1], "Кубок", "Юноши и девушки 14–17", "2026-07-20T10:00:00+05:00", "Бузулук", "Завершено", "региональное", "смешанный", "Календарный план Оренбургской области", "56-2026-04", 2},
		{"Первенство ПФО по алгоритмам", rules.Disciplines[0], "Межрегиональные соревнования", "Юниоры и юниорки 18–20", "2026-03-15T10:00:00+05:00", "Самара", "Завершено", "межрегиональное", "очно", "ЕКП Минспорта России", "ЕКП-2026-441", 3},
		{"Чемпионат области по ИИ", rules.Disciplines[3], "Чемпионат", "Мужчины и женщины 21+", "2025-11-12T10:00:00+05:00", "Оренбург", "Завершено", "региональное", "онлайн", "Календарный план Оренбургской области", "56-2025-21", 2},
		{"Первенство области по БАС", rules.Disciplines[5], "Первенство", "Юноши и девушки 14–17", "2025-04-18T10:00:00+05:00", "Орск", "Завершено", "региональное", "очно", "Календарный план Оренбургской области", "56-2025-07", 2},
	}
	for _, c := range competitions {
		until := time.Now().Add(20 * 24 * time.Hour)
		_, err := db.ExecContext(ctx, `INSERT INTO competitions(title, discipline, competition_type, age_group, starts_at, location, status, participants, sport_status, event_format, calendar_plan, calendar_number, regulation, applications_until)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
			ON CONFLICT (title) DO UPDATE SET discipline=EXCLUDED.discipline, competition_type=EXCLUDED.competition_type, age_group=EXCLUDED.age_group, sport_status=EXCLUDED.sport_status, event_format=EXCLUDED.event_format, calendar_plan=EXCLUDED.calendar_plan, calendar_number=EXCLUDED.calendar_number, regulation=EXCLUDED.regulation`,
			c.title, c.discipline, c.kind, c.age, c.starts, c.location, c.status, c.participants, c.sportStatus, c.format, c.plan, c.num, "Положение прилагается. Возрастные группы и дисциплины — по правилам вида спорта.", until)
		if err != nil {
			return err
		}
	}

	athletes := []struct {
		name, org, disc, age, coach, mun, birth, rank, fsp string
		year, points                                       int
		email                                              string
	}{
		{"Алексей Морозов", "Лицей № 1", rules.Disciplines[0], "Юниоры и юниорки 18–20", "Мария Крылова", "Оренбург", "2008-03-01", "КМС", "FSP56-000101", 2019, 0, "morozov@fsp56.ru"},
		{"Дарья Соколова", "ОГУ", rules.Disciplines[1], "Мужчины и женщины 21+", "Илья Волков", "Оренбург", "2002-09-14", "1 разряд", "FSP56-000102", 2018, 0, "sokolova@fsp56.ru"},
		{"Максим Орлов", "Гимназия № 2", rules.Disciplines[4], "Юноши и девушки 14–17", "Мария Крылова", "Оренбург", "2010-05-22", "2 разряд", "FSP56-000103", 2022, 0, ""},
		{"Полина Васильева", "ОГУ", rules.Disciplines[2], "Мужчины и женщины 21+", "Илья Волков", "Оренбург", "2001-12-03", "1 разряд", "FSP56-000104", 2017, 0, ""},
		{"Артём Кузнецов", "Лицей № 1", rules.Disciplines[0], "Юниоры и юниорки 18–20", "Мария Крылова", "Оренбург", "2007-08-19", "3 разряд", "FSP56-000105", 2021, 0, ""},
		{"София Попова", "IT-центр Орск", rules.Disciplines[1], "Юноши и девушки 14–17", "Никита Егоров", "Орск", "2011-01-09", "3 разряд", "FSP56-000106", 2023, 0, ""},
		{"Иван Лебедев", "ОГУ", rules.Disciplines[0], "Мужчины и женщины 21+", "Илья Волков", "Оренбург", "1999-04-27", "КМС", "FSP56-000107", 2016, 0, ""},
		{"Ева Фёдорова", "Гимназия № 2", rules.Disciplines[4], "Юноши и девушки 14–17", "Мария Крылова", "Оренбург", "2010-10-11", "2 разряд", "FSP56-000108", 2022, 0, ""},
		{"Кирилл Павлов", "IT-центр Орск", rules.Disciplines[2], "Мужчины и женщины 21+", "Никита Егоров", "Орск", "2000-07-30", "1 разряд", "FSP56-000109", 2018, 0, ""},
		{"Алиса Семёнова", "Лицей № 1", rules.Disciplines[3], "Юниоры и юниорки 18–20", "Мария Крылова", "Оренбург", "2006-02-14", "3 разряд", "FSP56-000110", 2020, 0, ""},
		{"Тимур Белов", "Авиацентр", rules.Disciplines[5], "Юноши и девушки 14–17", "Никита Егоров", "Бузулук", "2011-06-02", "3 разряд", "FSP56-000111", 2023, 0, ""},
		{"Ника Громова", "ОГУ", rules.Disciplines[3], "Мужчины и женщины 21+", "Илья Волков", "Оренбург", "2003-03-21", "2 разряд", "FSP56-000112", 2019, 0, ""},
	}
	for _, a := range athletes {
		var userID any
		if a.email != "" {
			var id int64
			if err := db.QueryRowContext(ctx, `SELECT id FROM users WHERE email=$1`, a.email).Scan(&id); err == nil {
				userID = id
			}
		}
		var athleteID int64
		err := db.QueryRowContext(ctx, `INSERT INTO athletes(full_name, organization, discipline, age_group, points, coach_name, status, user_id, birth_date, municipality, year_started, rank_title, rank_assigned_at, rank_expires_at, fsp_id)
			VALUES ($1,$2,$3,$4,$5,$6,'Активен',$7,$8,$9,$10,$11,'2024-09-01','2026-12-15',$12)
			ON CONFLICT (full_name, organization) DO UPDATE SET discipline=EXCLUDED.discipline, age_group=EXCLUDED.age_group, coach_name=EXCLUDED.coach_name, user_id=COALESCE(EXCLUDED.user_id, athletes.user_id), birth_date=EXCLUDED.birth_date, municipality=EXCLUDED.municipality, year_started=EXCLUDED.year_started, rank_title=EXCLUDED.rank_title, rank_expires_at=EXCLUDED.rank_expires_at, fsp_id=EXCLUDED.fsp_id
			RETURNING id`, a.name, a.org, a.disc, a.age, a.points, a.coach, userID, a.birth, a.mun, a.year, a.rank, a.fsp).Scan(&athleteID)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO athlete_disciplines(athlete_id,discipline) VALUES($1,$2) ON CONFLICT DO NOTHING`, athleteID, a.disc); err != nil {
			return err
		}
	}

	if err := seedLinksAndApps(ctx, db); err != nil {
		return err
	}
	if err := seedResults(ctx, db); err != nil {
		return err
	}
	if err := seedEnrollment(ctx, db); err != nil {
		return err
	}
	if err := seedInbox(ctx, db); err != nil {
		return err
	}
	return nil
}

func seedLinksAndApps(ctx context.Context, db *sql.DB) error {
	var coachID, athleteID, morozovAthlete, cupID, judgeID, volkovJudge int64
	_ = db.QueryRowContext(ctx, `SELECT id FROM coaches WHERE user_id=(SELECT id FROM users WHERE email='krylova@fsp56.ru')`).Scan(&coachID)
	_ = db.QueryRowContext(ctx, `SELECT id FROM athletes WHERE full_name='Алексей Морозов'`).Scan(&athleteID)
	_ = db.QueryRowContext(ctx, `SELECT id FROM athletes WHERE full_name='Алексей Морозов'`).Scan(&morozovAthlete)
	_ = db.QueryRowContext(ctx, `SELECT id FROM competitions WHERE title LIKE 'Кубок Оренбуржья%'`).Scan(&cupID)
	_ = db.QueryRowContext(ctx, `SELECT id FROM judges WHERE user_id=(SELECT id FROM users WHERE email='judge@fsp56.ru')`).Scan(&judgeID)
	_ = db.QueryRowContext(ctx, `SELECT id FROM judges WHERE user_id=(SELECT id FROM users WHERE email='volkov@fsp56.ru')`).Scan(&volkovJudge)
	var krylovaUser, morozovUser int64
	_ = db.QueryRowContext(ctx, `SELECT id FROM users WHERE email='krylova@fsp56.ru'`).Scan(&krylovaUser)
	_ = db.QueryRowContext(ctx, `SELECT id FROM users WHERE email='morozov@fsp56.ru'`).Scan(&morozovUser)

	if coachID != 0 && athleteID != 0 {
		_, _ = db.ExecContext(ctx, `INSERT INTO coach_links(coach_id,athlete_id,discipline,status) VALUES($1,$2,$3,'pending') ON CONFLICT DO NOTHING`, coachID, athleteID, rules.Disciplines[0])
	}
	var orlov int64
	_ = db.QueryRowContext(ctx, `SELECT id FROM athletes WHERE full_name='Максим Орлов'`).Scan(&orlov)
	if coachID != 0 && orlov != 0 {
		_, _ = db.ExecContext(ctx, `INSERT INTO coach_links(coach_id,athlete_id,discipline,status) VALUES($1,$2,$3,'active') ON CONFLICT DO NOTHING`, coachID, orlov, rules.Disciplines[4])
	}
	if cupID != 0 && morozovAthlete != 0 && morozovUser != 0 {
		_, _ = db.ExecContext(ctx, `INSERT INTO applications(competition_id,athlete_id,submitted_by,status) VALUES($1,$2,$3,'pending') ON CONFLICT DO NOTHING`, cupID, morozovAthlete, morozovUser)
	}
	var sprintID, kuznetsov int64
	_ = db.QueryRowContext(ctx, `SELECT id FROM competitions WHERE title LIKE 'Осенний алгоритмический%'`).Scan(&sprintID)
	_ = db.QueryRowContext(ctx, `SELECT id FROM athletes WHERE full_name='Артём Кузнецов'`).Scan(&kuznetsov)
	if sprintID != 0 && kuznetsov != 0 && krylovaUser != 0 {
		_, _ = db.ExecContext(ctx, `INSERT INTO applications(competition_id,athlete_id,submitted_by,status) VALUES($1,$2,$3,'approved') ON CONFLICT DO NOTHING`, sprintID, kuznetsov, krylovaUser)
	}
	if cupID != 0 && judgeID != 0 {
		_, _ = db.ExecContext(ctx, `INSERT INTO judge_assignments(competition_id,judge_id,position) VALUES($1,$2,'Главный судья') ON CONFLICT DO NOTHING`, cupID, judgeID)
	}
	if cupID != 0 && volkovJudge != 0 {
		_, _ = db.ExecContext(ctx, `INSERT INTO judge_assignments(competition_id,judge_id,position) VALUES($1,$2,'Судья-секретарь') ON CONFLICT DO NOTHING`, cupID, volkovJudge)
	}
	if morozovAthlete != 0 && morozovUser != 0 {
		var pfo int64
		_ = db.QueryRowContext(ctx, `SELECT id FROM competitions WHERE title LIKE 'Первенство ПФО%'`).Scan(&pfo)
		_, _ = db.ExecContext(ctx, `INSERT INTO rank_applications(athlete_id,submitted_by,competition_id,requested_rank,result_place,status)
			SELECT $1,$2,$3,'КМС',1,'pending' WHERE NOT EXISTS (SELECT 1 FROM rank_applications WHERE athlete_id=$1 AND requested_rank='КМС')`, morozovAthlete, morozovUser, pfo)
	}
	if judgeID != 0 {
		_, _ = db.ExecContext(ctx, `INSERT INTO category_applications(judge_id,requested_category,kind,status)
			SELECT $1,'Спортивный судья всероссийской категории','подтверждение','pending' WHERE NOT EXISTS (SELECT 1 FROM category_applications WHERE judge_id=$1)`, judgeID)
	}
	return nil
}

func seedResults(ctx context.Context, db *sql.DB) error {
	pairs := []struct {
		athlete, competition string
		place, score         int
	}{
		{"Алексей Морозов", "Первенство ПФО по алгоритмам", 1, 980},
		{"Артём Кузнецов", "Первенство ПФО по алгоритмам", 2, 910},
		{"Иван Лебедев", "Первенство ПФО по алгоритмам", 3, 870},
		{"Полина Васильева", "Региональная олимпиада по ИБ", 1, 940},
		{"Кирилл Павлов", "Региональная олимпиада по ИБ", 2, 888},
		{"Дарья Соколова", "Региональная олимпиада по ИБ", 3, 850},
		{"Дарья Соколова", "Летняя школа разработки", 1, 900},
		{"София Попова", "Летняя школа разработки", 2, 820},
		{"Ника Громова", "Чемпионат области по ИИ", 1, 930},
		{"Алиса Семёнова", "Чемпионат области по ИИ", 2, 870},
		{"Тимур Белов", "Первенство области по БАС", 1, 800},
		{"Максим Орлов", "Первенство области по БАС", 3, 700},
	}
	for _, p := range pairs {
		var athleteID, compID int64
		if err := db.QueryRowContext(ctx, `SELECT id FROM athletes WHERE full_name=$1`, p.athlete).Scan(&athleteID); err != nil {
			continue
		}
		if err := db.QueryRowContext(ctx, `SELECT id FROM competitions WHERE title=$1`, p.competition).Scan(&compID); err != nil {
			continue
		}
		_, err := db.ExecContext(ctx, `INSERT INTO applications(competition_id,athlete_id,submitted_by,status)
			SELECT $1,$2,(SELECT id FROM users WHERE email='federation@fsp56.ru'),'approved'
			WHERE NOT EXISTS (SELECT 1 FROM applications WHERE competition_id=$1 AND athlete_id=$2)`, compID, athleteID)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO results(athlete_id,competition_id,place,points,score,status) VALUES($1,$2,$3,0,$4,'финишировал')
			ON CONFLICT (athlete_id, competition_id) DO UPDATE SET place=EXCLUDED.place, score=EXCLUDED.score`, athleteID, compID, p.place, p.score); err != nil {
			return err
		}
	}

	rows, err := db.QueryContext(ctx, `SELECT r.id,r.place,c.sport_status,c.competition_type FROM results r JOIN competitions c ON c.id=r.competition_id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type rec struct {
		id, place int64
		status, kind string
	}
	list := []rec{}
	for rows.Next() {
		var item rec
		if err := rows.Scan(&item.id, &item.place, &item.status, &item.kind); err != nil {
			return err
		}
		list = append(list, item)
	}
	for _, item := range list {
		pts := rules.PlacePoints(item.status, item.kind, int(item.place))
		if _, err := db.ExecContext(ctx, `UPDATE results SET points=$1 WHERE id=$2`, pts, item.id); err != nil {
			return err
		}
	}
	athRows, err := db.QueryContext(ctx, `SELECT id FROM athletes`)
	if err != nil {
		return err
	}
	defer athRows.Close()
	ids := []int64{}
	for athRows.Next() {
		var id int64
		_ = athRows.Scan(&id)
		ids = append(ids, id)
	}
	for _, id := range ids {
		var raw, prizes int
		res, err := db.QueryContext(ctx, `SELECT r.place,r.points FROM results r JOIN competitions c ON c.id=r.competition_id WHERE r.athlete_id=$1 AND c.starts_at >= '2025-09-01'`, id)
		if err != nil {
			return err
		}
		for res.Next() {
			var place, points int
			_ = res.Scan(&place, &points)
			raw += points
			if place <= 3 && place > 0 {
				prizes++
			}
		}
		res.Close()
		total := int(float64(raw) * rules.StabilityMultiplier(prizes))
		if _, err := db.ExecContext(ctx, `UPDATE athletes SET points=$1 WHERE id=$2`, total, id); err != nil {
			return err
		}
	}

	var topID int64
	_ = db.QueryRowContext(ctx, `SELECT id FROM athletes ORDER BY points DESC LIMIT 1`).Scan(&topID)
	if topID != 0 {
		_, _ = db.ExecContext(ctx, `INSERT INTO team_roster(season,athlete_id,discipline,age_group)
			SELECT '2026', id, discipline, age_group FROM athletes WHERE points > 0 ORDER BY points DESC LIMIT 5
			ON CONFLICT DO NOTHING`)
	}
	return nil
}

func seedEnrollment(ctx context.Context, db *sql.DB) error {
	rows := [][]any{
		{2024, "Оренбург", "Лицей № 1", rules.Disciplines[0], "Юноши и девушки 14–17", 18},
		{2024, "Оренбург", "ОГУ", rules.Disciplines[1], "Мужчины и женщины 21+", 22},
		{2024, "Орск", "IT-центр Орск", rules.Disciplines[4], "Юноши и девушки 14–17", 14},
		{2024, "Бузулук", "Авиацентр", rules.Disciplines[5], "Юноши и девушки 14–17", 9},
		{2025, "Оренбург", "Лицей № 1", rules.Disciplines[0], "Юноши и девушки 14–17", 24},
		{2025, "Оренбург", "ОГУ", rules.Disciplines[1], "Мужчины и женщины 21+", 27},
		{2025, "Орск", "IT-центр Орск", rules.Disciplines[2], "Мужчины и женщины 21+", 16},
		{2025, "Бузулук", "Авиацентр", rules.Disciplines[5], "Юноши и девушки 14–17", 12},
		{2026, "Оренбург", "Лицей № 1", rules.Disciplines[0], "Юниоры и юниорки 18–20", 29},
		{2026, "Оренбург", "ОГУ", rules.Disciplines[3], "Мужчины и женщины 21+", 31},
		{2026, "Орск", "IT-центр Орск", rules.Disciplines[4], "Юноши и девушки 14–17", 19},
		{2026, "Бузулук", "Авиацентр", rules.Disciplines[5], "Юноши и девушки 14–17", 15},
	}
	for _, row := range rows {
		if _, err := db.ExecContext(ctx, `INSERT INTO enrollment_stats(year,municipality,organization,discipline,age_group,athletes_count) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, row...); err != nil {
			return err
		}
	}
	return nil
}

func seedInbox(ctx context.Context, db *sql.DB) error {
	items := [][]string{
		{"federation", "warning", "Истекают разряды", "У части спортсменов разряд действует до 15.12.2026 — предупредите тренеров."},
		{"federation", "info", "Заявки на модерации", "Есть заявки на соревнования, разряд и судейскую категорию."},
		{"judge", "warning", "Срок категории", "Проверьте срок судейской категории и подайте заявку на подтверждение."},
		{"athlete", "info", "Открыта регистрация", "Кубок Оренбуржья принимает заявки. Платформа проверит возраст и дисциплину."},
		{"coach", "info", "Привязка спортсмена", "Запрос на привязку ждёт подтверждения спортсмена."},
	}
	for _, item := range items {
		if _, err := db.ExecContext(ctx, `INSERT INTO inbox(role_scope,kind,title,message) SELECT $1,$2,$3,$4 WHERE NOT EXISTS (SELECT 1 FROM inbox WHERE title=$3 AND role_scope=$1)`, item[0], item[1], item[2], item[3]); err != nil {
			return err
		}
	}
	return nil
}

func splitCSV(value string) []string {
	parts := []string{}
	start := 0
	for i, r := range value {
		if r == ',' {
			parts = append(parts, trim(value[start:i]))
			start = i + 1
		}
	}
	parts = append(parts, trim(value[start:]))
	return parts
}

func trim(value string) string {
	i, j := 0, len(value)
	for i < j && (value[i] == ' ' || value[i] == '\t') {
		i++
	}
	for j > i && (value[j-1] == ' ' || value[j-1] == '\t') {
		j--
	}
	return value[i:j]
}
