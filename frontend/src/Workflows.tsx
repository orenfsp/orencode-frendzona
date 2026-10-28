import { useCallback, useEffect, useMemo, useState, type FormEvent, type ReactNode } from 'react'

type Msg = (message: string) => void
type Comp = { id: number; title: string; discipline: string; status: string; starts_at: string }
type Ath = { id: number; full_name: string; discipline: string; organization: string }
type Me = { full_name: string; athlete_id?: number; coach_id?: number; judge_id?: number }

async function xapi<T>(path: string, init?: RequestInit): Promise<T> {
  const token = localStorage.getItem('fsp_token')
  const response = await fetch(path, { ...init, headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) } })
  if (!response.ok) {
    const payload = await response.json().catch(() => ({}))
    throw new Error(payload.error || `Ошибка запроса: ${response.status}`)
  }
  return response.json() as Promise<T>
}
const post = <T,>(path: string, body: unknown) => xapi<T>(path, { method: 'POST', body: JSON.stringify(body) })
const fail = (onMessage: Msg) => (e: unknown) => onMessage(e instanceof Error ? e.message : 'Не удалось выполнить действие')

async function download(path: string, name: string, onMessage: Msg) {
  try {
    const token = localStorage.getItem('fsp_token')
    const res = await fetch(path, { headers: token ? { Authorization: `Bearer ${token}` } : {} })
    if (!res.ok) throw new Error('Не удалось скачать файл')
    const url = URL.createObjectURL(await res.blob())
    const a = document.createElement('a'); a.href = url; a.download = name; a.click(); URL.revokeObjectURL(url)
  } catch (e) { fail(onMessage)(e) }
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return <label className="ex-field"><span>{label}</span>{children}</label>
}
const Empty = ({ text }: { text: string }) => <div className="ex-empty">{text}</div>
const day = (iso?: string) => (iso || '').slice(0, 10)

const statusInfo: Record<string, [string, string]> = {
  pending: ['На рассмотрении', 'status-draft'], approved: ['Одобрена', 'status-open'], rejected: ['Отклонена', 'status-done'],
  active: ['Активна', 'status-open'], paused: ['Приостановлена', 'status-planned'], completed: ['Завершена', 'status-done'],
}
function Status({ value, pendingText }: { value: string; pendingText?: string }) {
  const [text, cls] = statusInfo[value] || [value, 'status-planned']
  return <span className={`status-pill ${cls}`}>{value === 'pending' && pendingText ? pendingText : text}</span>
}
function daysLeft(date?: string) { return date ? Math.ceil((new Date(date).getTime() - Date.now()) / 86400000) : null }
function Expiry({ date }: { date?: string }) {
  const days = daysLeft(date)
  if (days === null) return <span>—</span>
  const cls = days < 0 ? 'status-done' : days <= 90 ? 'status-draft' : 'status-open'
  const text = days < 0 ? `истёк ${date}` : days <= 90 ? `до ${date} · осталось ${days} дн.` : `до ${date}`
  return <span className={`status-pill ${cls}`}>{text}</span>
}

const DISCIPLINES = ['Программирование алгоритмическое', 'Программирование продуктовое', 'Программирование систем информационной безопасности', 'Программирование систем искусственного интеллекта', 'Программирование робототехники', 'Программирование беспилотных авиационных систем']
const RANKS = ['III юношеский разряд', 'II юношеский разряд', 'I юношеский разряд', 'III спортивный разряд', 'II спортивный разряд', 'I спортивный разряд', 'Кандидат в мастера спорта']
const CATEGORIES = ['Судья третьей категории', 'Судья второй категории', 'Судья первой категории', 'Судья всероссийской категории']
const POSITIONS = ['Главный судья', 'Заместитель главного судьи', 'Главный секретарь', 'Судья']

// ---------- С4: привязка тренера и спортсмена ----------
type Link = { id: number; coach: string; athlete_id: number; athlete: string; discipline: string; status: string; created_at: string }

