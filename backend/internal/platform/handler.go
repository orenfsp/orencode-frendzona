package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"orenkod/backend/internal/athletes"
	"orenkod/backend/internal/auth"
	"orenkod/backend/internal/competitions"
	"orenkod/backend/internal/emailcode"
	"orenkod/backend/internal/exportutil"
	"orenkod/backend/internal/httpserver"
	"orenkod/backend/internal/integrations/extapi"
	"orenkod/backend/internal/integrations/telegram"
	"orenkod/backend/internal/rules"
)

type Handler struct {
	db     *sql.DB
	auth   *auth.Service
	comp   *competitions.Service
	ath    *athletes.Repository
	notify *telegram.Client
	Codes  *emailcode.Service // коды подтверждения email; nil — проверка отключена
	Hooks  *extapi.Service    // вебхуки о событиях; nil — отключены
}

func New(db *sql.DB, authService *auth.Service, comp *competitions.Service, ath *athletes.Repository, notify *telegram.Client) *Handler {
	return &Handler{db: db, auth: authService, comp: comp, ath: ath, notify: notify}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var input auth.Registration
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, "Проверьте формат данных регистрации")
		return
	}
	if h.Codes != nil && !h.Codes.Verify(input.Email, input.EmailCode) {
		httpserver.WriteError(w, http.StatusBadRequest, "Неверный или просроченный код из письма")
		return
	}
	if err := h.auth.Register(r.Context(), input); err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "уже существует") {
			status = http.StatusConflict
		}
		httpserver.WriteError(w, status, err.Error())
		return
	}
	token, user, err := h.auth.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Аккаунт создан, но не удалось выполнить вход. Войдите вручную.")
		return
	}
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"token": token, "user": user})
}

func (h *Handler) SendCode(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || h.Codes == nil {
		httpserver.WriteError(w, http.StatusBadRequest, "Проверьте формат данных")
		return
	}
	delivered, err := h.Codes.Send(input.Email)
	if err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"status": "sent", "delivered": delivered})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, "Проверьте формат JSON")
		return
	}
	token, user, err := h.auth.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		httpserver.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"token": token, "user": user})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	h.auth.Logout(r.Context(), auth.TokenFrom(r))
	httpserver.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	httpserver.WriteJSON(w, http.StatusOK, auth.From(r))
}

func (h *Handler) SwitchRole(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, "Проверьте формат JSON")
		return
	}
	user, err := h.auth.SwitchRole(r.Context(), auth.TokenFrom(r), input.Role)
	if err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, user)
}

func (h *Handler) Catalogs(w http.ResponseWriter, r *http.Request) {
	types := []string{}
	rows, err := h.db.QueryContext(r.Context(), `SELECT title FROM competition_types ORDER BY id`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var title string
			_ = rows.Scan(&title)
			types = append(types, title)
		}
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{
		"disciplines": rules.Disciplines,
		"age_groups":  rules.AgeGroups,
		"statuses":    rules.SportStatuses,
		"formats":     rules.EventFormats,
		"types":       types,
	})
}

func (h *Handler) PublicHome(w http.ResponseWriter, r *http.Request) {
	items, _ := h.comp.ListUpcoming(r.Context())
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{
		"region":      "Оренбургская область",
		"disciplines": rules.Disciplines,
		"events":      items,
		"navigator": []map[string]string{
			{"title": "Что это за вид спорта", "text": "Спортивное программирование — официальный вид спорта: алгоритмы, продукт, ИБ, ИИ, робототехника и БАС."},
			{"title": "С чего начать", "text": "Выберите дисциплину, найдите организацию в регионе и подайте заявку на ближайшее соревнование по возрасту."},
			{"title": "Где заниматься", "text": "Лицей № 1, ОГУ, гимназия № 2, IT-центр Орск и клубы области. Карточки организаций — в реестре после входа."},
			{"title": "Как зарегистрироваться", "text": "Нажмите «Войти». Демо-аккаунты выданы команде; в боевой версии регистрация с согласием на персональные данные."},
		},
	})
}

func (h *Handler) ApproveAthlete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		httpserver.WriteError(w, http.StatusBadRequest, "Некорректный идентификатор спортсмена")
		return
	}
	result, err := h.db.ExecContext(r.Context(), `UPDATE athletes SET status='Активен' WHERE id=$1 AND status='На проверке'`, id)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось подтвердить профиль")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		httpserver.WriteError(w, http.StatusConflict, "Профиль не найден или уже подтверждён")
		return
	}
	var userID int64
	_ = h.db.QueryRowContext(r.Context(), `SELECT user_id FROM athletes WHERE id=$1`, id).Scan(&userID)
	if userID > 0 {
		_, _ = h.db.ExecContext(r.Context(), `INSERT INTO inbox(user_id,kind,title,message) VALUES($1,'success','Профиль подтверждён','Федерация подтвердила профиль спортсмена. Теперь можно подавать заявки на соревнования.')`, userID)
	}
	h.audit(r, "подтверждение профиля спортсмена", "athletes", strconv.FormatInt(id, 10))
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "Активен"})
}

func (h *Handler) CreateCompetition(w http.ResponseWriter, r *http.Request) {
	var input competitions.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, competitions.ErrInvalidJSON.Error())
		return
	}
	c, err := h.comp.Create(r.Context(), input)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, competitions.ErrDuplicate) {
			status = http.StatusConflict
		}
		httpserver.WriteError(w, status, err.Error())
		return
	}
	h.audit(r, "создание соревнования", "competitions", c.Title)
	if h.notify != nil {
		h.notify.NotifyDraftCreated(c.Title, c.StartsAt)
	}
	httpserver.WriteJSON(w, http.StatusCreated, c)
}

