import { useCallback, useEffect, useState } from 'react'
import QRCode from 'qrcode'

type Msg = (message: string) => void
async function xapi<T>(path: string, init?: RequestInit): Promise<T> {
  const token = localStorage.getItem('fsp_token')
  const response = await fetch(path, { ...init, headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) } })
  if (!response.ok) { const p = await response.json().catch(() => ({})); throw new Error(p.error || `Ошибка запроса: ${response.status}`) }
  return response.json() as Promise<T>
}
const send = (method: string, path: string, body: unknown) => xapi<{ passed?: boolean; to_stage?: string }>(path, { method, body: JSON.stringify(body) })
const fail = (m: Msg) => (e: unknown) => m(e instanceof Error ? e.message : 'Не удалось выполнить действие')
const rub = (n: number) => n.toLocaleString('ru-RU') + ' ₽'
async function downloadFile(path: string, name: string, m: Msg) {
  try {
    const token = localStorage.getItem('fsp_token')
    const r = await fetch(path, { headers: token ? { Authorization: `Bearer ${token}` } : {} })
    if (!r.ok) throw new Error('Не удалось сформировать документ')
    const url = URL.createObjectURL(await r.blob()); const a = document.createElement('a'); a.href = url; a.download = name; document.body.appendChild(a); a.click(); a.remove()
    window.setTimeout(() => URL.revokeObjectURL(url), 1000)
  } catch (e) { fail(m)(e) }
}

type Norm = { id: number; stage: string; discipline: string; name: string; unit: string; higher_is_better: boolean; threshold: number }
type Att = { athlete_id: number; athlete: string; group: string; stage: string; total: number; passed: number; next_stage: string; ready: boolean }

