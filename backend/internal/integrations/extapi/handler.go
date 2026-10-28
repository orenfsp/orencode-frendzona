package extapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"orenkod/backend/internal/auth"
)

type Middleware = func(http.HandlerFunc) http.HandlerFunc

// Mount регистрирует маршруты: администрирование (только «Федерация») и внешний API (Bearer-токен).
func (s *Service) Mount(mux *http.ServeMux, fed Middleware) {
	mux.HandleFunc("GET /api/integrations/console", servePage)
	mux.HandleFunc("GET /api/integrations/catalog", fed(s.catalog))
	mux.HandleFunc("GET /api/integrations/webhooks", fed(s.listWebhooks))
	mux.HandleFunc("POST /api/integrations/webhooks", fed(s.createWebhook))
	mux.HandleFunc("DELETE /api/integrations/webhooks/{id}", fed(s.deleteWebhook))
	mux.HandleFunc("POST /api/integrations/webhooks/{id}/toggle", fed(s.toggleWebhook))
	mux.HandleFunc("POST /api/integrations/webhooks/{id}/test", fed(s.testWebhook))
	mux.HandleFunc("GET /api/integrations/deliveries", fed(s.listDeliveries))
	mux.HandleFunc("POST /api/integrations/deliveries/{id}/redeliver", fed(s.redeliver))
	mux.HandleFunc("GET /api/integrations/tokens", fed(s.listTokens))
	mux.HandleFunc("POST /api/integrations/tokens", fed(s.createToken))
	mux.HandleFunc("DELETE /api/integrations/tokens/{id}", fed(s.revokeToken))

	mux.HandleFunc("GET /api/v1/openapi.yaml", serveSpec)
	mux.HandleFunc("GET /api/v1/competitions", s.extCompetitions)
	mux.HandleFunc("GET /api/v1/competitions/{id}/participants", s.extParticipants)
	mux.HandleFunc("GET /api/v1/competitions/{id}/protocol", s.extProtocol)
	mux.HandleFunc("POST /api/v1/competitions/{id}/results/import", s.extImport)
}

func (s *Service) audit(r *http.Request, action, details string) {
	u := auth.From(r)
	_, _ = s.db.ExecContext(r.Context(), `INSERT INTO audit_log(user_id,role,action,entity,details) VALUES($1,$2,$3,'integrations',$4)`, u.ID, u.ActiveRole, action, details)
}

func (s *Service) catalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"events": Events, "scopes": Scopes})
}

// ---------- вебхуки: администрирование ----------

func (s *Service) listWebhooks(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT e.id,e.name,e.url,e.events,e.active,e.created_at,
		COALESCE((SELECT COUNT(*) FROM webhook_deliveries d WHERE d.endpoint_id=e.id),0),
		COALESCE((SELECT COUNT(*) FROM webhook_deliveries d WHERE d.endpoint_id=e.id AND d.success),0)
		FROM webhook_endpoints e ORDER BY e.id`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось получить вебхуки")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var name, u, events string
		var active bool
		var created time.Time
		var total, ok int
		if rows.Scan(&id, &name, &u, &events, &active, &created, &total, &ok) != nil {
			continue
		}
		items = append(items, map[string]any{"id": id, "name": name, "url": u, "events": strings.Split(events, ","),
			"active": active, "created_at": created.Format("02.01.2006 15:04"), "deliveries": total, "delivered": ok})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Service) createWebhook(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name   string   `json:"name"`
		URL    string   `json:"url"`
		Events []string `json:"events"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Проверьте формат JSON")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		writeError(w, http.StatusBadRequest, "Укажите название подписки")
		return
	}
	if err := ValidateURL(in.URL); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	events := []string{}
	for _, e := range in.Events {
		if e == "*" {
			events = []string{"*"}
			break
		}
		if !validEvent(e) {
			writeError(w, http.StatusBadRequest, "Неизвестное событие: "+e)
			return
		}
		events = append(events, e)
	}
	if len(events) == 0 {
		events = []string{"*"}
	}
	secret := "whsec_" + randHex(24)
	var id int64
	err := s.db.QueryRowContext(r.Context(), `INSERT INTO webhook_endpoints(name,url,secret,events,created_by) VALUES($1,$2,$3,$4,$5) RETURNING id`,
		in.Name, strings.TrimSpace(in.URL), secret, strings.Join(events, ","), auth.From(r).ID).Scan(&id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось сохранить вебхук")
		return
	}
	s.audit(r, "создание вебхука", in.Name)
	// Секрет показывается один раз.
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "name": in.Name, "url": in.URL, "events": events, "secret": secret})
}

