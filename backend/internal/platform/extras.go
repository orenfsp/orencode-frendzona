package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"orenkod/backend/internal/auth"
	"orenkod/backend/internal/exportutil"
	"orenkod/backend/internal/httpserver"
	"orenkod/backend/internal/rules"
)

// MountExtras регистрирует маршруты Д3 (организации, группы, журнал), Д5 (апелляции и нарушения),
// Д11 (публичный навигатор) и Д12 (документы о судействе).
func (h *Handler) MountExtras(mux *http.ServeMux, a *auth.Service) {
	anyUser := a.Middleware()
	fed := a.Middleware(rules.RoleFederation)
	fedCoach := a.Middleware(rules.RoleFederation, rules.RoleCoach)
	fedCoachMin := a.Middleware(rules.RoleFederation, rules.RoleCoach, rules.RoleMinistry)
	fedJudgeMin := a.Middleware(rules.RoleFederation, rules.RoleJudge, rules.RoleMinistry)
	fedJudge := a.Middleware(rules.RoleFederation, rules.RoleJudge)
	coachAthlete := a.Middleware(rules.RoleCoach, rules.RoleAthlete)
	appealViewers := a.Middleware(rules.RoleFederation, rules.RoleCoach, rules.RoleAthlete, rules.RoleJudge)

	// Д11: публичный раздел без авторизации
	mux.HandleFunc("GET /api/public/navigator", servePublicPage)
	mux.HandleFunc("GET /api/public/organizations", h.PublicOrganizations)

	// Д12
	mux.HandleFunc("GET /api/documents/judges/composition", fedJudgeMin(h.JudgeComposition))
	mux.HandleFunc("GET /api/documents/judges/{id}/report", fedJudgeMin(h.JudgeReport))

	// Д5
	mux.HandleFunc("POST /api/appeals", coachAthlete(h.AppealCreate))
	mux.HandleFunc("GET /api/appeals/list", appealViewers(h.AppealList))
	mux.HandleFunc("POST /api/appeals/{id}/decide", fed(h.AppealDecide))
	mux.HandleFunc("POST /api/violations", fedJudge(h.ViolationCreate))
	mux.HandleFunc("GET /api/violations", appealViewers(h.ViolationList))
	mux.HandleFunc("POST /api/violations/{id}/decide", fed(h.ViolationDecide))
	mux.HandleFunc("GET /api/cases/results", appealViewers(h.CaseResults))

	// Д3
	mux.HandleFunc("GET /api/organizations", anyUser(h.OrganizationList))
	mux.HandleFunc("POST /api/organizations", fed(h.OrganizationCreate))
	mux.HandleFunc("GET /api/groups", fedCoachMin(h.GroupList))
	mux.HandleFunc("POST /api/groups", fedCoach(h.GroupCreate))
	mux.HandleFunc("GET /api/groups/{id}", fedCoachMin(h.GroupGet))
	mux.HandleFunc("POST /api/groups/{id}/members", fedCoach(h.MemberAdd))
	mux.HandleFunc("POST /api/groups/{id}/transfer", fedCoach(h.MemberTransfer))
	mux.HandleFunc("POST /api/groups/{id}/sessions", fedCoach(h.SessionCreate))

	h.mountExtras2(mux, a)
	h.mountReadiness(mux, a)
}

func parseID(s string) (int64, error) { return strconv.ParseInt(strings.TrimSpace(s), 10, 64) }

func itoaInt(n int) string { return strconv.Itoa(n) }

func pathID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, "Проверьте формат JSON")
		return false
	}
	return true
}

func validOptionalURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	return raw == "" || strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://")
}

// notifyUser кладёт персональное уведомление в кабинет и дублирует его в Telegram, если он привязан.
func (h *Handler) notifyUser(ctx context.Context, userID int64, kind, title, message string) {
	if userID == 0 {
		return
	}
	// role_scope '-' — чтобы уведомление видел только адресат, а не все роли.
	if _, err := h.db.ExecContext(ctx, `INSERT INTO inbox(user_id,role_scope,kind,title,message) VALUES($1,'-',$2,$3,$4)`, userID, kind, title, message); err != nil {
		return
	}
	var tg sql.NullInt64
	_ = h.db.QueryRowContext(ctx, `SELECT telegram_user_id FROM users WHERE id=$1`, userID).Scan(&tg)
	if h.notify != nil && tg.Valid {
		h.notify.NotifyUser(tg.Int64, title+"\n\n"+message)
	}
}

