package platform

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"orenkod/backend/internal/auth"
	"orenkod/backend/internal/httpserver"
)

// Д1. Региональный ФСП ID: платформа — провайдер OpenID Connect
// (authorization code + PKCE S256, discovery, userinfo, id_token HS256).

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
func sha(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func oidcBase(r *http.Request) string {
	if v := os.Getenv("PUBLIC_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	proto, host := "http", r.Host
	if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
		proto = p
	}
	if hh := r.Header.Get("X-Forwarded-Host"); hh != "" {
		host = hh
	}
	return proto + "://" + host
}

func oidcSecret() []byte {
	if v := os.Getenv("OIDC_SECRET"); v != "" {
		return []byte(v)
	}
	return []byte("orenkod-demo-oidc-secret")
}

func (h *Handler) redirectAllowed(clientID, uri string) (name string, ok bool) {
	var list string
	if h.db.QueryRowContext(ctxBg(), `SELECT name,redirect_uris FROM oidc_clients WHERE client_id=$1`, clientID).Scan(&name, &list) != nil {
		return "", false
	}
	for _, allowed := range strings.Fields(list) {
		if allowed == uri || (strings.HasPrefix(allowed, "*") && strings.HasSuffix(uri, allowed[1:]) && (strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://"))) {
			return name, true
		}
	}
	return name, false
}

func (h *Handler) OIDCDiscovery(w http.ResponseWriter, r *http.Request) {
	b := oidcBase(r) + "/api/oidc"
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{
		"issuer": b, "authorization_endpoint": b + "/authorize", "token_endpoint": b + "/token", "userinfo_endpoint": b + "/userinfo",
		"response_types_supported": []string{"code"}, "grant_types_supported": []string{"authorization_code"},
		"subject_types_supported": []string{"public"}, "id_token_signing_alg_values_supported": []string{"HS256"},
		"scopes_supported": []string{"openid", "profile", "email"}, "code_challenge_methods_supported": []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none", "client_secret_post"},
		"claims_supported":                      []string{"sub", "name", "email", "roles", "disciplines", "rank", "fsp_id"},
	})
}

// OIDCAuthorizeInfo — данные для страницы согласия (нужен вход в платформу).
func (h *Handler) OIDCAuthorizeInfo(w http.ResponseWriter, r *http.Request) {
	name, ok := h.redirectAllowed(r.URL.Query().Get("client_id"), r.URL.Query().Get("redirect_uri"))
	if !ok {
		httpserver.WriteError(w, http.StatusBadRequest, "Приложение не зарегистрировано или redirect_uri не разрешён")
		return
	}
	u := auth.From(r)
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"client": name, "user": u.FullName, "email": u.Email})
}

// OIDCApprove — пользователь дал согласие; выдаём одноразовый код.
func (h *Handler) OIDCApprove(w http.ResponseWriter, r *http.Request) {
	var in struct{ ClientID, RedirectURI, Scope, State, CodeChallenge, CodeChallengeMethod string }
	var raw map[string]string
	if json.NewDecoder(r.Body).Decode(&raw) != nil {
		httpserver.WriteError(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	in.ClientID, in.RedirectURI, in.Scope, in.State = raw["client_id"], raw["redirect_uri"], raw["scope"], raw["state"]
	in.CodeChallenge, in.CodeChallengeMethod = raw["code_challenge"], raw["code_challenge_method"]
	if _, ok := h.redirectAllowed(in.ClientID, in.RedirectURI); !ok {
		httpserver.WriteError(w, http.StatusBadRequest, "redirect_uri не разрешён")
		return
	}
	if in.CodeChallenge == "" || in.CodeChallengeMethod != "S256" {
		httpserver.WriteError(w, http.StatusBadRequest, "Требуется PKCE (code_challenge, S256)")
		return
	}
	if !strings.Contains(" "+in.Scope+" ", " openid ") {
		httpserver.WriteError(w, http.StatusBadRequest, "Нужен scope openid")
		return
	}
	code := randHex(24)
	if _, err := h.db.ExecContext(r.Context(), `INSERT INTO oidc_codes(code,client_id,user_id,redirect_uri,challenge,scope,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`,
		code, in.ClientID, auth.From(r).ID, in.RedirectURI, in.CodeChallenge, in.Scope, time.Now().Add(5*time.Minute)); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось выдать код")
		return
	}
	h.audit(r, "согласие на передачу данных ФСП ID", "oidc", in.ClientID)
	u, _ := url.Parse(in.RedirectURI)
	q := u.Query()
	q.Set("code", code)
	if in.State != "" {
		q.Set("state", in.State)
	}
	u.RawQuery = q.Encode()
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"redirect": u.String()})
}

