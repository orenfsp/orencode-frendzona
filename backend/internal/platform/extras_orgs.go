package platform

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"orenkod/backend/internal/auth"
	"orenkod/backend/internal/httpserver"
	"orenkod/backend/internal/rules"
)

// ---------- Д3: организации ----------

func (h *Handler) orgRows(r *http.Request) ([]map[string]any, error) {
	rows, err := h.db.QueryContext(r.Context(), `SELECT o.id,o.name,o.org_type,o.municipality,o.license,o.programs,
		(SELECT COUNT(*) FROM training_groups g WHERE g.organization_id=o.id),
		(SELECT COUNT(DISTINCT m.athlete_id) FROM group_members m JOIN training_groups g ON g.id=m.group_id WHERE g.organization_id=o.id AND m.left_at IS NULL),
		(SELECT COUNT(*) FROM athletes a WHERE a.organization=o.name),
		COALESCE((SELECT string_agg(u.full_name, ', ' ORDER BY u.full_name) FROM coaches c JOIN users u ON u.id=c.user_id WHERE c.organization=o.name),'')
		FROM organizations o ORDER BY o.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var name, typ, mun, license, programs, coaches string
		var groups, inGroups, athletes int
		if rows.Scan(&id, &name, &typ, &mun, &license, &programs, &groups, &inGroups, &athletes, &coaches) != nil {
			continue
		}
		items = append(items, map[string]any{"id": id, "name": name, "org_type": typ, "municipality": mun, "license": license, "programs": programs,
			"groups": groups, "athletes_in_groups": inGroups, "athletes": athletes, "coaches": coaches})
	}
	return items, rows.Err()
}

func (h *Handler) OrganizationList(w http.ResponseWriter, r *http.Request) {
	items, err := h.orgRows(r)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить организации")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "stages": rules.TrainingStages, "disciplines": rules.Disciplines})
}

// PublicOrganizations — публичный список «где заниматься» (без персональных данных).
func (h *Handler) PublicOrganizations(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `SELECT o.name,o.org_type,o.municipality,o.programs,
		COALESCE((SELECT string_agg(DISTINCT g.discipline, '; ') FROM training_groups g WHERE g.organization_id=o.id),'') FROM organizations o ORDER BY o.name`)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить организации")
		return
	}
	defer rows.Close()
	items := []map[string]string{}
	for rows.Next() {
		var name, typ, mun, programs, disc string
		if rows.Scan(&name, &typ, &mun, &programs, &disc) == nil {
			items = append(items, map[string]string{"name": name, "type": typ, "municipality": mun, "programs": programs, "disciplines": disc})
		}
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) OrganizationCreate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name, OrgType, Municipality, License, Programs string
	}
	var raw struct {
		Name         string `json:"name"`
		OrgType      string `json:"org_type"`
		Municipality string `json:"municipality"`
		License      string `json:"license"`
		Programs     string `json:"programs"`
	}
	if !decodeBody(w, r, &raw) {
		return
	}
	in.Name, in.OrgType, in.Municipality = strings.TrimSpace(raw.Name), strings.TrimSpace(raw.OrgType), strings.TrimSpace(raw.Municipality)
	in.License, in.Programs = strings.TrimSpace(raw.License), strings.TrimSpace(raw.Programs)
	if in.Name == "" {
		httpserver.WriteError(w, http.StatusBadRequest, "Укажите название организации")
		return
	}
	if in.OrgType == "" {
		in.OrgType = "Школа"
	}
	if in.Municipality == "" {
		in.Municipality = "Оренбург"
	}
	var id int64
	if err := h.db.QueryRowContext(r.Context(), `INSERT INTO organizations(name,org_type,municipality,license,programs) VALUES($1,$2,$3,$4,$5) RETURNING id`,
		in.Name, in.OrgType, in.Municipality, in.License, in.Programs).Scan(&id); err != nil {
		httpserver.WriteError(w, http.StatusConflict, "Организация с таким названием уже есть")
		return
	}
	h.audit(r, "создание организации", "organizations", in.Name)
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"ok": true, "id": id})
}

// ---------- Д3: группы ----------

func validStage(s string) bool {
	for _, st := range rules.TrainingStages {
		if st == s {
			return true
		}
	}
	return false
}

func validDiscipline(s string) bool {
	for _, d := range rules.Disciplines {
		if d == s {
			return true
		}
	}
	return false
}

// groupAccess возвращает данные группы и проверяет, что тренер работает со своей группой.
func (h *Handler) groupAccess(w http.ResponseWriter, r *http.Request, id int64) (stage string, ok bool) {
	var coachID *int64
	if err := h.db.QueryRowContext(r.Context(), `SELECT stage,coach_id FROM training_groups WHERE id=$1`, id).Scan(&stage, &coachID); err != nil {
		httpserver.WriteError(w, http.StatusNotFound, "Группа не найдена")
		return "", false
	}
	user := auth.From(r)
	if user.ActiveRole == rules.RoleCoach && (coachID == nil || *coachID != user.CoachID) {
		httpserver.WriteError(w, http.StatusForbidden, "Тренер работает только со своими группами")
		return "", false
	}
	return stage, true
}

func (h *Handler) GroupList(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	q := `SELECT g.id,g.organization_id,o.name,g.name,g.stage,g.discipline,COALESCE(u.full_name,'не назначен'),
		(SELECT COUNT(*) FROM group_members m WHERE m.group_id=g.id AND m.left_at IS NULL),
		(SELECT COUNT(*) FROM training_sessions s WHERE s.group_id=g.id),
		COALESCE((SELECT to_char(MAX(s.held_at),'DD.MM.YYYY') FROM training_sessions s WHERE s.group_id=g.id),'—')
		FROM training_groups g JOIN organizations o ON o.id=g.organization_id LEFT JOIN coaches c ON c.id=g.coach_id LEFT JOIN users u ON u.id=c.user_id`
	var args []any
	if user.ActiveRole == rules.RoleCoach {
		q += ` WHERE g.coach_id=$1`
		args = append(args, user.CoachID)
	}
	rows, err := h.db.QueryContext(r.Context(), q+` ORDER BY o.name,g.name`, args...)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить группы")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, oid int64
		var org, name, stage, disc, coach, last string
		var members, sessions int
		if rows.Scan(&id, &oid, &org, &name, &stage, &disc, &coach, &members, &sessions, &last) != nil {
			continue
		}
		items = append(items, map[string]any{"id": id, "organization_id": oid, "organization": org, "name": name, "stage": stage, "discipline": disc,
			"coach": coach, "members": members, "sessions": sessions, "last_session": last})
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) GroupCreate(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	var in struct {
		OrganizationID int64  `json:"organization_id"`
		Name           string `json:"name"`
		Stage          string `json:"stage"`
		Discipline     string `json:"discipline"`
		CoachID        int64  `json:"coach_id"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.OrganizationID == 0 || in.Name == "" || !validStage(in.Stage) || !validDiscipline(in.Discipline) {
		httpserver.WriteError(w, http.StatusBadRequest, "Укажите организацию, название, этап подготовки и дисциплину из справочника")
		return
	}
	coachID := in.CoachID
	if user.ActiveRole == rules.RoleCoach {
		coachID = user.CoachID
	}
	var id int64
	if err := h.db.QueryRowContext(r.Context(), `INSERT INTO training_groups(organization_id,name,stage,discipline,coach_id) VALUES($1,$2,$3,$4,$5) RETURNING id`,
		in.OrganizationID, in.Name, in.Stage, in.Discipline, nullIfZero(coachID)).Scan(&id); err != nil {
		httpserver.WriteError(w, http.StatusConflict, "Не удалось создать группу: проверьте организацию и уникальность названия")
		return
	}
	h.audit(r, "создание группы", "training_groups", in.Name)
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"ok": true, "id": id})
}

