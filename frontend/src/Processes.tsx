import { useCallback, useEffect, useState, type ReactNode } from 'react'

type Api = <T>(path: string, init?: RequestInit) => Promise<T>
type Row = Record<string, any>
type Props = {
  api: Api
  role: 'federation' | 'coach' | 'athlete' | 'judge'
  competitions: { id: number; title: string; discipline: string; status: string }[]
  athletes: { id: number; full_name: string; discipline: string }[]
  onMessage: (m: string) => void
}
const DISCIPLINES = ['Алгоритмическое программирование', 'Продуктовое программирование', 'Программирование систем информационной безопасности', 'Программирование систем искусственного интеллекта', 'Программирование робототехники', 'Программирование беспилотных авиационных систем']
const RANKS = ['3 юношеский разряд', '2 юношеский разряд', '1 юношеский разряд', '3 спортивный разряд', '2 спортивный разряд', '1 спортивный разряд', 'КМС']
const CATS = ['Спортивный судья третьей категории', 'Спортивный судья второй категории', 'Спортивный судья первой категории', 'Спортивный судья высшей категории']
const STATUS: Record<string, string> = { pending: 'На рассмотрении', approved: 'Одобрено', rejected: 'Отклонено', active: 'Активна', paused: 'Приостановлена', completed: 'Завершена' }

function Table({ head, rows }: { head: string[]; rows: ReactNode[][] }) {
  if (!rows.length) return <p className="proc-empty">Пока пусто</p>
  return <div className="table-scroll"><table className="proc-table"><thead><tr>{head.map(h => <th key={h}>{h}</th>)}</tr></thead><tbody>{rows.map((r, i) => <tr key={i}>{r.map((c, j) => <td key={j}>{c}</td>)}</tr>)}</tbody></table></div>
}
function Pill({ s }: { s: string }) { return <span className="status-pill">{STATUS[s] || s}</span> }