func (h *Handler) PublishCompetition(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var input struct {
		Open bool `json:"open"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)
	status := "Планируется"
	if input.Open {
		status = "Регистрация"
	}
	_, err := h.db.ExecContext(r.Context(), `UPDATE competitions SET status=$1, published_at=NOW() WHERE id=$2`, status, id)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось опубликовать")
		return
	}
	h.notifyRole(r, "", "info", "Соревнование опубликовано", "Федерация опубликовала событие в календаре.")
	h.audit(r, "публикация соревнования", "competitions", strconv.FormatInt(id, 10))
	h.emitCompetition("competition.published", id, map[string]any{"status": status})
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "status": status})
}

func (h *Handler) Apply(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var input struct {
		AthleteID int64 `json:"athlete_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)
	athleteID := input.AthleteID
	if user.ActiveRole == rules.RoleAthlete {
		athleteID = user.AthleteID
	}
	if athleteID == 0 {
		httpserver.WriteError(w, http.StatusBadRequest, "Не выбран спортсмен")
		return
	}
	if user.ActiveRole == rules.RoleCoach {
		var n int
		_ = h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM coach_links WHERE coach_id=$1 AND athlete_id=$2 AND status='active'`, user.CoachID, athleteID).Scan(&n)
		if n == 0 {
			httpserver.WriteError(w, http.StatusForbidden, "Можно подавать заявку только за подтверждённых спортсменов")
			return
		}
	}
	reason, err := h.validateApplication(r, id, athleteID)
	if err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if reason != "" {
		httpserver.WriteError(w, http.StatusUnprocessableEntity, reason)
		return
	}
	_, err = h.db.ExecContext(r.Context(), `INSERT INTO applications(competition_id,athlete_id,submitted_by,status) VALUES($1,$2,$3,'pending')
		ON CONFLICT (competition_id, athlete_id) DO UPDATE SET status='pending', submitted_by=EXCLUDED.submitted_by`, id, athleteID, user.ID)
	if err != nil {
		httpserver.WriteError(w, http.StatusConflict, "Заявка уже существует")
		return
	}
	h.refreshParticipants(r, id)
	h.notifyRole(r, rules.RoleFederation, "info", "Новая заявка", "Поступила заявка на соревнование, требуется модерация.")
	h.audit(r, "подача заявки", "applications", strconv.FormatInt(athleteID, 10))
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (h *Handler) Applications(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	query := `SELECT a.id,a.competition_id,c.title,a.athlete_id,ath.full_name,ath.discipline,ath.age_group,a.status,a.reject_reason,a.created_at
		FROM applications a JOIN competitions c ON c.id=a.competition_id JOIN athletes ath ON ath.id=a.athlete_id`
	args := []any{}
	if id != "" {
		query += ` WHERE a.competition_id=$1`
		args = append(args, id)
	}
	query += ` ORDER BY a.created_at DESC`
	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить заявки")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var appID, compID, athleteID int64
		var title, name, disc, age, status, reason string
		var created time.Time
		if err := rows.Scan(&appID, &compID, &title, &athleteID, &name, &disc, &age, &status, &reason, &created); err != nil {
			continue
		}
		items = append(items, map[string]any{
			"id": appID, "competition_id": compID, "competition": title, "athlete_id": athleteID,
			"athlete": name, "discipline": disc, "age_group": age, "status": status, "reject_reason": reason, "created_at": created.Format(time.RFC3339),
		})
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) DecideApplication(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var input struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)
	if input.Status != "approved" && input.Status != "rejected" {
		httpserver.WriteError(w, http.StatusBadRequest, "Статус: approved или rejected")
		return
	}
	_, err := h.db.ExecContext(r.Context(), `UPDATE applications SET status=$1, reject_reason=$2 WHERE id=$3`, input.Status, input.Reason, id)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить решение")
		return
	}
	var compID int64
	_ = h.db.QueryRowContext(r.Context(), `SELECT competition_id FROM applications WHERE id=$1`, id).Scan(&compID)
	h.refreshParticipants(r, compID)
	var telegramUserID sql.NullInt64
	var competitionTitle string
	_ = h.db.QueryRowContext(r.Context(), `SELECT u.telegram_user_id,c.title FROM applications ap JOIN competitions c ON c.id=ap.competition_id JOIN athletes ath ON ath.id=ap.athlete_id LEFT JOIN users u ON u.id=ath.user_id WHERE ap.id=$1`, id).Scan(&telegramUserID, &competitionTitle)
	if h.notify != nil && telegramUserID.Valid {
		message := "Заявка на соревнование «" + competitionTitle + "» одобрена."
		if input.Status == "rejected" {
			message = "Заявка на соревнование «" + competitionTitle + "» отклонена."
			if input.Reason != "" {
				message += " Причина: " + input.Reason
			}
		}
		h.notify.NotifyUser(telegramUserID.Int64, message)
	}
	h.audit(r, "модерация заявки", "applications", input.Status)
	h.emitApplication(r, id, input.Status, input.Reason)
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) RequestLink(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	var input struct {
		AthleteID  int64  `json:"athlete_id"`
		Discipline string `json:"discipline"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)
	if input.AthleteID == 0 || input.Discipline == "" {
		httpserver.WriteError(w, http.StatusBadRequest, "Нужны спортсмен и дисциплина")
		return
	}
	_, err := h.db.ExecContext(r.Context(), `INSERT INTO coach_links(coach_id,athlete_id,discipline,status) VALUES($1,$2,$3,'pending')
		ON CONFLICT (coach_id,athlete_id,discipline) DO UPDATE SET status='pending', updated_at=NOW()`, user.CoachID, input.AthleteID, input.Discipline)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось отправить запрос")
		return
	}
	h.audit(r, "запрос привязки", "coach_links", strconv.FormatInt(input.AthleteID, 10))
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (h *Handler) DecideLink(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var input struct {
		Status string `json:"status"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)
	if input.Status != "active" && input.Status != "paused" && input.Status != "completed" {
		httpserver.WriteError(w, http.StatusBadRequest, "Некорректный статус связи")
		return
	}
	res, err := h.db.ExecContext(r.Context(), `UPDATE coach_links SET status=$1, updated_at=NOW() WHERE id=$2 AND athlete_id=$3`, input.Status, id, user.AthleteID)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось обновить связь")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		httpserver.WriteError(w, http.StatusForbidden, "Подтвердить связь может только спортсмен")
		return
	}
	if input.Status == "active" {
		var coachName string
		_ = h.db.QueryRowContext(r.Context(), `SELECT u.full_name FROM coach_links l JOIN coaches c ON c.id=l.coach_id JOIN users u ON u.id=c.user_id WHERE l.id=$1`, id).Scan(&coachName)
		_, _ = h.db.ExecContext(r.Context(), `UPDATE athletes SET coach_name=$1 WHERE id=$2`, coachName, user.AthleteID)
	}
	h.audit(r, "подтверждение привязки", "coach_links", input.Status)
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) Links(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	query := `SELECT l.id,l.coach_id,u.full_name,l.athlete_id,a.full_name,l.discipline,l.status,l.created_at
		FROM coach_links l JOIN coaches c ON c.id=l.coach_id JOIN users u ON u.id=c.user_id JOIN athletes a ON a.id=l.athlete_id`
	args := []any{}
	switch user.ActiveRole {
	case rules.RoleCoach:
		query += ` WHERE l.coach_id=$1`
		args = append(args, user.CoachID)
	case rules.RoleAthlete:
		query += ` WHERE l.athlete_id=$1`
		args = append(args, user.AthleteID)
	case rules.RoleFederation:
		// Federation may inspect all coach-athlete links.
	default:
		httpserver.WriteError(w, http.StatusForbidden, "Связи тренеров и спортсменов недоступны для этой роли")
		return
	}
	query += ` ORDER BY l.created_at DESC`
	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить связи")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, coachID, athleteID int64
		var coach, athlete, disc, status string
		var created time.Time
		if err := rows.Scan(&id, &coachID, &coach, &athleteID, &athlete, &disc, &status, &created); err != nil {
			continue
		}
		items = append(items, map[string]any{"id": id, "coach_id": coachID, "coach": coach, "athlete_id": athleteID, "athlete": athlete, "discipline": disc, "status": status, "created_at": created.Format(time.RFC3339)})
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) AssignJudge(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var input struct {
		JudgeID  int64  `json:"judge_id"`
		Position string `json:"position"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)
	if input.Position == "" {
		input.Position = "Судья"
	}
	_, err := h.db.ExecContext(r.Context(), `INSERT INTO judge_assignments(competition_id,judge_id,position) VALUES($1,$2,$3)
		ON CONFLICT (competition_id,judge_id) DO UPDATE SET position=EXCLUDED.position`, id, input.JudgeID, input.Position)
	if err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, "Не удалось назначить судью")
		return
	}
	h.audit(r, "назначение судьи", "judge_assignments", input.Position)
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (h *Handler) Judges(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `SELECT j.id,u.full_name,j.category,j.category_expires_at,j.organization FROM judges j JOIN users u ON u.id=j.user_id ORDER BY u.full_name`)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить судей")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var name, cat, org string
		var exp sql.NullTime
		if err := rows.Scan(&id, &name, &cat, &exp, &org); err != nil {
			continue
		}
		expires := ""
		if exp.Valid {
			expires = exp.Time.Format("2006-01-02")
		}
		items = append(items, map[string]any{"id": id, "full_name": name, "category": cat, "category_expires_at": expires, "organization": org})
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) Assignments(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	query := `SELECT a.id,c.id,c.title,c.starts_at,u.full_name,a.position,j.category,j.category_expires_at
		FROM judge_assignments a JOIN competitions c ON c.id=a.competition_id JOIN judges j ON j.id=a.judge_id JOIN users u ON u.id=j.user_id`
	args := []any{}
	if user.ActiveRole == rules.RoleJudge {
		query += ` WHERE a.judge_id=$1`
		args = append(args, user.JudgeID)
	}
	if cid := r.URL.Query().Get("competition_id"); cid != "" {
		if len(args) == 0 {
			query += ` WHERE a.competition_id=$1`
		} else {
			query += ` AND a.competition_id=$2`
		}
		args = append(args, cid)
	}
	query += ` ORDER BY c.starts_at DESC`
	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить назначения")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, cid int64
		var title, judge, pos, cat string
		var starts time.Time
		var exp sql.NullTime
		if err := rows.Scan(&id, &cid, &title, &starts, &judge, &pos, &cat, &exp); err != nil {
			continue
		}
		expires := ""
		if exp.Valid {
			expires = exp.Time.Format("2006-01-02")
		}
		items = append(items, map[string]any{"id": id, "competition_id": cid, "competition": title, "starts_at": starts.Format(time.RFC3339), "judge": judge, "position": pos, "category": cat, "category_expires_at": expires})
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) SaveResults(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	user := auth.From(r)
	if user.ActiveRole == rules.RoleJudge {
		var assigned int
		if err := h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM judge_assignments WHERE competition_id=$1 AND judge_id=$2`, id, user.JudgeID).Scan(&assigned); err != nil || assigned == 0 {
			httpserver.WriteError(w, http.StatusForbidden, "Вводить результаты может только назначенный на это соревнование судья")
			return
		}
	}
	var input struct {
		Items []struct {
			AthleteID int64 `json:"athlete_id"`
			Place     int   `json:"place"`
			Score     int   `json:"score"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, "Проверьте формат JSON")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить протокол")
		return
	}
	defer tx.Rollback()
	for _, item := range input.Items {
		var approved int
		_ = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM applications WHERE competition_id=$1 AND athlete_id=$2 AND status='approved'`, id, item.AthleteID).Scan(&approved)
		if approved == 0 {
			httpserver.WriteError(w, http.StatusUnprocessableEntity, "В протокол попадают только одобренные заявки")
			return
		}
		_, err = tx.ExecContext(r.Context(), `INSERT INTO results(athlete_id,competition_id,place,points,score,status) VALUES($1,$2,$3,$4,$5,'финишировал')
			ON CONFLICT (athlete_id,competition_id) DO UPDATE SET place=EXCLUDED.place, points=EXCLUDED.points, score=EXCLUDED.score`,
			item.AthleteID, id, item.Place, 0, item.Score)
		if err != nil {
			httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось записать результат")
			return
		}
	}
	_, _ = tx.ExecContext(r.Context(), `UPDATE competitions SET status='Завершено' WHERE id=$1`, id)
	if err = tx.Commit(); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить протокол")
		return
	}
	h.recalculatePoints(r)
	h.notifyInbox(r, rules.RoleAthlete, "success", "Протокол опубликован", "Результаты соревнования доступны в истории выступлений.")
	h.notifyLinkedAthletesOfResults(r, id)
	h.audit(r, "ввод результатов", "results", strconv.FormatInt(id, 10))
	h.emitCompetition("protocol.published", id, map[string]any{"source": "federation", "results": len(input.Items)})
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) Protocol(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	c, err := h.comp.List(r.Context())
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Нет данных протокола")
		return
	}
	var title, discipline, location, starts string
	for _, item := range c {
		if item.ID == id {
			title, discipline, location, starts = item.Title, item.Discipline, item.Location, item.StartsAt
		}
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT ath.full_name,ath.organization,ath.age_group,r.place,r.score,r.points
		FROM results r JOIN athletes ath ON ath.id=r.athlete_id WHERE r.competition_id=$1 ORDER BY r.place`, id)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Нет данных протокола")
		return
	}
	defer rows.Close()
	table := [][]string{{"Место", "Спортсмен", "Организация", "Группа", "Баллы", "Рейтинговые очки"}}
	lines := []string{"Протокол: " + title, "Дисциплина: " + discipline, "Место: " + location, "Дата: " + starts, ""}
	for rows.Next() {
		var name, org, age string
		var place, score, points int
		if err := rows.Scan(&name, &org, &age, &place, &score, &points); err != nil {
			continue
		}
		table = append(table, []string{strconv.Itoa(place), name, org, age, strconv.Itoa(score), strconv.Itoa(points)})
		lines = append(lines, strconv.Itoa(place)+". "+name+" — "+org+" — "+strconv.Itoa(score)+" б.")
	}
	if r.URL.Query().Get("format") == "docx" {
		doc, err := exportutil.DOCX("Протокол соревнования", lines)
		if err != nil {
			httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сформировать документ")
			return
		}
		writeFile(w, "protocol-"+strconv.FormatInt(id, 10)+".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", doc)
		return
	}
	xlsx, err := exportutil.XLSX("Протокол", table)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сформировать таблицу")
		return
	}
	writeFile(w, "protocol-"+strconv.FormatInt(id, 10)+".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", xlsx)
}