func oidcErr(w http.ResponseWriter, code int, e, d string) {
	httpserver.WriteJSON(w, code, map[string]any{"error": e, "error_description": d})
}

func (h *Handler) OIDCToken(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	if r.PostForm.Get("grant_type") != "authorization_code" {
		oidcErr(w, 400, "unsupported_grant_type", "поддерживается authorization_code")
		return
	}
	code, clientID := r.PostForm.Get("code"), r.PostForm.Get("client_id")
	var uid int64
	var redirect, challenge, scope, secretHash string
	var exp time.Time
	var used bool
	err := h.db.QueryRowContext(r.Context(), `SELECT c.user_id,c.redirect_uri,c.challenge,c.scope,c.expires_at,c.used,cl.secret_hash FROM oidc_codes c JOIN oidc_clients cl ON cl.client_id=c.client_id WHERE c.code=$1 AND c.client_id=$2`, code, clientID).
		Scan(&uid, &redirect, &challenge, &scope, &exp, &used, &secretHash)
	if err != nil || used || time.Now().After(exp) {
		oidcErr(w, 400, "invalid_grant", "код недействителен или просрочен")
		return
	}
	if r.PostForm.Get("redirect_uri") != redirect {
		oidcErr(w, 400, "invalid_grant", "redirect_uri не совпадает")
		return
	}
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if !hmac.Equal([]byte(b64(sum[:])), []byte(challenge)) {
		oidcErr(w, 400, "invalid_grant", "PKCE: code_verifier не подходит")
		return
	}
	if secretHash != "" && sha(r.PostForm.Get("client_secret")) != secretHash {
		oidcErr(w, 401, "invalid_client", "неверный client_secret")
		return
	}
	res, _ := h.db.ExecContext(r.Context(), `UPDATE oidc_codes SET used=TRUE WHERE code=$1 AND used=FALSE`, code)
	if n, _ := res.RowsAffected(); n == 0 {
		oidcErr(w, 400, "invalid_grant", "код уже использован")
		return
	}
	access := randHex(32)
	_, _ = h.db.ExecContext(r.Context(), `INSERT INTO oidc_tokens(token_hash,client_id,user_id,scope,expires_at) VALUES($1,$2,$3,$4,$5)`, sha(access), clientID, uid, scope, time.Now().Add(time.Hour))
	claims := h.oidcClaims(r, uid, scope)
	claims["iss"], claims["aud"], claims["iat"], claims["exp"] = oidcBase(r)+"/api/oidc", clientID, time.Now().Unix(), time.Now().Add(time.Hour).Unix()
	hd, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	pl, _ := json.Marshal(claims)
	unsigned := b64(hd) + "." + b64(pl)
	mac := hmac.New(sha256.New, oidcSecret())
	mac.Write([]byte(unsigned))
	w.Header().Set("Cache-Control", "no-store")
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"access_token": access, "token_type": "Bearer", "expires_in": 3600, "scope": scope, "id_token": unsigned + "." + b64(mac.Sum(nil))})
}

