package platform

import (
	"net/http"
	"strconv"

	"orenkod/backend/internal/auth"
	"orenkod/backend/internal/httpserver"
	"orenkod/backend/internal/rules"
)

// Participants returns athletes with approved applications for a competition together with their saved results.
// A judge may read the list only for competitions they are assigned to.
func (h *Handler) Participants(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	user := auth.From(r)
	if user.ActiveRole == rules.RoleJudge {
		var assigned int
		if err := h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM judge_assignments WHERE competition_id=$1 AND judge_id=$2`, id, user.JudgeID).Scan(&assigned); err != nil || assigned == 0 {
			httpserver.WriteError(w, http.StatusForbidden, "Список участников доступен только назначенному на соревнование судье")
			return
		}
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT ath.id, ath.full_name, ath.organization, COALESCE(r.place,0), COALESCE(r.score,0)
		FROM applications a JOIN athletes ath ON ath.id=a.athlete_id
		LEFT JOIN results r ON r.athlete_id=a.athlete_id AND r.competition_id=a.competition_id
		WHERE a.competition_id=$1 AND a.status='approved'
		GROUP BY ath.id, ath.full_name, ath.organization, r.place, r.score
		ORDER BY COALESCE(r.place,0)=0, COALESCE(r.place,0), ath.full_name`, id)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось получить участников")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var athleteID int64
		var name, org string
		var place, score int
		if err := rows.Scan(&athleteID, &name, &org, &place, &score); err != nil {
			continue
		}
		items = append(items, map[string]any{"athlete_id": athleteID, "full_name": name, "organization": org, "place": place, "score": score})
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}