func (h *Handler) ResultsHistory(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	query := `SELECT c.id,c.title,c.discipline,c.sport_status,c.starts_at,r.place,r.score,r.points,ath.full_name
		FROM results r JOIN competitions c ON c.id=r.competition_id JOIN athletes ath ON ath.id=r.athlete_id`
	args := []any{}
	if user.ActiveRole == rules.RoleAthlete {
		query += ` WHERE r.athlete_id=$1`
		args = append(args, user.AthleteID)
	} else if user.ActiveRole == rules.RoleCoach {
		query += ` WHERE r.athlete_id IN (SELECT athlete_id FROM coach_links WHERE coach_id=$1 AND status='active')`
		args = append(args, user.CoachID)
	} else if aid := r.URL.Query().Get("athlete_id"); aid != "" {
		query += ` WHERE r.athlete_id=$1`
		args = append(args, aid)
	}
	query += ` ORDER BY c.starts_at DESC`
	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить историю")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var cid int64
		var title, disc, status, name string
		var starts time.Time
		var place, score, points int
		if err := rows.Scan(&cid, &title, &disc, &status, &starts, &place, &score, &points, &name); err != nil {
			continue
		}
		items = append(items, map[string]any{"competition_id": cid, "competition": title, "discipline": disc, "sport_status": status, "starts_at": starts.Format(time.RFC3339), "place": place, "score": score, "points": points, "athlete": name})
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) RankApply(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	var input struct {
		AthleteID     int64  `json:"athlete_id"`
		CompetitionID int64  `json:"competition_id"`
		RequestedRank string `json:"requested_rank"`
		Place         int    `json:"place"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)
	athleteID := input.AthleteID
	if user.ActiveRole == rules.RoleAthlete {
		var n int
		_ = h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM coach_links WHERE athlete_id=$1 AND status='active'`, user.AthleteID).Scan(&n)
		if n > 0 {
			httpserver.WriteError(w, http.StatusForbidden, "Заявку на разряд подаёт тренер")
			return
		}
		athleteID = user.AthleteID
	}
	if athleteID == 0 || input.RequestedRank == "" {
		httpserver.WriteError(w, http.StatusBadRequest, "Нужны спортсмен и разряд")
		return
	}
	_, err := h.db.ExecContext(r.Context(), `INSERT INTO rank_applications(athlete_id,submitted_by,competition_id,requested_rank,result_place,status) VALUES($1,$2,$3,$4,$5,'pending')`,
		athleteID, user.ID, nullIfZero(input.CompetitionID), input.RequestedRank, input.Place)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось подать заявку")
		return
	}
	h.notifyRole(r, rules.RoleFederation, "info", "Заявка на разряд", "Поступила заявка на присвоение или подтверждение разряда.")
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (h *Handler) RankList(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	query := `SELECT r.id,ath.full_name,r.requested_rank,r.result_place,r.status,c.title,r.decision_note,r.created_at,ath.rank_expires_at
		FROM rank_applications r JOIN athletes ath ON ath.id=r.athlete_id LEFT JOIN competitions c ON c.id=r.competition_id`
	args := []any{}
	if user.ActiveRole == rules.RoleAthlete {
		query += ` WHERE r.athlete_id=$1`
		args = append(args, user.AthleteID)
	} else if user.ActiveRole == rules.RoleCoach {
		query += ` WHERE r.athlete_id IN (SELECT athlete_id FROM coach_links WHERE coach_id=$1 AND status='active')`
		args = append(args, user.CoachID)
	}
	query += ` ORDER BY r.created_at DESC`
	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить заявки на разряд")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var name, rank, status, note string
		var title sql.NullString
		var place int
		var created time.Time
		var expires sql.NullTime
		if err := rows.Scan(&id, &name, &rank, &place, &status, &title, &note, &created, &expires); err != nil {
			continue
		}
		exp := ""
		if expires.Valid {
			exp = expires.Time.Format("2006-01-02")
		}
		items = append(items, map[string]any{"id": id, "athlete": name, "requested_rank": rank, "place": place, "status": status, "competition": title.String, "decision_note": note, "created_at": created.Format(time.RFC3339), "rank_expires_at": exp})
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) RankDecide(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var input struct {
		Status string `json:"status"`
		Note   string `json:"note"`
		Order  string `json:"order"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)
	_, err := h.db.ExecContext(r.Context(), `UPDATE rank_applications SET status=$1, decision_note=$2 WHERE id=$3`, input.Status, input.Note, id)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить решение")
		return
	}
	if input.Status == "approved" {
		_, _ = h.db.ExecContext(r.Context(), `UPDATE athletes SET rank_title=r.requested_rank, rank_assigned_at=CURRENT_DATE, rank_expires_at=CURRENT_DATE + INTERVAL '2 years', rank_order=$1
			FROM rank_applications r WHERE athletes.id=r.athlete_id AND r.id=$2`, input.Order, id)
	}
	var telegramUserID sql.NullInt64
	var requestedRank string
	_ = h.db.QueryRowContext(r.Context(), `SELECT u.telegram_user_id,ra.requested_rank FROM rank_applications ra JOIN athletes ath ON ath.id=ra.athlete_id LEFT JOIN users u ON u.id=ath.user_id WHERE ra.id=$1`, id).Scan(&telegramUserID, &requestedRank)
	if h.notify != nil && telegramUserID.Valid {
		message := "Заявка на разряд «" + requestedRank + "» рассмотрена: " + input.Status + "."
		if input.Note != "" {
			message += " Комментарий Федерации: " + input.Note
		}
		h.notify.NotifyUser(telegramUserID.Int64, message)
	}
	h.audit(r, "решение по разряду", "rank_applications", input.Status)
	h.Hooks.Emit("rank.decided", map[string]any{"application_id": id, "status": input.Status, "rank": requestedRank, "order": input.Order})
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) CategoryApply(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	var input struct {
		Requested string `json:"requested_category"`
		Kind      string `json:"kind"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)
	if input.Kind == "" {
		input.Kind = "присвоение"
	}
	_, err := h.db.ExecContext(r.Context(), `INSERT INTO category_applications(judge_id,requested_category,kind,status) VALUES($1,$2,$3,'pending')`, user.JudgeID, input.Requested, input.Kind)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось подать заявку")
		return
	}
	h.notifyRole(r, rules.RoleFederation, "info", "Заявка на судейскую категорию", "Поступила заявка на присвоение или подтверждение судейской категории.")
	h.audit(r, "подача заявки на судейскую категорию", "category_applications", input.Requested)
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (h *Handler) CategoryList(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	query := `SELECT c.id,u.full_name,c.requested_category,c.kind,c.status,c.decision_note,c.created_at
		FROM category_applications c JOIN judges j ON j.id=c.judge_id JOIN users u ON u.id=j.user_id`
	args := []any{}
	if user.ActiveRole == rules.RoleJudge {
		query += ` WHERE c.judge_id=$1`
		args = append(args, user.JudgeID)
	}
	query += ` ORDER BY c.created_at DESC`
	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить заявки")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var name, cat, kind, status, note string
		var created time.Time
		if err := rows.Scan(&id, &name, &cat, &kind, &status, &note, &created); err != nil {
			continue
		}
		items = append(items, map[string]any{"id": id, "judge": name, "requested_category": cat, "kind": kind, "status": status, "decision_note": note, "created_at": created.Format(time.RFC3339)})
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) CategoryDecide(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var input struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)
	_, err := h.db.ExecContext(r.Context(), `UPDATE category_applications SET status=$1, decision_note=$2 WHERE id=$3`, input.Status, input.Note, id)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить")
		return
	}
	if input.Status == "approved" {
		_, _ = h.db.ExecContext(r.Context(), `UPDATE judges SET category=c.requested_category, category_expires_at=CURRENT_DATE + INTERVAL '4 years'
			FROM category_applications c WHERE judges.id=c.judge_id AND c.id=$1`, id)
	}
	var telegramUserID sql.NullInt64
	var requestedCategory string
	_ = h.db.QueryRowContext(r.Context(), `SELECT u.telegram_user_id,c.requested_category FROM category_applications c JOIN judges j ON j.id=c.judge_id LEFT JOIN users u ON u.id=j.user_id WHERE c.id=$1`, id).Scan(&telegramUserID, &requestedCategory)
	if h.notify != nil && telegramUserID.Valid {
		message := "Заявка на судейскую категорию «" + requestedCategory + "» рассмотрена: " + input.Status + "."
		if input.Note != "" {
			message += " Комментарий Федерации: " + input.Note
		}
		h.notify.NotifyUser(telegramUserID.Int64, message)
	}
	h.audit(r, "решение по судейской категории", "category_applications", input.Status)
	h.Hooks.Emit("category.decided", map[string]any{"application_id": id, "status": input.Status, "category": requestedCategory})
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) Rating(w http.ResponseWriter, r *http.Request) {
	items, err := h.buildRating(r, r.URL.Query().Get("discipline"), r.URL.Query().Get("age_group"))
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось рассчитать рейтинг")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{
		"items":   items,
		"formula": "Баллы за 1–3 места по уровню соревнования + места 4–10 с понижающим коэффициентом. При двух и более подиумах сезона коэффициент стабильности ×1.2. Дисквалифицированные не входят в рейтинг.",
		"table": map[string][3]int{
			"Чемпионат России":  {100, 80, 60},
			"Первенство России": {80, 60, 40},
			"Межрегиональные":   {60, 45, 30},
			"Региональные":      {40, 30, 20},
		},
	})
}

