package platform

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"orenkod/backend/internal/auth"
	"orenkod/backend/internal/exportutil"
	"orenkod/backend/internal/httpserver"
	"orenkod/backend/internal/rules"
)

func (h *Handler) mountExtras2(mux *http.ServeMux, a *auth.Service) {
	fed := a.Middleware(rules.RoleFederation)
	fedCoach := a.Middleware(rules.RoleFederation, rules.RoleCoach)
	fedMin := a.Middleware(rules.RoleFederation, rules.RoleMinistry)
	athlete := a.Middleware(rules.RoleAthlete)
	// Д4
	mux.HandleFunc("GET /api/norms", fedCoach(h.NormList))
	mux.HandleFunc("POST /api/norms", fed(h.NormSave))
	mux.HandleFunc("POST /api/norm-results", fedCoach(h.NormResultAdd))
	mux.HandleFunc("GET /api/norm-results", a.Middleware(rules.RoleFederation, rules.RoleCoach, rules.RoleAthlete)(h.NormResultList))
	mux.HandleFunc("GET /api/attestation", fedCoach(h.AttestationList))
	mux.HandleFunc("POST /api/attestation/promote", fedCoach(h.AttestationPromote))
	// Д8
	mux.HandleFunc("GET /api/bonuses", fedMin(h.BonusList))
	mux.HandleFunc("PUT /api/bonus-rates", fed(h.BonusRateSave))
	mux.HandleFunc("POST /api/bonuses/petition", fed(h.BonusPetition))
	mux.HandleFunc("GET /api/bonuses/package", fed(h.BonusPackage))
	// Д6
	mux.HandleFunc("POST /api/development/docs", fed(h.DevDocAdd))
	// Д7, Д10
	mux.HandleFunc("GET /api/path", athlete(h.AthletePath))
	mux.HandleFunc("GET /api/public/glossary", h.Glossary)
	mux.HandleFunc("GET /api/public/verify/{code}", serveVerifyPage)
}

// ---------- Д6: показатели развития вида спорта ----------

func (h *Handler) Development(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	if year == 0 {
		year = time.Now().Year()
	}
	years := []map[string]any{}
	if rows, err := h.db.QueryContext(ctx, `SELECT year,SUM(athletes_count) FROM enrollment_stats GROUP BY year ORDER BY year`); err == nil {
		for rows.Next() {
			var y, n int
			if rows.Scan(&y, &n) == nil {
				years = append(years, map[string]any{"year": y, "athletes": n})
			}
		}
		rows.Close()
	}
	orgs := []map[string]any{}
	if rows, err := h.db.QueryContext(ctx, `SELECT o.name,o.license,(SELECT COUNT(*) FROM athletes a WHERE a.organization=o.name),(SELECT COUNT(*) FROM training_groups g WHERE g.organization_id=o.id) FROM organizations o ORDER BY o.name`); err == nil {
		for rows.Next() {
			var name, lic string
			var ath, grp int
			if rows.Scan(&name, &lic, &ath, &grp) == nil {
				orgs = append(orgs, map[string]any{"name": name, "license": lic, "athletes": ath, "groups": grp})
			}
		}
		rows.Close()
	}
	comps := []map[string]any{}
	if rows, err := h.db.QueryContext(ctx, `SELECT title,sport_status,status,calendar_plan,calendar_number,starts_at FROM competitions WHERE EXTRACT(YEAR FROM starts_at)=$1 ORDER BY starts_at`, year); err == nil {
		for rows.Next() {
			var title, ss, st, plan, num string
			var at time.Time
			if rows.Scan(&title, &ss, &st, &plan, &num, &at) == nil {
				comps = append(comps, map[string]any{"title": title, "sport_status": ss, "status": st, "calendar_plan": plan, "calendar_number": num, "date": at.Format("02.01.2006")})
			}
		}
		rows.Close()
	}
	docs := []map[string]any{}
	if rows, err := h.db.QueryContext(ctx, `SELECT id,indicator,title,url FROM indicator_docs ORDER BY id DESC`); err == nil {
		for rows.Next() {
			var id int64
			var ind, title, url string
			if rows.Scan(&id, &ind, &title, &url) == nil {
				docs = append(docs, map[string]any{"id": id, "indicator": ind, "title": title, "url": url})
			}
		}
		rows.Close()
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"year": year, "years": years, "organizations": orgs, "competitions": comps, "docs": docs,
		"sources": map[string]string{"years": "реестр спортсменов (enrollment_stats)", "organizations": "карточки организаций", "competitions": "календарь соревнований и протоколы"}})
}

