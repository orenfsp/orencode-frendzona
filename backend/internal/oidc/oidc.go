// Package oidc реализует «ФСП ID»: платформа выступает провайдером OpenID Connect
// (authorization code + PKCE S256, discovery, JWKS, userinfo, id_token RS256).
package oidc

import (
	"crypto"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"orenkod/backend/internal/auth"
)

const (
	codeTTL        = 5 * time.Minute
	accessTokenTTL = time.Hour
	DemoClientID   = "fsp-demo-client"
)

// Scopes и их описание для страницы согласия.
var scopeLabels = map[string]string{
	"openid":  "Идентификатор аккаунта ФСП ID",
	"profile": "ФИО",
	"email":   "Адрес электронной почты",
	"roles":   "Роли в платформе (спортсмен, тренер, судья, федерация)",
	"sport":   "Дисциплины, спортивный разряд и номер ФСП ID",
}

var scopeOrder = []string{"openid", "profile", "email", "roles", "sport"}

type Middleware = func(http.HandlerFunc) http.HandlerFunc

type Provider struct {
	db     *sql.DB
	key    *rsa.PrivateKey
	kid    string
	issuer string // необязательно: OIDC_ISSUER, иначе определяется по запросу
}

// New загружает (или создаёт) ключ подписи и регистрирует демонстрационного клиента.
func New(ctx context.Context, db *sql.DB, issuer string) (*Provider, error) {
	p := &Provider{db: db, issuer: strings.TrimRight(strings.TrimSpace(issuer), "/")}
	if err := p.loadKey(ctx); err != nil {
		return nil, err
	}
	if err := p.seedDemoClient(ctx); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Provider) loadKey(ctx context.Context) error {
	var kid, pemText string
	err := p.db.QueryRowContext(ctx, `SELECT kid, private_pem FROM oidc_keys ORDER BY created_at DESC LIMIT 1`).Scan(&kid, &pemText)
	if err == nil {
		block, _ := pem.Decode([]byte(pemText))
		if block != nil {
			if key, perr := x509.ParsePKCS1PrivateKey(block.Bytes); perr == nil {
				p.key, p.kid = key, kid
				return nil
			}
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	kid = "fsp-" + randHex(6)
	pemText = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	if _, err := p.db.ExecContext(ctx, `INSERT INTO oidc_keys(kid, private_pem) VALUES($1,$2)`, kid, pemText); err != nil {
		return err
	}
	p.key, p.kid = key, kid
	return nil
}

func (p *Provider) seedDemoClient(ctx context.Context) error {
	uris := "http://localhost:8080/oidc/demo\nhttp://localhost:8081/oidc/demo\nhttp://127.0.0.1:8080/oidc/demo"
	_, err := p.db.ExecContext(ctx, `INSERT INTO oidc_clients(client_id,name,secret_hash,redirect_uris)
		VALUES($1,'Тестовое приложение (демо)','',$2) ON CONFLICT (client_id) DO UPDATE SET redirect_uris=EXCLUDED.redirect_uris`, DemoClientID, uris)
	return err
}

// Mount регистрирует маршруты провайдера.
func (p *Provider) Mount(mux *http.ServeMux, anyUser, fed Middleware) {
	mux.HandleFunc("GET /.well-known/openid-configuration", p.discovery)
	mux.HandleFunc("GET /oidc/jwks.json", p.jwks)
	mux.HandleFunc("GET /oidc/authorize", servePage(consentPage))
	mux.HandleFunc("GET /oidc/authorize/info", p.authorizeInfo)
	mux.HandleFunc("POST /oidc/authorize/decision", anyUser(p.decision))
	mux.HandleFunc("POST /oidc/token", p.token)
	mux.HandleFunc("GET /oidc/userinfo", p.userinfo)
	mux.HandleFunc("POST /oidc/userinfo", p.userinfo)
	mux.HandleFunc("GET /oidc/demo", servePage(demoPage))
	mux.HandleFunc("GET /oidc/admin", servePage(adminPage))
	mux.HandleFunc("GET /api/oidc/clients", fed(p.listClients))
	mux.HandleFunc("POST /api/oidc/clients", fed(p.createClient))
	mux.HandleFunc("DELETE /api/oidc/clients/{id}", fed(p.deleteClient))
}

// ---------- discovery / jwks ----------

func (p *Provider) issuerFor(r *http.Request) string {
	if p.issuer != "" {
		return p.issuer
	}
	scheme := "http"
	if v := r.Header.Get("X-Forwarded-Proto"); v != "" {
		scheme = strings.TrimSpace(strings.Split(v, ",")[0])
	} else if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if v := r.Header.Get("X-Forwarded-Host"); v != "" {
		host = strings.TrimSpace(strings.Split(v, ",")[0])
	}
	return scheme + "://" + host
}

func (p *Provider) discovery(w http.ResponseWriter, r *http.Request) {
	iss := p.issuerFor(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                iss,
		"authorization_endpoint":                iss + "/oidc/authorize",
		"token_endpoint":                        iss + "/oidc/token",
		"userinfo_endpoint":                     iss + "/oidc/userinfo",
		"jwks_uri":                              iss + "/oidc/jwks.json",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported":                      scopeOrder,
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none", "client_secret_post", "client_secret_basic"},
		"claims_supported":                      []string{"sub", "name", "email", "roles", "disciplines", "rank", "rank_expires_at", "fsp_id"},
	})
}

func (p *Provider) jwks(w http.ResponseWriter, _ *http.Request) {
	pub := p.key.PublicKey
	writeJSON(w, http.StatusOK, map[string]any{"keys": []map[string]string{{
		"kty": "RSA", "use": "sig", "alg": "RS256", "kid": p.kid,
		"n": b64(pub.N.Bytes()),
		"e": b64(big.NewInt(int64(pub.E)).Bytes()),
	}}})
}

// ---------- клиенты ----------

type client struct {
	ID           string
	Name         string
	SecretHash   string
	RedirectURIs []string
}

func (p *Provider) getClient(ctx context.Context, id string) (client, error) {
	var c client
	var uris string
	err := p.db.QueryRowContext(ctx, `SELECT client_id,name,secret_hash,redirect_uris FROM oidc_clients WHERE client_id=$1`, id).Scan(&c.ID, &c.Name, &c.SecretHash, &uris)
	if err != nil {
		return c, err
	}
	for _, u := range strings.Split(uris, "\n") {
		if u = strings.TrimSpace(u); u != "" {
			c.RedirectURIs = append(c.RedirectURIs, u)
		}
	}
	return c, nil
}

func (c client) allows(redirect string) bool {
	for _, u := range c.RedirectURIs {
		if u == redirect {
			return true
		}
	}
	// Встроенное демо-приложение работает на любом адресе платформы (localhost, Render и т.д.):
	// принимаем redirect_uri вида <origin>/oidc/demo, если это https или localhost.
	if c.ID == DemoClientID && validRedirect(redirect) {
		if u, err := url.Parse(redirect); err == nil && u.Path == "/oidc/demo" && u.RawQuery == "" {
			return true
		}
	}
	return false
}

func (p *Provider) listClients(w http.ResponseWriter, r *http.Request) {
	rows, err := p.db.QueryContext(r.Context(), `SELECT client_id,name,secret_hash<>'',redirect_uris,created_at FROM oidc_clients ORDER BY created_at`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось получить клиентов")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, name, uris string
		var conf bool
		var created time.Time
		if err := rows.Scan(&id, &name, &conf, &uris, &created); err != nil {
			continue
		}
		items = append(items, map[string]any{"client_id": id, "name": name, "confidential": conf, "redirect_uris": strings.Split(uris, "\n"), "created_at": created.Format("02.01.2006 15:04")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (p *Provider) createClient(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name         string   `json:"name"`
		RedirectURIs []string `json:"redirect_uris"`
		Confidential bool     `json:"confidential"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Проверьте формат JSON")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		writeError(w, http.StatusBadRequest, "Укажите название приложения")
		return
	}
	var uris []string
	for _, raw := range in.RedirectURIs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if !validRedirect(raw) {
			writeError(w, http.StatusBadRequest, "Некорректный redirect URI: "+raw)
			return
		}
		uris = append(uris, raw)
	}
	if len(uris) == 0 {
		writeError(w, http.StatusBadRequest, "Укажите хотя бы один redirect URI")
		return
	}
	id := "fsp_" + randHex(8)
	secret, hash := "", ""
	if in.Confidential {
		secret = randHex(24)
		b, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Не удалось создать секрет")
			return
		}
		hash = string(b)
	}
	if _, err := p.db.ExecContext(r.Context(), `INSERT INTO oidc_clients(client_id,name,secret_hash,redirect_uris) VALUES($1,$2,$3,$4)`, id, in.Name, hash, strings.Join(uris, "\n")); err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось сохранить клиента")
		return
	}
	u := auth.From(r)
	_, _ = p.db.ExecContext(r.Context(), `INSERT INTO audit_log(user_id,role,action,entity,details) VALUES($1,$2,'регистрация клиента ФСП ID','oidc_clients',$3)`, u.ID, u.ActiveRole, in.Name)
	resp := map[string]any{"client_id": id, "name": in.Name, "redirect_uris": uris}
	if secret != "" {
		resp["client_secret"] = secret // показывается один раз
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (p *Provider) deleteClient(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == DemoClientID {
		writeError(w, http.StatusBadRequest, "Демонстрационного клиента удалить нельзя")
		return
	}
	if _, err := p.db.ExecContext(r.Context(), `DELETE FROM oidc_clients WHERE client_id=$1`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось удалить клиента")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func validRedirect(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Fragment != "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	h := u.Hostname()
	return u.Scheme == "http" && (h == "localhost" || h == "127.0.0.1")
}

// ---------- authorize ----------

type authRequest struct {
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	Scope               string `json:"scope"`
	State               string `json:"state"`
	Nonce               string `json:"nonce"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	ResponseType        string `json:"response_type"`
	Approve             bool   `json:"approve"`
}

// validate проверяет запрос и возвращает клиента и нормализованный набор scope.
func (p *Provider) validate(ctx context.Context, a authRequest) (client, string, error) {
	c, err := p.getClient(ctx, a.ClientID)
	if err != nil {
		return c, "", errors.New("Неизвестное приложение (client_id)")
	}
	if !c.allows(a.RedirectURI) {
		return c, "", errors.New("redirect_uri не зарегистрирован для этого приложения")
	}
	if a.ResponseType != "code" {
		return c, "", errors.New("Поддерживается только response_type=code")
	}
	if a.CodeChallengeMethod != "S256" || len(a.CodeChallenge) < 43 {
		return c, "", errors.New("Требуется PKCE: code_challenge (S256)")
	}
	scope, err := normalizeScope(a.Scope)
	if err != nil {
		return c, "", err
	}
	return c, scope, nil
}

func normalizeScope(raw string) (string, error) {
	want := map[string]bool{}
	for _, s := range strings.Fields(raw) {
		if _, ok := scopeLabels[s]; !ok {
			return "", errors.New("Неизвестный scope: " + s)
		}
		want[s] = true
	}
	if !want["openid"] {
		return "", errors.New("Требуется scope openid")
	}
	var out []string
	for _, s := range scopeOrder {
		if want[s] {
			out = append(out, s)
		}
	}
	return strings.Join(out, " "), nil
}

func requestFromQuery(q url.Values) authRequest {
	return authRequest{
		ClientID: q.Get("client_id"), RedirectURI: q.Get("redirect_uri"), Scope: q.Get("scope"),
		State: q.Get("state"), Nonce: q.Get("nonce"), CodeChallenge: q.Get("code_challenge"),
		CodeChallengeMethod: q.Get("code_challenge_method"), ResponseType: q.Get("response_type"),
	}
}

func (p *Provider) authorizeInfo(w http.ResponseWriter, r *http.Request) {
	c, scope, err := p.validate(r.Context(), requestFromQuery(r.URL.Query()))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	type item struct {
		Scope string `json:"scope"`
		Label string `json:"label"`
	}
	var items []item
	for _, s := range strings.Fields(scope) {
		items = append(items, item{s, scopeLabels[s]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"client_name": c.Name, "scopes": items})
}

func (p *Provider) decision(w http.ResponseWriter, r *http.Request) {
	var a authRequest
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		writeError(w, http.StatusBadRequest, "Проверьте формат JSON")
		return
	}
	a.ResponseType = "code"
	_, scope, err := p.validate(r.Context(), a)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	target, _ := url.Parse(a.RedirectURI)
	q := target.Query()
	if a.State != "" {
		q.Set("state", a.State)
	}
	user := auth.From(r)
	if !a.Approve {
		q.Set("error", "access_denied")
		target.RawQuery = q.Encode()
		writeJSON(w, http.StatusOK, map[string]string{"redirect": target.String()})
		return
	}
	code := randHex(24)
	_, err = p.db.ExecContext(r.Context(), `INSERT INTO oidc_codes(code_hash,client_id,user_id,redirect_uri,scope,nonce,code_challenge,expires_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, hashOf(code), a.ClientID, user.ID, a.RedirectURI, scope, a.Nonce, a.CodeChallenge, time.Now().Add(codeTTL))
	if err != nil {
		log.Printf("oidc: save code: %v", err)
		writeError(w, http.StatusInternalServerError, "Не удалось выдать код авторизации")
		return
	}
	_, _ = p.db.ExecContext(r.Context(), `INSERT INTO audit_log(user_id,role,action,entity,details) VALUES($1,$2,'согласие ФСП ID','oidc_clients',$3)`, user.ID, user.ActiveRole, a.ClientID)
	q.Set("code", code)
	target.RawQuery = q.Encode()
	writeJSON(w, http.StatusOK, map[string]string{"redirect": target.String()})
}

// ---------- token ----------

func (p *Provider) token(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	if err := r.ParseForm(); err != nil {
		tokenError(w, http.StatusBadRequest, "invalid_request", "Некорректное тело запроса")
		return
	}
	if r.PostForm.Get("grant_type") != "authorization_code" {
		tokenError(w, http.StatusBadRequest, "unsupported_grant_type", "Поддерживается только authorization_code")
		return
	}
	clientID, secret := r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	if id, pass, ok := r.BasicAuth(); ok {
		clientID, secret = id, pass
	}
	c, err := p.getClient(r.Context(), clientID)
	if err != nil {
		tokenError(w, http.StatusUnauthorized, "invalid_client", "Неизвестный клиент")
		return
	}
	if c.SecretHash != "" && bcrypt.CompareHashAndPassword([]byte(c.SecretHash), []byte(secret)) != nil {
		tokenError(w, http.StatusUnauthorized, "invalid_client", "Неверный client_secret")
		return
	}
	var userID int64
	var redirect, scope, nonce, challenge, codeClient string
	err = p.db.QueryRowContext(r.Context(), `UPDATE oidc_codes SET used=TRUE
		WHERE code_hash=$1 AND used=FALSE AND expires_at>NOW()
		RETURNING client_id,user_id,redirect_uri,scope,nonce,code_challenge`, hashOf(r.PostForm.Get("code"))).
		Scan(&codeClient, &userID, &redirect, &scope, &nonce, &challenge)
	if err != nil || codeClient != c.ID {
		tokenError(w, http.StatusBadRequest, "invalid_grant", "Код недействителен, просрочен или уже использован")
		return
	}
	if redirect != r.PostForm.Get("redirect_uri") {
		tokenError(w, http.StatusBadRequest, "invalid_grant", "redirect_uri не совпадает с запросом авторизации")
		return
	}
	if !VerifyPKCE(r.PostForm.Get("code_verifier"), challenge) {
		tokenError(w, http.StatusBadRequest, "invalid_grant", "code_verifier не соответствует code_challenge")
		return
	}
	access := randHex(32)
	if _, err := p.db.ExecContext(r.Context(), `INSERT INTO oidc_tokens(token_hash,client_id,user_id,scope,expires_at) VALUES($1,$2,$3,$4,$5)`,
		hashOf(access), c.ID, userID, scope, time.Now().Add(accessTokenTTL)); err != nil {
		tokenError(w, http.StatusInternalServerError, "server_error", "Не удалось выдать токен")
		return
	}
	claims, err := p.claims(r.Context(), userID, scope)
	if err != nil {
		tokenError(w, http.StatusInternalServerError, "server_error", "Не удалось получить данные пользователя")
		return
	}
	now := time.Now()
	claims["iss"], claims["aud"] = p.issuerFor(r), c.ID
	claims["iat"], claims["exp"] = now.Unix(), now.Add(accessTokenTTL).Unix()
	if nonce != "" {
		claims["nonce"] = nonce
	}
	idToken, err := p.sign(claims)
	if err != nil {
		tokenError(w, http.StatusInternalServerError, "server_error", "Не удалось подписать id_token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": access, "token_type": "Bearer", "expires_in": int(accessTokenTTL.Seconds()),
		"id_token": idToken, "scope": scope,
	})
}

// VerifyPKCE проверяет code_verifier по code_challenge (метод S256).
func VerifyPKCE(verifier, challenge string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	return subtle.ConstantTimeCompare([]byte(b64(sum[:])), []byte(challenge)) == 1
}

func (p *Provider) sign(claims map[string]any) (string, error) {
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": p.kid})
	body, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	input := b64(header) + "." + b64(body)
	sum := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, p.key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return input + "." + b64(sig), nil
}

// ---------- userinfo ----------

func (p *Provider) userinfo(w http.ResponseWriter, r *http.Request) {
	h := r.Header.Get("Authorization")
	if len(h) < 8 || !strings.EqualFold(h[:7], "bearer ") {
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
		writeError(w, http.StatusUnauthorized, "Нужен заголовок Authorization: Bearer <access_token>")
		return
	}
	var userID int64
	var scope string
	err := p.db.QueryRowContext(r.Context(), `SELECT user_id,scope FROM oidc_tokens WHERE token_hash=$1 AND expires_at>NOW()`, hashOf(strings.TrimSpace(h[7:]))).Scan(&userID, &scope)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
		writeError(w, http.StatusUnauthorized, "Токен недействителен или просрочен")
		return
	}
	claims, err := p.claims(r.Context(), userID, scope)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось получить данные пользователя")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, claims)
}

// claims собирает данные пользователя в пределах выданных scope.
func (p *Provider) claims(ctx context.Context, userID int64, scope string) (map[string]any, error) {
	has := map[string]bool{}
	for _, s := range strings.Fields(scope) {
		has[s] = true
	}
	var name, email string
	if err := p.db.QueryRowContext(ctx, `SELECT full_name,email FROM users WHERE id=$1`, userID).Scan(&name, &email); err != nil {
		return nil, err
	}
	out := map[string]any{"sub": fmt.Sprintf("%d", userID)}
	if has["profile"] {
		out["name"] = name
	}
	if has["email"] {
		out["email"] = email
	}
	if has["roles"] {
		roles := []string{}
		if rows, err := p.db.QueryContext(ctx, `SELECT role FROM user_roles WHERE user_id=$1 ORDER BY role`, userID); err == nil {
			for rows.Next() {
				var role string
				if rows.Scan(&role) == nil {
					roles = append(roles, role)
				}
			}
			rows.Close()
		}
		out["roles"] = roles
	}
	if has["sport"] {
		fspID := fmt.Sprintf("ФСП-56-%06d", userID)
		disciplines, rank := []string{}, ""
		var rankExpires string
		seen := map[string]bool{}
		if rows, err := p.db.QueryContext(ctx, `SELECT COALESCE(fsp_id,''),discipline,COALESCE(rank_title,''),COALESCE(to_char(rank_expires_at,'YYYY-MM-DD'),'') FROM athletes WHERE user_id=$1 ORDER BY id`, userID); err == nil {
			for rows.Next() {
				var id, d, rt, exp string
				if rows.Scan(&id, &d, &rt, &exp) != nil {
					continue
				}
				if id != "" {
					fspID = id
				}
				if d != "" && !seen[d] {
					seen[d] = true
					disciplines = append(disciplines, d)
				}
				if rt != "" && rank == "" {
					rank, rankExpires = rt, exp
				}
			}
			rows.Close()
		}
		if rows, err := p.db.QueryContext(ctx, `SELECT DISTINCT cl.discipline FROM coach_links cl JOIN coaches c ON c.id=cl.coach_id WHERE c.user_id=$1`, userID); err == nil {
			for rows.Next() {
				var d string
				if rows.Scan(&d) == nil && d != "" && !seen[d] {
					seen[d] = true
					disciplines = append(disciplines, d)
				}
			}
			rows.Close()
		}
		out["fsp_id"], out["disciplines"] = fspID, disciplines
		if rank != "" {
			out["rank"] = rank
			if rankExpires != "" {
				out["rank_expires_at"] = rankExpires
			}
		}
	}
	return out, nil
}

// ---------- helpers ----------

func tokenError(w http.ResponseWriter, status int, code, desc string) {
	writeJSON(w, status, map[string]string{"error": code, "error_description": desc})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func servePage(html string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Frame-Options", "DENY")
		_, _ = w.Write([]byte(html))
	}
}

func hashOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func randHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
