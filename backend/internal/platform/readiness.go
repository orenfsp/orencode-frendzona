package platform

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strings"
	"time"

	"orenkod/backend/internal/auth"
	"orenkod/backend/internal/httpserver"
	"orenkod/backend/internal/readiness"
	"orenkod/backend/internal/rules"
)

// Собственный сценарий команды: «Готовность к старту».
// Для соревнования платформа строит чек-лист допуска по каждому участнику (возраст, дисциплина, заявка,
// санкции, антидопинг, согласие представителя, срок разряда, тренер), показывает процент готовности и блокеры,
// а за 7, 3 и 1 день до старта сама напоминает неготовым участникам в кабинет и в Telegram.

func (h *Handler) mountReadiness(mux *http.ServeMux, a *auth.Service) {
	mux.HandleFunc("GET /api/readiness", a.Middleware(rules.RoleFederation, rules.RoleCoach, rules.RoleAthlete)(h.ReadinessGet))
	mux.HandleFunc("POST /api/readiness/remind", a.Middleware(rules.RoleFederation)(h.ReadinessRemind))
}

type readinessComp struct {
	ID         int64
	Title      string
	Discipline string
	AgeGroup   string
	StartsAt   time.Time
	Until      *time.Time
}

type readinessRow struct {
	AthleteID int64            `json:"athlete_id"`
	Athlete   string           `json:"athlete"`
	UserID    int64            `json:"-"`
	Report    readiness.Report `json:"report"`
}

func (h *Handler) readinessComp(ctx context.Context, id int64) (readinessComp, error) {
	var c readinessComp
	var until sql.NullTime
	err := h.db.QueryRowContext(ctx, `SELECT id,title,discipline,age_group,starts_at,applications_until FROM competitions WHERE id=$1`, id).
		Scan(&c.ID, &c.Title, &c.Discipline, &c.AgeGroup, &c.StartsAt, &until)
	if until.Valid {
		t := until.Time
		c.Until = &t
	}
	return c, err
}

// readinessRows считает отчёты. scope: "all" — все участники с заявкой; "coach" — подопечные тренера; "athlete" — один спортсмен.
func (h *Handler) readinessRows(ctx context.Context, c readinessComp, scope string, id int64) ([]readinessRow, error) {
	q := `SELECT a.id,a.full_name,COALESCE(a.user_id,0),a.birth_date,a.discipline,COALESCE(a.rank_title,''),a.rank_expires_at,a.disqualified,
		COALESCE(u.rusada_completed,FALSE),COALESCE(u.guardian_consent,FALSE),
		COALESCE((SELECT p.status FROM applications p WHERE p.competition_id=$1 AND p.athlete_id=a.id ORDER BY p.id DESC LIMIT 1),''),
		EXISTS(SELECT 1 FROM coach_links l WHERE l.athlete_id=a.id AND l.status='active'),
		COALESCE((SELECT string_agg(d.discipline,'|') FROM athlete_disciplines d WHERE d.athlete_id=a.id),'')
		FROM athletes a LEFT JOIN users u ON u.id=a.user_id WHERE `
	args := []any{c.ID}
	switch scope {
	case "athlete":
		q += `a.id=$2`
		args = append(args, id)
	case "coach":
		q += `a.id IN (SELECT athlete_id FROM coach_links WHERE coach_id=$2 AND status='active')`
		args = append(args, id)
	default:
		q += `a.id IN (SELECT athlete_id FROM applications WHERE competition_id=$1)`
	}
	rows, err := h.db.QueryContext(ctx, q+` ORDER BY a.full_name`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := time.Now()
	var out []readinessRow
	for rows.Next() {
		var r readinessRow
		var birth, rankExp sql.NullTime
		var disc, rank, appStatus, discs string
		var disq, rusada, consent, coach bool
		if err := rows.Scan(&r.AthleteID, &r.Athlete, &r.UserID, &birth, &disc, &rank, &rankExp, &disq, &rusada, &consent, &appStatus, &coach, &discs); err != nil {
			continue
		}
		in := readiness.Input{
			Now: now, StartsAt: c.StartsAt, ApplicationsUntil: c.Until,
			CompetitionAgeGroup: c.AgeGroup, CompetitionDiscipline: c.Discipline,
			AthleteDisciplines: append(strings.Split(discs, "|"), disc),
			ApplicationStatus:  appStatus, HasRank: rank != "", RusadaCompleted: rusada,
			GuardianConsent: consent, Disqualified: disq, HasCoach: coach,
		}
		if birth.Valid {
			t := birth.Time
			in.Birth = &t
		}
		if rankExp.Valid {
			t := rankExp.Time
			in.RankExpiresAt = &t
		}
		r.Report = readiness.Evaluate(in)
		out = append(out, r)
	}
	return out, nil
}

// ReadinessGet — GET /api/readiness?competition_id=N. Спортсмен видит себя, тренер — подопечных, федерация — всех участников.
func (h *Handler) ReadinessGet(w http.ResponseWriter, r *http.Request) {
	cid, _ := parseID(r.URL.Query().Get("competition_id"))
	c, err := h.readinessComp(r.Context(), cid)
	if err != nil {
		httpserver.WriteError(w, http.StatusNotFound, "Соревнование не найдено")
		return
	}
	u := auth.From(r)
	scope, id := "all", int64(0)
	switch u.ActiveRole {
	case rules.RoleAthlete:
		scope, id = "athlete", u.AthleteID
	case rules.RoleCoach:
		scope, id = "coach", u.CoachID
	}
	items, err := h.readinessRows(r.Context(), c, scope, id)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось посчитать готовность")
		return
	}
	ready := 0
	for _, it := range items {
		if it.Report.Ready {
			ready++
		}
	}
	if items == nil {
		items = []readinessRow{}
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{
		"competition": map[string]any{"id": c.ID, "title": c.Title, "starts_at": c.StartsAt.Format(time.RFC3339), "days_left": readiness.DaysUntil(time.Now(), c.StartsAt)},
		"total":       len(items), "ready": ready, "items": items,
	})
}