export function LinksView({ role, onMessage }: { role: string; onMessage: Msg }) {
  const [links, setLinks] = useState<Link[]>([])
  const [athletes, setAthletes] = useState<Ath[]>([])
  const [query, setQuery] = useState('')
  const [athleteId, setAthleteId] = useState('')
  const [discipline, setDiscipline] = useState(DISCIPLINES[0])
  const load = useCallback(async () => {
    try {
      setLinks((await xapi<{ items: Link[] }>('/api/coach-links')).items)
      if (role === 'coach') setAthletes((await xapi<{ items: Ath[] }>('/api/athletes')).items)
    } catch (e) { fail(onMessage)(e) }
  }, [role, onMessage])
  useEffect(() => { void load() }, [load])
  const found = useMemo(() => athletes.filter(a => `${a.full_name} ${a.organization}`.toLowerCase().includes(query.toLowerCase())).slice(0, 50), [athletes, query])
  function pickAthlete(id: string) {
    setAthleteId(id)
    const a = athletes.find(x => String(x.id) === id)
    if (a && DISCIPLINES.includes(a.discipline)) setDiscipline(a.discipline)
  }
  async function send(e: FormEvent) {
    e.preventDefault()
    if (!athleteId) { onMessage('Выберите спортсмена'); return }
    try { await post('/api/coach-links', { athlete_id: Number(athleteId), discipline }); onMessage('Запрос отправлен. Спортсмен должен подтвердить привязку.'); setAthleteId(''); await load() } catch (err) { fail(onMessage)(err) }
  }
  async function decide(id: number, status: string, text: string) {
    try { await post(`/api/coach-links/${id}/decide`, { status }); onMessage(text); await load() } catch (err) { fail(onMessage)(err) }
  }
  return <div className="ex-stack">
    {role === 'coach' && <section className="panel ex-card"><div className="panel-kicker">ПРИВЯЗКА</div><h3>Запросить привязку спортсмена</h3>
      <form className="ex-form" onSubmit={send}>
        <Field label="Поиск по ФИО или организации"><input value={query} onChange={e => setQuery(e.target.value)} placeholder="Начните вводить" /></Field>
        <Field label="Спортсмен"><select value={athleteId} onChange={e => pickAthlete(e.target.value)}><option value="">Выберите спортсмена</option>{found.map(a => <option key={a.id} value={a.id}>{a.full_name} — {a.organization}</option>)}</select></Field>
        <Field label="Дисциплина"><select value={discipline} onChange={e => setDiscipline(e.target.value)}>{DISCIPLINES.map(d => <option key={d}>{d}</option>)}</select></Field>
        <button className="button button-primary">Отправить запрос</button>
      </form></section>}
    <section className="panel ex-card"><div className="panel-kicker">СВЯЗИ</div><h3>{role === 'athlete' ? 'Мои тренеры' : role === 'coach' ? 'Мои спортсмены' : 'Все связи тренеров и спортсменов'}</h3>
      {links.length === 0 ? <Empty text={role === 'athlete' ? 'Запросов от тренеров пока нет.' : 'Связей пока нет.'} /> :
        <div className="table-scroll"><table className="ex-table"><thead><tr><th>Тренер</th><th>Спортсмен</th><th>Дисциплина</th><th>Дата запроса</th><th>Статус</th>{role === 'athlete' && <th />}</tr></thead><tbody>
          {links.map(l => <tr key={l.id}><td>{l.coach}</td><td>{l.athlete}</td><td>{l.discipline}</td><td>{day(l.created_at)}</td><td><Status value={l.status} pendingText="Ожидает подтверждения" /></td>
            {role === 'athlete' && <td><div className="ex-btns">
              {l.status === 'pending' && <><button className="button button-primary" onClick={() => void decide(l.id, 'active', 'Привязка подтверждена')}>Подтвердить</button><button className="button" onClick={() => void decide(l.id, 'completed', 'Запрос отклонён')}>Отклонить</button></>}
              {l.status === 'active' && <><button className="button" onClick={() => void decide(l.id, 'paused', 'Связь приостановлена')}>Приостановить</button><button className="button" onClick={() => void decide(l.id, 'completed', 'Связь завершена')}>Завершить</button></>}
              {l.status === 'paused' && <button className="button" onClick={() => void decide(l.id, 'active', 'Связь возобновлена')}>Возобновить</button>}
            </div></td>}</tr>)}
        </tbody></table></div>}
    </section>
  </div>
}