func (s *Service) deleteWebhook(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if _, err := s.db.ExecContext(r.Context(), `DELETE FROM webhook_endpoints WHERE id=$1`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось удалить")
		return
	}
	s.audit(r, "удаление вебхука", strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Service) toggleWebhook(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var active bool
	if err := s.db.QueryRowContext(r.Context(), `UPDATE webhook_endpoints SET active=NOT active WHERE id=$1 RETURNING active`, id).Scan(&active); err != nil {
		writeError(w, http.StatusNotFound, "Вебхук не найден")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "active": active})
}

func (s *Service) testWebhook(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	ok, status, msg, err := s.SendNow(r.Context(), id, "webhook.test", map[string]any{"message": "Проверка соединения с платформой ФСП Оренбургской области"})
	if err != nil {
		writeError(w, http.StatusNotFound, "Вебхук не найден")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": ok, "status_code": status, "error": msg})
}

func (s *Service) listDeliveries(w http.ResponseWriter, r *http.Request) {
	q := `SELECT d.id,d.endpoint_id,e.name,d.event,d.status_code,d.success,d.attempts,d.error,d.created_at FROM webhook_deliveries d JOIN webhook_endpoints e ON e.id=d.endpoint_id`
	args := []any{}
	if v := r.URL.Query().Get("endpoint_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			q += ` WHERE d.endpoint_id=$1`
			args = append(args, id)
		}
	}
	rows, err := s.db.QueryContext(r.Context(), q+` ORDER BY d.id DESC LIMIT 50`, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось получить журнал")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, eid int64
		var name, event, msg string
		var status, attempts int
		var ok bool
		var created time.Time
		if rows.Scan(&id, &eid, &name, &event, &status, &ok, &attempts, &msg, &created) != nil {
			continue
		}
		items = append(items, map[string]any{"id": id, "endpoint_id": eid, "endpoint": name, "event": event, "status_code": status,
			"success": ok, "attempts": attempts, "error": msg, "created_at": created.Format("02.01.2006 15:04:05")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Service) redeliver(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var eid int64
	var event, payload string
	if err := s.db.QueryRowContext(r.Context(), `SELECT endpoint_id,event,payload FROM webhook_deliveries WHERE id=$1`, id).Scan(&eid, &event, &payload); err != nil {
		writeError(w, http.StatusNotFound, "Доставка не найдена")
		return
	}
	var env struct {
		Data map[string]any `json:"data"`
	}
	_ = json.Unmarshal([]byte(payload), &env)
	ok, status, msg, err := s.SendNow(r.Context(), eid, event, env.Data)
	if err != nil {
		writeError(w, http.StatusNotFound, "Вебхук не найден")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": ok, "status_code": status, "error": msg})
}

// ---------- токены: администрирование ----------

func (s *Service) listTokens(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT id,name,prefix,scopes,created_at,last_used_at,revoked FROM api_tokens ORDER BY id DESC`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось получить токены")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var name, prefix, scopes string
		var created time.Time
		var last *time.Time
		var revoked bool
		if rows.Scan(&id, &name, &prefix, &scopes, &created, &last, &revoked) != nil {
			continue
		}
		lastUsed := ""
		if last != nil {
			lastUsed = last.Format("02.01.2006 15:04")
		}
		items = append(items, map[string]any{"id": id, "name": name, "prefix": prefix, "scopes": strings.Fields(scopes),
			"created_at": created.Format("02.01.2006 15:04"), "last_used_at": lastUsed, "revoked": revoked})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Service) createToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Проверьте формат JSON")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		writeError(w, http.StatusBadRequest, "Укажите название системы-потребителя")
		return
	}
	if len(in.Scopes) == 0 {
		writeError(w, http.StatusBadRequest, "Выберите хотя бы одно право доступа")
		return
	}
	for _, sc := range in.Scopes {
		if !validScope(sc) {
			writeError(w, http.StatusBadRequest, "Неизвестное право: "+sc)
			return
		}
	}
	token := "fspt_" + randHex(24)
	var id int64
	err := s.db.QueryRowContext(r.Context(), `INSERT INTO api_tokens(name,token_hash,prefix,scopes,created_by) VALUES($1,$2,$3,$4,$5) RETURNING id`,
		in.Name, hashToken(token), token[:9], strings.Join(in.Scopes, " "), auth.From(r).ID).Scan(&id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось выдать токен")
		return
	}
	s.audit(r, "выдача API-токена", in.Name+": "+strings.Join(in.Scopes, " "))
	// Сам токен хранится только в виде хеша и показывается один раз.
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "name": in.Name, "scopes": in.Scopes, "token": token})
}

func (s *Service) revokeToken(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if _, err := s.db.ExecContext(r.Context(), `UPDATE api_tokens SET revoked=TRUE WHERE id=$1`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось отозвать токен")
		return
	}
	s.audit(r, "отзыв API-токена", strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---------- внешний API ----------

// guard проверяет токен и право; при ошибке сам пишет ответ 401/403.
func (s *Service) guard(w http.ResponseWriter, r *http.Request, scope string) bool {
	_, err := s.Authenticate(r, scope)
	if err == nil {
		return true
	}
	if err == errToken {
		w.Header().Set("WWW-Authenticate", `Bearer realm="fsp"`)
		writeError(w, http.StatusUnauthorized, "Нужен действующий API-токен: Authorization: Bearer <token>")
		return false
	}
	writeError(w, http.StatusForbidden, "У токена нет права "+scope)
	return false
}

func (s *Service) extCompetitions(w http.ResponseWriter, r *http.Request) {
	if !s.guard(w, r, "competitions:read") {
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT id,title,discipline,competition_type,age_group,starts_at,location,status,participants,
		sport_status,event_format,calendar_plan,calendar_number FROM competitions WHERE published_at IS NOT NULL OR status<>'Черновик' ORDER BY starts_at`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось получить календарь")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var title, disc, kind, age, loc, status, ss, format, plan, num string
		var starts time.Time
		var parts int
		if rows.Scan(&id, &title, &disc, &kind, &age, &starts, &loc, &status, &parts, &ss, &format, &plan, &num) != nil {
			continue
		}
		items = append(items, map[string]any{"id": id, "title": title, "discipline": disc, "competition_type": kind, "age_group": age,
			"starts_at": starts.Format(time.RFC3339), "location": loc, "status": status, "participants": parts,
			"sport_status": ss, "event_format": format, "calendar_plan": plan, "calendar_number": num})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Service) extParticipants(w http.ResponseWriter, r *http.Request) {
	if !s.guard(w, r, "participants:read") {
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	rows, err := s.db.QueryContext(r.Context(), `SELECT a.id,a.fsp_id,a.full_name,a.organization,a.discipline,a.age_group
		FROM applications ap JOIN athletes a ON a.id=ap.athlete_id WHERE ap.competition_id=$1 AND ap.status='approved' ORDER BY a.full_name`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось получить участников")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var aid int64
		var fsp, name, org, disc, age string
		if rows.Scan(&aid, &fsp, &name, &org, &disc, &age) != nil {
			continue
		}
		items = append(items, map[string]any{"athlete_id": aid, "fsp_id": fsp, "full_name": name, "organization": org, "discipline": disc, "age_group": age})
	}
	writeJSON(w, http.StatusOK, map[string]any{"competition_id": id, "items": items})
}

func (s *Service) extProtocol(w http.ResponseWriter, r *http.Request) {
	if !s.guard(w, r, "protocols:read") {
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	items, err := s.protocol(r, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось получить протокол")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"competition_id": id, "items": items})
}

func (s *Service) protocol(r *http.Request, id int64) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT a.id,a.fsp_id,a.full_name,a.organization,a.age_group,res.place,res.score,res.points
		FROM results res JOIN athletes a ON a.id=res.athlete_id WHERE res.competition_id=$1 ORDER BY res.place,a.full_name`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var aid int64
		var fsp, name, org, age string
		var place, score, points int
		if rows.Scan(&aid, &fsp, &name, &org, &age, &place, &score, &points) != nil {
			continue
		}
		items = append(items, map[string]any{"athlete_id": aid, "fsp_id": fsp, "full_name": name, "organization": org, "age_group": age,
			"place": place, "score": score, "rating_points": points})
	}
	return items, rows.Err()
}

// extImport принимает результаты из внешней соревновательной платформы и формирует протокол.
func (s *Service) extImport(w http.ResponseWriter, r *http.Request) {
	if !s.guard(w, r, "results:write") {
		return
	}
	compID, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var in struct {
		Items []struct {
			AthleteID int64  `json:"athlete_id"`
			FspID     string `json:"fsp_id"`
			Place     int    `json:"place"`
			Score     int    `json:"score"`
		} `json:"items"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Проверьте формат JSON")
		return
	}
	if len(in.Items) == 0 || len(in.Items) > 5000 {
		writeError(w, http.StatusBadRequest, "Передайте от 1 до 5000 результатов в поле items")
		return
	}
	var title string
	if err := s.db.QueryRowContext(r.Context(), `SELECT title FROM competitions WHERE id=$1`, compID).Scan(&title); err != nil {
		writeError(w, http.StatusNotFound, "Соревнование не найдено")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось начать импорт")
		return
	}
	defer tx.Rollback()
	imported := 0
	for i, it := range in.Items {
		if it.Place <= 0 {
			writeError(w, http.StatusUnprocessableEntity, "Строка "+strconv.Itoa(i+1)+": место должно быть больше нуля")
			return
		}
		aid := it.AthleteID
		if aid == 0 && strings.TrimSpace(it.FspID) != "" {
			_ = tx.QueryRowContext(r.Context(), `SELECT id FROM athletes WHERE fsp_id=$1`, strings.TrimSpace(it.FspID)).Scan(&aid)
		}
		if aid == 0 {
			writeError(w, http.StatusUnprocessableEntity, "Строка "+strconv.Itoa(i+1)+": спортсмен не найден (укажите athlete_id или fsp_id)")
			return
		}
		var approved int
		_ = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM applications WHERE competition_id=$1 AND athlete_id=$2 AND status='approved'`, compID, aid).Scan(&approved)
		if approved == 0 {
			writeError(w, http.StatusUnprocessableEntity, "Строка "+strconv.Itoa(i+1)+": у спортсмена "+strconv.FormatInt(aid, 10)+" нет одобренной заявки на это соревнование")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO results(athlete_id,competition_id,place,points,score,status) VALUES($1,$2,$3,0,$4,'финишировал')
			ON CONFLICT (athlete_id,competition_id) DO UPDATE SET place=EXCLUDED.place, score=EXCLUDED.score`, aid, compID, it.Place, it.Score); err != nil {
			writeError(w, http.StatusInternalServerError, "Не удалось записать результат")
			return
		}
		imported++
	}
	_, _ = tx.ExecContext(r.Context(), `UPDATE competitions SET status='Завершено' WHERE id=$1`, compID)
	_, _ = tx.ExecContext(r.Context(), `INSERT INTO inbox(role_scope,kind,title,message) VALUES('athlete','success','Протокол опубликован',$1)`, "Результаты соревнования «"+title+"» импортированы из внешней системы.")
	_, _ = tx.ExecContext(r.Context(), `INSERT INTO audit_log(user_id,role,action,entity,details) VALUES(NULL,'api','импорт результатов (внешний API)','results',$1)`, strconv.FormatInt(compID, 10))
	if err = tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось сохранить протокол")
		return
	}
	if s.Recalc != nil {
		s.Recalc(r.Context())
	}
	s.Emit("protocol.published", map[string]any{"competition_id": compID, "title": title, "source": "external_import", "results": imported})
	protocol, _ := s.protocol(r, compID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "imported": imported, "protocol": protocol})
}

func serveSpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	_, _ = w.Write([]byte(openAPISpec))
}

// Собственные помощники: пакет подключается из httpserver, поэтому импортировать httpserver нельзя.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