func (h *Handler) caseNote(ctx context.Context, kind string, id int64, status, note string, userID int64) {
	_, _ = h.db.ExecContext(ctx, `INSERT INTO case_history(case_type,case_id,status,note,user_id) VALUES($1,$2,$3,$4,$5)`, kind, id, status, note, nullIfZero(userID))
}

func (h *Handler) caseHistory(ctx context.Context, kind string, id int64) []map[string]string {
	out := []map[string]string{}
	rows, err := h.db.QueryContext(ctx, `SELECT h.status,h.note,h.created_at,COALESCE(u.full_name,'Система') FROM case_history h LEFT JOIN users u ON u.id=h.user_id WHERE h.case_type=$1 AND h.case_id=$2 ORDER BY h.id`, kind, id)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var st, note, who string
		var at time.Time
		if rows.Scan(&st, &note, &at, &who) == nil {
			out = append(out, map[string]string{"status": st, "note": note, "when": at.Format("02.01.2006 15:04"), "who": who})
		}
	}
	return out
}

func (h *Handler) coachLinked(ctx context.Context, coachID, athleteID int64) bool {
	var n int
	_ = h.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM coach_links WHERE coach_id=$1 AND athlete_id=$2 AND status='active'`, coachID, athleteID).Scan(&n)
	return n > 0
}

func (h *Handler) judgeAssigned(ctx context.Context, judgeID, competitionID int64) bool {
	var n int
	_ = h.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM judge_assignments WHERE judge_id=$1 AND competition_id=$2`, judgeID, competitionID).Scan(&n)
	return n > 0
}

// ---------- Д12: документы о судействе ----------

func respondDoc(w http.ResponseWriter, r *http.Request, base, title string, lines []string, table [][]string) {
	if r.URL.Query().Get("format") == "xlsx" {
		data, err := exportutil.XLSX("Документ", table)
		if err != nil {
			httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сформировать таблицу")
			return
		}
		writeFile(w, base+".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
		return
	}
	data, err := exportutil.DOCX(title, lines)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сформировать документ")
		return
	}
	writeFile(w, base+".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", data)
}

// JudgeComposition — справка о составе судейской коллегии соревнования.
func (h *Handler) JudgeComposition(w http.ResponseWriter, r *http.Request) {
	compID, _ := strconv.ParseInt(r.URL.Query().Get("competition_id"), 10, 64)
	user := auth.From(r)
	if user.ActiveRole == rules.RoleJudge && !h.judgeAssigned(r.Context(), user.JudgeID, compID) {
		httpserver.WriteError(w, http.StatusForbidden, "Справка доступна по соревнованиям, на которые вы назначены")
		return
	}
	var title, discipline, location string
	var starts time.Time
	if err := h.db.QueryRowContext(r.Context(), `SELECT title,discipline,location,starts_at FROM competitions WHERE id=$1`, compID).Scan(&title, &discipline, &location, &starts); err != nil {
		httpserver.WriteError(w, http.StatusNotFound, "Соревнование не найдено")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT u.full_name,ja.position,j.category,COALESCE(to_char(j.category_expires_at,'DD.MM.YYYY'),'—'),j.organization
		FROM judge_assignments ja JOIN judges j ON j.id=ja.judge_id JOIN users u ON u.id=j.user_id WHERE ja.competition_id=$1 ORDER BY ja.id`, compID)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить состав коллегии")
		return
	}
	defer rows.Close()
	lines := []string{"Соревнование: " + title, "Дисциплина: " + discipline, "Место проведения: " + location, "Дата начала: " + starts.Format("02.01.2006"), "", "Состав судейской коллегии:"}
	table := [][]string{{"№", "ФИО", "Судейская должность", "Категория", "Действует до", "Организация"}}
	n := 0
	for rows.Next() {
		var name, pos, cat, until, org string
		if rows.Scan(&name, &pos, &cat, &until, &org) != nil {
			continue
		}
		n++
		lines = append(lines, strconv.Itoa(n)+". "+name+" — "+pos+", "+cat+" (до "+until+"), "+org)
		table = append(table, []string{strconv.Itoa(n), name, pos, cat, until, org})
	}
	if n == 0 {
		lines = append(lines, "Судейская коллегия ещё не назначена.")
	}
	lines = append(lines, "", "Справка сформирована платформой ФСП Оренбургской области: "+time.Now().Format("02.01.2006 15:04"))
	respondDoc(w, r, "judges-composition-"+strconv.FormatInt(compID, 10), "Справка о составе судейской коллегии", lines, table)
}

// JudgeReport — отчёт о соревнованиях, на которых работал судья.
func (h *Handler) JudgeReport(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	user := auth.From(r)
	if user.ActiveRole == rules.RoleJudge && user.JudgeID != id {
		httpserver.WriteError(w, http.StatusForbidden, "Судья может выгрузить только собственный отчёт")
		return
	}
	var name, category, until string
	if err := h.db.QueryRowContext(r.Context(), `SELECT u.full_name,j.category,COALESCE(to_char(j.category_expires_at,'DD.MM.YYYY'),'—') FROM judges j JOIN users u ON u.id=j.user_id WHERE j.id=$1`, id).Scan(&name, &category, &until); err != nil {
		httpserver.WriteError(w, http.StatusNotFound, "Судья не найден")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT c.title,c.discipline,c.sport_status,c.starts_at,ja.position FROM judge_assignments ja JOIN competitions c ON c.id=ja.competition_id WHERE ja.judge_id=$1 ORDER BY c.starts_at`, id)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить отчёт")
		return
	}
	defer rows.Close()
	lines := []string{"Судья: " + name, "Судейская категория: " + category + " (действует до " + until + ")", "", "Соревнования, на которых работал судья:"}
	table := [][]string{{"№", "Соревнование", "Дисциплина", "Статус", "Дата", "Судейская должность"}}
	n := 0
	for rows.Next() {
		var title, disc, status, pos string
		var starts time.Time
		if rows.Scan(&title, &disc, &status, &starts, &pos) != nil {
			continue
		}
		n++
		lines = append(lines, strconv.Itoa(n)+". "+title+" ("+disc+", "+status+") — "+starts.Format("02.01.2006")+", "+pos)
		table = append(table, []string{strconv.Itoa(n), title, disc, status, starts.Format("02.01.2006"), pos})
	}
	if n == 0 {
		lines = append(lines, "Назначений пока нет.")
	}
	lines = append(lines, "", "Всего соревнований: "+strconv.Itoa(n), "Отчёт сформирован: "+time.Now().Format("02.01.2006 15:04"))
	respondDoc(w, r, "judge-report-"+strconv.FormatInt(id, 10), "Отчёт о судействе", lines, table)
}

