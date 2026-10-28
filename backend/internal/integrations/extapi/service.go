// Package extapi — интеграции с внешними системами (Д2): API-токены с ограничением прав,
// подписанные вебхуки о событиях платформы (с повторами и журналом доставок),
// внешний API и импорт результатов соревнований.
package extapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// События платформы, на которые можно подписаться.
var Events = []Event{
	{"competition.published", "Соревнование опубликовано"},
	{"application.approved", "Заявка на соревнование одобрена"},
	{"application.rejected", "Заявка на соревнование отклонена"},
	{"protocol.published", "Опубликован протокол соревнования"},
	{"rank.decided", "Принято решение по заявке на разряд"},
	{"category.decided", "Принято решение по заявке на судейскую категорию"},
	{"webhook.test", "Тестовое событие"},
}

// Права (scope) API-токенов.
var Scopes = []Event{
	{"competitions:read", "Чтение календаря соревнований"},
	{"participants:read", "Чтение списка допущенных участников"},
	{"protocols:read", "Чтение протоколов"},
	{"results:write", "Импорт результатов соревнования"},
}

type Event struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

type Service struct {
	db     *sql.DB
	client *http.Client
	// Recalc пересчитывает рейтинговые баллы после импорта результатов.
	Recalc func(ctx context.Context)
	// Delays — паузы перед 2-й и 3-й попыткой доставки (переопределяются в тестах).
	Delays []time.Duration
}

func New(db *sql.DB) *Service {
	return &Service{
		db: db,
		client: &http.Client{
			Timeout: 8 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		Delays: []time.Duration{2 * time.Second, 10 * time.Second},
	}
}

// ---------- подпись и валидация ----------

// Sign возвращает значение заголовка X-FSP-Signature: sha256=HMAC(secret, timestamp + "." + body).
func Sign(secret, timestamp string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(timestamp))
	m.Write([]byte("."))
	m.Write(body)
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

func hashToken(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func validEvent(code string) bool {
	for _, e := range Events {
		if e.Code == code {
			return true
		}
	}
	return false
}

func validScope(code string) bool {
	for _, e := range Scopes {
		if e.Code == code {
			return true
		}
	}
	return false
}

// ValidateURL разрешает только http(s) и запрещает адреса метаданных облака.
func ValidateURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return errors.New("Некорректный URL получателя")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("URL должен начинаться с http:// или https://")
	}
	if strings.HasPrefix(u.Hostname(), "169.254.") {
		return errors.New("Этот адрес запрещён")
	}
	return nil
}

// ---------- доставка вебхуков ----------

type endpoint struct {
	ID     int64
	URL    string
	Secret string
	Events string
}

func (e endpoint) wants(event string) bool {
	for _, p := range strings.Split(e.Events, ",") {
		p = strings.TrimSpace(p)
		if p == "*" || p == event {
			return true
		}
	}
	return false
}

// Emit рассылает событие всем активным подписчикам. Не блокирует вызывающий код.
func (s *Service) Emit(event string, data map[string]any) {
	if s == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		rows, err := s.db.QueryContext(ctx, `SELECT id,url,secret,events FROM webhook_endpoints WHERE active=TRUE`)
		if err != nil {
			cancel()
			log.Printf("webhooks: список подписчиков: %v", err)
			return
		}
		var targets []endpoint
		for rows.Next() {
			var e endpoint
			if rows.Scan(&e.ID, &e.URL, &e.Secret, &e.Events) == nil && e.wants(event) {
				targets = append(targets, e)
			}
		}
		rows.Close()
		cancel()
		for _, e := range targets {
			id, payload, err := s.queue(e.ID, event, data)
			if err != nil {
				log.Printf("webhooks: очередь: %v", err)
				continue
			}
			go s.deliverWithRetry(e, id, event, payload)
		}
	}()
}

