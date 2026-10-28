package extapi

import "net/http"

func servePage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Frame-Options", "DENY")
	_, _ = w.Write([]byte(consolePage))
}

const consolePage = `<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Интеграции — ФСП</title>
<style>
:root{color-scheme:light dark;--bg:#f4f6fb;--card:#fff;--text:#14213d;--muted:#5b6785;--accent:#1f4fd8;--line:#d9dfee;--ok:#1f9d55;--bad:#d64545}
@media(prefers-color-scheme:dark){:root{--bg:#0e1424;--card:#171f36;--text:#eef1fb;--muted:#9aa6c7;--accent:#6b8cff;--line:#2a3558}}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--text);font-family:system-ui,-apple-system,Segoe UI,Roboto,sans-serif;padding:16px}
main{max-width:980px;margin:0 auto;display:grid;gap:16px}.card{background:var(--card);border:1px solid var(--line);border-radius:16px;padding:20px}
h1{font-size:22px;margin:0}h2{font-size:17px;margin:0 0 8px}p{color:var(--muted);margin:4px 0}.brand{font-weight:700;color:var(--accent);letter-spacing:.04em}
input,select{padding:9px 11px;border-radius:10px;border:1px solid var(--line);background:transparent;color:var(--text);font:inherit;width:100%;margin:4px 0}
button{padding:8px 14px;border-radius:10px;border:0;background:var(--accent);color:#fff;font:inherit;cursor:pointer}button.ghost{background:transparent;color:var(--text);border:1px solid var(--line)}button.danger{background:var(--bad)}
.table{overflow-x:auto}table{width:100%;border-collapse:collapse;font-size:14px}td,th{padding:7px;border-bottom:1px solid var(--line);text-align:left;vertical-align:top}
.chk{display:flex;flex-wrap:wrap;gap:4px 14px;margin:6px 0}.chk label{font-size:14px;display:flex;gap:6px;align-items:center}.chk input{width:auto;margin:0}
.ok{color:var(--ok)}.bad{color:var(--bad)}.err{color:var(--bad)}pre{overflow:auto;background:rgba(127,127,127,.12);padding:12px;border-radius:10px;font-size:13px;white-space:pre-wrap;word-break:break-all}
.row{display:flex;gap:8px;flex-wrap:wrap;align-items:center}code{font-size:13px}.grid{display:grid;grid-template-columns:1fr 1fr;gap:16px}@media(max-width:760px){.grid{grid-template-columns:1fr}}
</style></head><body><main>
<div class="card"><div class="brand">ФСП · Оренбургская область</div><h1>Интеграции с внешними системами</h1>
<p>Доступно активной роли «Федерация». Войдите на платформе и обновите страницу. <a href="/api/v1/openapi.yaml" style="color:var(--accent)">Описание API (OpenAPI)</a> · <a href="/" style="color:var(--accent)">На платформу</a></p><div class="err" id="auth"></div></div>
<div id="secret" class="card" style="display:none"></div>
<div class="grid">
<div class="card"><h2>Вебхуки</h2><p>Платформа отправляет подписанный POST при событиях; 3 попытки доставки, журнал ниже.</p>
<input id="wn" placeholder="Название (например, «Соревновательная платформа»)"><input id="wu" placeholder="URL получателя, https://…">
<div class="chk" id="wev"></div><button id="wadd">Добавить подписку</button><div class="err" id="werr"></div></div>
<div class="card"><h2>API-токены</h2><p>Токен даёт внешней системе доступ только к выбранным правам. Показывается один раз.</p>
<input id="tn" placeholder="Название системы"><div class="chk" id="tsc"></div><button id="tadd">Выдать токен</button><div class="err" id="terr"></div></div></div>
<div class="card table"><h2>Подписки</h2><div id="wl"></div></div>
<div class="card table"><h2>Токены</h2><div id="tl"></div></div>
<div class="card table"><h2>Журнал доставок</h2><div id="dl"></div></div>
</main>
<script>
const $=id=>document.getElementById(id);
const esc=s=>String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const H=()=>({'Content-Type':'application/json',Authorization:'Bearer '+localStorage.getItem('fsp_token')});
async function j(u,o){const r=await fetch(u,Object.assign({headers:H()},o||{}));const d=await r.json().catch(()=>({}));if(!r.ok)throw new Error(d.error||'Ошибка '+r.status);return d}
function chk(box,items,name){box.innerHTML=items.map(i=>'<label><input type="checkbox" name="'+name+'" value="'+esc(i.code)+'">'+esc(i.label)+'</label>').join('')}
const picked=name=>[...document.querySelectorAll('input[name='+name+']:checked')].map(x=>x.value);
function showSecret(title,label,value,hint){const b=$('secret');b.style.display='block';b.innerHTML='<h2>'+esc(title)+'</h2><p>'+esc(hint)+'</p><pre>'+esc(label)+': '+esc(value)+'</pre>'}
async function load(){
  try{
    if(!$('wev').children.length){const c=await j('/api/integrations/catalog');chk($('wev'),c.events.filter(e=>e.code!=='webhook.test'),'ev');chk($('tsc'),c.scopes,'sc')}
    const [w,t,d]=await Promise.all([j('/api/integrations/webhooks'),j('/api/integrations/tokens'),j('/api/integrations/deliveries')]);
    $('auth').textContent='';
    $('wl').innerHTML=w.items.length?'<table><tr><th>Название</th><th>URL</th><th>События</th><th>Доставки</th><th></th></tr>'+w.items.map(x=>'<tr><td>'+esc(x.name)+(x.active?'':' <i>(выключен)</i>')+'</td><td><code>'+esc(x.url)+'</code></td><td>'+esc(x.events.join(', '))+'</td><td>'+x.delivered+' / '+x.deliveries+'</td><td class="row"><button class="ghost" onclick="test('+x.id+')">Проверить</button><button class="ghost" onclick="tog('+x.id+')">'+(x.active?'Выключить':'Включить')+'</button><button class="danger" onclick="delw('+x.id+')">Удалить</button></td></tr>').join('')+'</table>':'<p>Подписок пока нет.</p>';
    $('tl').innerHTML=t.items.length?'<table><tr><th>Система</th><th>Токен</th><th>Права</th><th>Использован</th><th></th></tr>'+t.items.map(x=>'<tr><td>'+esc(x.name)+'</td><td><code>'+esc(x.prefix)+'…</code></td><td>'+esc(x.scopes.join(' '))+'</td><td>'+esc(x.last_used_at||'—')+'</td><td>'+(x.revoked?'<i>отозван</i>':'<button class="danger" onclick="rev('+x.id+')">Отозвать</button>')+'</td></tr>').join('')+'</table>':'<p>Токенов пока нет.</p>';
    $('dl').innerHTML=d.items.length?'<table><tr><th>Время</th><th>Подписка</th><th>Событие</th><th>Результат</th><th></th></tr>'+d.items.map(x=>'<tr><td>'+esc(x.created_at)+'</td><td>'+esc(x.endpoint)+'</td><td><code>'+esc(x.event)+'</code></td><td class="'+(x.success?'ok':'bad')+'">'+(x.success?'доставлено, HTTP '+x.status_code:'ошибка: '+esc(x.error||'ожидание')+' (попыток: '+x.attempts+')')+'</td><td>'+(x.success?'':'<button class="ghost" onclick="redo('+x.id+')">Повторить</button>')+'</td></tr>').join('')+'</table>':'<p>Событий пока не было.</p>';
  }catch(e){needFed(e)}
}
$('wadd').onclick=async()=>{$('werr').textContent='';try{const r=await j('/api/integrations/webhooks',{method:'POST',body:JSON.stringify({name:$('wn').value,url:$('wu').value,events:picked('ev')})});showSecret('Подписка создана','Секрет подписи',r.secret,'Сохраните секрет: он нужен получателю для проверки подписи и больше не показывается.');$('wn').value='';$('wu').value='';load()}catch(e){$('werr').textContent=e.message}};
$('tadd').onclick=async()=>{$('terr').textContent='';try{const r=await j('/api/integrations/tokens',{method:'POST',body:JSON.stringify({name:$('tn').value,scopes:picked('sc')})});showSecret('Токен выдан','API-токен',r.token,'Сохраните токен: в базе хранится только его хеш, повторно показать его нельзя.');$('tn').value='';load()}catch(e){$('terr').textContent=e.message}};
async function needFed(e){
 const box=$('auth');box.textContent=e.message+'. ';
 try{const me=await j('/api/auth/me');
  if((me.roles||[]).includes('federation')){const bt=document.createElement('button');bt.textContent='Переключить на роль «Федерация»';
   bt.onclick=async()=>{try{await j('/api/auth/switch-role',{method:'POST',body:JSON.stringify({role:'federation'})});location.reload()}catch(x){box.textContent=x.message}};box.appendChild(bt)}
  else box.appendChild(document.createTextNode('У этого аккаунта нет роли «Федерация» — войдите на платформе федерацией.'))}
 catch(_){box.appendChild(document.createTextNode('Войдите на платформе под ролью «Федерация» и обновите страницу.'))}}
async function act(u,m){try{return await j(u,{method:m||'POST'})}catch(e){alert(e.message)}finally{load()}}
async function test(id){const r=await act('/api/integrations/webhooks/'+id+'/test');if(r)alert(r.success?'Доставлено, HTTP '+r.status_code:'Ошибка: '+(r.error||r.status_code))}
async function redo(id){const r=await act('/api/integrations/deliveries/'+id+'/redeliver');if(r)alert(r.success?'Доставлено':'Ошибка: '+(r.error||r.status_code))}
const tog=id=>act('/api/integrations/webhooks/'+id+'/toggle');
const delw=id=>confirm('Удалить подписку?')&&act('/api/integrations/webhooks/'+id,'DELETE');
const rev=id=>confirm('Отозвать токен? Система потеряет доступ.')&&act('/api/integrations/tokens/'+id,'DELETE');
load();setInterval(load,15000);
</script></body></html>`
