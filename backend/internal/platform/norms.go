package platform

import (
	"encoding/json"
	"net/http"
	"strconv"

	"orenkod/backend/internal/auth"
	"orenkod/backend/internal/httpserver"
	"orenkod/backend/internal/rules"
)

var stages = []string{"начальная подготовка", "учебно-тренировочный этап", "совершенствование спортивного мастерства", "высшее спортивное мастерство"}

// canManageAthlete: федерация — любого; тренер — только привязанного (active).
func (h *Handler) canManageAthlete(r *http.Request, athleteID int64) bool {
	u := auth.From(r)
	if u.ActiveRole == rules.RoleFederation {
		return true
	}
	if u.ActiveRole != rules.RoleCoach {
		return false
	}
	var n int
	_ = h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM coach_links WHERE coach_id=$1 AND athlete_id=$2 AND status='active'`, u.CoachID, athleteID).Scan(&n)
	return n > 0
}

// Norms — справочник нормативов ФССП.
func (h *Handler) Norms(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `SELECT id,name,stage,discipline,unit,threshold::float8,higher_is_better FROM norms ORDER BY stage,name`)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить нормативы")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var name, stage, disc, unit string
		var th float64
		var hib bool
		if rows.Scan(&id, &name, &stage, &disc, &unit, &th, &hib) == nil {
			items = append(items, map[string]any{"id": id, "name": name, "stage": stage, "discipline": disc, "unit": unit, "threshold": th, "higher_is_better": hib})
		}
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "stages": stages})
}

// SaveNorm — федерация добавляет или меняет норматив.
func (h *Handler) SaveNorm(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name, Stage, Discipline, Unit string
		Threshold                     float64
		HigherIsBetter                *bool `json:"higher_is_better"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.Name == "" || in.Stage == "" {
		httpserver.WriteError(w, http.StatusBadRequest, "Нужны название и этап")
		return
	}
	hib := in.HigherIsBetter == nil || *in.HigherIsBetter
	_, err := h.db.ExecContext(r.Context(), `INSERT INTO norms(name,stage,discipline,unit,threshold,higher_is_better) VALUES($1,$2,$3,$4,$5,$6)
		ON CONFLICT (name,stage,discipline) DO UPDATE SET unit=EXCLUDED.unit, threshold=EXCLUDED.threshold, higher_is_better=EXCLUDED.higher_is_better`, in.Name, in.Stage, in.Discipline, in.Unit, in.Threshold, hib)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить норматив")
		return
	}
	h.audit(r, "изменение норматива", "norms", in.Name)
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