func (h *Handler) GroupGet(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if _, ok := h.groupAccess(w, r, id); !ok {
		return
	}
	mrows, err := h.db.QueryContext(r.Context(), `SELECT a.id,a.full_name,a.rank_title,to_char(m.joined_at,'DD.MM.YYYY') FROM group_members m JOIN athletes a ON a.id=m.athlete_id WHERE m.group_id=$1 AND m.left_at IS NULL ORDER BY a.full_name`, id)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить состав группы")
		return
	}
	members := []map[string]any{}
	for mrows.Next() {
		var aid int64
		var name, rank, joined string
		if mrows.Scan(&aid, &name, &rank, &joined) == nil {
			members = append(members, map[string]any{"athlete_id": aid, "full_name": name, "rank_title": rank, "joined_at": joined})
		}
	}
	mrows.Close()
	srows, err := h.db.QueryContext(r.Context(), `SELECT s.id,to_char(s.held_at,'DD.MM.YYYY'),s.topic,s.format,s.proof_url,(SELECT COUNT(*) FROM session_attendance x WHERE x.session_id=s.id) FROM training_sessions s WHERE s.group_id=$1 ORDER BY s.held_at DESC,s.id DESC LIMIT 60`, id)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить журнал занятий")
		return
	}
	sessions := []map[string]any{}
	for srows.Next() {
		var sid int64
		var held, topic, format, proof string
		var present int
		if srows.Scan(&sid, &held, &topic, &format, &proof, &present) == nil {
			sessions = append(sessions, map[string]any{"id": sid, "held_at": held, "topic": topic, "format": format, "proof_url": proof, "present": present})
		}
	}
	srows.Close()
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"members": members, "sessions": sessions})
}