func (h *Handler) oidcClaims(r *http.Request, uid int64, scope string) map[string]any {
	c := map[string]any{"sub": "fsp-" + itoa(uid)}
	var name, email string
	_ = h.db.QueryRowContext(r.Context(), `SELECT full_name,email FROM users WHERE id=$1`, uid).Scan(&name, &email)
	has := func(s string) bool { return strings.Contains(" "+scope+" ", " "+s+" ") }
	if has("email") {
		c["email"] = email
	}
	if has("profile") {
		c["name"] = name
		roles := []string{}
		if rr, err := h.db.QueryContext(r.Context(), `SELECT role FROM user_roles WHERE user_id=$1`, uid); err == nil {
			for rr.Next() {
				var s string
				if rr.Scan(&s) == nil {
					roles = append(roles, s)
				}
			}
			rr.Close()
		}
		c["roles"] = roles
		discs := []string{}
		var rank, fsp sql.NullString
		_ = h.db.QueryRowContext(r.Context(), `SELECT rank_title,fsp_id FROM athletes WHERE user_id=$1 LIMIT 1`, uid).Scan(&rank, &fsp)
		if dr, err := h.db.QueryContext(r.Context(), `SELECT d.discipline FROM athlete_disciplines d JOIN athletes a ON a.id=d.athlete_id WHERE a.user_id=$1`, uid); err == nil {
			for dr.Next() {
				var s string
				if dr.Scan(&s) == nil {
					discs = append(discs, s)
				}
			}
			dr.Close()
		}
		c["disciplines"], c["rank"], c["fsp_id"] = discs, rank.String, fsp.String
	}
	return c
}

func (h *Handler) OIDCUserinfo(w http.ResponseWriter, r *http.Request) {
	tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	var uid int64
	var scope string
	var exp time.Time
	if tok == "" || h.db.QueryRowContext(r.Context(), `SELECT user_id,scope,expires_at FROM oidc_tokens WHERE token_hash=$1`, sha(tok)).Scan(&uid, &scope, &exp) != nil || time.Now().After(exp) {
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
		oidcErr(w, 401, "invalid_token", "токен недействителен")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, h.oidcClaims(r, uid, scope))
}