// SaveNormResult — внесение результата сдачи; платформа сама сравнивает с порогом.
func (h *Handler) SaveNormResult(w http.ResponseWriter, r *http.Request) {
	var in struct {
		AthleteID int64   `json:"athlete_id"`
		NormID    int64   `json:"norm_id"`
		Value     float64 `json:"value"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.AthleteID == 0 || in.NormID == 0 {
		httpserver.WriteError(w, http.StatusBadRequest, "Нужны спортсмен, норматив и результат")
		return
	}
	if !h.canManageAthlete(r, in.AthleteID) {
		httpserver.WriteError(w, http.StatusForbidden, "Нет доступа к этому спортсмену")
		return
	}
	var th float64
	var hib bool
	if h.db.QueryRowContext(r.Context(), `SELECT threshold::float8,higher_is_better FROM norms WHERE id=$1`, in.NormID).Scan(&th, &hib) != nil {
		httpserver.WriteError(w, http.StatusNotFound, "Норматив не найден")
		return
	}
	passed := in.Value >= th
	if !hib {
		passed = in.Value <= th
	}
	if _, err := h.db.ExecContext(r.Context(), `INSERT INTO norm_results(athlete_id,norm_id,value,passed,entered_by) VALUES($1,$2,$3,$4,$5)`, in.AthleteID, in.NormID, in.Value, passed, auth.From(r).ID); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить результат")
		return
	}
	h.audit(r, "результат норматива", "norm_results", strconv.FormatInt(in.AthleteID, 10))
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"passed": passed, "threshold": th})
}

// Attestation — по каждому спортсмену: этап, сколько нормативов этапа выполнено, готовность к переводу.
// Спортсмен видит себя, тренер — подопечных, федерация — всех. История результатов в поле history.
func (h *Handler) Attestation(w http.ResponseWriter, r *http.Request) {
	u := auth.From(r)
	q := `SELECT a.id,a.full_name,a.stage,
		(SELECT COUNT(*) FROM norms n WHERE n.stage=a.stage AND (n.discipline='' OR n.discipline=a.discipline)) AS total,
		(SELECT COUNT(*) FROM norms n WHERE n.stage=a.stage AND (n.discipline='' OR n.discipline=a.discipline) AND COALESCE((SELECT nr.passed FROM norm_results nr WHERE nr.athlete_id=a.id AND nr.norm_id=n.id ORDER BY nr.created_at DESC,nr.id DESC LIMIT 1),FALSE)) AS passed
		FROM athletes a`
	args := []any{}
	switch u.ActiveRole {
	case rules.RoleAthlete:
		q += ` WHERE a.id=$1`
		args = append(args, u.AthleteID)
	case rules.RoleCoach:
		q += ` WHERE a.id IN (SELECT athlete_id FROM coach_links WHERE coach_id=$1 AND status='active')`
		args = append(args, u.CoachID)
	case rules.RoleFederation:
	default:
		httpserver.WriteError(w, http.StatusForbidden, "Недоступно для этой роли")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), q+` ORDER BY a.full_name`, args...)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить аттестацию")
		return
	}
	type row struct {
		id            int64
		name, stage   string
		total, passed int
	}
	var list []row
	for rows.Next() {
		var x row
		if rows.Scan(&x.id, &x.name, &x.stage, &x.total, &x.passed) == nil {
			list = append(list, x)
		}
	}
	rows.Close()
	items := []map[string]any{}
	for _, x := range list {
		hist := []map[string]any{}
		hr, err := h.db.QueryContext(r.Context(), `SELECT n.name,n.stage,nr.value::float8,n.unit,nr.passed,nr.created_at::date::text FROM norm_results nr JOIN norms n ON n.id=nr.norm_id WHERE nr.athlete_id=$1 ORDER BY nr.created_at DESC LIMIT 20`, x.id)
		if err == nil {
			for hr.Next() {
				var name, stage, unit, date string
				var v float64
				var ok bool
				if hr.Scan(&name, &stage, &v, &unit, &ok, &date) == nil {
					hist = append(hist, map[string]any{"norm": name, "stage": stage, "value": v, "unit": unit, "passed": ok, "date": date})
				}
			}
			hr.Close()
		}
		items = append(items, map[string]any{"athlete_id": x.id, "athlete": x.name, "stage": x.stage, "next_stage": nextStage(x.stage),
			"total": x.total, "passed": x.passed, "ready": x.total > 0 && x.passed == x.total && nextStage(x.stage) != "", "history": hist})
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

// Promote — перевод на следующий этап (только если все нормативы выполнены).
func (h *Handler) Promote(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if !h.canManageAthlete(r, id) {
		httpserver.WriteError(w, http.StatusForbidden, "Нет доступа к этому спортсмену")
		return
	}
	var stage, disc string
	if h.db.QueryRowContext(r.Context(), `SELECT stage,discipline FROM athletes WHERE id=$1`, id).Scan(&stage, &disc) != nil {
		httpserver.WriteError(w, http.StatusNotFound, "Спортсмен не найден")
		return
	}
	var total, passed int
	_ = h.db.QueryRowContext(r.Context(), `SELECT COUNT(*), COUNT(*) FILTER (WHERE COALESCE((SELECT nr.passed FROM norm_results nr WHERE nr.athlete_id=$1 AND nr.norm_id=n.id ORDER BY nr.created_at DESC,nr.id DESC LIMIT 1),FALSE))
		FROM norms n WHERE n.stage=$2 AND (n.discipline='' OR n.discipline=$3)`, id, stage, disc).Scan(&total, &passed)
	next := nextStage(stage)
	if next == "" || total == 0 || passed != total {
		httpserver.WriteError(w, http.StatusUnprocessableEntity, "Не все нормативы этапа выполнены")
		return
	}
	_, _ = h.db.ExecContext(r.Context(), `UPDATE athletes SET stage=$1 WHERE id=$2`, next, id)
	h.audit(r, "перевод на этап: "+next, "athletes", strconv.FormatInt(id, 10))
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"stage": next})
}