// ---------- Д4: нормативы и аттестация ----------
export function NormsView({ role, athletes, onMessage }: { role: string; athletes: { id: number; full_name: string }[]; onMessage: Msg }) {
  const [norms, setNorms] = useState<Norm[]>([])
  const [stages, setStages] = useState<string[]>([])
  const [att, setAtt] = useState<Att[]>([])
  const [form, setForm] = useState({ athlete_id: '', norm_id: '', value: '' })
  const [nf, setNf] = useState({ stage: '', discipline: '', name: '', unit: '', higher_is_better: true, threshold: '' })
  const [hist, setHist] = useState<{ name: string; stage: string; unit: string; value: number; threshold: number; passed: boolean; date: string }[]>([])
  const load = useCallback(async () => {
    try {
      const n = await xapi<{ items: Norm[]; stages: string[] }>('/api/norms'); setNorms(n.items); setStages(n.stages)
      setAtt((await xapi<{ items: Att[] }>('/api/attestation')).items)
    } catch (e) { fail(onMessage)(e) }
  }, [onMessage])
  useEffect(() => { void load() }, [load])
  const run = async (fn: () => Promise<unknown>, ok: string | ((r: any) => string)) => { try { const r = await fn(); onMessage(typeof ok === 'string' ? ok : ok(r)); await load() } catch (e) { fail(onMessage)(e) } }
  async function showHistory(id: string) {
    setForm(f => ({ ...f, athlete_id: id }))
    if (!id) { setHist([]); return }
    try { setHist((await xapi<{ items: typeof hist }>(`/api/norm-results?athlete_id=${id}`)).items) } catch { setHist([]) }
  }
  const sel = (v: string, set: (x: string) => void, ph: string, opts: { v: string | number; l: string }[]) => <select value={v} onChange={e => set(e.target.value)}><option value="">{ph}</option>{opts.map(o => <option key={o.v} value={o.v}>{o.l}</option>)}</select>
  return <div className="ex-stack">
    <section className="panel ex-card"><div className="panel-kicker">ВНЕСЕНИЕ РЕЗУЛЬТАТА</div><h3>Сдача норматива</h3>
      <form className="ex-inline" onSubmit={e => { e.preventDefault(); void run(() => send('POST', '/api/norm-results', { athlete_id: Number(form.athlete_id), norm_id: Number(form.norm_id), value: Number(form.value) }), r => r.passed ? 'Норматив выполнен' : 'Норматив не выполнен').then(() => showHistory(form.athlete_id)) }}>
        {sel(form.athlete_id, v => void showHistory(v), 'Спортсмен', athletes.map(a => ({ v: a.id, l: a.full_name })))}
        {sel(form.norm_id, v => setForm({ ...form, norm_id: v }), 'Норматив', norms.map(n => ({ v: n.id, l: `${n.name} (${n.stage}${n.discipline ? ', ' + n.discipline.replace('Программирование ', '') : ''}) — порог ${n.higher_is_better ? '≥' : '≤'} ${n.threshold} ${n.unit}` })))}
        <input required type="number" step="any" placeholder="Результат" value={form.value} onChange={e => setForm({ ...form, value: e.target.value })} />
        <button className="button button-primary" disabled={!form.athlete_id || !form.norm_id}>Сохранить</button></form>
      {hist.length > 0 && <div className="table-scroll"><table className="ex-table"><thead><tr><th>Дата</th><th>Норматив</th><th>Этап</th><th>Результат</th><th>Порог</th><th>Итог</th></tr></thead><tbody>
        {hist.map((h, i) => <tr key={i}><td>{h.date}</td><td>{h.name}</td><td>{h.stage}</td><td>{h.value} {h.unit}</td><td>{h.threshold}</td><td><span className={`status-pill ${h.passed ? 'status-open' : 'status-planned'}`}>{h.passed ? 'выполнен' : 'не выполнен'}</span></td></tr>)}</tbody></table></div>}
    </section>
    <section className="panel ex-card"><div className="panel-kicker">ПРОМЕЖУТОЧНАЯ АТТЕСТАЦИЯ</div><h3>Готовность к переводу на следующий этап</h3>
      <div className="table-scroll"><table className="ex-table"><thead><tr><th>Спортсмен</th><th>Группа</th><th>Этап</th><th>Нормативы</th><th>Следующий этап</th><th /></tr></thead><tbody>
        {att.map(a => <tr key={a.athlete_id}><td>{a.athlete}</td><td>{a.group}</td><td>{a.stage}</td><td><span className={`status-pill ${a.ready ? 'status-open' : 'status-draft'}`}>{a.passed} из {a.total}</span></td><td>{a.next_stage || 'высший этап'}</td>
          <td><button className="button button-light" disabled={!a.ready} onClick={() => void run(() => send('POST', '/api/attestation/promote', { athlete_id: a.athlete_id }), 'Спортсмен переведён на следующий этап')}>Оформить перевод</button></td></tr>)}</tbody></table></div>
      {att.length === 0 && <div className="ex-empty">Нет спортсменов в учебно-тренировочных группах.</div>}
    </section>
    <section className="panel ex-card"><div className="panel-kicker">СПРАВОЧНИК</div><h3>Нормативы ФССП (демо-пороги)</h3>
      <div className="table-scroll"><table className="ex-table"><thead><tr><th>Этап</th><th>Дисциплина</th><th>Норматив</th><th>Порог</th></tr></thead><tbody>
        {norms.map(n => <tr key={n.id}><td>{n.stage}</td><td>{n.discipline ? n.discipline.replace('Программирование ', '') : 'все'}</td><td>{n.name}</td><td>{n.higher_is_better ? '≥' : '≤'} {n.threshold} {n.unit}</td></tr>)}</tbody></table></div>
      {role === 'federation' && <form className="ex-inline" onSubmit={e => { e.preventDefault(); void run(() => send('POST', '/api/norms', { ...nf, threshold: Number(nf.threshold) }), 'Норматив сохранён') }}>
        {sel(nf.stage, v => setNf({ ...nf, stage: v }), 'Этап', stages.map(s => ({ v: s, l: s })))}<input required placeholder="Название" value={nf.name} onChange={e => setNf({ ...nf, name: e.target.value })} />
        <input placeholder="Ед. изм." value={nf.unit} onChange={e => setNf({ ...nf, unit: e.target.value })} /><input required type="number" step="any" placeholder="Порог" value={nf.threshold} onChange={e => setNf({ ...nf, threshold: e.target.value })} />
        <label><input type="checkbox" checked={nf.higher_is_better} onChange={e => setNf({ ...nf, higher_is_better: e.target.checked })} /> чем больше, тем лучше</label>
        <button className="button button-primary" disabled={!nf.stage}>Добавить / изменить</button></form>}
    </section></div>
}

