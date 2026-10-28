import { useCallback, useEffect, useState } from 'react'

type Msg = (message: string) => void
type Comp = { id: number; title: string; starts_at: string; status: string }
type Check = { code: string; title: string; status: 'ok' | 'warn' | 'fail'; hint: string; blocking: boolean }
type Row = { athlete_id: number; athlete: string; report: { checks: Check[]; score: number; ready: boolean; blockers: number; days_left: number } }
type Data = { competition: { id: number; title: string; days_left: number }; total: number; ready: number; items: Row[] }

async function call<T>(path: string, init?: RequestInit): Promise<T> {
  const token = localStorage.getItem('fsp_token')
  const res = await fetch(path, { ...init, headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) } })
  if (!res.ok) {
    const p = await res.json().catch(() => ({}))
    throw new Error(p.error || `Ошибка запроса: ${res.status}`)
  }
  return res.json() as Promise<T>
}

const ICON = { ok: '✅', warn: '⚠️', fail: '⛔' } as const

function Checklist({ checks }: { checks: Check[] }) {
  return <ul className="readiness-list">{checks.map(c => <li key={c.code} className={`readiness-${c.status}`}>
    <span>{ICON[c.status]}</span><div><b>{c.title}</b>{c.hint && <small>{c.hint}</small>}</div></li>)}</ul>
}

export function ReadinessView({ role, competitions, onMessage }: { role: string; competitions: Comp[]; onMessage: Msg }) {
  const [id, setId] = useState<number | ''>('')
  const [data, setData] = useState<Data | null>(null)
  const [open, setOpen] = useState<number | null>(null)
  const [busy, setBusy] = useState(false)
  const upcoming = competitions.filter(c => new Date(c.starts_at).getTime() > Date.now() - 86400000)

  const load = useCallback(async (cid: number) => {
    try { setData(await call<Data>(`/api/readiness?competition_id=${cid}`)); setOpen(null) }
    catch (e) { setData(null); onMessage(e instanceof Error ? e.message : 'Не удалось получить готовность') }
  }, [onMessage])
  useEffect(() => { if (id) void load(Number(id)) }, [id, load])
  useEffect(() => { if (!id && upcoming.length) setId(upcoming[0].id) }, [upcoming, id])

  async function remind() {
    if (!id) return
    setBusy(true)
    try { const r = await call<{ sent: number }>(`/api/readiness/remind?competition_id=${id}`, { method: 'POST', body: '{}' }); onMessage(`Напоминаний отправлено: ${r.sent}`) }
    catch (e) { onMessage(e instanceof Error ? e.message : 'Не удалось отправить') }
    finally { setBusy(false) }
  }

  return <div className="readiness">
    <section className="panel readiness-head">
      <label className="field-label">Соревнование
        <select value={id} onChange={e => setId(e.target.value ? Number(e.target.value) : '')}>
          <option value="">Выберите</option>
          {upcoming.map(c => <option key={c.id} value={c.id}>{c.title}</option>)}
        </select></label>
      {data && <div className="readiness-sum"><b>{data.ready} из {data.total}</b><span>допущены без блокеров</span><span>до старта: {data.competition.days_left} дн.</span></div>}
      {role === 'federation' && <button className="button button-primary" disabled={busy || !id} onClick={() => void remind()}>Напомнить неготовым</button>}
    </section>
    {data && data.items.length === 0 && <section className="panel"><p className="readiness-empty">Нет участников для проверки: нужна заявка на это соревнование{role === 'coach' ? ' или привязанные спортсмены' : ''}.</p></section>}
    {data && data.items.map(row => <section className="panel readiness-card" key={row.athlete_id}>
      <button className="readiness-row" onClick={() => setOpen(open === row.athlete_id ? null : row.athlete_id)} aria-expanded={role === 'athlete' || open === row.athlete_id}>
        <b>{row.athlete}</b>
        <span className="readiness-bar"><i style={{ width: `${row.report.score}%` }} className={row.report.ready ? 'ok' : 'bad'} /></span>
        <span className="status-pill">{row.report.ready ? 'Допущен' : `Блокеров: ${row.report.blockers}`} · {row.report.score}%</span>
      </button>
      {(role === 'athlete' || open === row.athlete_id) && <Checklist checks={row.report.checks} />}
    </section>)}
  </div>
}