func (h *Handler) TeamGet(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `SELECT t.athlete_id,a.full_name,a.organization,t.discipline,t.age_group,a.points
		FROM team_roster t JOIN athletes a ON a.id=t.athlete_id WHERE t.season='2026' ORDER BY a.points DESC`)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить сборную")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var name, org, disc, age string
		var points int
		if err := rows.Scan(&id, &name, &org, &disc, &age, &points); err != nil {
			continue
		}
		items = append(items, map[string]any{"athlete_id": id, "athlete": name, "organization": org, "discipline": disc, "age_group": age, "points": points})
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "season": "2026"})
}

func (h *Handler) TeamSave(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Items []struct {
			AthleteID  int64  `json:"athlete_id"`
			Discipline string `json:"discipline"`
			AgeGroup   string `json:"age_group"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, "Проверьте формат JSON")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить состав")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM team_roster WHERE season='2026'`); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить состав")
		return
	}
	for _, item := range input.Items {
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO team_roster(season,athlete_id,discipline,age_group) VALUES('2026',$1,$2,$3)`, item.AthleteID, item.Discipline, item.AgeGroup); err != nil {
			httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить состав")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить состав")
		return
	}
	h.audit(r, "формирование сборной", "team_roster", strconv.Itoa(len(input.Items)))
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) TeamOrder(w http.ResponseWriter, r *http.Request) {
	rows, _ := h.db.QueryContext(r.Context(), `SELECT a.full_name,t.discipline,t.age_group FROM team_roster t JOIN athletes a ON a.id=t.athlete_id WHERE t.season='2026' ORDER BY a.full_name`)
	defer func() {
		if rows != nil {
			rows.Close()
		}
	}()
	lines := []string{"Проект приказа о составе сборной Оренбургской области по спортивному программированию", "Сезон 2026", ""}
	i := 1
	if rows != nil {
		for rows.Next() {
			var name, disc, age string
			_ = rows.Scan(&name, &disc, &age)
			lines = append(lines, strconv.Itoa(i)+". "+name+" — "+disc+" — "+age)
			i++
		}
	}
	doc, err := exportutil.DOCX("Проект приказа", lines)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сформировать приказ")
		return
	}
	writeFile(w, "team-order-2026.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", doc)
}

func (h *Handler) Enrollment(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `SELECT year,municipality,organization,discipline,age_group,athletes_count FROM enrollment_stats ORDER BY year, municipality`)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Нет отчёта")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	table := [][]string{{"Год", "Муниципалитет", "Организация", "Дисциплина", "Возрастная группа", "Численность"}}
	for rows.Next() {
		var year, count int
		var mun, org, disc, age string
		if err := rows.Scan(&year, &mun, &org, &disc, &age, &count); err != nil {
			continue
		}
		items = append(items, map[string]any{"year": year, "municipality": mun, "organization": org, "discipline": disc, "age_group": age, "athletes_count": count})
		table = append(table, []string{strconv.Itoa(year), mun, org, disc, age, strconv.Itoa(count)})
	}
	groupRows := h.groupStageRows(r)
	groupItems := []map[string]any{}
	if len(groupRows) > 0 {
		table = append(table, []string{}, []string{"Учебно-тренировочные группы (на текущую дату)", "Организация", "Этап подготовки", "Дисциплина", "", "Численность"})
		for _, g := range groupRows {
			table = append(table, []string{"", g[0], g[1], g[2], "", g[3]})
			groupItems = append(groupItems, map[string]any{"organization": g[0], "stage": g[1], "discipline": g[2], "athletes_count": g[3]})
		}
	}
	if r.URL.Query().Get("format") == "xlsx" {
		xlsx, err := exportutil.XLSX("Численность", table)
		if err != nil {
			httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось выгрузить")
			return
		}
		writeFile(w, "enrollment.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", xlsx)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "groups": groupItems})
}

func (h *Handler) AthleteReport(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	user := auth.From(r)
	switch user.ActiveRole {
	case rules.RoleFederation:
		// Federation may access all athlete reports.
	case rules.RoleAthlete:
		if user.AthleteID != id {
			httpserver.WriteError(w, http.StatusForbidden, "Спортсмен может открыть только собственный отчёт")
			return
		}
	case rules.RoleCoach:
		var linked int
		if err := h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM coach_links WHERE coach_id=$1 AND athlete_id=$2 AND status='active'`, user.CoachID, id).Scan(&linked); err != nil || linked == 0 {
			httpserver.WriteError(w, http.StatusForbidden, "Тренер может открыть отчёт только подтверждённого спортсмена")
			return
		}
	default:
		httpserver.WriteError(w, http.StatusForbidden, "Для этой роли отчёты спортсменов недоступны")
		return
	}
	var name, org, disc, rank string
	if err := h.db.QueryRowContext(r.Context(), `SELECT full_name,organization,discipline,rank_title FROM athletes WHERE id=$1`, id).Scan(&name, &org, &disc, &rank); err != nil {
		httpserver.WriteError(w, http.StatusNotFound, "Спортсмен не найден")
		return
	}
	lines := []string{"Отчёт по спортсмену", name, "Организация: " + org, "Дисциплина: " + disc, "Разряд: " + rank, "", "История выступлений:"}
	rows, _ := h.db.QueryContext(r.Context(), `SELECT c.title,c.starts_at,r.place,r.score FROM results r JOIN competitions c ON c.id=r.competition_id WHERE r.athlete_id=$1 ORDER BY c.starts_at`, id)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var title string
			var starts time.Time
			var place, score int
			_ = rows.Scan(&title, &starts, &place, &score)
			lines = append(lines, starts.Format("02.01.2006")+" — "+title+" — место "+strconv.Itoa(place)+", баллы "+strconv.Itoa(score))
		}
	}
	doc, err := exportutil.DOCX("Отчёт по спортсмену", lines)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сформировать отчёт")
		return
	}
	writeFile(w, "athlete-"+strconv.FormatInt(id, 10)+".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", doc)
}

