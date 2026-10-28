package platform

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"orenkod/backend/internal/httpserver"
)

// Д2. Интеграции с внешними системами: токены доступа с ограничением прав,
// вебхуки о событиях, импорт результатов из внешней системы.

var integrationScopes = []string{"competitions:read", "rating:read", "results:write"}
var webhookEvents = []string{"competition.published", "application.approved", "protocol.published"}

func (h *Handler) apiTokenOK(r *http.Request, scope string) bool {
	tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if tok == "" {
		return false
	}
	var id int64
	var scopes string
	if h.db.QueryRowContext(r.Context(), `SELECT id,scopes FROM api_tokens WHERE token_hash=$1`, sha(tok)).Scan(&id, &scopes) != nil {
		return false
	}
	if !strings.Contains(" "+scopes+" ", " "+scope+" ") {
		return false
	}
	_, _ = h.db.ExecContext(r.Context(), `UPDATE api_tokens SET last_used_at=NOW() WHERE id=$1`, id)
	return true
}

func extDenied(w http.ResponseWriter, scope string) {
	httpserver.WriteError(w, http.StatusUnauthorized, "Нужен API-токен с правом "+scope)
}

// ---- управление (федерация) ----

func (h *Handler) TokenCreate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.Name == "" || len(in.Scopes) == 0 {
		httpserver.WriteError(w, http.StatusBadRequest, "Нужны название и права")
		return
	}
	for _, s := range in.Scopes {
		ok := false
		for _, a := range integrationScopes {
			ok = ok || a == s
		}
		if !ok {
			httpserver.WriteError(w, http.StatusBadRequest, "Неизвестное право: "+s)
			return
		}
	}
	tok := "fsp_" + randHex(24)
	if _, err := h.db.ExecContext(r.Context(), `INSERT INTO api_tokens(name,token_hash,scopes) VALUES($1,$2,$3)`, in.Name, sha(tok), strings.Join(in.Scopes, " ")); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось выдать токен")
		return
	}
	h.audit(r, "выдан API-токен", "api_tokens", in.Name)
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"token": tok, "scopes": in.Scopes, "note": "токен показан один раз"})
}

func (h *Handler) IntegrationsList(w http.ResponseWriter, r *http.Request) {
	tokens, hooks, logs := []map[string]any{}, []map[string]any{}, []map[string]any{}
	if rows, err := h.db.QueryContext(r.Context(), `SELECT id,name,scopes,COALESCE(last_used_at::text,'—') FROM api_tokens ORDER BY id`); err == nil {
		for rows.Next() {
			var id int64
			var n, s, l string
			if rows.Scan(&id, &n, &s, &l) == nil {
				tokens = append(tokens, map[string]any{"id": id, "name": n, "scopes": s, "last_used": l})
			}
		}
		rows.Close()
	}
	if rows, err := h.db.QueryContext(r.Context(), `SELECT id,url,events,active FROM webhooks ORDER BY id`); err == nil {
		for rows.Next() {
			var id int64
			var u, e string
			var a bool
			if rows.Scan(&id, &u, &e, &a) == nil {
				hooks = append(hooks, map[string]any{"id": id, "url": u, "events": e, "active": a})
			}
		}
		rows.Close()
	}
	if rows, err := h.db.QueryContext(r.Context(), `SELECT webhook_id,event,status_code,error,created_at::text FROM webhook_log ORDER BY id DESC LIMIT 15`); err == nil {
		for rows.Next() {
			var id int64
			var ev, er, at string
			var code int
			if rows.Scan(&id, &ev, &code, &er, &at) == nil {
				logs = append(logs, map[string]any{"webhook_id": id, "event": ev, "status": code, "error": er, "at": at})
			}
		}
		rows.Close()
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"tokens": tokens, "webhooks": hooks, "log": logs, "scopes": integrationScopes, "events": webhookEvents})
}

