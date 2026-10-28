package platform

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"orenkod/backend/internal/exportutil"
	"orenkod/backend/internal/httpserver"
)

// Premiums — автоматический отбор претендентов на премию Губернатора по протоколам за год.
// ?year=2026 (по умолчанию текущий), ?format=xlsx — выгрузка для пакета ходатайства.
func (h *Handler) Premiums(w http.ResponseWriter, r *http.Request) {
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	if year == 0 {
		year = time.Now().Year()
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT a.id,a.full_name,a.organization,c.title,c.sport_status,r.place,s.amount
		FROM results r JOIN athletes a ON a.id=r.athlete_id JOIN competitions c ON c.id=r.competition_id
		JOIN prize_scale s ON s.sport_status=c.sport_status AND s.place=r.place
		WHERE r.place BETWEEN 1 AND 3 AND EXTRACT(YEAR FROM c.starts_at)=$1 ORDER BY a.full_name, r.place`, year)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сформировать список")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	table := [][]string{{"Спортсмен", "Организация", "Соревнование", "Статус", "Место", "Сумма, руб."}}
	total := 0
	for rows.Next() {
		var id int64
		var name, org, title, status string
		var place, amount int
		if rows.Scan(&id, &name, &org, &title, &status, &place, &amount) != nil {
			continue
		}
		total += amount
		items = append(items, map[string]any{"athlete_id": id, "athlete": name, "organization": org, "competition": title, "sport_status": status, "place": place, "amount": amount})
		table = append(table, []string{name, org, title, status, strconv.Itoa(place), strconv.Itoa(amount)})
	}
	if r.URL.Query().Get("format") == "xlsx" {
		table = append(table, []string{"ИТОГО", "", "", "", "", strconv.Itoa(total)})
		b, err := exportutil.XLSX("Премии", table)
		if err != nil {
			httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось выгрузить")
			return
		}
		writeFile(w, "premiums-"+strconv.Itoa(year)+".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", b)
		return
	}
	status := "подготовлено"
	_ = h.db.QueryRowContext(r.Context(), `SELECT status FROM prize_petitions WHERE year=$1`, year).Scan(&status)
	deadline := time.Date(year+1, time.February, 1, 0, 0, 0, 0, time.UTC)
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"year": year, "items": items, "total": total, "status": status,
		"deadline": deadline.Format("2006-01-02"), "days_left": int(time.Until(deadline).Hours() / 24)})
}

// PremiumStatus — статус ходатайства: подготовлено → направлено → премия назначена.
func (h *Handler) PremiumStatus(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Year   int    `json:"year"`
		Status string `json:"status"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.Year == 0 || (in.Status != "подготовлено" && in.Status != "направлено" && in.Status != "премия назначена") {
		httpserver.WriteError(w, http.StatusBadRequest, "Некорректный год или статус")
		return
	}
	_, err := h.db.ExecContext(r.Context(), `INSERT INTO prize_petitions(year,status) VALUES($1,$2) ON CONFLICT (year) DO UPDATE SET status=EXCLUDED.status, updated_at=NOW()`, in.Year, in.Status)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить статус")
		return
	}
	h.audit(r, "ходатайство на премию: "+in.Status, "prize_petitions", strconv.Itoa(in.Year))
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// PremiumScale — федерация меняет размер премии.
func (h *Handler) PremiumScale(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SportStatus string `json:"sport_status"`
		Place       int    `json:"place"`
		Amount      int    `json:"amount"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.Place < 1 || in.Place > 3 || in.Amount < 0 || in.SportStatus == "" {
		httpserver.WriteError(w, http.StatusBadRequest, "Нужны статус, место 1–3 и сумма")
		return
	}
	_, err := h.db.ExecContext(r.Context(), `INSERT INTO prize_scale(sport_status,place,amount) VALUES($1,$2,$3) ON CONFLICT (sport_status,place) DO UPDATE SET amount=EXCLUDED.amount`, in.SportStatus, in.Place, in.Amount)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить")
		return
	}
	h.audit(r, "изменение размера премии", "prize_scale", in.SportStatus)
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}