// OIDCClients — регистрация клиентских приложений администратором (федерацией).
func (h *Handler) OIDCClientCreate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name         string   `json:"name"`
		RedirectURIs []string `json:"redirect_uris"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.Name == "" || len(in.RedirectURIs) == 0 {
		httpserver.WriteError(w, http.StatusBadRequest, "Нужны название и redirect_uris")
		return
	}
	id, secret := "app-"+randHex(6), randHex(24)
	if _, err := h.db.ExecContext(r.Context(), `INSERT INTO oidc_clients(client_id,secret_hash,name,redirect_uris) VALUES($1,$2,$3,$4)`, id, sha(secret), in.Name, strings.Join(in.RedirectURIs, " ")); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Не удалось зарегистрировать")
		return
	}
	h.audit(r, "регистрация клиента ФСП ID", "oidc_clients", id)
	httpserver.WriteJSON(w, http.StatusCreated, map[string]any{"client_id": id, "client_secret": secret, "note": "секрет показан один раз"})
}

func (h *Handler) OIDCClientList(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `SELECT client_id,name,redirect_uris,created_at FROM oidc_clients ORDER BY created_at`)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "Ошибка")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, name, uris string
		var t time.Time
		if rows.Scan(&id, &name, &uris, &t) == nil {
			items = append(items, map[string]any{"client_id": id, "name": name, "redirect_uris": strings.Fields(uris), "created_at": t.Format("2006-01-02")})
		}
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) OIDCAuthorizePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(consentHTML))
}
func (h *Handler) OIDCDemoPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(demoHTML))
}

const consentHTML = `<!doctype html><html lang="ru"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Вход через ФСП ID</title>
<style>body{font-family:system-ui,sans-serif;background:#0f1117;color:#eee;display:grid;place-items:center;min-height:100vh;margin:0}main{background:#1a1d27;padding:32px;border-radius:16px;max-width:420px;width:calc(100% - 32px)}button,a.b{display:inline-block;padding:12px 18px;border-radius:10px;border:0;font-weight:700;cursor:pointer;text-decoration:none;margin-right:8px}.ok{background:#7c5cff;color:#fff}.no{background:#2a2e3b;color:#eee}</style>
<main><h2>Вход через ФСП ID</h2><div id="c">Загрузка…</div></main>
<script>
const q=new URLSearchParams(location.search),tok=localStorage.getItem('fsp_token'),c=document.getElementById('c');
const esc=s=>String(s).replace(/[&<>"]/g,m=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[m]));
(async()=>{
 if(!tok){c.innerHTML='Сначала войдите в платформу ФСП: <a style="color:#9d8bff" href="/" target="_blank">открыть вход</a>, затем обновите эту страницу.';return}
 const h={Authorization:'Bearer '+tok,'Content-Type':'application/json'};
 const r=await fetch('/api/oidc/authorize/info?client_id='+encodeURIComponent(q.get('client_id')||'')+'&redirect_uri='+encodeURIComponent(q.get('redirect_uri')||''),{headers:h});
 const d=await r.json();
 if(!r.ok){c.textContent=d.error||'Ошибка';return}
 c.innerHTML='<p>Приложение <b>'+esc(d.client)+'</b> запрашивает доступ к данным аккаунта <b>'+esc(d.user)+'</b> ('+esc(d.email)+'):</p><ul><li>ФИО, роли, дисциплины, разряд, ФСП ID</li><li>email (если запрошен)</li></ul><button class="ok" id="y">Разрешить</button><button class="no" id="n">Отмена</button>';
 document.getElementById('n').onclick=()=>{const u=new URL(q.get('redirect_uri'));u.searchParams.set('error','access_denied');if(q.get('state'))u.searchParams.set('state',q.get('state'));location.href=u};
 document.getElementById('y').onclick=async()=>{
  const body={};for(const k of ['client_id','redirect_uri','scope','state','code_challenge','code_challenge_method'])body[k]=q.get(k)||'';
  const r=await fetch('/api/oidc/authorize/approve',{method:'POST',headers:h,body:JSON.stringify(body)});const d=await r.json();
  if(r.ok)location.href=d.redirect;else c.textContent=d.error||'Ошибка'};
})();
</script></html>`

const demoHTML = `<!doctype html><html lang="ru"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Тестовое приложение-партнёр</title>
<style>body{font-family:system-ui,sans-serif;background:#f4f4f8;color:#222;display:grid;place-items:center;min-height:100vh;margin:0}main{background:#fff;padding:32px;border-radius:16px;max-width:560px;width:calc(100% - 32px)}button{padding:12px 18px;border-radius:10px;border:0;background:#7c5cff;color:#fff;font-weight:700;cursor:pointer}pre{background:#f0f0f6;padding:12px;border-radius:8px;overflow:auto;font-size:12px}</style>
<main><h2>Партнёрский сайт</h2><p>Тестовое клиентское приложение для «Войти через ФСП ID» (authorization code + PKCE).</p><div id="o"><button id="go">Войти через ФСП ID</button></div></main>
<script>
const b64=b=>btoa(String.fromCharCode(...new Uint8Array(b))).replace(/\+/g,'-').replace(/\//g,'_').replace(/=+$/,'');
const redirect=location.origin+'/api/oidc/demo',cid='fsp-demo',o=document.getElementById('o'),q=new URLSearchParams(location.search);
document.getElementById('go').onclick=async()=>{
 const v=b64(crypto.getRandomValues(new Uint8Array(32))),st=b64(crypto.getRandomValues(new Uint8Array(8)));
 sessionStorage.v=v;sessionStorage.st=st;
 const ch=b64(await crypto.subtle.digest('SHA-256',new TextEncoder().encode(v)));
 location.href='/api/oidc/authorize?'+new URLSearchParams({response_type:'code',client_id:cid,redirect_uri:redirect,scope:'openid profile email',state:st,code_challenge:ch,code_challenge_method:'S256'});
};
(async()=>{
 if(q.get('error')){o.textContent='Доступ отклонён';return}
 if(!q.get('code'))return;
 if(q.get('state')!==sessionStorage.st){o.textContent='Неверный state';return}
 const t=await(await fetch('/api/oidc/token',{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded'},body:new URLSearchParams({grant_type:'authorization_code',code:q.get('code'),redirect_uri:redirect,client_id:cid,code_verifier:sessionStorage.v})})).json();
 if(!t.access_token){o.textContent='Ошибка: '+(t.error_description||t.error);return}
 const u=await(await fetch('/api/oidc/userinfo',{headers:{Authorization:'Bearer '+t.access_token}})).json();
 o.innerHTML='<p>✅ Вы вошли через ФСП ID. Ответ userinfo:</p><pre>'+JSON.stringify(u,null,2).replace(/</g,'&lt;')+'</pre>';
})();
</script></html>`

func itoa(n int64) string    { return strconv.FormatInt(n, 10) }
func ctxBg() context.Context { return context.Background() }