type Cand = { athlete: string; organization: string; competition: string; level: string; place: number; amount: number }
// ---------- Д8 + Д6: премии и показатели развития ----------
export function BonusesView({ role, onMessage }: { role: string; onMessage: Msg }) {
  const [year, setYear] = useState(new Date().getFullYear())
  const [b, setB] = useState<{ rates: { level: string; place: number; amount: number }[]; candidates: Cand[]; total: number; status: string; deadline: string; days_left: number } | null>(null)
  const [dev, setDev] = useState<{ years: { year: number; athletes: number }[]; organizations: { name: string; license: string; athletes: number; groups: number }[]; competitions: { title: string; sport_status: string; status: string; calendar_plan: string; calendar_number: string; date: string }[]; docs: { id: number; indicator: string; title: string; url: string }[] } | null>(null)
  const [doc, setDoc] = useState({ indicator: 'Численность занимающихся', title: '', url: '' })
  const load = useCallback(async () => {
    try { setB(await xapi(`/api/bonuses?year=${year}`)); setDev(await xapi(`/api/development?year=${year}`)) } catch (e) { fail(onMessage)(e) }
  }, [year, onMessage])
  useEffect(() => { void load() }, [load])
  const fed = role === 'federation'
  const act = async (fn: () => Promise<unknown>, ok: string) => { try { await fn(); onMessage(ok); await load() } catch (e) { fail(onMessage)(e) } }
  const labels: Record<string, string> = { draft: 'Черновик', prepared: 'Подготовлено', sent: 'Направлено', awarded: 'Премия назначена' }
  const max = Math.max(1, ...(dev?.years.map(y => y.athletes) || [1]))
  return <div className="ex-stack">
    <section className="panel ex-card"><div className="panel-kicker">ПРЕМИИ ГУБЕРНАТОРА · УКАЗ № 93-УК</div><h3>Претенденты за {year} год</h3>
      <div className="ex-inline"><select value={year} onChange={e => setYear(Number(e.target.value))}>{[0, 1, 2].map(i => <option key={i} value={new Date().getFullYear() - i}>{new Date().getFullYear() - i}</option>)}</select>
        {b && <><span className={`status-pill ${b.days_left < 30 && b.days_left >= 0 ? 'status-planned' : 'status-draft'}`}>Подать до {b.deadline}{b.days_left >= 0 ? ` · осталось ${b.days_left} дн.` : ' · срок прошёл'}</span><span className="status-pill status-open">{labels[b.status] || b.status}</span><b>Итого: {rub(b.total)}</b></>}</div>
      <div className="table-scroll"><table className="ex-table"><thead><tr><th>Спортсмен</th><th>Организация</th><th>Соревнование</th><th>Уровень</th><th>Место</th><th>Сумма</th></tr></thead><tbody>
        {b?.candidates.map((c, i) => <tr key={i}><td>{c.athlete}</td><td>{c.organization}</td><td>{c.competition}</td><td>{c.level}</td><td>{c.place}</td><td>{rub(c.amount)}</td></tr>)}</tbody></table></div>
      {b && b.candidates.length === 0 && <div className="ex-empty">В {year} году призовых результатов по протоколам нет.</div>}
      {fed && b && <div className="ex-inline">{(['prepared', 'sent', 'awarded'] as const).map(s => <button key={s} className="button button-light" onClick={() => void act(() => send('POST', '/api/bonuses/petition', { year, status: s }), 'Статус ходатайства обновлён')}>{labels[s]}</button>)}
        <button className="button button-primary" onClick={() => void downloadFile(`/api/bonuses/package?year=${year}`, `bonus-${year}.docx`, onMessage)}>Пакет документов (DOCX)</button></div>}
    </section>
    {fed && b && <section className="panel ex-card"><div className="panel-kicker">СПРАВОЧНИК</div><h3>Размеры премий</h3><div className="ex-inline">{b.rates.map(r => <label key={r.level + r.place} className="ex-field">{r.level}, {r.place} место
      <input type="number" min={0} defaultValue={r.amount} onBlur={e => { if (Number(e.target.value) !== r.amount) void act(() => send('PUT', '/api/bonus-rates', { level: r.level, place: r.place, amount: Number(e.target.value) }), 'Размер премии изменён') }} /></label>)}</div></section>}
    {dev && <section className="panel ex-card"><div className="panel-kicker">ПОКАЗАТЕЛИ РАЗВИТИЯ ВИДА СПОРТА</div><h3>Сводка по региону</h3>
      <h4>Численность занимающихся (источник: реестр)</h4>
      {dev.years.map(y => <div key={y.year} className="ex-row"><span>{y.year}</span><span style={{ flex: 1, margin: '0 12px' }}><i style={{ display: 'block', height: 10, borderRadius: 5, background: 'var(--violet)', width: `${(y.athletes / max) * 100}%` }} /></span><b>{y.athletes}</b></div>)}
      <h4 style={{ marginTop: 18 }}>Организации, ведущие подготовку (источник: карточки организаций)</h4>
      {dev.organizations.map(o => <div key={o.name} className="ex-row"><span>{o.name}</span><span>{o.groups} групп · {o.athletes} спортсменов · {o.license || 'лицензия не указана'}</span></div>)}
      <h4 style={{ marginTop: 18 }}>Официальные соревнования за {year} год (источник: календарь и протоколы)</h4>
      {dev.competitions.map((c, i) => <div key={i} className="ex-row"><span>{c.date} · {c.title}</span><span>{c.sport_status} · {c.status} · {c.calendar_plan ? `${c.calendar_plan} № ${c.calendar_number || '—'}` : 'не включено в план'}</span></div>)}
      <h4 style={{ marginTop: 18 }}>Подтверждающие документы</h4>
      {dev.docs.map(d => <div key={d.id} className="ex-row"><span>{d.indicator}</span><a href={d.url} target="_blank" rel="noreferrer">{d.title}</a></div>)}
      {fed && <form className="ex-inline" onSubmit={e => { e.preventDefault(); void act(() => send('POST', '/api/development/docs', doc), 'Документ прикреплён') }}>
        <select value={doc.indicator} onChange={e => setDoc({ ...doc, indicator: e.target.value })}><option>Численность занимающихся</option><option>Организации</option><option>Официальные соревнования</option></select>
        <input required placeholder="Название документа" value={doc.title} onChange={e => setDoc({ ...doc, title: e.target.value })} /><input required placeholder="https://" value={doc.url} onChange={e => setDoc({ ...doc, url: e.target.value })} />
        <button className="button button-primary">Прикрепить</button></form>}
    </section>}
  </div>
}