func (h *Handler) DevDocAdd(w http.ResponseWriter, r *http.Request) {
	var in struct{ Indicator, Title, URL string }
	var raw struct {
		Indicator string `json:"indicator"`
		Title     string `json:"title"`
		URL       string `json:"url"`
	}
	if !decodeBody(w, r, &raw) {
		return
	}
	in.Indicator, in.Title, in.URL = strings.TrimSpace(raw.Indicator), strings.TrimSpace(raw.Title), strings.TrimSpace(raw.URL)
	if in.Indicator == "" || in.Title == "" || in.URL == "" || !validOptionalURL(in.URL) {
		httpserver.WriteError(w, http.StatusBadRequest, "Укажите показатель, название документа и ссылку (http/https)")
		return
	}
	if _, err := h.db.ExecContext(r.Context(), `INSERT INTO indicator_docs(indicator,title,url) VALUES($1,$2,$3)`, in.Indicator, in.Title, in.URL); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось прикрепить документ")
		return
	}
	httpserver.WriteJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

// ---------- Д4: нормативы и аттестация ----------

func (h *Handler) NormList(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `SELECT id,stage,discipline,name,unit,higher_is_better,threshold FROM norms ORDER BY stage,discipline,name`)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить нормативы")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var stage, disc, name, unit string
		var higher bool
		var th float64
		if rows.Scan(&id, &stage, &disc, &name, &unit, &higher, &th) == nil {
			items = append(items, map[string]any{"id": id, "stage": stage, "discipline": disc, "name": name, "unit": unit, "higher_is_better": higher, "threshold": th})
		}
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "stages": rules.TrainingStages})
}

func (h *Handler) NormSave(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Stage          string  `json:"stage"`
		Discipline     string  `json:"discipline"`
		Name           string  `json:"name"`
		Unit           string  `json:"unit"`
		HigherIsBetter bool    `json:"higher_is_better"`
		Threshold      float64 `json:"threshold"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if !validStage(in.Stage) || in.Name == "" || (in.Discipline != "" && !validDiscipline(in.Discipline)) {
		httpserver.WriteError(w, http.StatusBadRequest, "Укажите этап из справочника, название и (необязательно) дисциплину из справочника")
		return
	}
	if _, err := h.db.ExecContext(r.Context(), `INSERT INTO norms(stage,discipline,name,unit,higher_is_better,threshold) VALUES($1,$2,$3,$4,$5,$6)
		ON CONFLICT (stage,discipline,name) DO UPDATE SET unit=EXCLUDED.unit, higher_is_better=EXCLUDED.higher_is_better, threshold=EXCLUDED.threshold`,
		in.Stage, in.Discipline, in.Name, strings.TrimSpace(in.Unit), in.HigherIsBetter, in.Threshold); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить норматив")
		return
	}
	h.audit(r, "изменение норматива", "norms", in.Name)
	httpserver.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) NormResultAdd(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	var in struct {
		AthleteID int64   `json:"athlete_id"`
		NormID    int64   `json:"norm_id"`
		Value     float64 `json:"value"`
		TakenAt   string  `json:"taken_at"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	if user.ActiveRole == rules.RoleCoach && !h.coachLinked(r.Context(), user.CoachID, in.AthleteID) {
		httpserver.WriteError(w, http.StatusForbidden, "Результат можно внести только за привязанного спортсмена")
		return
	}
	taken := time.Now()
	if in.TakenAt != "" {
		t, err := time.Parse("2006-01-02", in.TakenAt)
		if err != nil {
			httpserver.WriteError(w, http.StatusBadRequest, "Дата в формате ГГГГ-ММ-ДД")
			return
		}
		taken = t
	}
	var higher bool
	var th float64
	if err := h.db.QueryRowContext(r.Context(), `SELECT higher_is_better,threshold FROM norms WHERE id=$1`, in.NormID).Scan(&higher, &th); err != nil {
		httpserver.WriteError(w, http.StatusNotFound, "Норматив не найден")
		return
	}
	passed := in.Value >= th
	if !higher {
		passed = in.Value <= th
	}
	if _, err := h.db.ExecContext(r.Context(), `INSERT INTO norm_results(athlete_id,norm_id,value,passed,taken_at,entered_by) VALUES($1,$2,$3,$4,$5,$6)`, in.AthleteID, in.NormID, in.Value, passed, taken, user.ID); err != nil {
		httpserver.WriteError(w, http.StatusNotFound, "Спортсмен не найден")
		return
	}
	h.audit(r, "результат норматива", "norm_results", strconv.FormatInt(in.AthleteID, 10))
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"ok": true, "passed": passed, "threshold": th})
}