// ---------- С5: результаты, протокол, история ----------
type Participant = { athlete_id: number; full_name: string; organization: string; place: number; score: number }
type HistoryItem = { competition_id: number; competition: string; discipline: string; sport_status: string; starts_at: string; place: number; score: number; points: number; athlete: string }

export function ResultsView({ role, competitions, onMessage }: { role: string; competitions: Comp[]; onMessage: Msg }) {
  const canEnter = role === 'federation' || role === 'judge'
  const [options, setOptions] = useState<{ id: number; title: string }[]>([])
  const [cid, setCid] = useState('')
  const [rows, setRows] = useState<Participant[]>([])
  const [draft, setDraft] = useState<Record<number, { place: string; score: string }>>({})
  const [history, setHistory] = useState<HistoryItem[]>([])
  const loadHistory = useCallback(async () => {
    if (role === 'judge') return
    try { setHistory((await xapi<{ items: HistoryItem[] }>('/api/results')).items) } catch (e) { fail(onMessage)(e) }
  }, [role, onMessage])
  useEffect(() => { void loadHistory() }, [loadHistory])
  useEffect(() => {
    if (!canEnter) return
    if (role === 'federation') { setOptions(competitions.filter(c => c.status !== 'Черновик').map(c => ({ id: c.id, title: c.title }))); return }
    xapi<{ items: { competition_id: number; competition: string }[] }>('/api/assignments')
      .then(r => setOptions(r.items.map(i => ({ id: i.competition_id, title: i.competition })))).catch(fail(onMessage))
  }, [canEnter, role, competitions, onMessage])
  const loadParticipants = useCallback(async (id: string) => {
    setCid(id); setRows([]); setDraft({})
    if (!id) return
    try {
      const r = await xapi<{ items: Participant[] }>(`/api/competitions/${id}/participants`)
      setRows(r.items)
      setDraft(Object.fromEntries(r.items.map(p => [p.athlete_id, { place: p.place ? String(p.place) : '', score: p.score ? String(p.score) : '' }] as [number, { place: string; score: string }])))
    } catch (e) { fail(onMessage)(e) }
  }, [onMessage])
  async function save(e: FormEvent) {
    e.preventDefault()
    const items = rows.map(p => ({ athlete_id: p.athlete_id, place: Number(draft[p.athlete_id]?.place) || 0, score: Number(draft[p.athlete_id]?.score) || 0 })).filter(i => i.place > 0)
    if (items.length === 0) { onMessage('Укажите место хотя бы одному участнику'); return }
    try {
      await post(`/api/competitions/${cid}/results`, { items })
      onMessage('Результаты сохранены. Протокол сформирован, история спортсменов обновлена.')
      await loadParticipants(cid); await loadHistory()
    } catch (err) { fail(onMessage)(err) }
  }
  const setField = (id: number, key: 'place' | 'score', value: string) => setDraft(d => ({ ...d, [id]: { ...(d[id] || { place: '', score: '' }), [key]: value } }))
  return <div className="ex-stack">
    {canEnter && <section className="panel ex-card"><div className="panel-kicker">ПРОТОКОЛ</div><h3>Внести результаты соревнования</h3>
      {options.length === 0 ? <Empty text={role === 'judge' ? 'Вас пока не назначили ни на одно соревнование. Назначение делает Федерация.' : 'Нет опубликованных соревнований.'} /> : <>
        <div className="ex-form"><Field label="Соревнование"><select value={cid} onChange={e => void loadParticipants(e.target.value)}><option value="">Выберите соревнование</option>{options.map(o => <option key={o.id} value={o.id}>{o.title}</option>)}</select></Field></div>
        {cid && (rows.length === 0 ? <Empty text="В протокол попадают участники с одобренными заявками. Таких пока нет." /> :
          <form onSubmit={save}><div className="table-scroll"><table className="ex-table"><thead><tr><th>Участник</th><th>Организация</th><th>Место</th><th>Баллы</th></tr></thead><tbody>
            {rows.map(p => <tr key={p.athlete_id}><td>{p.full_name}</td><td>{p.organization}</td>
              <td><input type="number" min={1} style={{ width: 80 }} value={draft[p.athlete_id]?.place || ''} onChange={e => setField(p.athlete_id, 'place', e.target.value)} /></td>
              <td><input type="number" min={0} style={{ width: 100 }} value={draft[p.athlete_id]?.score || ''} onChange={e => setField(p.athlete_id, 'score', e.target.value)} /></td></tr>)}
          </tbody></table></div>
          <div className="ex-inline"><button className="button button-primary">Сохранить результаты</button>
            <button type="button" className="button" onClick={() => void download(`/api/competitions/${cid}/protocol`, `protocol-${cid}.xlsx`, onMessage)}>Протокол XLSX</button>
            <button type="button" className="button" onClick={() => void download(`/api/competitions/${cid}/protocol?format=docx`, `protocol-${cid}.docx`, onMessage)}>Протокол DOCX</button></div></form>)}
      </>}
    </section>}
    {role !== 'judge' && <section className="panel ex-card"><div className="panel-kicker">ИСТОРИЯ</div><h3>{role === 'athlete' ? 'История моих выступлений' : role === 'coach' ? 'Результаты подопечных' : 'Все результаты'}</h3>
      {history.length === 0 ? <Empty text="Результатов пока нет." /> :
        <div className="table-scroll"><table className="ex-table"><thead><tr><th>Соревнование</th><th>Дисциплина</th><th>Статус</th><th>Дата</th>{role !== 'athlete' && <th>Спортсмен</th>}<th>Место</th><th>Баллы</th><th>Рейтинг</th><th>Протокол</th></tr></thead><tbody>
          {history.map((h, i) => <tr key={i}><td>{h.competition}</td><td>{h.discipline}</td><td>{h.sport_status}</td><td>{day(h.starts_at)}</td>{role !== 'athlete' && <td>{h.athlete}</td>}<td>{h.place}</td><td>{h.score}</td><td>{h.points}</td>
            <td><button className="button" onClick={() => void download(`/api/competitions/${h.competition_id}/protocol`, `protocol-${h.competition_id}.xlsx`, onMessage)}>XLSX</button></td></tr>)}
        </tbody></table></div>}
    </section>}
  </div>
}