func (h *Handler) RegistryXLSX(w http.ResponseWriter, r *http.Request) {
	items, err := h.ath.List(r.Context())
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось выгрузить реестр")
		return
	}
	table := [][]string{{"ФИО", "ФСП ID", "Организация", "Муниципалитет", "Дисциплина", "Группа", "Разряд", "Тренер", "Очки"}}
	for _, a := range items {
		table = append(table, []string{a.FullName, a.FSPID, a.Organization, a.Municipality, a.Discipline, a.AgeGroup, a.RankTitle, a.CoachName, strconv.Itoa(a.Points)})
	}
	xlsx, err := exportutil.XLSX("Реестр", table)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось выгрузить реестр")
		return
	}
	writeFile(w, "registry.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", xlsx)
}

func (h *Handler) Inbox(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	rows, err := h.db.QueryContext(r.Context(), `SELECT id,kind,title,message,created_at FROM inbox WHERE user_id=$1 OR role_scope IN ('',$2) ORDER BY created_at DESC LIMIT 40`, user.ID, user.ActiveRole)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить уведомления")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var kind, title, message string
		var created time.Time
		if err := rows.Scan(&id, &kind, &title, &message, &created); err != nil {
			continue
		}
		items = append(items, map[string]any{"id": id, "kind": kind, "title": title, "message": message, "when": created.Format("02.01.2006 15:04")})
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) Audit(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `SELECT COALESCE(u.full_name,''),a.role,a.action,a.entity,a.details,a.created_at FROM audit_log a LEFT JOIN users u ON u.id=a.user_id ORDER BY a.created_at DESC LIMIT 80`)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Нет журнала")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var name, role, action, entity, details string
		var created time.Time
		if err := rows.Scan(&name, &role, &action, &entity, &details, &created); err != nil {
			continue
		}
		items = append(items, map[string]any{"user": name, "role": role, "action": action, "entity": entity, "details": details, "created_at": created.Format(time.RFC3339)})
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) IssueCertificate(w http.ResponseWriter, r *http.Request) {
	user := auth.From(r)
	var input struct {
		CompetitionID int64 `json:"competition_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)
	code := strings.ToUpper(strconv.FormatInt(time.Now().Unix()%1000000, 36) + strconv.FormatInt(user.AthleteID, 10))
	_, err := h.db.ExecContext(r.Context(), `INSERT INTO certificates(code,athlete_id,competition_id) VALUES($1,$2,$3)`, code, user.AthleteID, input.CompetitionID)
	if err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, "Не удалось выпустить справку")
		return
	}
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"code": code, "url": "/api/public/verify/" + code})
}

func (h *Handler) VerifyCertificate(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	var name, title string
	var place, score int
	var starts time.Time
	err := h.db.QueryRowContext(r.Context(), `SELECT a.full_name,c.title,c.starts_at,COALESCE(r.place,0),COALESCE(r.score,0)
		FROM certificates cert JOIN athletes a ON a.id=cert.athlete_id JOIN competitions c ON c.id=cert.competition_id
		LEFT JOIN results r ON r.athlete_id=cert.athlete_id AND r.competition_id=cert.competition_id
		WHERE cert.code=$1`, code).Scan(&name, &title, &starts, &place, &score)
	if err != nil {
		httpserver.WriteError(w, http.StatusNotFound, "Справка не найдена")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"valid": true, "athlete": name, "competition": title, "starts_at": starts.Format("2006-01-02"), "place": place, "score": score, "code": code})
}

func (h *Handler) AddType(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title string `json:"title"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)
	if strings.TrimSpace(input.Title) == "" {
		httpserver.WriteError(w, http.StatusBadRequest, "Название типа обязательно")
		return
	}
	_, err := h.db.ExecContext(r.Context(), `INSERT INTO competition_types(title) VALUES($1) ON CONFLICT (title) DO NOTHING`, strings.TrimSpace(input.Title))
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось добавить тип")
		return
	}
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (h *Handler) validateApplication(r *http.Request, competitionID, athleteID int64) (string, error) {
	var disc, ageGroup, status string
	var starts time.Time
	if err := h.db.QueryRowContext(r.Context(), `SELECT discipline,age_group,status,starts_at FROM competitions WHERE id=$1`, competitionID).Scan(&disc, &ageGroup, &status, &starts); err != nil {
		return "", errors.New("Соревнование не найдено")
	}
	if status != "Регистрация" {
		return "Приём заявок закрыт", nil
	}
	var birth time.Time
	var athleteDisc, athleteStatus string
	if err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(birth_date, CURRENT_DATE - INTERVAL '16 years'), discipline, status FROM athletes WHERE id=$1`, athleteID).Scan(&birth, &athleteDisc, &athleteStatus); err != nil {
		return "", errors.New("Спортсмен не найден")
	}
	if athleteStatus == "На проверке" {
		return "Профиль спортсмена ожидает подтверждения Федерации", nil
	}
	discs := []string{athleteDisc}
	rows, err := h.db.QueryContext(r.Context(), `SELECT discipline FROM athlete_disciplines WHERE athlete_id=$1`, athleteID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var d string
			_ = rows.Scan(&d)
			discs = append(discs, d)
		}
	}
	if !rules.MatchesDiscipline(discs, disc) {
		return "Дисциплина спортсмена не совпадает с дисциплиной соревнования", nil
	}
	age := rules.AgeAt(birth, starts)
	if !rules.MatchesAgeGroup(ageGroup, age) {
		return "Возраст на дату соревнования не подходит под возрастную группу", nil
	}
	return "", nil
}

func (h *Handler) refreshParticipants(r *http.Request, competitionID int64) {
	_, _ = h.db.ExecContext(r.Context(), `UPDATE competitions SET participants=(SELECT COUNT(*) FROM applications WHERE competition_id=$1 AND status='approved') WHERE id=$1`, competitionID)
}

func (h *Handler) recalculatePoints(r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `SELECT r.athlete_id,r.place,c.sport_status,c.competition_type FROM results r JOIN competitions c ON c.id=r.competition_id WHERE c.starts_at >= '2025-09-01'`)
	if err != nil {
		log.Printf("recalc: %v", err)
		return
	}
	defer rows.Close()
	type acc struct {
		raw, prizes int
	}
	byAthlete := map[int64]*acc{}
	for rows.Next() {
		var athleteID int64
		var place int
		var status, kind string
		if err := rows.Scan(&athleteID, &place, &status, &kind); err != nil {
			continue
		}
		if byAthlete[athleteID] == nil {
			byAthlete[athleteID] = &acc{}
		}
		pts := rules.PlacePoints(status, kind, place)
		byAthlete[athleteID].raw += pts
		if place >= 1 && place <= 3 {
			byAthlete[athleteID].prizes++
		}
		_, _ = h.db.ExecContext(r.Context(), `UPDATE results SET points=$1 WHERE athlete_id=$2 AND competition_id IN (SELECT id FROM competitions WHERE sport_status=$3 AND competition_type=$4) AND place=$5`, pts, athleteID, status, kind, place)
	}
	for id, a := range byAthlete {
		total := int(float64(a.raw) * rules.StabilityMultiplier(a.prizes))
		_, _ = h.db.ExecContext(r.Context(), `UPDATE athletes SET points=$1 WHERE id=$2`, total, id)
	}
}

type ratingRow struct {
	Rank         int              `json:"rank"`
	AthleteID    int64            `json:"athlete_id"`
	Athlete      string           `json:"athlete"`
	Organization string           `json:"organization"`
	Discipline   string           `json:"discipline"`
	AgeGroup     string           `json:"age_group"`
	RankTitle    string           `json:"rank_title"`
	Points       int              `json:"points"`
	RawPoints    int              `json:"raw_points"`
	Coefficient  float64          `json:"coefficient"`
	PrizeCount   int              `json:"prize_count"`
	Breakdown    []map[string]any `json:"breakdown"`
}

func (h *Handler) buildRating(r *http.Request, discipline, age string) ([]ratingRow, error) {
	query := `SELECT a.id,a.full_name,a.organization,a.discipline,a.age_group,a.disqualified,a.rank_title FROM athletes a WHERE a.disqualified=FALSE`
	args := []any{}
	if discipline != "" && discipline != "Все дисциплины" {
		query += ` AND (a.discipline=$1 OR EXISTS (SELECT 1 FROM athlete_disciplines d WHERE d.athlete_id=a.id AND d.discipline=$1))`
		args = append(args, discipline)
	}
	if age != "" && age != "Все группы" {
		query += ` AND a.age_group=$` + strconv.Itoa(len(args)+1)
		args = append(args, age)
	}
	query += ` ORDER BY a.full_name`
	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ratingRow{}
	for rows.Next() {
		var row ratingRow
		var dq bool
		if err := rows.Scan(&row.AthleteID, &row.Athlete, &row.Organization, &row.Discipline, &row.AgeGroup, &dq, &row.RankTitle); err != nil {
			continue
		}
		res, _ := h.db.QueryContext(r.Context(), `SELECT c.title,c.sport_status,c.competition_type,r.place FROM results r JOIN competitions c ON c.id=r.competition_id WHERE r.athlete_id=$1 AND c.starts_at >= '2025-09-01'`, row.AthleteID)
		if res != nil {
			for res.Next() {
				var title, status, kind string
				var place int
				_ = res.Scan(&title, &status, &kind, &place)
				pts := rules.PlacePoints(status, kind, place)
				row.RawPoints += pts
				if place <= 3 {
					row.PrizeCount++
				}
				row.Breakdown = append(row.Breakdown, map[string]any{"competition": title, "place": place, "points": pts, "level": status})
			}
			res.Close()
		}
		row.Coefficient = rules.StabilityMultiplier(row.PrizeCount)
		row.Points = int(float64(row.RawPoints) * row.Coefficient)
		out = append(out, row)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Points > out[i].Points {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	for i := range out {
		out[i].Rank = i + 1
	}
	return out, nil
}

func (h *Handler) notifyLinkedAthletesOfResults(r *http.Request, competitionID int64) {
	if h.notify == nil {
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT DISTINCT c.title, u.telegram_user_id
		FROM applications ap
		JOIN competitions c ON c.id=ap.competition_id
		JOIN athletes a ON a.id=ap.athlete_id
		JOIN users u ON u.id=a.user_id
		WHERE ap.competition_id=$1 AND ap.status='approved' AND u.telegram_user_id IS NOT NULL`, competitionID)
	if err != nil {
		log.Printf("Telegram result notification lookup: %v", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var title string
		var telegramUserID int64
		if err := rows.Scan(&title, &telegramUserID); err != nil {
			log.Printf("Telegram result notification scan: %v", err)
			continue
		}
		h.notify.NotifyUser(telegramUserID, "Опубликованы результаты соревнования «"+title+"». Откройте платформу ФСП 56, чтобы посмотреть место и баллы.")
	}
	if err := rows.Err(); err != nil {
		log.Printf("Telegram result notification rows: %v", err)
	}
}