export default function ProcessesView({ api, role, competitions, athletes, onMessage }: Props) {
  const [links, setLinks] = useState<Row[]>([])
  const [ranks, setRanks] = useState<Row[]>([])
  const [cats, setCats] = useState<Row[]>([])
  const [assign, setAssign] = useState<Row[]>([])
  const [judges, setJudges] = useState<Row[]>([])
  const [history, setHistory] = useState<Row[]>([])
  const [apps, setApps] = useState<Row[]>([])
  const [compId, setCompId] = useState<number | ''>('')
  const [f, setF] = useState<Record<string, string>>({})
  const [places, setPlaces] = useState<Record<number, { place: string; score: string }>>({})
  const [cert, setCert] = useState('')
  const [att, setAtt] = useState<Row[]>([])
  const [norms, setNorms] = useState<Row[]>([])
  const [clients, setClients] = useState<Row[]>([])
  const [newCred, setNewCred] = useState('')
  const [integ, setInteg] = useState<Row | null>(null)
  const [prem, setPrem] = useState<Row | null>(null)
  const [year, setYear] = useState(new Date().getFullYear())
  const set = (k: string, v: string) => setF(p => ({ ...p, [k]: v }))
  const safe = async <T,>(p: string): Promise<T[]> => { try { return (await api<{ items: T[] }>(p)).items } catch { return [] } }

  const load = useCallback(async () => {
    if (role !== 'judge') setLinks(await safe<Row>('/api/coach-links'))
    if (role !== 'judge') setRanks(await safe<Row>('/api/ranks'))
    if (role === 'judge' || role === 'federation') { setCats(await safe<Row>('/api/categories')); setAssign(await safe<Row>('/api/assignments')) }
    if (role === 'federation') { try { setInteg(await api<Row>('/api/integrations')) } catch { setInteg(null) } }
    if (role === 'federation') setClients(await safe<Row>('/api/oidc/clients'))
    if (role === 'federation') { setJudges(await safe<Row>('/api/judges')); try { setPrem(await api<Row>(`/api/premiums?year=${year}`)) } catch { setPrem(null) } }
    if (role !== 'judge') { setAtt(await safe<Row>('/api/attestation')); setNorms(await safe<Row>('/api/norms')) }
    if (role === 'athlete' || role === 'coach') setHistory(await safe<Row>('/api/results'))
  }, [api, role, year])
  useEffect(() => { void load() }, [load])
  useEffect(() => { if (role === 'federation' && compId) void safe<Row>(`/api/competitions/${compId}/applications`).then(a => setApps(a.filter(x => x.status === 'approved'))); else setApps([]) }, [compId, role]) // eslint-disable-line

  async function act(fn: () => Promise<unknown>, ok: string) {
    try { await fn(); onMessage(ok); await load() } catch (e) { onMessage(e instanceof Error ? e.message : 'Ошибка') }
  }
  const post = (p: string, body: object) => api(p, { method: 'POST', body: JSON.stringify(body) })
  async function download(path: string, name: string) {
    try {
      const r = await fetch(path, { headers: { Authorization: `Bearer ${localStorage.getItem('fsp_token') || ''}` } })
      if (!r.ok) throw new Error('Не удалось скачать файл')
      const a = document.createElement('a'); a.href = URL.createObjectURL(await r.blob()); a.download = name; a.click(); URL.revokeObjectURL(a.href)
    } catch (e) { onMessage(e instanceof Error ? e.message : 'Ошибка') }
  }
  const btn = (label: string, fn: () => void, light = false) => <button className={`button ${light ? 'button-light' : 'button-primary'}`} onClick={fn}>{label}</button>
  const sel = (k: string, label: string, opts: [string, string][], ph = 'Выберите') => <label className="field-label">{label}<select value={f[k] || ''} onChange={e => set(k, e.target.value)}><option value="">{ph}</option>{opts.map(([v, l]) => <option key={v} value={v}>{l}</option>)}</select></label>
  const compOpts: [string, string][] = competitions.map(c => [String(c.id), c.title])

  const rankForm = (athleteOpts: [string, string][] | null) => <section className="panel proc-card"><div className="panel-heading"><h2>Заявка на разряд</h2></div>
    {athleteOpts && sel('rk_ath', 'Спортсмен', athleteOpts)}
    {sel('rk_rank', 'Разряд', RANKS.map(r => [r, r]))}
    {sel('rk_comp', 'Соревнование-основание', compOpts)}
    <label className="field-label">Занятое место<input type="number" min={1} value={f.rk_place || ''} onChange={e => set('rk_place', e.target.value)} /></label>
    {btn('Подать заявку', () => act(() => post('/api/ranks', { athlete_id: Number(f.rk_ath) || 0, requested_rank: f.rk_rank, competition_id: Number(f.rk_comp) || 0, place: Number(f.rk_place) || 0 }), 'Заявка на разряд отправлена в федерацию'))}</section>

  const rankTable = <section className="panel proc-card wide"><div className="panel-heading"><h2>Разряды</h2></div>
    <Table head={['Спортсмен', 'Разряд', 'Основание', 'Место', 'Статус', role === 'federation' ? 'Решение' : 'Срок действия']} rows={ranks.map(r => [r.athlete, r.requested_rank, r.competition || '—', r.place || '—', <Pill s={r.status} />,
      role === 'federation' ? (r.status === 'pending' ? <span className="proc-actions">{btn('Присвоить', () => act(() => post(`/api/ranks/${r.id}/decide`, { status: 'approved', note: 'Одобрено', order: f.order || '' }), 'Разряд присвоен'))}{btn('Отклонить', () => act(() => post(`/api/ranks/${r.id}/decide`, { status: 'rejected', note: 'Отклонено' }), 'Заявка отклонена'), true)}</span> : '—') : (r.expires_at || r.rank_expires_at || '—')])} />
    {role === 'federation' && <label className="field-label">№ приказа (для присвоения)<input value={f.order || ''} onChange={e => set('order', e.target.value)} placeholder="напр. 12-П от 28.10.2026" /></label>}</section>

  return <div className="proc-grid">
    {role === 'coach' && <>
      <section className="panel proc-card"><div className="panel-heading"><h2>Привязать спортсмена</h2></div>
        {sel('lk_ath', 'Спортсмен', athletes.map(a => [String(a.id), a.full_name]))}
        {sel('lk_disc', 'Дисциплина', DISCIPLINES.map(d => [d, d]))}
        {btn('Отправить запрос', () => act(() => post('/api/coach-links', { athlete_id: Number(f.lk_ath), discipline: f.lk_disc }), 'Запрос отправлен — спортсмен должен подтвердить'))}</section>
      {rankForm(links.filter(l => l.status === 'active').map(l => [String(l.athlete_id), `${l.athlete} (${l.discipline})`]))}
    </>}
    {role !== 'judge' && <section className="panel proc-card wide"><div className="panel-heading"><h2>Связи «тренер — спортсмен»</h2></div>
      <Table head={['Тренер', 'Спортсмен', 'Дисциплина', 'Статус', role === 'athlete' ? 'Действие' : '']} rows={links.map(l => [l.coach, l.athlete, l.discipline, <Pill s={l.status} />,
        role === 'athlete' && l.status === 'pending' ? <span className="proc-actions">{btn('Подтвердить', () => act(() => post(`/api/coach-links/${l.id}/decide`, { status: 'active' }), 'Привязка подтверждена'))}{btn('Отклонить', () => act(() => post(`/api/coach-links/${l.id}/decide`, { status: 'completed' }), 'Привязка отклонена'), true)}</span>
          : role === 'athlete' && l.status === 'active' ? btn('Приостановить', () => act(() => post(`/api/coach-links/${l.id}/decide`, { status: 'paused' }), 'Привязка приостановлена'), true) : ''])} /></section>}
    {role === 'athlete' && !links.some(l => l.status === 'active') && rankForm(null)}
    {role !== 'judge' && rankTable}
    {(role === 'athlete' || role === 'coach') && <section className="panel proc-card wide"><div className="panel-heading"><h2>История выступлений</h2></div>
      <Table head={['Соревнование', 'Дисциплина', 'Спортсмен', 'Место', 'Баллы', role === 'athlete' ? 'Справка' : '']} rows={history.map(h => [h.title || h.competition, h.discipline, h.athlete || h.full_name || '', h.place, h.score ?? h.points ?? '',
        role === 'athlete' ? btn('Справка с QR', () => act(async () => { const r = await api<{ code: string }>('/api/certificates', { method: 'POST', body: JSON.stringify({ competition_id: h.id || h.competition_id }) }); setCert(r.code) }, 'Справка выпущена'), true) : ''])} />
      {cert && <p className="proc-cert">Справка № <b>{cert}</b> · проверка: <a href={`/api/verify/${cert}`} target="_blank" rel="noreferrer">/api/verify/{cert}</a><br /><img alt="QR" width={120} height={120} src={`https://api.qrserver.com/v1/create-qr-code/?size=240x240&data=${encodeURIComponent(location.origin + '/api/verify/' + cert)}`} /></p>}</section>}


    {role !== 'judge' && <>
      {role !== 'athlete' && <section className="panel proc-card"><div className="panel-heading"><h2>Сдача норматива (ФССП)</h2></div>
        {sel('nr_ath', 'Спортсмен', athletes.map(a => [String(a.id), a.full_name]))}
        {sel('nr_norm', 'Норматив', norms.map(n => [String(n.id), `${n.name} · ${n.stage} (порог ${n.threshold} ${n.unit})`]))}
        <label className="field-label">Результат<input type="number" step="any" value={f.nr_val || ''} onChange={e => set('nr_val', e.target.value)} /></label>
        {btn('Внести результат', () => act(async () => { const r = await api<{ passed: boolean; threshold: number }>('/api/norm-results', { method: 'POST', body: JSON.stringify({ athlete_id: Number(f.nr_ath), norm_id: Number(f.nr_norm), value: Number(f.nr_val) }) }); onMessage(r.passed ? 'Норматив выполнен' : `Норматив не выполнен (порог ${r.threshold})`) }, 'Результат сохранён'))}</section>}
      <section className="panel proc-card wide"><div className="panel-heading"><h2>Нормативы и аттестация по этапам подготовки</h2></div>
        <Table head={['Спортсмен', 'Этап', 'Выполнено', 'Готов к переводу', role !== 'athlete' ? 'Действие' : '']} rows={att.map(a => [a.athlete, a.stage, `${a.passed} из ${a.total}`, a.ready ? `Да → ${a.next_stage}` : 'Нет',
          role !== 'athlete' && a.ready ? btn('Перевести', () => act(() => post(`/api/attestation/${a.athlete_id}/promote`, {}), 'Спортсмен переведён на следующий этап')) : ''])} />
        {att.some(a => a.history?.length) && <Table head={['Спортсмен', 'Норматив', 'Результат', 'Итог', 'Дата']} rows={att.flatMap(a => (a.history || []).map((h: Row) => [a.athlete, `${h.norm} (${h.stage})`, `${h.value} ${h.unit}`, h.passed ? 'Выполнен' : 'Не выполнен', h.date]))} />}</section>
    </>}

    {role === 'judge' && <>
      <section className="panel proc-card"><div className="panel-heading"><h2>Заявка на категорию</h2></div>
        {sel('ct_cat', 'Категория', CATS.map(c => [c, c]))}
        {sel('ct_kind', 'Тип', [['присвоение', 'Присвоение'], ['подтверждение', 'Подтверждение']], 'Присвоение')}
        {btn('Подать заявку', () => act(() => post('/api/categories', { requested_category: f.ct_cat, kind: f.ct_kind || 'присвоение' }), 'Заявка на категорию отправлена'))}</section>
    </>}
    {(role === 'judge' || role === 'federation') && <section className="panel proc-card wide"><div className="panel-heading"><h2>{role === 'judge' ? 'Мои назначения и категория' : 'Судейские коллегии'}</h2></div>
      <Table head={['Соревнование', 'Судья', 'Должность', 'Категория', 'Действует до']} rows={assign.map(a => [a.competition, a.judge, a.position, a.category, a.category_expires_at || '—'])} /></section>}
    {(role === 'judge' || role === 'federation') && <section className="panel proc-card wide"><div className="panel-heading"><h2>Заявки на судейскую категорию</h2></div>
      <Table head={['Судья', 'Категория', 'Тип', 'Статус', role === 'federation' ? 'Решение' : '']} rows={cats.map(c => [c.judge, c.requested_category, c.kind, <Pill s={c.status} />,
        role === 'federation' && c.status === 'pending' ? <span className="proc-actions">{btn('Присвоить', () => act(() => post(`/api/categories/${c.id}/decide`, { status: 'approved', note: 'Одобрено' }), 'Категория присвоена'))}{btn('Отклонить', () => act(() => post(`/api/categories/${c.id}/decide`, { status: 'rejected', note: 'Отклонено' }), 'Заявка отклонена'), true)}</span> : ''])} /></section>}

    {role === 'federation' && <section className="panel proc-card wide"><div className="panel-heading"><h2>ФСП ID — «Войти через ФСП ID» (OpenID Connect)</h2></div>
      <p className="proc-empty">Discovery: <a href="/api/oidc/.well-known/openid-configuration" target="_blank" rel="noreferrer">/api/oidc/.well-known/openid-configuration</a> · поток authorization code + PKCE (S256) · userinfo: ФИО, роли, дисциплины, разряд</p>
      <Table head={['Приложение', 'client_id', 'redirect_uri']} rows={clients.map(c => [c.name, c.client_id, (c.redirect_uris || []).join(', ')])} />
      <label className="field-label">Название приложения<input value={f.oc_name || ''} onChange={e => set('oc_name', e.target.value)} /></label>
      <label className="field-label">redirect_uri (через пробел)<input value={f.oc_uri || ''} onChange={e => set('oc_uri', e.target.value)} placeholder="https://partner.example/callback" /></label>
      <div className="proc-actions">
        {btn('Зарегистрировать приложение', () => act(async () => { const r = await api<{ client_id: string; client_secret: string }>('/api/oidc/clients', { method: 'POST', body: JSON.stringify({ name: f.oc_name, redirect_uris: (f.oc_uri || '').split(/\s+/).filter(Boolean) }) }); setNewCred(`client_id: ${r.client_id} · client_secret: ${r.client_secret} (показан один раз)`) }, 'Приложение зарегистрировано'))}
        {btn('Открыть тестовое приложение', () => { window.open('/api/oidc/demo', '_blank') }, true)}</div>
      {newCred && <p className="proc-cert"><b>{newCred}</b></p>}</section>}

    {role === 'federation' && integ && <section className="panel proc-card wide"><div className="panel-heading"><h2>Интеграции: API-токены и вебхуки</h2></div>
      <Table head={['Токен', 'Права', 'Последнее использование']} rows={(integ.tokens || []).map((t: Row) => [t.name, t.scopes, t.last_used])} />
      <label className="field-label">Название системы<input value={f.tk_name || ''} onChange={e => set('tk_name', e.target.value)} /></label>
      <label className="field-label">Права<select value={f.tk_scope || 'competitions:read'} onChange={e => set('tk_scope', e.target.value)}>{(integ.scopes || []).map((x: string) => <option key={x} value={x}>{x}</option>)}</select></label>
      <div className="proc-actions">{btn('Выдать токен', () => act(async () => { const r = await api<{ token: string }>('/api/integrations/tokens', { method: 'POST', body: JSON.stringify({ name: f.tk_name, scopes: [f.tk_scope || 'competitions:read'] }) }); setNewCred(`API-токен: ${r.token} (показан один раз)`) }, 'Токен выдан'))}</div>
      <Table head={['Вебхук', 'События', 'Активен']} rows={(integ.webhooks || []).map((x: Row) => [x.url, x.events, x.active ? 'да' : 'нет'])} />
      <label className="field-label">URL вебхука<input value={f.wh_url || ''} onChange={e => set('wh_url', e.target.value)} placeholder="https://partner.example/hook" /></label>
      <div className="proc-actions">{btn('Подписать на все события', () => act(async () => { const r = await api<{ secret: string }>('/api/integrations/webhooks', { method: 'POST', body: JSON.stringify({ url: f.wh_url, events: integ.events }) }); setNewCred(`Секрет подписи вебхука: ${r.secret} (показан один раз)`) }, 'Вебхук создан'))}</div>
      <Table head={['Вебхук №', 'Событие', 'HTTP', 'Ошибка', 'Время']} rows={(integ.log || []).map((l: Row) => [l.webhook_id, l.event, l.status, l.error || '—', l.at])} />
      {newCred && <p className="proc-cert"><b>{newCred}</b></p>}</section>}

    {role === 'federation' && prem && <section className="panel proc-card wide"><div className="panel-heading"><h2>Премии Губернатора (Указ № 93-ук)</h2></div>
      <label className="field-label">Год<input type="number" value={year} onChange={e => setYear(Number(e.target.value) || year)} /></label>
      <Table head={['Спортсмен', 'Организация', 'Соревнование', 'Статус', 'Место', 'Сумма, ₽']} rows={(prem.items || []).map((p: Row) => [p.athlete, p.organization, p.competition, p.sport_status, p.place, p.amount])} />
      <p className="proc-cert">Итого: <b>{prem.total} ₽</b> · Статус ходатайства: <b>{prem.status}</b> · Срок подачи {prem.deadline} (осталось дней: {prem.days_left})</p>
      <div className="proc-actions">
        {['подготовлено', 'направлено', 'премия назначена'].map(st => btn(st, () => act(() => post('/api/premiums/status', { year, status: st }), `Статус: ${st}`), st !== prem.status))}
        {btn('Скачать пакет (xlsx)', () => void download(`/api/premiums?year=${year}&format=xlsx`, `premiums-${year}.xlsx`), true)}</div></section>}

    {role === 'federation' && <>
      <section className="panel proc-card"><div className="panel-heading"><h2>Назначить судью</h2></div>
        {sel('as_comp', 'Соревнование', compOpts)}
        {sel('as_judge', 'Судья', judges.map(j => [String(j.id), `${j.full_name} · ${j.category || 'без категории'}`]))}
        {sel('as_pos', 'Должность', ['Главный судья', 'Главный секретарь', 'Судья', 'Технический делегат'].map(x => [x, x]), 'Судья')}
        {btn('Назначить', () => act(() => post(`/api/competitions/${f.as_comp}/judges`, { judge_id: Number(f.as_judge), position: f.as_pos || 'Судья' }), 'Судья назначен'))}</section>
      <section className="panel proc-card wide"><div className="panel-heading"><h2>Ввод результатов и протокол</h2></div>
        <label className="field-label">Соревнование<select value={compId} onChange={e => setCompId(e.target.value ? Number(e.target.value) : '')}><option value="">Выберите</option>{competitions.map(c => <option key={c.id} value={c.id}>{c.title}</option>)}</select></label>
        {compId !== '' && <>
          <Table head={['Участник (одобренная заявка)', 'Место', 'Баллы']} rows={apps.map(a => [a.athlete, <input className="proc-num" type="number" min={1} value={places[a.athlete_id]?.place || ''} onChange={e => setPlaces(p => ({ ...p, [a.athlete_id]: { place: e.target.value, score: p[a.athlete_id]?.score || '' } }))} />, <input className="proc-num" type="number" value={places[a.athlete_id]?.score || ''} onChange={e => setPlaces(p => ({ ...p, [a.athlete_id]: { place: p[a.athlete_id]?.place || '', score: e.target.value } }))} />])} />
          <div className="proc-actions">
            {btn('Сохранить результаты и сформировать протокол', () => act(() => post(`/api/competitions/${compId}/results`, { items: apps.filter(a => places[a.athlete_id]?.place).map(a => ({ athlete_id: a.athlete_id, place: Number(places[a.athlete_id].place), score: Number(places[a.athlete_id].score) || 0 })) }), 'Протокол сформирован, результаты попали в историю спортсменов'))}
            {btn('Скачать протокол', () => void download(`/api/competitions/${compId}/protocol`, `protocol-${compId}`), true)}</div></>}</section>
    </>}
  </div>
}