// ---------- С6: судейская коллегия и категории ----------
type Judge = { id: number; full_name: string; category: string; category_expires_at: string; organization: string }
type Assign = { id: number; competition_id: number; competition: string; starts_at: string; judge: string; position: string; category: string; category_expires_at: string }
type CatApp = { id: number; judge: string; requested_category: string; kind: string; status: string; decision_note: string; created_at: string }

export function JudgingView({ role, user, competitions, onMessage }: { role: string; user: Me; competitions: Comp[]; onMessage: Msg }) {
  const isFed = role === 'federation'
  const [judges, setJudges] = useState<Judge[]>([])
  const [assigns, setAssigns] = useState<Assign[]>([])
  const [apps, setApps] = useState<CatApp[]>([])
  const [cid, setCid] = useState('')
  const [judgeId, setJudgeId] = useState('')
  const [position, setPosition] = useState(POSITIONS[0])
  const [category, setCategory] = useState(CATEGORIES[0])
  const [kind, setKind] = useState('присвоение')
  const [notes, setNotes] = useState<Record<number, string>>({})
  const load = useCallback(async () => {
    try {
      const [j, a, c] = await Promise.all([xapi<{ items: Judge[] }>('/api/judges'), xapi<{ items: Assign[] }>('/api/assignments'), xapi<{ items: CatApp[] }>('/api/categories')])
      setJudges(j.items); setAssigns(a.items); setApps(c.items)
    } catch (e) { fail(onMessage)(e) }
  }, [onMessage])
  useEffect(() => { void load() }, [load])
  const me = judges.find(j => j.id === user.judge_id)
  async function assign(e: FormEvent) {
    e.preventDefault()
    if (!cid || !judgeId) { onMessage('Выберите соревнование и судью'); return }
    try { await post(`/api/competitions/${cid}/judges`, { judge_id: Number(judgeId), position }); onMessage('Судья назначен в коллегию соревнования'); await load() } catch (err) { fail(onMessage)(err) }
  }
  async function apply(e: FormEvent) {
    e.preventDefault()
    try { await post('/api/categories', { requested_category: category, kind }); onMessage('Заявка отправлена в Федерацию'); await load() } catch (err) { fail(onMessage)(err) }
  }
  async function decide(id: number, status: string) {
    try { await post(`/api/categories/${id}/decide`, { status, note: notes[id] || '' }); onMessage(status === 'approved' ? 'Категория присвоена, срок действия 4 года' : 'Заявка отклонена'); await load() } catch (err) { fail(onMessage)(err) }
  }
  return <div className="ex-stack">
    {!isFed && <section className="panel ex-card"><div className="panel-kicker">КАТЕГОРИЯ</div><h3>Моя судейская категория</h3>
      {me ? <div className="ex-row"><span>{me.category || 'Категория не присвоена'}</span><Expiry date={me.category_expires_at} /></div> : <Empty text="Данные о категории не найдены." />}
    </section>}
    {isFed && <section className="panel ex-card"><div className="panel-kicker">КОЛЛЕГИЯ</div><h3>Назначить судью на соревнование</h3>
      <form className="ex-form" onSubmit={assign}>
        <Field label="Соревнование"><select value={cid} onChange={e => setCid(e.target.value)}><option value="">Выберите соревнование</option>{competitions.map(c => <option key={c.id} value={c.id}>{c.title}</option>)}</select></Field>
        <Field label="Судья"><select value={judgeId} onChange={e => setJudgeId(e.target.value)}><option value="">Выберите судью</option>{judges.map(j => <option key={j.id} value={j.id}>{j.full_name} — {j.category || 'без категории'}</option>)}</select></Field>
        <Field label="Судейская должность"><select value={position} onChange={e => setPosition(e.target.value)}>{POSITIONS.map(p => <option key={p}>{p}</option>)}</select></Field>
        <button className="button button-primary">Назначить</button>
      </form></section>}
    <section className="panel ex-card"><div className="panel-kicker">НАЗНАЧЕНИЯ</div><h3>{isFed ? 'Судейские коллегии соревнований' : 'Мои назначения'}</h3>
      {assigns.length === 0 ? <Empty text={isFed ? 'Назначений пока нет.' : 'Вас пока не назначили на соревнования.'} /> :
        <div className="table-scroll"><table className="ex-table"><thead><tr><th>Соревнование</th><th>Дата</th>{isFed && <th>Судья</th>}<th>Должность</th><th>Категория</th></tr></thead><tbody>
          {assigns.map(a => <tr key={a.id}><td>{a.competition}</td><td>{day(a.starts_at)}</td>{isFed && <td>{a.judge}</td>}<td>{a.position}</td><td>{a.category || '—'} <Expiry date={a.category_expires_at} /></td></tr>)}
        </tbody></table></div>}
    </section>
    {isFed && <section className="panel ex-card"><div className="panel-kicker">РЕЕСТР СУДЕЙ</div><h3>Категории и сроки действия</h3>
      {judges.length === 0 ? <Empty text="Судей в реестре нет." /> : <div className="table-scroll"><table className="ex-table"><thead><tr><th>Судья</th><th>Организация</th><th>Категория</th><th>Срок</th></tr></thead><tbody>
        {judges.map(j => <tr key={j.id}><td>{j.full_name}</td><td>{j.organization}</td><td>{j.category || '—'}</td><td><Expiry date={j.category_expires_at} /></td></tr>)}
      </tbody></table></div>}
    </section>}
    {role === 'judge' && <section className="panel ex-card"><div className="panel-kicker">ЗАЯВКА</div><h3>Присвоение или подтверждение категории</h3>
      <form className="ex-form" onSubmit={apply}>
        <Field label="Категория"><select value={category} onChange={e => setCategory(e.target.value)}>{CATEGORIES.map(c => <option key={c}>{c}</option>)}</select></Field>
        <Field label="Тип заявки"><select value={kind} onChange={e => setKind(e.target.value)}><option value="присвоение">Присвоение</option><option value="подтверждение">Подтверждение</option></select></Field>
        <button className="button button-primary">Подать заявку</button>
      </form></section>}
    <section className="panel ex-card"><div className="panel-kicker">ЗАЯВКИ НА КАТЕГОРИЮ</div><h3>{isFed ? 'Заявки судей' : 'Мои заявки'}</h3>
      {apps.length === 0 ? <Empty text="Заявок пока нет." /> : apps.map(a => <div className="ex-item" key={a.id}>
        <div className="ex-item-head"><b>{a.judge} — {a.requested_category} ({a.kind})</b><Status value={a.status} /></div>
        <p>Подана {day(a.created_at)}{a.decision_note ? ` · Комментарий Федерации: ${a.decision_note}` : ''}</p>
        {isFed && a.status === 'pending' && <div className="ex-decide"><input placeholder="Комментарий (необязательно)" value={notes[a.id] || ''} onChange={e => setNotes(n => ({ ...n, [a.id]: e.target.value }))} />
          <button className="button button-primary" onClick={() => void decide(a.id, 'approved')}>Присвоить</button><button className="button" onClick={() => void decide(a.id, 'rejected')}>Отклонить</button></div>}
      </div>)}
    </section>
  </div>
}