func (h *Handler) NormResultList(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	athleteID, _ := strconv.ParseInt(r.URL.Query().Get("athlete_id"), 10, 64)
	switch user.ActiveRole {
	case rules.RoleAthlete:
		athleteID = user.AthleteID
	case rules.RoleCoach:
		if !h.coachLinked(r.Context(), user.CoachID, athleteID) {
			httpserver.WriteError(w, http.StatusForbidden, "Нет доступа к этому спортсмену")
			return
		}
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT n.name,n.stage,n.unit,nr.value,n.threshold,nr.passed,to_char(nr.taken_at,'DD.MM.YYYY') FROM norm_results nr JOIN norms n ON n.id=nr.norm_id WHERE nr.athlete_id=$1 ORDER BY nr.taken_at DESC,nr.id DESC LIMIT 100`, athleteID)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить историю нормативов")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var name, stage, unit, when string
		var v, th float64
		var ok bool
		if rows.Scan(&name, &stage, &unit, &v, &th, &ok, &when) == nil {
			items = append(items, map[string]any{"name": name, "stage": stage, "unit": unit, "value": v, "threshold": th, "passed": ok, "date": when})
		}
	}
	at := []map[string]string{}
	if ar, err := h.db.QueryContext(r.Context(), `SELECT from_stage,to_stage,to_char(created_at,'DD.MM.YYYY') FROM attestations WHERE athlete_id=$1 ORDER BY id DESC`, athleteID); err == nil {
		for ar.Next() {
			var f, t, d string
			if ar.Scan(&f, &t, &d) == nil {
				at = append(at, map[string]string{"from": f, "to": t, "date": d})
			}
		}
		ar.Close()
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "attestations": at})
}

// normProgress считает, сколько нормативов этапа выполнено (по последнему результату).
func (h *Handler) normProgress(r *http.Request, athleteID int64, stage, disc string) (total, passed int) {
	_ = h.db.QueryRowContext(r.Context(), `SELECT COUNT(*), COUNT(*) FILTER (WHERE COALESCE((SELECT nr.passed FROM norm_results nr WHERE nr.athlete_id=$1 AND nr.norm_id=n.id ORDER BY nr.taken_at DESC,nr.id DESC LIMIT 1),FALSE))
		FROM norms n WHERE n.stage=$2 AND (n.discipline='' OR n.discipline=$3)`, athleteID, stage, disc).Scan(&total, &passed)
	return
}

func nextStage(stage string) string {
	for i, s := range rules.TrainingStages {
		if s == stage && i+1 < len(rules.TrainingStages) {
			return rules.TrainingStages[i+1]
		}
	}
	return ""
}

func (h *Handler) AttestationList(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	q := `SELECT a.id,a.full_name,g.name,g.stage,g.discipline FROM group_members m JOIN training_groups g ON g.id=m.group_id JOIN athletes a ON a.id=m.athlete_id WHERE m.left_at IS NULL`
	var args []any
	if user.ActiveRole == rules.RoleCoach {
		q += ` AND g.coach_id=$1`
		args = append(args, user.CoachID)
	}
	rows, err := h.db.QueryContext(r.Context(), q+` ORDER BY g.stage,a.full_name`, args...)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить аттестацию")
		return
	}
	type row struct {
		id                      int64
		name, group, stage, dis string
	}
	var list []row
	for rows.Next() {
		var x row
		if rows.Scan(&x.id, &x.name, &x.group, &x.stage, &x.dis) == nil {
			list = append(list, x)
		}
	}
	rows.Close()
	items := []map[string]any{}
	for _, x := range list {
		total, passed := h.normProgress(r, x.id, x.stage, x.dis)
		next := nextStage(x.stage)
		items = append(items, map[string]any{"athlete_id": x.id, "athlete": x.name, "group": x.group, "stage": x.stage, "total": total, "passed": passed,
			"next_stage": next, "ready": total > 0 && passed == total && next != ""})
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) AttestationPromote(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	var in struct {
		AthleteID int64 `json:"athlete_id"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	if user.ActiveRole == rules.RoleCoach && !h.coachLinked(r.Context(), user.CoachID, in.AthleteID) {
		httpserver.WriteError(w, http.StatusForbidden, "Перевод доступен только для привязанного спортсмена")
		return
	}
	var stage, disc string
	if err := h.db.QueryRowContext(r.Context(), `SELECT g.stage,g.discipline FROM group_members m JOIN training_groups g ON g.id=m.group_id WHERE m.athlete_id=$1 AND m.left_at IS NULL ORDER BY m.id DESC LIMIT 1`, in.AthleteID).Scan(&stage, &disc); err != nil {
		httpserver.WriteError(w, http.StatusNotFound, "Спортсмен не состоит в учебно-тренировочной группе")
		return
	}
	total, passed := h.normProgress(r, in.AthleteID, stage, disc)
	next := nextStage(stage)
	if next == "" || total == 0 || passed < total {
		httpserver.WriteError(w, http.StatusUnprocessableEntity, "Требования этапа не выполнены: "+strconv.Itoa(passed)+" из "+strconv.Itoa(total)+" нормативов")
		return
	}
	_, _ = h.db.ExecContext(r.Context(), `UPDATE athletes SET training_stage=$1 WHERE id=$2`, next, in.AthleteID)
	_, _ = h.db.ExecContext(r.Context(), `INSERT INTO attestations(athlete_id,from_stage,to_stage,decided_by) VALUES($1,$2,$3,$4)`, in.AthleteID, stage, next, user.ID)
	h.audit(r, "перевод на следующий этап", "attestations", next)
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "to_stage": next})
}