func (h *Handler) WebhookCreate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		URL    string   `json:"url"`
		Events []string `json:"events"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || !(strings.HasPrefix(in.URL, "http://") || strings.HasPrefix(in.URL, "https://")) || len(in.Events) == 0 {
		httpserver.WriteError(w, http.StatusBadRequest, "Нужны URL (http/https) и события")
		return
	}
	secret := randHex(16)
	if _, err := h.db.ExecContext(r.Context(), `INSERT INTO webhooks(url,events,secret) VALUES($1,$2,$3)`, in.URL, strings.Join(in.Events, " "), secret); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось создать вебхук")
		return
	}
	h.audit(r, "создан вебхук", "webhooks", in.URL)
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"secret": secret, "note": "подпись: заголовок X-FSP-Signature = hex(HMAC-SHA256(secret, тело))"})
}

// emit отправляет событие всем подходящим вебхукам (асинхронно, с подписью и журналом).
func (h *Handler) emit(event string, data map[string]any) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rows, err := h.db.QueryContext(ctx, `SELECT id,url,secret FROM webhooks WHERE active AND (' '||events||' ') LIKE '% '||$1||' %'`, event)
		if err != nil {
			return
		}
		type hook struct {
			id        int64
			url, sect string
		}
		var hooks []hook
		for rows.Next() {
			var x hook
			if rows.Scan(&x.id, &x.url, &x.sect) == nil {
				hooks = append(hooks, x)
			}
		}
		rows.Close()
		body, _ := json.Marshal(map[string]any{"event": event, "data": data, "sent_at": time.Now().UTC().Format(time.RFC3339)})
		for _, x := range hooks {
			mac := hmac.New(sha256.New, []byte(x.sect))
			mac.Write(body)
			req, _ := http.NewRequestWithContext(ctx, http.MethodPost, x.url, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-FSP-Event", event)
			req.Header.Set("X-FSP-Signature", hex.EncodeToString(mac.Sum(nil)))
			code, msg := 0, ""
			resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
			if err != nil {
				msg = err.Error()
			} else {
				code = resp.StatusCode
				resp.Body.Close()
			}
			if _, e := h.db.ExecContext(ctx, `INSERT INTO webhook_log(webhook_id,event,status_code,error) VALUES($1,$2,$3,$4)`, x.id, event, code, msg); e != nil {
				log.Printf("webhook log: %v", e)
			}
		}
	}()
}

// ---- внешний API (по токену) ----

func (h *Handler) ExtCompetitions(w http.ResponseWriter, r *http.Request) {
	if !h.apiTokenOK(r, "competitions:read") {
		extDenied(w, "competitions:read")
		return
	}
	items, err := h.comp.List(r.Context())
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Ошибка")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) ExtRating(w http.ResponseWriter, r *http.Request) {
	if !h.apiTokenOK(r, "rating:read") {
		extDenied(w, "rating:read")
		return
	}
	items, err := h.buildRating(r, r.URL.Query().Get("discipline"), r.URL.Query().Get("age_group"))
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Ошибка")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ExtImportResults — импорт результатов из внешней соревновательной платформы.
// Тело: {"items":[{"fsp_id":"...", "athlete_id":0, "place":1, "score":100}]}.
// В протокол попадают только участники с одобренными заявками.
func (h *Handler) ExtImportResults(w http.ResponseWriter, r *http.Request) {
	if !h.apiTokenOK(r, "results:write") {
		extDenied(w, "results:write")
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var in struct {
		Items []struct {
			FspID     string `json:"fsp_id"`
			AthleteID int64  `json:"athlete_id"`
			Place     int    `json:"place"`
			Score     int    `json:"score"`
		} `json:"items"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.Items) == 0 {
		httpserver.WriteError(w, http.StatusBadRequest, "Нужен непустой список items")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Ошибка")
		return
	}
	defer tx.Rollback()
	imported, skipped := 0, []string{}
	for _, it := range in.Items {
		aid := it.AthleteID
		if aid == 0 && it.FspID != "" {
			_ = tx.QueryRowContext(r.Context(), `SELECT id FROM athletes WHERE fsp_id=$1`, it.FspID).Scan(&aid)
		}
		var ok int
		if aid != 0 {
			_ = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM applications WHERE competition_id=$1 AND athlete_id=$2 AND status='approved'`, id, aid).Scan(&ok)
		}
		if ok == 0 || it.Place < 1 {
			skipped = append(skipped, it.FspID+"#"+strconv.FormatInt(it.AthleteID, 10))
			continue
		}
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO results(athlete_id,competition_id,place,points,score,status) VALUES($1,$2,$3,0,$4,'финишировал')
			ON CONFLICT (athlete_id,competition_id) DO UPDATE SET place=EXCLUDED.place, score=EXCLUDED.score`, aid, id, it.Place, it.Score); err != nil {
			httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось записать результат")
			return
		}
		imported++
	}
	if imported > 0 {
		_, _ = tx.ExecContext(r.Context(), `UPDATE competitions SET status='Завершено' WHERE id=$1`, id)
	}
	if tx.Commit() != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить протокол")
		return
	}
	if imported > 0 {
		h.recalculatePoints(r)
		h.emit("protocol.published", map[string]any{"competition_id": id, "source": "import"})
	}
	_, _ = h.db.ExecContext(r.Context(), `INSERT INTO audit_log(role,action,entity,details) VALUES('integration','импорт результатов','results',$1)`, strconv.FormatInt(id, 10))
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"imported": imported, "skipped": skipped})
}