// ---------- Д5: апелляции ----------

func (h *Handler) AppealCreate(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	var in struct {
		CompetitionID int64  `json:"competition_id"`
		AthleteID     int64  `json:"athlete_id"`
		Subject       string `json:"subject"`
		Description   string `json:"description"`
		EvidenceURL   string `json:"evidence_url"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	athleteID := in.AthleteID
	if user.ActiveRole == rules.RoleAthlete {
		athleteID = user.AthleteID
	} else if !h.coachLinked(r.Context(), user.CoachID, athleteID) {
		httpserver.WriteError(w, http.StatusForbidden, "Апелляцию можно подать только за привязанного спортсмена")
		return
	}
	in.Subject, in.Description = strings.TrimSpace(in.Subject), strings.TrimSpace(in.Description)
	if athleteID == 0 || in.CompetitionID == 0 || in.Subject == "" || len(in.Description) < 10 {
		httpserver.WriteError(w, http.StatusBadRequest, "Укажите соревнование, тему и описание (не короче 10 символов)")
		return
	}
	if !validOptionalURL(in.EvidenceURL) {
		httpserver.WriteError(w, http.StatusBadRequest, "Ссылка на доказательства должна начинаться с http:// или https://")
		return
	}
	var n int
	_ = h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM results WHERE competition_id=$1 AND athlete_id=$2`, in.CompetitionID, athleteID).Scan(&n)
	if n == 0 {
		httpserver.WriteError(w, http.StatusUnprocessableEntity, "Апелляцию можно подать только на опубликованный результат спортсмена")
		return
	}
	var id int64
	if err := h.db.QueryRowContext(r.Context(), `INSERT INTO appeals(competition_id,athlete_id,submitted_by,subject,description,evidence_url) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`,
		in.CompetitionID, athleteID, user.ID, in.Subject, in.Description, strings.TrimSpace(in.EvidenceURL)).Scan(&id); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось подать апелляцию")
		return
	}
	h.caseNote(r.Context(), "appeal", id, "submitted", "Апелляция подана", user.ID)
	h.notifyInbox(r, rules.RoleFederation, "warning", "Новая апелляция", "Тема: "+in.Subject)
	h.audit(r, "подача апелляции", "appeals", strconv.FormatInt(id, 10))
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"ok": true, "id": id})
}