// ---------- С7: заявки на разряд ----------
type RankApp = { id: number; athlete: string; requested_rank: string; place: number; status: string; competition: string; decision_note: string; created_at: string; rank_expires_at: string }

export function RanksView({ role, user, onMessage }: { role: string; user: Me; onMessage: Msg }) {
  const [ranks, setRanks] = useState<RankApp[]>([])
  const [links, setLinks] = useState<Link[]>([])
  const [history, setHistory] = useState<HistoryItem[]>([])
  const [athleteName, setAthleteName] = useState('')
  const [athleteId, setAthleteId] = useState('')
  const [cid, setCid] = useState('')
  const [rank, setRank] = useState(RANKS[0])
  const [place, setPlace] = useState('')
  const [notes, setNotes] = useState<Record<number, { note: string; order: string }>>({})
  const load = useCallback(async () => {
    try {
      const [r, h] = await Promise.all([xapi<{ items: RankApp[] }>('/api/ranks'), role === 'federation' ? Promise.resolve({ items: [] as HistoryItem[] }) : xapi<{ items: HistoryItem[] }>('/api/results')])
      setRanks(r.items); setHistory(h.items)
      if (role !== 'federation') setLinks((await xapi<{ items: Link[] }>('/api/coach-links')).items)
    } catch (e) { fail(onMessage)(e) }
  }, [role, onMessage])
  useEffect(() => { void load() }, [load])
  const activeLinks = links.filter(l => l.status === 'active')
  const myAthleteId = role === 'athlete' ? String(user.athlete_id || '') : athleteId
  const myAthleteName = role === 'athlete' ? user.full_name : athleteName
  const blocked = role === 'athlete' && activeLinks.length > 0
  const results = role === 'athlete' ? history : history.filter(h => h.athlete === myAthleteName)
  const coachAthletes = useMemo(() => Array.from(new Map(activeLinks.map(l => [l.athlete_id, l.athlete] as [number, string])).entries()), [activeLinks])
  function pickAthlete(id: string) {
    setAthleteId(id); setCid(''); setPlace('')
    setAthleteName(coachAthletes.find(([aid]) => String(aid) === id)?.[1] || '')
  }
  function pickCompetition(id: string) {
    setCid(id)
    const r = results.find(x => String(x.competition_id) === id)
    setPlace(r ? String(r.place) : '')
  }
  async function apply(e: FormEvent) {
    e.preventDefault()
    if (!myAthleteId || !cid) { onMessage('Выберите спортсмена и соревнование, на основании которого запрашивается разряд'); return }
    try {
      await post('/api/ranks', { athlete_id: Number(myAthleteId), competition_id: Number(cid), requested_rank: rank, place: Number(place) || 0 })
      onMessage('Заявка на разряд отправлена в Федерацию'); setCid(''); setPlace(''); await load()
    } catch (err) { fail(onMessage)(err) }
  }
  async function decide(id: number, status: string) {
    const d = notes[id] || { note: '', order: '' }
    try { await post(`/api/ranks/${id}/decide`, { status, note: d.note, order: d.order }); onMessage(status === 'approved' ? 'Разряд присвоен, срок действия 2 года' : 'Заявка отклонена'); await load() } catch (err) { fail(onMessage)(err) }
  }
  const setDraft = (id: number, patch: Partial<{ note: string; order: string }>) => setNotes(n => ({ ...n, [id]: { ...(n[id] || { note: '', order: '' }), ...patch } }))
  const expiring = ranks.filter(r => r.status === 'approved' && r.rank_expires_at && (daysLeft(r.rank_expires_at) ?? 999) <= 90)
  return <div className="ex-stack">
    {expiring.length > 0 && <section className="panel ex-card"><div className="panel-kicker">СРОКИ</div><h3>Истекающие разряды</h3>
      {expiring.map(r => <div className="ex-row" key={r.id}><span>{r.athlete} — {r.requested_rank}</span><Expiry date={r.rank_expires_at} /></div>)}</section>}
    {role !== 'federation' && <section className="panel ex-card"><div className="panel-kicker">ЗАЯВКА</div><h3>Присвоение или подтверждение разряда</h3>
      {blocked ? <Empty text="У вас есть активный тренер — заявку на разряд подаёт он. Статус заявки виден ниже." /> :
        role === 'coach' && coachAthletes.length === 0 ? <Empty text="Сначала привяжите спортсмена в разделе «Подопечные»: заявку можно подать только за активного подопечного." /> :
        <form className="ex-form" onSubmit={apply}>
          {role === 'coach' && <Field label="Спортсмен"><select value={athleteId} onChange={e => pickAthlete(e.target.value)}><option value="">Выберите спортсмена</option>{coachAthletes.map(([id, name]) => <option key={id} value={id}>{name}</option>)}</select></Field>}
          <Field label="Соревнование и результат, на основании которого запрашивается разряд"><select value={cid} onChange={e => pickCompetition(e.target.value)}><option value="">{results.length ? 'Выберите соревнование' : 'У спортсмена нет результатов'}</option>{results.map(r => <option key={r.competition_id} value={r.competition_id}>{r.competition} — {r.place} место</option>)}</select></Field>
          <Field label="Запрашиваемый разряд"><select value={rank} onChange={e => setRank(e.target.value)}>{RANKS.map(r => <option key={r}>{r}</option>)}</select></Field>
          <Field label="Занятое место"><input type="number" min={1} value={place} onChange={e => setPlace(e.target.value)} /></Field>
          <button className="button button-primary">Подать заявку</button>
        </form>}
    </section>}
    <section className="panel ex-card"><div className="panel-kicker">ЗАЯВКИ</div><h3>{role === 'federation' ? 'Заявки на разряд' : 'Мои заявки и разряды'}</h3>
      {ranks.length === 0 ? <Empty text="Заявок на разряд пока нет." /> : ranks.map(r => <div className="ex-item" key={r.id}>
        <div className="ex-item-head"><b>{r.athlete} — {r.requested_rank}</b><Status value={r.status} /></div>
        <p>{r.competition ? `Основание: ${r.competition}, ${r.place} место` : 'Соревнование не указано'} · подана {day(r.created_at)}{r.decision_note ? ` · Комментарий: ${r.decision_note}` : ''}</p>
        {r.status === 'approved' && <p>Срок действия разряда: <Expiry date={r.rank_expires_at} /></p>}
        {role === 'federation' && r.status === 'pending' && <div className="ex-decide">
          <input placeholder="Номер и дата приказа" value={notes[r.id]?.order || ''} onChange={e => setDraft(r.id, { order: e.target.value })} />
          <input placeholder="Комментарий (необязательно)" value={notes[r.id]?.note || ''} onChange={e => setDraft(r.id, { note: e.target.value })} />
          <button className="button button-primary" onClick={() => void decide(r.id, 'approved')}>Присвоить</button><button className="button" onClick={() => void decide(r.id, 'rejected')}>Отклонить</button></div>}
      </div>)}
    </section>
  </div>
}