// ---------- Д8: премии Губернатора ----------

func (h *Handler) bonusData(r *http.Request, year int) (rates []map[string]any, cands []map[string]any, total int, status string) {
	ctx := r.Context()
	rates, cands = []map[string]any{}, []map[string]any{}
	status = "draft"
	_ = h.db.QueryRowContext(ctx, `SELECT status FROM bonus_petitions WHERE year=$1`, year).Scan(&status)
	if rows, err := h.db.QueryContext(ctx, `SELECT level,place,amount FROM bonus_rates ORDER BY level,place`); err == nil {
		for rows.Next() {
			var level string
			var place, amount int
			if rows.Scan(&level, &place, &amount) == nil {
				rates = append(rates, map[string]any{"level": level, "place": place, "amount": amount})
			}
		}
		rows.Close()
	}
	rows, err := h.db.QueryContext(ctx, `SELECT a.id,a.full_name,a.organization,c.title,c.sport_status,r.place,br.amount FROM results r
		JOIN competitions c ON c.id=r.competition_id JOIN athletes a ON a.id=r.athlete_id JOIN bonus_rates br ON br.level=c.sport_status AND br.place=r.place
		WHERE r.place<=3 AND EXTRACT(YEAR FROM c.starts_at)=$1 AND a.disqualified=FALSE ORDER BY a.full_name,c.starts_at`, year)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var name, org, title, level string
		var place, amount int
		if rows.Scan(&id, &name, &org, &title, &level, &place, &amount) == nil {
			cands = append(cands, map[string]any{"athlete_id": id, "athlete": name, "organization": org, "competition": title, "level": level, "place": place, "amount": amount})
			total += amount
		}
	}
	return
}

func (h *Handler) BonusList(w http.ResponseWriter, r *http.Request) {
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	if year == 0 {
		year = time.Now().Year()
	}
	rates, cands, total, status := h.bonusData(r, year)
	deadline := time.Date(year+1, 2, 1, 23, 59, 0, 0, time.Local)
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"year": year, "rates": rates, "candidates": cands, "total": total, "status": status,
		"deadline": deadline.Format("02.01.2006"), "days_left": int(time.Until(deadline).Hours() / 24)})
}

