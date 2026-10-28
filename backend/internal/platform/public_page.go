package platform

import "net/http"

func servePublicPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(publicPage))
}

// Д11: публичный «Навигатор для новичка» — работает без авторизации.
const publicPage = `<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Навигатор для новичка — ФСП Оренбургской области</title>
<style>
:root{color-scheme:light dark;--bg:#f4f6fb;--card:#fff;--text:#14213d;--muted:#5b6785;--accent:#1f4fd8;--line:#d9dfee;--hero:#1f2fe0}
@media(prefers-color-scheme:dark){:root{--bg:#0e1424;--card:#171f36;--text:#eef1fb;--muted:#9aa6c7;--accent:#6b8cff;--line:#2a3558;--hero:#2a35c9}}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--text);font-family:system-ui,-apple-system,Segoe UI,Roboto,sans-serif;line-height:1.5}
.hero{background:var(--hero);color:#fff;padding:36px 16px}.wrap{max-width:1040px;margin:0 auto}
.hero h1{margin:6px 0 8px;font-size:clamp(26px,5vw,40px)}.hero p{margin:0 0 18px;max-width:640px;opacity:.92}.tag{font-weight:700;letter-spacing:.06em;font-size:13px;opacity:.85}
.btn{display:inline-block;padding:11px 20px;border-radius:12px;background:#fff;color:#1f2fe0;font-weight:700;text-decoration:none;margin:0 8px 8px 0}.btn.ghost{background:transparent;color:#fff;border:1px solid rgba(255,255,255,.6)}
section{padding:26px 16px 4px}h2{font-size:22px;margin:0 0 12px}.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(250px,1fr));gap:12px}
.card{background:var(--card);border:1px solid var(--line);border-radius:16px;padding:16px}.card h3{margin:0 0 6px;font-size:16px}.card p{margin:0;color:var(--muted);font-size:14px}
.step{display:flex;gap:12px}.num{flex:none;width:32px;height:32px;border-radius:50%;background:var(--accent);color:#fff;display:grid;place-items:center;font-weight:700}
.meta{color:var(--muted);font-size:13px;margin-top:6px}.empty{color:var(--muted)}footer{padding:28px 16px;color:var(--muted);font-size:14px}a{color:var(--accent)}
</style></head><body>
<div class="hero"><div class="wrap"><div class="tag">ФСП · ОРЕНБУРГСКАЯ ОБЛАСТЬ</div><h1>Спортивное программирование — с чего начать</h1>
<p>Официальный вид спорта: соревнования по алгоритмам, продуктовой разработке, информационной безопасности, ИИ, робототехнике и беспилотным системам. Разберёмся, куда идти и как стать участником.</p>
<a class="btn" href="/">Войти или зарегистрироваться</a><a class="btn ghost" href="#events">Ближайшие соревнования</a></div></div>
<main class="wrap">
<section><h2>Что это за вид спорта</h2><div class="card"><p>Спортсмены решают задачи на время и по правилам, утверждённым Минспортом России. Есть разряды и звания, чемпионаты и первенства разных уровней, возрастные группы и сборная региона. Занятия и соревнования проходят очно, онлайн и в смешанном формате.</p></div></section>
<section><h2>Шесть дисциплин</h2><div class="grid" id="disc"></div></section>
<section><h2>С чего начать</h2><div class="grid">
<div class="card step"><div class="num">1</div><div><h3>Выберите дисциплину</h3><p>Определитесь с направлением: от олимпиадных алгоритмов до робототехники.</p></div></div>
<div class="card step"><div class="num">2</div><div><h3>Найдите, где заниматься</h3><p>Выберите организацию в вашем муниципалитете (список ниже) и тренера.</p></div></div>
<div class="card step"><div class="num">3</div><div><h3>Зарегистрируйтесь</h3><p>Создайте аккаунт на платформе, дайте согласие на обработку данных (для несовершеннолетних — согласие родителя).</p></div></div>
<div class="card step"><div class="num">4</div><div><h3>Подайте заявку на старт</h3><p>Платформа сама покажет соревнования, подходящие по возрасту и дисциплине.</p></div></div>
<div class="card step"><div class="num">5</div><div><h3>Пройдите антидопинговый курс</h3><p>Онлайн-курс РУСАДА обязателен для спортсменов: <a href="https://rusada.ru" target="_blank" rel="noreferrer">rusada.ru</a>.</p></div></div>
</div></section>
<section id="events"><h2>Ближайшие соревнования</h2><div class="grid" id="events-list"><p class="empty">Загрузка…</p></div></section>
<section><h2>Где заниматься в области</h2><div class="grid" id="orgs"><p class="empty">Загрузка…</p></div></section>
<section><h2>Как зарегистрироваться на платформе</h2><div class="card"><p>Нажмите «Войти или зарегистрироваться» на главной странице, выберите роль — спортсмен, тренер или судья — и заполните профиль. Роль «Федерация» выдаёт администратор. Один аккаунт может иметь несколько ролей.</p></div></section>
</main>
<footer><div class="wrap">© ФСП · Оренбургская область · <a href="/">На платформу</a></div></footer>
<script>
const esc=s=>String(s==null?'':s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const fmt=d=>{const x=new Date(d);return isNaN(x)?'':x.toLocaleString('ru-RU',{day:'numeric',month:'long',hour:'2-digit',minute:'2-digit'})};
fetch('/api/public/home').then(r=>r.json()).then(d=>{
  document.getElementById('disc').innerHTML=(d.disciplines||[]).map(x=>'<div class="card"><h3>'+esc(x.replace('Программирование ',''))+'</h3></div>').join('');
  const ev=d.events||[];
  document.getElementById('events-list').innerHTML=ev.length?ev.map(e=>'<div class="card"><h3>'+esc(e.title)+'</h3><p>'+esc(e.discipline)+'</p><div class="meta">'+esc(fmt(e.starts_at))+' · '+esc(e.location)+'<br>'+esc(e.age_group)+' · '+esc(e.event_format||'')+'</div></div>').join(''):'<p class="empty">Ближайших соревнований пока нет — загляните позже.</p>';
}).catch(()=>{document.getElementById('events-list').innerHTML='<p class="empty">Не удалось загрузить данные.</p>'});
fetch('/api/public/organizations').then(r=>r.json()).then(d=>{
  const it=d.items||[];
  document.getElementById('orgs').innerHTML=it.length?it.map(o=>'<div class="card"><h3>'+esc(o.name)+'</h3><p>'+esc(o.type)+' · '+esc(o.municipality)+'</p><div class="meta">'+esc(o.disciplines||o.programs)+'</div></div>').join(''):'<p class="empty">Список организаций скоро появится.</p>';
}).catch(()=>{document.getElementById('orgs').innerHTML='<p class="empty">Не удалось загрузить данные.</p>'});
</script></body></html>`
