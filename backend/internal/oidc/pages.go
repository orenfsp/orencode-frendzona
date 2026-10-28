package oidc

const baseStyle = `<style>
:root{color-scheme:light dark;--bg:#f4f6fb;--card:#fff;--text:#14213d;--muted:#5b6785;--accent:#1f4fd8;--line:#d9dfee}
@media(prefers-color-scheme:dark){:root{--bg:#0e1424;--card:#171f36;--text:#eef1fb;--muted:#9aa6c7;--accent:#6b8cff;--line:#2a3558}}
*{box-sizing:border-box}body{margin:0;min-height:100vh;display:grid;place-items:center;background:var(--bg);color:var(--text);font-family:system-ui,-apple-system,Segoe UI,Roboto,sans-serif;padding:16px}
.card{width:100%;max-width:460px;background:var(--card);border:1px solid var(--line);border-radius:16px;padding:24px;box-shadow:0 8px 30px rgba(0,0,0,.08)}
.wide{max-width:720px}h1{font-size:22px;margin:0 0 4px}p{color:var(--muted);margin:6px 0}
input{width:100%;padding:10px 12px;margin:6px 0;border-radius:10px;border:1px solid var(--line);background:transparent;color:var(--text);font:inherit}
button{padding:10px 16px;border-radius:10px;border:0;background:var(--accent);color:#fff;font:inherit;cursor:pointer;margin-top:8px}
button.ghost{background:transparent;color:var(--text);border:1px solid var(--line)}button.danger{background:#c0392b}
ul{padding-left:18px}li{margin:4px 0}.err{color:#d64545;margin-top:8px}pre{overflow:auto;background:rgba(127,127,127,.12);padding:12px;border-radius:10px;font-size:13px}
table{width:100%;border-collapse:collapse;font-size:14px}td,th{padding:6px;border-bottom:1px solid var(--line);text-align:left;vertical-align:top}
.row{display:flex;gap:8px;flex-wrap:wrap}.brand{font-weight:700;color:var(--accent);letter-spacing:.04em;margin-bottom:8px}
</style>`

// consentPage: страница входа и согласия на передачу данных (ФСП ID).
const consentPage = `<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Вход через ФСП ID</title>` + baseStyle + `</head><body>
<div class="card" id="app"><div class="brand">ФСП ID · Оренбургская область</div><p>Загрузка…</p></div>
<script>
const q=new URLSearchParams(location.search);
const app=document.getElementById('app');
const esc=s=>String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const token=()=>localStorage.getItem('fsp_token');
async function j(url,opt){const r=await fetch(url,opt);const d=await r.json().catch(()=>({}));if(!r.ok)throw new Error(d.error||'Ошибка '+r.status);return d}
const H=()=>({'Content-Type':'application/json',Authorization:'Bearer '+token()});
async function init(){
  let info;
  try{info=await j('/oidc/authorize/info?'+q.toString())}catch(e){app.innerHTML='<div class="brand">ФСП ID</div><h1>Ошибка запроса</h1><p class="err">'+esc(e.message)+'</p>';return}
  let me=null;
  if(token()){try{me=await j('/api/auth/me',{headers:H()})}catch(e){localStorage.removeItem('fsp_token')}}
  if(!me){return loginView(info)}
  consentView(info,me)
}
function loginView(info){
  app.innerHTML='<div class="brand">ФСП ID</div><h1>Войдите в аккаунт</h1><p>Приложение «'+esc(info.client_name)+'» запрашивает вход через ФСП ID.</p><input id="em" type="email" placeholder="Email" autocomplete="username"><input id="pw" type="password" placeholder="Пароль" autocomplete="current-password"><button id="go">Войти</button><div class="err" id="er"></div>';
  document.getElementById('go').onclick=async()=>{try{const d=await j('/api/auth/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({email:em.value,password:pw.value})});localStorage.setItem('fsp_token',d.token);init()}catch(e){er.textContent=e.message}}
}
function consentView(info,me){
  app.innerHTML='<div class="brand">ФСП ID</div><h1>Разрешить доступ?</h1><p>Вы вошли как <b>'+esc(me.full_name)+'</b>.</p><p>Приложение <b>'+esc(info.client_name)+'</b> получит:</p><ul>'+info.scopes.map(s=>'<li>'+esc(s.label)+'</li>').join('')+'</ul><div class="row"><button id="ok">Разрешить</button><button class="ghost" id="no">Отклонить</button><button class="ghost" id="sw">Другой аккаунт</button></div><div class="err" id="er"></div>';
  const send=async approve=>{try{const body={client_id:q.get('client_id'),redirect_uri:q.get('redirect_uri'),scope:q.get('scope'),state:q.get('state')||'',nonce:q.get('nonce')||'',code_challenge:q.get('code_challenge'),code_challenge_method:q.get('code_challenge_method'),approve};const d=await j('/oidc/authorize/decision',{method:'POST',headers:H(),body:JSON.stringify(body)});location.href=d.redirect}catch(e){er.textContent=e.message}};
  ok.onclick=()=>send(true);no.onclick=()=>send(false);sw.onclick=()=>{localStorage.removeItem('fsp_token');init()}
}
init();
</script></body></html>`