// notifyRole stores a notification in the in-app inbox and mirrors it to Telegram:
// personally to every linked user holding the role (all linked users when role is empty)
// and to the shared chat when TELEGRAM_DEFAULT_CHAT_ID is configured.
func (h *Handler) notifyRole(r *http.Request, role, kind, title, message string) {
	h.notifyInbox(r, role, kind, title, message)
	if h.notify == nil {
		return
	}
	h.notify.NotifyChannel(title, message)
	h.notifyLinkedByRole(r, role, title+"\n\n"+message)
}

// notifyInbox only writes to the in-app inbox (no Telegram).
func (h *Handler) notifyInbox(r *http.Request, role, kind, title, message string) {
	if _, err := h.db.ExecContext(r.Context(), `INSERT INTO inbox(role_scope,kind,title,message) VALUES($1,$2,$3,$4)`, role, kind, title, message); err != nil {
		log.Printf("inbox insert: %v", err)
	}
}

func (h *Handler) notifyLinkedByRole(r *http.Request, role, text string) {
	if h.notify == nil || !h.notify.Configured() {
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT DISTINCT u.telegram_user_id
		FROM users u
		WHERE u.telegram_user_id IS NOT NULL
		  AND ($1 = '' OR EXISTS (SELECT 1 FROM user_roles ur WHERE ur.user_id=u.id AND ur.role=$1))`, role)
	if err != nil {
		log.Printf("Telegram role notification lookup: %v", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var tgID int64
		if err := rows.Scan(&tgID); err != nil {
			continue
		}
		h.notify.NotifyUser(tgID, text)
	}
}

func (h *Handler) audit(r *http.Request, action, entity, details string) {
	user := auth.From(r)
	_, _ = h.db.ExecContext(r.Context(), `INSERT INTO audit_log(user_id,role,action,entity,details) VALUES($1,$2,$3,$4,$5)`, user.ID, user.ActiveRole, action, entity, details)
}

func writeFile(w http.ResponseWriter, name, contentType string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func nullIfZero(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

// RecalcPoints пересчитывает рейтинговые баллы вне HTTP-запроса (например, после импорта результатов).
func (h *Handler) RecalcPoints(ctx context.Context) {
	h.recalculatePoints((&http.Request{}).WithContext(ctx))
}

// emitCompetition отправляет событие с данными соревнования.
func (h *Handler) emitCompetition(event string, id int64, extra map[string]any) {
	if h.Hooks == nil {
		return
	}
	var title, discipline, kind, age, status string
	var starts time.Time
	if err := h.db.QueryRow(`SELECT title,discipline,competition_type,age_group,status,starts_at FROM competitions WHERE id=$1`, id).Scan(&title, &discipline, &kind, &age, &status, &starts); err != nil {
		return
	}
	data := map[string]any{"competition_id": id, "title": title, "discipline": discipline, "competition_type": kind,
		"age_group": age, "competition_status": status, "starts_at": starts.Format(time.RFC3339)}
	for k, v := range extra {
		data[k] = v
	}
	h.Hooks.Emit(event, data)
}

// emitApplication отправляет application.approved / application.rejected.
func (h *Handler) emitApplication(r *http.Request, id int64, status, reason string) {
	if h.Hooks == nil {
		return
	}
	var compID, athleteID int64
	var athlete, fsp, title string
	if err := h.db.QueryRowContext(r.Context(), `SELECT ap.competition_id,ap.athlete_id,a.full_name,a.fsp_id,c.title FROM applications ap
		JOIN athletes a ON a.id=ap.athlete_id JOIN competitions c ON c.id=ap.competition_id WHERE ap.id=$1`, id).Scan(&compID, &athleteID, &athlete, &fsp, &title); err != nil {
		return
	}
	data := map[string]any{"application_id": id, "competition_id": compID, "competition": title, "athlete_id": athleteID, "athlete": athlete, "fsp_id": fsp}
	if status == "rejected" {
		data["reason"] = reason
	}
	h.Hooks.Emit("application."+status, data)
}