func (h *Handler) MemberAdd(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	stage, ok := h.groupAccess(w, r, id)
	if !ok {
		return
	}
	var in struct {
		AthleteID int64 `json:"athlete_id"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	user := auth.From(r)
	if user.ActiveRole == rules.RoleCoach && !h.coachLinked(r.Context(), user.CoachID, in.AthleteID) {
		httpserver.WriteError(w, http.StatusForbidden, "Зачислить можно только привязанного спортсмена")
		return
	}
	if _, err := h.db.ExecContext(r.Context(), `INSERT INTO group_members(group_id,athlete_id) VALUES($1,$2)`, id, in.AthleteID); err != nil {
		httpserver.WriteError(w, http.StatusConflict, "Спортсмен не найден или уже зачислен в эту группу")
		return
	}
	_, _ = h.db.ExecContext(r.Context(), `UPDATE athletes SET training_stage=$1 WHERE id=$2`, stage, in.AthleteID)
	h.audit(r, "зачисление в группу", "group_members", strconv.FormatInt(in.AthleteID, 10))
	httpserver.WriteJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (h *Handler) MemberTransfer(w http.ResponseWriter, r *http.Request) {
	from := pathID(r)
	if _, ok := h.groupAccess(w, r, from); !ok {
		return
	}
	var in struct {
		AthleteID int64 `json:"athlete_id"`
		ToGroupID int64 `json:"to_group_id"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	var toStage string
	if err := h.db.QueryRowContext(r.Context(), `SELECT stage FROM training_groups WHERE id=$1`, in.ToGroupID).Scan(&toStage); err != nil || in.ToGroupID == from {
		httpserver.WriteError(w, http.StatusBadRequest, "Выберите другую существующую группу")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось выполнить перевод")
		return
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(r.Context(), `UPDATE group_members SET left_at=CURRENT_DATE, note='Переведён в другую группу' WHERE group_id=$1 AND athlete_id=$2 AND left_at IS NULL`, from, in.AthleteID)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось выполнить перевод")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httpserver.WriteError(w, http.StatusNotFound, "Спортсмен не состоит в этой группе")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO group_members(group_id,athlete_id,note) VALUES($1,$2,'Переведён из группы '||$3::text)`, in.ToGroupID, in.AthleteID, strconv.FormatInt(from, 10)); err != nil {
		httpserver.WriteError(w, http.StatusConflict, "Спортсмен уже состоит в целевой группе")
		return
	}
	_, _ = tx.ExecContext(r.Context(), `UPDATE athletes SET training_stage=$1 WHERE id=$2`, toStage, in.AthleteID)
	if err = tx.Commit(); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось выполнить перевод")
		return
	}
	h.audit(r, "перевод между группами", "group_members", strconv.FormatInt(in.AthleteID, 10))
	httpserver.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) SessionCreate(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if _, ok := h.groupAccess(w, r, id); !ok {
		return
	}
	var in struct {
		HeldAt   string  `json:"held_at"`
		Topic    string  `json:"topic"`
		Format   string  `json:"format"`
		ProofURL string  `json:"proof_url"`
		Present  []int64 `json:"present"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	held, err := time.Parse("2006-01-02", in.HeldAt)
	if err != nil || held.After(time.Now().Add(24*time.Hour)) {
		httpserver.WriteError(w, http.StatusBadRequest, "Укажите дату занятия (не позже сегодняшней)")
		return
	}
	in.Topic, in.ProofURL = strings.TrimSpace(in.Topic), strings.TrimSpace(in.ProofURL)
	if in.Topic == "" || (in.Format != "очно" && in.Format != "онлайн") {
		httpserver.WriteError(w, http.StatusBadRequest, "Укажите тему и формат занятия: очно или онлайн")
		return
	}
	if in.Format == "онлайн" && in.ProofURL == "" {
		httpserver.WriteError(w, http.StatusUnprocessableEntity, "Для онлайн-занятия нужно подтверждение: ссылка на запись или лог платформы")
		return
	}
	if !validOptionalURL(in.ProofURL) {
		httpserver.WriteError(w, http.StatusBadRequest, "Ссылка должна начинаться с http:// или https://")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить занятие")
		return
	}
	defer tx.Rollback()
	var sid int64
	if err = tx.QueryRowContext(r.Context(), `INSERT INTO training_sessions(group_id,held_at,topic,format,proof_url,created_by) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`,
		id, held, in.Topic, in.Format, in.ProofURL, auth.From(r).ID).Scan(&sid); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить занятие")
		return
	}
	for _, aid := range in.Present {
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO session_attendance(session_id,athlete_id)
			SELECT $1,athlete_id FROM group_members WHERE group_id=$2 AND athlete_id=$3 AND left_at IS NULL ON CONFLICT DO NOTHING`, sid, id, aid); err != nil {
			httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить посещаемость")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить занятие")
		return
	}
	h.audit(r, "запись в журнале занятий", "training_sessions", strconv.FormatInt(sid, 10))
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"ok": true, "id": sid})
}

// groupStageRows — численность в учебно-тренировочных группах для отчёта о динамике численности.
func (h *Handler) groupStageRows(r *http.Request) [][]string {
	out := [][]string{}
	rows, err := h.db.QueryContext(r.Context(), `SELECT o.name,g.stage,g.discipline,COUNT(DISTINCT m.athlete_id) FROM training_groups g
		JOIN organizations o ON o.id=g.organization_id JOIN group_members m ON m.group_id=g.id AND m.left_at IS NULL
		GROUP BY o.name,g.stage,g.discipline ORDER BY o.name,g.stage,g.discipline`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var org, stage, disc string
		var n int
		if rows.Scan(&org, &stage, &disc, &n) == nil {
			out = append(out, []string{org, stage, disc, strconv.Itoa(n)})
		}
	}
	return out
}