// ReadinessRemind — федерация вручную рассылает напоминания неготовым участникам выбранного соревнования.
func (h *Handler) ReadinessRemind(w http.ResponseWriter, r *http.Request) {
	cid, _ := parseID(r.URL.Query().Get("competition_id"))
	c, err := h.readinessComp(r.Context(), cid)
	if err != nil {
		httpserver.WriteError(w, http.StatusNotFound, "Соревнование не найдено")
		return
	}
	n := h.sendReadinessReminders(r.Context(), c, false)
	h.audit(r, "напоминания о готовности к старту", "competitions", c.Title)
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"sent": n})
}

// sendReadinessReminders уведомляет участников, у которых есть замечания. auto=true — только в окна 7/3/1 день и без повторов.
func (h *Handler) sendReadinessReminders(ctx context.Context, c readinessComp, auto bool) int {
	rows, err := h.readinessRows(ctx, c, "all", 0)
	if err != nil {
		log.Printf("readiness: %v", err)
		return 0
	}
	days := readiness.DaysUntil(time.Now(), c.StartsAt)
	bucket := readiness.Bucket(days)
	if auto && bucket == 0 {
		return 0
	}
	sent := 0
	for _, row := range rows {
		problems := row.Report.Problems()
		if len(problems) == 0 || row.UserID == 0 {
			continue
		}
		if auto {
			res, err := h.db.ExecContext(ctx, `INSERT INTO readiness_reminders(competition_id,athlete_id,bucket) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, c.ID, row.AthleteID, bucket)
			if err != nil {
				continue
			}
			if n, _ := res.RowsAffected(); n == 0 {
				continue
			}
		}
		msg := "До старта «" + c.Title + "» осталось дней: " + itoaInt(days) + ". Готовность " + itoaInt(row.Report.Score) + "%.\n• " + strings.Join(problems, "\n• ")
		h.notifyUser(ctx, row.UserID, "warning", "Готовность к старту", msg)
		sent++
	}
	return sent
}

// RunReadinessReminders раз в 6 часов проверяет ближайшие соревнования и шлёт напоминания за 7, 3 и 1 день.
func (h *Handler) RunReadinessReminders(ctx context.Context) {
	tick := time.NewTicker(6 * time.Hour)
	defer tick.Stop()
	for {
		h.remindUpcoming(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (h *Handler) remindUpcoming(ctx context.Context) {
	rows, err := h.db.QueryContext(ctx, `SELECT id FROM competitions WHERE starts_at > NOW() AND starts_at < NOW() + INTERVAL '8 days'`)
	if err != nil {
		return
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		if c, err := h.readinessComp(ctx, id); err == nil {
			h.sendReadinessReminders(ctx, c, true)
		}
	}
}