// ---------- Д7 + Д10: личный путь, справочник, справка с QR ----------
export function PathView({ role, onMessage }: { role: string; onMessage: Msg }) {
  const [path, setPath] = useState<{ rank: string; next_rank: string; discipline: string; age: number; stage: string; hint: string; competitions: { id: number; title: string; sport_status: string; age_group: string; date: string }[] } | null>(null)
  const [terms, setTerms] = useState<{ term: string; text: string }[]>([])
  const [options, setOptions] = useState<{ competition_id: number; competition: string; place: number }[]>([])
  const [pick, setPick] = useState('')
  const [cert, setCert] = useState<{ code: string; link: string; qr: string } | null>(null)
  const [hist, setHist] = useState<{ items: { name: string; value: number; unit: string; passed: boolean; date: string }[]; attestations: { from: string; to: string; date: string }[] } | null>(null)
  const athlete = role === 'athlete'
  useEffect(() => {
    xapi<{ items: { term: string; text: string }[] }>('/api/public/glossary').then(r => setTerms(r.items)).catch(() => undefined)
    if (!athlete) return
    xapi<typeof path>('/api/path').then(setPath).catch(fail(onMessage))
    xapi<{ items: typeof options }>('/api/cases/results').then(r => setOptions(r.items)).catch(() => undefined)
    xapi<NonNullable<typeof hist>>('/api/norm-results').then(setHist).catch(() => undefined)
  }, [athlete, onMessage])
  async function issue() {
    try {
      const r = await xapi<{ code: string; url: string }>('/api/certificates', { method: 'POST', body: JSON.stringify({ competition_id: Number(pick) }) })
      const link = window.location.origin + r.url
      setCert({ code: r.code, link, qr: await QRCode.toDataURL(link, { width: 220, margin: 1 }) }); onMessage('Справка сформирована')
    } catch (e) { fail(onMessage)(e) }
  }
  return <div className="ex-stack">
    {athlete && path && <section className="panel ex-card"><div className="panel-kicker">ЛИЧНЫЙ ПУТЬ</div><h3>Сейчас: {path.rank || 'без разряда'} → дальше: {path.next_rank}</h3>
      <p>{path.hint}</p><p>Этап подготовки: <b>{path.stage || 'не определён'}</b> · возраст {path.age}</p>
      <h4>Подходящие соревнования (по возрасту и дисциплине)</h4>
      {path.competitions.map(c => <div key={c.id} className="ex-row"><span>{c.date} · {c.title}</span><span>{c.sport_status} · {c.age_group}</span></div>)}
      {path.competitions.length === 0 && <div className="ex-empty">Сейчас нет открытых соревнований, подходящих вам.</div>}
      {hist && (hist.items.length > 0 || hist.attestations.length > 0) && <><h4 style={{ marginTop: 16 }}>Нормативы и аттестация</h4>
        {hist.items.slice(0, 6).map((h, i) => <div key={i} className="ex-row"><span>{h.date} · {h.name}</span><span>{h.value} {h.unit} · {h.passed ? 'выполнен' : 'не выполнен'}</span></div>)}
        {hist.attestations.map((a, i) => <div key={i} className="ex-row"><span>{a.date} · перевод</span><span>{a.from} → {a.to}</span></div>)}</>}
    </section>}
    {athlete && <section className="panel ex-card"><div className="panel-kicker">СПРАВКА ОБ УЧАСТИИ</div><h3>Справка с QR-кодом для портфолио и поступления</h3>
      <div className="ex-inline"><select value={pick} onChange={e => setPick(e.target.value)}><option value="">Выберите соревнование</option>{options.map(o => <option key={o.competition_id} value={o.competition_id}>{o.competition} — {o.place} место</option>)}</select>
        <button className="button button-primary" disabled={!pick} onClick={() => void issue()}>Сформировать справку</button></div>
      {cert && <div className="ex-inline"><img src={cert.qr} alt="QR-код справки" width={160} height={160} /><div><p>Номер справки: <b>{cert.code}</b></p><p><a href={cert.link} target="_blank" rel="noreferrer">Страница проверки подлинности</a></p></div></div>}
    </section>}
    <section className="panel ex-card"><div className="panel-kicker">СПРАВОЧНИК</div><h3>Понятия простым языком</h3>
      {terms.map(t => <div key={t.term} className="ex-item"><b>{t.term}</b><p>{t.text}</p></div>)}</section></div>
}