// demoPage: тестовое клиентское приложение с кнопкой «Войти через ФСП ID».
const demoPage = `<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Демо-клиент ФСП ID</title>` + baseStyle + `</head><body>
<div class="card wide"><div class="brand">Тестовое приложение партнёра</div><h1>Войти через ФСП ID</h1>
<p>Демонстрация потока authorization code + PKCE. Приложение — внешний клиент <code>fsp-demo-client</code>.</p>
<button id="login">Войти через ФСП ID</button> <button class="ghost" id="reset">Сбросить</button>
<div id="out"></div></div>
<script>
const CLIENT='fsp-demo-client',REDIRECT=location.origin+'/oidc/demo';
const out=document.getElementById('out');
const b64=buf=>btoa(String.fromCharCode(...new Uint8Array(buf))).replace(/\+/g,'-').replace(/\//g,'_').replace(/=+$/,'');
const rnd=n=>b64(crypto.getRandomValues(new Uint8Array(n)));
const show=(t,o)=>out.insertAdjacentHTML('beforeend','<h3>'+t+'</h3><pre>'+String(typeof o==='string'?o:JSON.stringify(o,null,2)).replace(/[<&]/g,c=>c==='<'?'&lt;':'&amp;')+'</pre>');
document.getElementById('reset').onclick=()=>{sessionStorage.clear();location.href='/oidc/demo'};
document.getElementById('login').onclick=async()=>{
  const verifier=rnd(48),state=rnd(12),nonce=rnd(12);
  sessionStorage.setItem('pkce',JSON.stringify({verifier,state}));
  const challenge=b64(await crypto.subtle.digest('SHA-256',new TextEncoder().encode(verifier)));
  const p=new URLSearchParams({response_type:'code',client_id:CLIENT,redirect_uri:REDIRECT,scope:'openid profile email roles sport',state,nonce,code_challenge:challenge,code_challenge_method:'S256'});
  location.href='/oidc/authorize?'+p;
};
(async()=>{
  const q=new URLSearchParams(location.search);
  if(q.get('error')){show('Отказ',q.get('error'));return}
  if(!q.get('code'))return;
  const saved=JSON.parse(sessionStorage.getItem('pkce')||'{}');
  if(q.get('state')!==saved.state){show('Ошибка','state не совпадает');return}
  const r=await fetch('/oidc/token',{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded'},body:new URLSearchParams({grant_type:'authorization_code',code:q.get('code'),redirect_uri:REDIRECT,client_id:CLIENT,code_verifier:saved.verifier})});
  const t=await r.json();
  if(!r.ok){show('Ошибка токена',t);return}
  const claims=JSON.parse(atob(t.id_token.split('.')[1].replace(/-/g,'+').replace(/_/g,'/')));
  show('id_token (payload)',claims);
  const u=await (await fetch('/oidc/userinfo',{headers:{Authorization:'Bearer '+t.access_token}})).json();
  show('userinfo',u);
  history.replaceState(null,'','/oidc/demo');
})();
</script></body></html>`

// adminPage: регистрация клиентских приложений (только роль «Федерация»).
const adminPage = `<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>ФСП ID — клиенты</title>` + baseStyle + `</head><body>
<div class="card wide"><div class="brand">ФСП ID · Администрирование</div><h1>Клиентские приложения</h1>
<p>Доступно активной роли «Федерация». Войдите на платформе, затем обновите страницу.</p>
<div id="list"></div>
<h3>Зарегистрировать приложение</h3>
<input id="cname" placeholder="Название приложения"><textarea id="uris" rows="3" style="width:100%;padding:10px;border-radius:10px;border:1px solid var(--line);background:transparent;color:var(--text)" placeholder="Redirect URI, по одному в строке (https://… или http://localhost…)"></textarea>
<label><input type="checkbox" id="conf" style="width:auto"> Конфиденциальный клиент (выдать client_secret)</label><br>
<button id="add">Зарегистрировать</button><div class="err" id="er"></div><pre id="res" style="display:none"></pre>
<p><a href="/.well-known/openid-configuration" style="color:var(--accent)">discovery</a> · <a href="/oidc/demo" style="color:var(--accent)">демо-клиент</a></p></div>
<script>
const H=()=>({'Content-Type':'application/json',Authorization:'Bearer '+localStorage.getItem('fsp_token')});
const esc=s=>String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
async function j(u,o){const r=await fetch(u,o);const d=await r.json().catch(()=>({}));if(!r.ok)throw new Error(d.error||'Ошибка '+r.status);return d}
async function load(){try{const d=await j('/api/oidc/clients',{headers:H()});list.innerHTML='<table><tr><th>client_id</th><th>Название</th><th>Redirect URI</th><th></th></tr>'+d.items.map(c=>'<tr><td><code>'+esc(c.client_id)+'</code></td><td>'+esc(c.name)+(c.confidential?' 🔒':'')+'</td><td>'+c.redirect_uris.map(esc).join('<br>')+'</td><td>'+(c.client_id==='fsp-demo-client'?'':'<button class="danger" data-id="'+esc(c.client_id)+'">Удалить</button>')+'</td></tr>').join('')+'</table>';list.querySelectorAll('button[data-id]').forEach(b=>b.onclick=async()=>{await j('/api/oidc/clients/'+b.dataset.id,{method:'DELETE',headers:H()});load()})}catch(e){list.innerHTML='<p class="err">'+esc(e.message)+'</p>'}}
add.onclick=async()=>{er.textContent='';try{const d=await j('/api/oidc/clients',{method:'POST',headers:H(),body:JSON.stringify({name:document.getElementById("cname").value,redirect_uris:uris.value.split('\n'),confidential:conf.checked})});res.style.display='block';res.textContent=JSON.stringify(d,null,2)+(d.client_secret?'\n\nСохраните client_secret — он показывается один раз.':'');load()}catch(e){er.textContent=e.message}};
load();
</script></body></html>`