func (h *Handler) BonusRateSave(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Level  string `json:"level"`
		Place  int    `json:"place"`
		Amount int    `json:"amount"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	if (in.Level != "региональное" && in.Level != "межрегиональное" && in.Level != "всероссийское") || in.Place < 1 || in.Place > 3 || in.Amount < 0 {
		httpserver.WriteError(w, http.StatusBadRequest, "Уровень (региональное, межрегиональное, всероссийское), место 1–3 и сумма не меньше нуля")
		return
	}
	_, _ = h.db.ExecContext(r.Context(), `INSERT INTO bonus_rates(level,place,amount) VALUES($1,$2,$3) ON CONFLICT (level,place) DO UPDATE SET amount=EXCLUDED.amount`, in.Level, in.Place, in.Amount)
	h.audit(r, "изменение размера премии", "bonus_rates", in.Level)
	httpserver.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) BonusPetition(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Year   int    `json:"year"`
		Status string `json:"status"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Year == 0 || (in.Status != "draft" && in.Status != "prepared" && in.Status != "sent" && in.Status != "awarded") {
		httpserver.WriteError(w, http.StatusBadRequest, "Статус: draft, prepared, sent или awarded")
		return
	}
	_, _ = h.db.ExecContext(r.Context(), `INSERT INTO bonus_petitions(year,status) VALUES($1,$2) ON CONFLICT (year) DO UPDATE SET status=EXCLUDED.status, updated_at=NOW()`, in.Year, in.Status)
	h.audit(r, "статус ходатайства о премиях", "bonus_petitions", in.Status)
	httpserver.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// BonusPackage — проект ходатайства; паспортные данные, ИНН, СНИЛС и реквизиты в демо — маски.
func (h *Handler) BonusPackage(w http.ResponseWriter, r *http.Request) {
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	if year == 0 {
		year = time.Now().Year()
	}
	_, cands, total, _ := h.bonusData(r, year)
	lines := []string{"Ходатайство о назначении премий Губернатора Оренбургской области (Указ от 04.03.2011 № 93-ук)", "Спортивный сезон: " + strconv.Itoa(year), "", "Спортсмены, результаты которых дают право на премию:"}
	for i, c := range cands {
		lines = append(lines, strconv.Itoa(i+1)+". "+c["athlete"].(string)+" — "+c["competition"].(string)+" ("+c["level"].(string)+"), "+strconv.Itoa(c["place"].(int))+" место, "+strconv.Itoa(c["amount"].(int))+" руб. Паспорт: **** ******, ИНН: ************, СНИЛС: ***-***-*** **, реквизиты: ****")
	}
	lines = append(lines, "", "Итого: "+strconv.Itoa(total)+" руб.", "Срок подачи ходатайства: до 01.02."+strconv.Itoa(year+1))
	data, err := exportutil.DOCX("Ходатайство о премиях Губернатора", lines)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сформировать пакет")
		return
	}
	writeFile(w, "bonus-petition-"+strconv.Itoa(year)+".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", data)
}

// ---------- Д7: личный путь и справочник ----------

var rankLadder = []string{"Без разряда", "3 разряд", "2 разряд", "1 разряд", "КМС"}