func (s *Service) queue(endpointID int64, event string, data map[string]any) (int64, []byte, error) {
	var id int64
	payload := []byte("{}")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.db.QueryRowContext(ctx, `INSERT INTO webhook_deliveries(endpoint_id,event,payload) VALUES($1,$2,'{}') RETURNING id`, endpointID, event).Scan(&id); err != nil {
		return 0, nil, err
	}
	payload, err := json.Marshal(map[string]any{
		"id":         "evt_" + strconv.FormatInt(id, 10),
		"event":      event,
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"data":       data,
	})
	if err != nil {
		return 0, nil, err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE webhook_deliveries SET payload=$1 WHERE id=$2`, string(payload), id)
	return id, payload, err
}

func (s *Service) deliverWithRetry(e endpoint, deliveryID int64, event string, payload []byte) {
	attempts := 1 + len(s.Delays)
	for i := 0; i < attempts; i++ {
		if i > 0 {
			time.Sleep(s.Delays[i-1])
		}
		if ok := s.attempt(e, deliveryID, event, payload, i+1); ok {
			return
		}
	}
}

// attempt выполняет одну попытку доставки и записывает результат в журнал.
func (s *Service) attempt(e endpoint, deliveryID int64, event string, payload []byte, n int) bool {
	status, err := s.post(e, deliveryID, event, payload)
	ok := err == nil && status >= 200 && status < 300
	msg := ""
	if err != nil {
		msg = err.Error()
	} else if !ok {
		msg = "Получатель ответил статусом " + strconv.Itoa(status)
	}
	var delivered any
	if ok {
		delivered = time.Now()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, derr := s.db.ExecContext(ctx, `UPDATE webhook_deliveries SET status_code=$1, success=$2, attempts=$3, error=$4, delivered_at=$5 WHERE id=$6`,
		status, ok, n, msg, delivered, deliveryID); derr != nil {
		log.Printf("webhooks: журнал доставки: %v", derr)
	}
	return ok
}

func (s *Service) post(e endpoint, deliveryID int64, event string, payload []byte) (int, error) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	req, err := http.NewRequest(http.MethodPost, e.URL, bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "FSP-Orenburg-Webhook/1.0")
	req.Header.Set("X-FSP-Event", event)
	req.Header.Set("X-FSP-Delivery", strconv.FormatInt(deliveryID, 10))
	req.Header.Set("X-FSP-Timestamp", ts)
	req.Header.Set("X-FSP-Signature", Sign(e.Secret, ts, payload))
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	return resp.StatusCode, nil
}

// SendNow доставляет событие одному подписчику синхронно (проверка соединения, повторная отправка).
func (s *Service) SendNow(ctx context.Context, endpointID int64, event string, data map[string]any) (bool, int, string, error) {
	var e endpoint
	if err := s.db.QueryRowContext(ctx, `SELECT id,url,secret,events FROM webhook_endpoints WHERE id=$1`, endpointID).Scan(&e.ID, &e.URL, &e.Secret, &e.Events); err != nil {
		return false, 0, "", err
	}
	id, payload, err := s.queue(e.ID, event, data)
	if err != nil {
		return false, 0, "", err
	}
	ok := s.attempt(e, id, event, payload, 1)
	var status int
	var msg string
	_ = s.db.QueryRowContext(ctx, `SELECT status_code,error FROM webhook_deliveries WHERE id=$1`, id).Scan(&status, &msg)
	return ok, status, msg, nil
}

// ---------- токены ----------

type TokenInfo struct {
	ID       int64
	Name     string
	Scopes   []string
	LastUsed sql.NullTime
}

var errToken = fmt.Errorf("invalid token")

// Authenticate проверяет Bearer-токен и наличие нужного права.
func (s *Service) Authenticate(r *http.Request, scope string) (TokenInfo, error) {
	h := r.Header.Get("Authorization")
	if len(h) < 8 || !strings.EqualFold(h[:7], "bearer ") {
		return TokenInfo{}, errToken
	}
	raw := strings.TrimSpace(h[7:])
	var t TokenInfo
	var scopes string
	err := s.db.QueryRowContext(r.Context(), `SELECT id,name,scopes FROM api_tokens WHERE token_hash=$1 AND revoked=FALSE`, hashToken(raw)).Scan(&t.ID, &t.Name, &scopes)
	if err != nil {
		return TokenInfo{}, errToken
	}
	t.Scopes = strings.Fields(strings.ReplaceAll(scopes, ",", " "))
	for _, sc := range t.Scopes {
		if sc == scope {
			_, _ = s.db.ExecContext(r.Context(), `UPDATE api_tokens SET last_used_at=NOW() WHERE id=$1`, t.ID)
			return t, nil
		}
	}
	return t, errors.New("forbidden")
}