func (h *Handler) AppealList(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	q := `SELECT ap.id,ap.competition_id,c.title,ap.athlete_id,a.full_name,ap.subject,ap.description,ap.evidence_url,ap.status,ap.decision,
		COALESCE(ap.new_place,0),ap.created_at,COALESCE(res.place,0)
		FROM appeals ap JOIN competitions c ON c.id=ap.competition_id JOIN athletes a ON a.id=ap.athlete_id
		LEFT JOIN results res ON res.competition_id=ap.competition_id AND res.athlete_id=ap.athlete_id`
	var args []any
	switch user.ActiveRole {
	case rules.RoleAthlete:
		q += ` WHERE ap.athlete_id=$1`
		args = append(args, user.AthleteID)
	case rules.RoleCoach:
		q += ` WHERE ap.athlete_id IN (SELECT athlete_id FROM coach_links WHERE coach_id=$1 AND status='active')`
		args = append(args, user.CoachID)
	case rules.RoleJudge:
		q += ` WHERE ap.competition_id IN (SELECT competition_id FROM judge_assignments WHERE judge_id=$1)`
		args = append(args, user.JudgeID)
	}
	rows, err := h.db.QueryContext(r.Context(), q+` ORDER BY ap.id DESC`, args...)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить апелляции")
		return
	}
	type item = map[string]any
	items := []item{}
	for rows.Next() {
		var id, cid, aid int64
		var title, athlete, subject, desc, url, status, decision string
		var newPlace, place int
		var created time.Time
		if rows.Scan(&id, &cid, &title, &aid, &athlete, &subject, &desc, &url, &status, &decision, &newPlace, &created, &place) != nil {
			continue
		}
		items = append(items, item{"id": id, "competition_id": cid, "competition": title, "athlete_id": aid, "athlete": athlete, "subject": subject,
			"description": desc, "evidence_url": url, "status": status, "decision": decision, "new_place": newPlace, "current_place": place,
			"created_at": created.Format("02.01.2006 15:04")})
	}
	rows.Close()
	for _, it := range items {
		it["history"] = h.caseHistory(r.Context(), "appeal", it["id"].(int64))
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) AppealDecide(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	user := auth.From(r)
	var in struct {
		Status   string `json:"status"`
		Decision string `json:"decision"`
		NewPlace int    `json:"new_place"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Status != "review" && in.Status != "accepted" && in.Status != "rejected" {
		httpserver.WriteError(w, http.StatusBadRequest, "Статус: review, accepted или rejected")
		return
	}
	in.Decision = strings.TrimSpace(in.Decision)
	if in.Status != "review" && len(in.Decision) < 5 {
		httpserver.WriteError(w, http.StatusBadRequest, "Решение должно содержать обоснование")
		return
	}
	var compID, athleteID, submittedBy int64
	var current string
	if err := h.db.QueryRowContext(r.Context(), `SELECT competition_id,athlete_id,submitted_by,status FROM appeals WHERE id=$1`, id).Scan(&compID, &athleteID, &submittedBy, &current); err != nil {
		httpserver.WriteError(w, http.StatusNotFound, "Апелляция не найдена")
		return
	}
	if current == "accepted" || current == "rejected" {
		httpserver.WriteError(w, http.StatusConflict, "Решение по апелляции уже принято")
		return
	}
	corrected := false
	if in.Status == "accepted" && in.NewPlace > 0 {
		res, err := h.db.ExecContext(r.Context(), `UPDATE results SET place=$1 WHERE competition_id=$2 AND athlete_id=$3`, in.NewPlace, compID, athleteID)
		if err != nil {
			httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось скорректировать протокол")
			return
		}
		if n, _ := res.RowsAffected(); n > 0 {
			corrected = true
		}
	}
	var newPlace any
	if corrected {
		newPlace = in.NewPlace
	}
	if _, err := h.db.ExecContext(r.Context(), `UPDATE appeals SET status=$1, decision=$2, new_place=$3, decided_at=CASE WHEN $1::text='review' THEN NULL ELSE NOW() END WHERE id=$4`, in.Status, in.Decision, newPlace, id); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить решение")
		return
	}
	note := in.Decision
	if corrected {
		note += " Протокол скорректирован: новое место — " + strconv.Itoa(in.NewPlace) + "."
		h.recalculatePoints(r)
		h.emitCompetition("protocol.published", compID, map[string]any{"source": "appeal", "appeal_id": id})
	}
	h.caseNote(r.Context(), "appeal", id, in.Status, note, user.ID)
	label := map[string]string{"review": "принята к рассмотрению", "accepted": "удовлетворена", "rejected": "отклонена"}[in.Status]
	h.notifyUser(r.Context(), submittedBy, "info", "Апелляция "+label, strings.TrimSpace(note))
	h.audit(r, "решение по апелляции", "appeals", in.Status)
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "corrected": corrected})
}

// CaseResults — результаты соревнований для выбора при подаче апелляции/рапорта.
func (h *Handler) CaseResults(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	q := `SELECT res.competition_id,c.title,res.athlete_id,a.full_name,res.place FROM results res JOIN competitions c ON c.id=res.competition_id JOIN athletes a ON a.id=res.athlete_id`
	var args []any
	switch user.ActiveRole {
	case rules.RoleAthlete:
		q += ` WHERE res.athlete_id=$1`
		args = append(args, user.AthleteID)
	case rules.RoleCoach:
		q += ` WHERE res.athlete_id IN (SELECT athlete_id FROM coach_links WHERE coach_id=$1 AND status='active')`
		args = append(args, user.CoachID)
	case rules.RoleJudge:
		q += ` WHERE res.competition_id IN (SELECT competition_id FROM judge_assignments WHERE judge_id=$1)`
		args = append(args, user.JudgeID)
	}
	rows, err := h.db.QueryContext(r.Context(), q+` ORDER BY c.starts_at DESC, res.place LIMIT 300`, args...)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить результаты")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var cid, aid int64
		var title, name string
		var place int
		if rows.Scan(&cid, &title, &aid, &name, &place) == nil {
			items = append(items, map[string]any{"competition_id": cid, "competition": title, "athlete_id": aid, "athlete": name, "place": place})
		}
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ---------- Д5: нарушения ----------

func (h *Handler) ViolationCreate(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	var in struct {
		CompetitionID int64  `json:"competition_id"`
		AthleteID     int64  `json:"athlete_id"`
		Kind          string `json:"kind"`
		Report        string `json:"report"`
		EvidenceURL   string `json:"evidence_url"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	in.Kind, in.Report = strings.TrimSpace(in.Kind), strings.TrimSpace(in.Report)
	if in.CompetitionID == 0 || in.AthleteID == 0 || in.Kind == "" || len(in.Report) < 10 {
		httpserver.WriteError(w, http.StatusBadRequest, "Укажите соревнование, спортсмена, вид нарушения и рапорт (не короче 10 символов)")
		return
	}
	if !validOptionalURL(in.EvidenceURL) {
		httpserver.WriteError(w, http.StatusBadRequest, "Ссылка на доказательства должна начинаться с http:// или https://")
		return
	}
	if user.ActiveRole == rules.RoleJudge && !h.judgeAssigned(r.Context(), user.JudgeID, in.CompetitionID) {
		httpserver.WriteError(w, http.StatusForbidden, "Рапорт может подать судья, назначенный на это соревнование")
		return
	}
	var n int
	_ = h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM applications WHERE competition_id=$1 AND athlete_id=$2 AND status='approved'`, in.CompetitionID, in.AthleteID).Scan(&n)
	if n == 0 {
		httpserver.WriteError(w, http.StatusUnprocessableEntity, "Спортсмен не участвует в этом соревновании (нет одобренной заявки)")
		return
	}
	var id int64
	if err := h.db.QueryRowContext(r.Context(), `INSERT INTO violations(competition_id,athlete_id,reported_by,kind,report,evidence_url) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`,
		in.CompetitionID, in.AthleteID, user.ID, in.Kind, in.Report, strings.TrimSpace(in.EvidenceURL)).Scan(&id); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось зафиксировать нарушение")
		return
	}
	h.caseNote(r.Context(), "violation", id, "reported", "Рапорт подан: "+in.Kind, user.ID)
	h.notifyInbox(r, rules.RoleFederation, "warning", "Рапорт о нарушении", in.Kind)
	h.audit(r, "рапорт о нарушении", "violations", strconv.FormatInt(id, 10))
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"ok": true, "id": id})
}

func (h *Handler) ViolationList(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	q := `SELECT v.id,v.competition_id,c.title,v.athlete_id,a.full_name,v.kind,v.report,v.evidence_url,v.status,v.sanction,v.decision,v.created_at
		FROM violations v JOIN competitions c ON c.id=v.competition_id JOIN athletes a ON a.id=v.athlete_id`
	var args []any
	switch user.ActiveRole {
	case rules.RoleAthlete:
		q += ` WHERE v.athlete_id=$1 AND v.status='decided'`
		args = append(args, user.AthleteID)
	case rules.RoleCoach:
		q += ` WHERE v.status='decided' AND v.athlete_id IN (SELECT athlete_id FROM coach_links WHERE coach_id=$1 AND status='active')`
		args = append(args, user.CoachID)
	case rules.RoleJudge:
		q += ` WHERE v.competition_id IN (SELECT competition_id FROM judge_assignments WHERE judge_id=$1)`
		args = append(args, user.JudgeID)
	}
	rows, err := h.db.QueryContext(r.Context(), q+` ORDER BY v.id DESC`, args...)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить нарушения")
		return
	}
	type item = map[string]any
	items := []item{}
	for rows.Next() {
		var id, cid, aid int64
		var title, athlete, kind, report, url, status, sanction, decision string
		var created time.Time
		if rows.Scan(&id, &cid, &title, &aid, &athlete, &kind, &report, &url, &status, &sanction, &decision, &created) != nil {
			continue
		}
		items = append(items, item{"id": id, "competition_id": cid, "competition": title, "athlete_id": aid, "athlete": athlete, "kind": kind, "report": report,
			"evidence_url": url, "status": status, "sanction": sanction, "decision": decision, "created_at": created.Format("02.01.2006 15:04")})
	}
	rows.Close()
	for _, it := range items {
		it["history"] = h.caseHistory(r.Context(), "violation", it["id"].(int64))
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) ViolationDecide(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	user := auth.From(r)
	var in struct {
		Sanction string `json:"sanction"`
		Decision string `json:"decision"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	switch in.Sanction {
	case "none", "warning", "technical_defeat", "disqualification":
	default:
		httpserver.WriteError(w, http.StatusBadRequest, "Санкция: none, warning, technical_defeat или disqualification")
		return
	}
	in.Decision = strings.TrimSpace(in.Decision)
	if len(in.Decision) < 5 {
		httpserver.WriteError(w, http.StatusBadRequest, "Решение должно содержать обоснование")
		return
	}
	var compID, athleteID int64
	var status string
	var athleteUser sql.NullInt64
	if err := h.db.QueryRowContext(r.Context(), `SELECT v.competition_id,v.athlete_id,v.status,a.user_id FROM violations v JOIN athletes a ON a.id=v.athlete_id WHERE v.id=$1`, id).Scan(&compID, &athleteID, &status, &athleteUser); err != nil {
		httpserver.WriteError(w, http.StatusNotFound, "Нарушение не найдено")
		return
	}
	if status == "decided" {
		httpserver.WriteError(w, http.StatusConflict, "Решение уже принято")
		return
	}
	effect := ""
	switch in.Sanction {
	case "disqualification":
		_, _ = h.db.ExecContext(r.Context(), `UPDATE athletes SET disqualified=TRUE, status='Дисквалифицирован' WHERE id=$1`, athleteID)
		effect = " Спортсмен исключён из рейтинга."
	case "technical_defeat":
		res, err := h.db.ExecContext(r.Context(), `UPDATE results SET place=(SELECT COALESCE(MAX(place),0)+1 FROM results WHERE competition_id=$1), score=0 WHERE competition_id=$1 AND athlete_id=$2`, compID, athleteID)
		if err != nil {
			httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось применить санкцию")
			return
		}
		if n, _ := res.RowsAffected(); n > 0 {
			effect = " Результат в протоколе аннулирован (техническое поражение)."
			h.recalculatePoints(r)
			h.emitCompetition("protocol.published", compID, map[string]any{"source": "violation", "violation_id": id})
		}
	}
	if _, err := h.db.ExecContext(r.Context(), `UPDATE violations SET status='decided', sanction=$1, decision=$2, decided_at=NOW() WHERE id=$3`, in.Sanction, in.Decision, id); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить решение")
		return
	}
	h.caseNote(r.Context(), "violation", id, "decided", in.Decision+effect, user.ID)
	if athleteUser.Valid {
		h.notifyUser(r.Context(), athleteUser.Int64, "warning", "Решение по нарушению", in.Decision+effect)
	}
	h.audit(r, "решение по нарушению", "violations", in.Sanction)
	httpserver.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