func (h *Handler) AthletePath(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	var rank, disc, ageGroup, stage string
	var birth time.Time
	if err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(rank_title,''),discipline,age_group,COALESCE(training_stage,''),COALESCE(birth_date,CURRENT_DATE-INTERVAL '16 years') FROM athletes WHERE id=$1`, user.AthleteID).Scan(&rank, &disc, &ageGroup, &stage, &birth); err != nil {
		httpserver.WriteError(w, http.StatusNotFound, "Профиль спортсмена не найден")
		return
	}
	next := "Максимальный разряд в региональной системе — дальше только звания"
	for i, rk := range rankLadder {
		if rk == rank && i+1 < len(rankLadder) {
			next = rankLadder[i+1]
		}
	}
	age := rules.AgeAt(birth, time.Now())
	items := []map[string]any{}
	rows, err := h.db.QueryContext(r.Context(), `SELECT id,title,sport_status,age_group,starts_at FROM competitions WHERE status='Регистрация' AND (discipline=$1 OR discipline='') ORDER BY starts_at`, disc)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var title, ss, ag string
			var at time.Time
			if rows.Scan(&id, &title, &ss, &ag, &at) == nil && rules.MatchesAgeGroup(ag, age) {
				items = append(items, map[string]any{"id": id, "title": title, "sport_status": ss, "age_group": ag, "date": at.Format("02.01.2006")})
			}
		}
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"rank": rank, "next_rank": next, "discipline": disc, "age": age, "stage": stage, "competitions": items,
		"hint": "Для разряда нужен результат на соревновании подходящего статуса по требованиям ЕВСК: подайте заявку на разряд из кабинета тренера или спортсмена, указав соревнование и результат."})
}

func (h *Handler) Glossary(w http.ResponseWriter, r *http.Request) {
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": []map[string]string{
		{"term": "Разряд", "text": "Спортивная квалификация по ЕВСК: 3, 2, 1 разряд, КМС. Присваивается за результат на соревнованиях нужного статуса, имеет срок действия."},
		{"term": "Звание", "text": "Высшая квалификация (МС, МСМК, ЗМС) — присваивается по результатам всероссийских и международных стартов."},
		{"term": "Статус соревнования", "text": "Региональное, межрегиональное или всероссийское: от него зависят баллы рейтинга и возможность получить разряд."},
		{"term": "Чемпионат и первенство", "text": "Чемпионат — для взрослых и всех желающих по правилам вида спорта; первенство — для определённых возрастных групп, например юниоров."},
		{"term": "Кубок", "text": "Соревнование, чаще всего проводимое серией этапов или как отдельный турнир."},
		{"term": "ЕКП", "text": "Единый календарный план Минспорта России: в него включают всероссийские и межрегиональные соревнования."},
		{"term": "Антидопинг", "text": "Спортсмен обязан пройти онлайн-курс РУСАДА и знать запрещённый список: актуальные материалы на rusada.ru."},
		{"term": "Как стать тренером или судьёй", "text": "Зарегистрируйтесь на платформе с ролью «Тренер» или «Судья», подайте документы в Федерацию; судья получает категорию по заявке."},
		{"term": "Как организации начать подготовку", "text": "Обратитесь в Федерацию: организация вносится в реестр, после чего создаёт группы по этапам подготовки по ФССП."},
	}})
}

func serveVerifyPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(verifyPage))
}

const verifyPage = `<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Проверка справки — ФСП</title>
<style>body{margin:0;font-family:system-ui,sans-serif;background:#f4f6fb;color:#14213d;display:grid;place-items:center;min-height:100vh;padding:16px}@media(prefers-color-scheme:dark){body{background:#0e1424;color:#eef1fb}.c{background:#171f36!important}}
.c{background:#fff;border-radius:18px;padding:28px;max-width:460px;width:100%;box-shadow:0 10px 40px rgba(0,0,0,.12)}.ok{color:#1f9d55;font-weight:700;font-size:20px}.bad{color:#d64545;font-weight:700;font-size:20px}dt{color:#7b86a6;font-size:13px;margin-top:12px}dd{margin:2px 0;font-weight:600}</style></head>
<body><div class="c"><div id="r">Проверяем…</div></div><script>
const code=location.pathname.split('/').pop();const e=s=>String(s).replace(/[&<>"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]));
fetch('/api/verify/'+encodeURIComponent(code)).then(r=>r.ok?r.json():Promise.reject()).then(d=>{document.getElementById('r').innerHTML='<div class="ok">✔ Справка подлинная</div><dl><dt>Спортсмен</dt><dd>'+e(d.athlete)+'</dd><dt>Соревнование</dt><dd>'+e(d.competition)+'</dd><dt>Дата</dt><dd>'+e(d.starts_at)+'</dd><dt>Место / баллы</dt><dd>'+e(d.place||'—')+' / '+e(d.score)+'</dd><dt>Номер справки</dt><dd>'+e(d.code)+'</dd></dl>'}).catch(()=>{document.getElementById('r').innerHTML='<div class="bad">✖ Справка не найдена</div><p>Проверьте номер или QR-код.</p>'});
</script></body></html>`
