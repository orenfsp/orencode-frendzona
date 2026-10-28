import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react'

type Msg = (message: string) => void
type ResultOption = { competition_id: number; competition: string; athlete_id: number; athlete: string; place: number }
type Hist = { status: string; note: string; when: string; who: string }
type Appeal = { id: number; competition: string; athlete: string; subject: string; description: string; evidence_url: string; status: string; decision: string; new_place: number; current_place: number; created_at: string; history: Hist[] }
type Violation = { id: number; competition: string; athlete: string; kind: string; report: string; evidence_url: string; status: string; sanction: string; decision: string; created_at: string; history: Hist[] }
type Org = { id: number; name: string; org_type: string; municipality: string; license: string; programs: string; groups: number; athletes_in_groups: number; athletes: number; coaches: string }
type Group = { id: number; organization_id: number; organization: string; name: string; stage: string; discipline: string; coach: string; members: number; sessions: number; last_session: string }
type Member = { athlete_id: number; full_name: string; rank_title: string; joined_at: string }
type Session = { id: number; held_at: string; topic: string; format: string; proof_url: string; present: number }

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

const appealStatus: Record<string, [string, string]> = { submitted: ['Подана', 'status-planned'], review: ['На рассмотрении', 'status-draft'], accepted: ['Удовлетворена', 'status-open'], rejected: ['Отклонена', 'status-done'] }
const sanctionLabel: Record<string, string> = { none: 'Без санкций', warning: 'Предупреждение', technical_defeat: 'Техническое поражение', disqualification: 'Дисквалификация' }

function Pill({ map, value }: { map: Record<string, [string, string]>; value: string }) {
  const [label, cls] = map[value] || [value, 'status-done']
  return <span className={`status-pill ${cls}`}>{label}</span>
}
function History({ items }: { items: Hist[] }) {
  if (!items?.length) return null
  return <ul className="ex-history">{items.map((h, i) => <li key={i}><b>{h.when}</b> · {h.who}: {h.note}</li>)}</ul>
}
function Field({ label, children }: { label: string; children: ReactNode }) {
  return <label className="ex-field"><span>{label}</span>{children}</label>
}
const Empty = ({ text }: { text: string }) => <div className="ex-empty">{text}</div>

// ---------- Д5: апелляции и нарушения ----------
export function AppealsView({ role, onMessage }: { role: string; onMessage: Msg }) {
  const [appeals, setAppeals] = useState<Appeal[]>([])
  const [violations, setViolations] = useState<Violation[]>([])
  const [options, setOptions] = useState<ResultOption[]>([])
  const [pick, setPick] = useState('')
  const [subject, setSubject] = useState('')
  const [text, setText] = useState('')
  const [url, setUrl] = useState('')
  const [kind, setKind] = useState('Нарушение правил соревнования')
  const [drafts, setDrafts] = useState<Record<string, { status?: string; decision?: string; place?: string; sanction?: string }>>({})
  const canAppeal = role === 'athlete' || role === 'coach'
  const canReport = role === 'federation' || role === 'judge'
  const load = useCallback(async () => {
    try {
      const [a, v, o] = await Promise.all([xapi<{ items: Appeal[] }>('/api/appeals/list'), xapi<{ items: Violation[] }>('/api/violations'), xapi<{ items: ResultOption[] }>('/api/cases/results')])
      setAppeals(a.items); setViolations(v.items); setOptions(o.items)
    } catch (e) { fail(onMessage)(e) }
  }, [onMessage])
  useEffect(() => { void load() }, [load])
  const chosen = pick === '' ? undefined : options[Number(pick)]
  const draft = (key: string) => drafts[key] || {}
  const setDraft = (key: string, patch: object) => setDrafts(d => ({ ...d, [key]: { ...d[key], ...patch } }))

  async function submitAppeal(e: FormEvent) {
    e.preventDefault()
    if (!chosen) { onMessage('Выберите результат, на который подаётся апелляция'); return }
    try {
      await post('/api/appeals', { competition_id: chosen.competition_id, athlete_id: chosen.athlete_id, subject, description: text, evidence_url: url })
      setSubject(''); setText(''); setUrl(''); onMessage('Апелляция подана. Решение появится в этом разделе.'); await load()
    } catch (err) { fail(onMessage)(err) }
  }
  async function submitViolation(e: FormEvent) {
    e.preventDefault()
    if (!chosen) { onMessage('Выберите спортсмена и соревнование'); return }
    try {
      await post('/api/violations', { competition_id: chosen.competition_id, athlete_id: chosen.athlete_id, kind, report: text, evidence_url: url })
      setText(''); setUrl(''); onMessage('Рапорт зафиксирован и передан Федерации.'); await load()
    } catch (err) { fail(onMessage)(err) }
  }
  async function decideAppeal(a: Appeal) {
    const d = draft('a' + a.id)
    try {
      await post(`/api/appeals/${a.id}/decide`, { status: d.status || 'accepted', decision: d.decision || '', new_place: Number(d.place) || 0 })
      onMessage('Решение по апелляции сохранено, заявитель уведомлён.'); await load()
    } catch (err) { fail(onMessage)(err) }
  }
  async function decideViolation(v: Violation) {
    const d = draft('v' + v.id)
    try {
      await post(`/api/violations/${v.id}/decide`, { sanction: d.sanction || 'warning', decision: d.decision || '' })
      onMessage('Решение по нарушению принято.'); await load()
    } catch (err) { fail(onMessage)(err) }
  }
  const select = <select value={pick} onChange={e => setPick(e.target.value)}><option value="">Выберите результат</option>{options.map((o, i) => <option key={i} value={i}>{o.competition} — {o.athlete} ({o.place} место)</option>)}</select>

  return <div className="ex-stack">
    {(canAppeal || canReport) && <div className="ex-two">
      {canAppeal && <section className="panel ex-card"><div className="panel-kicker">АПЕЛЛЯЦИЯ</div><h3>Подать апелляцию на результат</h3>
        <form onSubmit={submitAppeal} className="ex-form"><Field label="Результат">{select}</Field><Field label="Тема"><input required value={subject} onChange={e => setSubject(e.target.value)} placeholder="Например, не засчитана задача C" /></Field>
          <Field label="Описание"><textarea required minLength={10} rows={3} value={text} onChange={e => setText(e.target.value)} /></Field><Field label="Ссылка на доказательства (логи, скриншоты, видео)"><input value={url} onChange={e => setUrl(e.target.value)} placeholder="https://" /></Field>
          <button className="button button-primary">Отправить апелляцию</button></form></section>}
      {canReport && <section className="panel ex-card"><div className="panel-kicker">НАРУШЕНИЕ</div><h3>Зафиксировать нарушение (рапорт)</h3>
        <form onSubmit={submitViolation} className="ex-form"><Field label="Спортсмен и соревнование">{select}</Field>
          <Field label="Вид нарушения"><select value={kind} onChange={e => setKind(e.target.value)}><option>Нарушение правил соревнования</option><option>Использование запрещённой помощи</option><option>Неспортивное поведение</option><option>Нарушение антидопинговых правил</option></select></Field>
          <Field label="Рапорт"><textarea required minLength={10} rows={3} value={text} onChange={e => setText(e.target.value)} /></Field><Field label="Ссылка на доказательства"><input value={url} onChange={e => setUrl(e.target.value)} placeholder="https://" /></Field>
          <button className="button button-primary">Подать рапорт</button></form></section>}
    </div>}

    <section className="panel ex-card"><div className="panel-kicker">ЖУРНАЛ</div><h3>Апелляции</h3>
      {appeals.length === 0 && <Empty text="Апелляций пока нет." />}
      {appeals.map(a => <article className="ex-item" key={a.id}>
        <div className="ex-item-head"><b>{a.subject}</b><Pill map={appealStatus} value={a.status} /></div>
        <p>{a.competition} · {a.athlete} · текущее место {a.current_place || '—'} · {a.created_at}</p><p>{a.description}</p>
        {a.evidence_url && <p><a href={a.evidence_url} target="_blank" rel="noreferrer">Доказательства</a></p>}
        {a.decision && <p className="ex-decision">Решение: {a.decision}{a.new_place ? ` Новое место: ${a.new_place}.` : ''}</p>}
        <History items={a.history} />
        {role === 'federation' && (a.status === 'submitted' || a.status === 'review') && <div className="ex-decide">
          <select value={draft('a' + a.id).status || 'accepted'} onChange={e => setDraft('a' + a.id, { status: e.target.value })}><option value="review">Взять на рассмотрение</option><option value="accepted">Удовлетворить</option><option value="rejected">Отклонить</option></select>
          <input type="number" min={1} placeholder="Новое место (необязательно)" value={draft('a' + a.id).place || ''} onChange={e => setDraft('a' + a.id, { place: e.target.value })} />
          <input placeholder="Обоснование решения" value={draft('a' + a.id).decision || ''} onChange={e => setDraft('a' + a.id, { decision: e.target.value })} />
          <button className="button button-light" onClick={() => void decideAppeal(a)}>Сохранить решение</button></div>}
      </article>)}
    </section>

    <section className="panel ex-card"><div className="panel-kicker">ЖУРНАЛ</div><h3>Нарушения и санкции</h3>
      {violations.length === 0 && <Empty text="Зафиксированных нарушений нет." />}
      {violations.map(v => <article className="ex-item" key={v.id}>
        <div className="ex-item-head"><b>{v.kind}</b><span className={`status-pill ${v.status === 'decided' ? 'status-done' : 'status-planned'}`}>{v.status === 'decided' ? sanctionLabel[v.sanction] || 'Решение принято' : 'Рапорт подан'}</span></div>
        <p>{v.competition} · {v.athlete} · {v.created_at}</p><p>{v.report}</p>
        {v.evidence_url && <p><a href={v.evidence_url} target="_blank" rel="noreferrer">Доказательства</a></p>}
        {v.decision && <p className="ex-decision">Решение: {v.decision}</p>}
        <History items={v.history} />
        {role === 'federation' && v.status === 'reported' && <div className="ex-decide">
          <select value={draft('v' + v.id).sanction || 'warning'} onChange={e => setDraft('v' + v.id, { sanction: e.target.value })}>{Object.entries(sanctionLabel).map(([k, l]) => <option key={k} value={k}>{l}</option>)}</select>
          <input placeholder="Обоснование решения" value={draft('v' + v.id).decision || ''} onChange={e => setDraft('v' + v.id, { decision: e.target.value })} />
          <button className="button button-light" onClick={() => void decideViolation(v)}>Принять решение</button></div>}
      </article>)}
    </section>
  </div>
}

// ---------- Д3: организации, группы, журнал занятий ----------
export function OrganizationsView({ role, athletes, onMessage }: { role: string; athletes: { id: number; full_name: string }[]; onMessage: Msg }) {
  const [orgs, setOrgs] = useState<Org[]>([])
  const [groups, setGroups] = useState<Group[]>([])
  const [stages, setStages] = useState<string[]>([])
  const [disciplines, setDisciplines] = useState<string[]>([])
  const [open, setOpen] = useState<number | null>(null)
  const [members, setMembers] = useState<Member[]>([])
  const [sessions, setSessions] = useState<Session[]>([])
  const [orgForm, setOrgForm] = useState({ name: '', org_type: 'Школа', municipality: 'Оренбург', license: '', programs: '' })
  const [grpForm, setGrpForm] = useState({ organization_id: '', name: '', stage: '', discipline: '' })
  const [addId, setAddId] = useState('')
  const [moveTo, setMoveTo] = useState<Record<number, string>>({})
  const [ses, setSes] = useState({ held_at: new Date().toISOString().slice(0, 10), topic: '', format: 'очно', proof_url: '' })
  const [present, setPresent] = useState<number[]>([])
  const canEdit = role === 'federation' || role === 'coach'
  const load = useCallback(async () => {
    try {
      const o = await xapi<{ items: Org[]; stages: string[]; disciplines: string[] }>('/api/organizations')
      setOrgs(o.items); setStages(o.stages); setDisciplines(o.disciplines)
      if (role !== 'athlete' && role !== 'judge') setGroups((await xapi<{ items: Group[] }>('/api/groups')).items)
    } catch (e) { fail(onMessage)(e) }
  }, [onMessage, role])
  useEffect(() => { void load() }, [load])
  async function openGroup(id: number) {
    try {
      const d = await xapi<{ members: Member[]; sessions: Session[] }>(`/api/groups/${id}`)
      setOpen(id); setMembers(d.members); setSessions(d.sessions); setPresent(d.members.map(m => m.athlete_id))
    } catch (e) { fail(onMessage)(e) }
  }
  const run = async (fn: () => Promise<unknown>, ok: string, reopen?: number) => { try { await fn(); onMessage(ok); await load(); if (reopen) await openGroup(reopen) } catch (e) { fail(onMessage)(e) } }
  const current = groups.find(g => g.id === open)

  return <div className="ex-stack">
    <section className="panel ex-card"><div className="panel-kicker">РЕЕСТР</div><h3>Организации спортивной подготовки</h3>
      <div className="ex-cards">{orgs.map(o => <article className="ex-item" key={o.id}><b>{o.name}</b><p>{o.org_type} · {o.municipality}</p><p>Лицензия: {o.license || '—'}</p><p>{o.programs}</p>
        <p>Тренеры: {o.coaches || '—'}</p><p><b>{o.groups}</b> групп · <b>{o.athletes_in_groups}</b> спортсменов в группах · {o.athletes} в реестре</p></article>)}</div>
      {orgs.length === 0 && <Empty text="Организаций пока нет." />}
      {role === 'federation' && <form className="ex-form ex-inline" onSubmit={e => { e.preventDefault(); void run(() => post('/api/organizations', orgForm), 'Организация добавлена') }}>
        <input required placeholder="Название организации" value={orgForm.name} onChange={e => setOrgForm({ ...orgForm, name: e.target.value })} />
        <input placeholder="Тип" value={orgForm.org_type} onChange={e => setOrgForm({ ...orgForm, org_type: e.target.value })} />
        <input placeholder="Муниципалитет" value={orgForm.municipality} onChange={e => setOrgForm({ ...orgForm, municipality: e.target.value })} />
        <input placeholder="Реквизиты лицензии" value={orgForm.license} onChange={e => setOrgForm({ ...orgForm, license: e.target.value })} />
        <input placeholder="Программы подготовки" value={orgForm.programs} onChange={e => setOrgForm({ ...orgForm, programs: e.target.value })} />
        <button className="button button-primary">Добавить организацию</button></form>}
    </section>

    {role !== 'athlete' && role !== 'judge' && <section className="panel ex-card"><div className="panel-kicker">ГРУППЫ</div><h3>Учебно-тренировочные группы по этапам подготовки</h3>
      <div className="table-scroll"><table className="ex-table"><thead><tr><th>Группа</th><th>Организация</th><th>Этап</th><th>Дисциплина</th><th>Тренер</th><th>Состав</th><th>Занятий</th><th /></tr></thead><tbody>
        {groups.map(g => <tr key={g.id}><td>{g.name}</td><td>{g.organization}</td><td>{g.stage}</td><td>{g.discipline.replace('Программирование ', '')}</td><td>{g.coach}</td><td>{g.members}</td><td>{g.sessions}</td><td><button className="button button-light" onClick={() => void openGroup(g.id)}>Открыть</button></td></tr>)}
      </tbody></table></div>
      {groups.length === 0 && <Empty text="Групп пока нет." />}
      {canEdit && <form className="ex-form ex-inline" onSubmit={e => { e.preventDefault(); void run(() => post('/api/groups', { ...grpForm, organization_id: Number(grpForm.organization_id) }), 'Группа создана') }}>
        <select required value={grpForm.organization_id} onChange={e => setGrpForm({ ...grpForm, organization_id: e.target.value })}><option value="">Организация</option>{orgs.map(o => <option key={o.id} value={o.id}>{o.name}</option>)}</select>
        <input required placeholder="Название группы" value={grpForm.name} onChange={e => setGrpForm({ ...grpForm, name: e.target.value })} />
        <select required value={grpForm.stage} onChange={e => setGrpForm({ ...grpForm, stage: e.target.value })}><option value="">Этап подготовки</option>{stages.map(s => <option key={s}>{s}</option>)}</select>
        <select required value={grpForm.discipline} onChange={e => setGrpForm({ ...grpForm, discipline: e.target.value })}><option value="">Дисциплина</option>{disciplines.map(d => <option key={d}>{d}</option>)}</select>
        <button className="button button-primary">Создать группу</button></form>}
    </section>}

    {current && <section className="panel ex-card"><div className="panel-kicker">ГРУППА</div><h3>{current.name} · {current.stage}</h3>
      <div className="ex-two"><div><h4>Состав</h4>{members.length === 0 && <Empty text="В группе пока никого нет." />}
        {members.map(m => <div className="ex-row" key={m.athlete_id}><span>{m.full_name} · {m.rank_title || 'без разряда'} · с {m.joined_at}</span>
          {canEdit && <span className="ex-move"><select value={moveTo[m.athlete_id] || ''} onChange={e => setMoveTo({ ...moveTo, [m.athlete_id]: e.target.value })}><option value="">Перевести в…</option>{groups.filter(g => g.id !== current.id).map(g => <option key={g.id} value={g.id}>{g.name} — {g.stage}</option>)}</select>
            <button className="button button-light" disabled={!moveTo[m.athlete_id]} onClick={() => void run(() => post(`/api/groups/${current.id}/transfer`, { athlete_id: m.athlete_id, to_group_id: Number(moveTo[m.athlete_id]) }), 'Спортсмен переведён', current.id)}>Перевести</button></span>}</div>)}
        {canEdit && <div className="ex-inline"><select value={addId} onChange={e => setAddId(e.target.value)}><option value="">Зачислить спортсмена…</option>{athletes.map(a => <option key={a.id} value={a.id}>{a.full_name}</option>)}</select>
          <button className="button button-primary" disabled={!addId} onClick={() => void run(() => post(`/api/groups/${current.id}/members`, { athlete_id: Number(addId) }), 'Спортсмен зачислен', current.id)}>Зачислить</button></div>}</div>
        <div><h4>Журнал занятий</h4>{sessions.length === 0 && <Empty text="Занятий пока не проводилось." />}
          {sessions.map(s => <div className="ex-row" key={s.id}><span>{s.held_at} · {s.topic}</span><span><span className={`status-pill ${s.format === 'онлайн' ? 'status-draft' : 'status-open'}`}>{s.format}</span> {s.proof_url && <a href={s.proof_url} target="_blank" rel="noreferrer">запись</a>} · присутствовало {s.present}</span></div>)}
          {canEdit && <form className="ex-form" onSubmit={e => { e.preventDefault(); void run(() => post(`/api/groups/${current.id}/sessions`, { ...ses, present }), 'Занятие записано в журнал', current.id) }}>
            <div className="ex-inline"><input type="date" required value={ses.held_at} onChange={e => setSes({ ...ses, held_at: e.target.value })} /><select value={ses.format} onChange={e => setSes({ ...ses, format: e.target.value })}><option>очно</option><option>онлайн</option></select></div>
            <input required placeholder="Тема занятия" value={ses.topic} onChange={e => setSes({ ...ses, topic: e.target.value })} />
            <input placeholder={ses.format === 'онлайн' ? 'Ссылка на запись или лог (обязательно)' : 'Ссылка на подтверждение (необязательно)'} value={ses.proof_url} onChange={e => setSes({ ...ses, proof_url: e.target.value })} />
            <div className="ex-checks">{members.map(m => <label key={m.athlete_id}><input type="checkbox" checked={present.includes(m.athlete_id)} onChange={e => setPresent(p => e.target.checked ? [...p, m.athlete_id] : p.filter(x => x !== m.athlete_id))} />{m.full_name}</label>)}</div>
            <button className="button button-primary">Записать занятие</button></form>}</div></div>
    </section>}
  </div>
}

// ---------- Д12: документы о судействе (карточки для раздела «Документы») ----------
export function JudgeDocs({ role, judgeId, competitions, onMessage }: { role: string; judgeId?: number; competitions: { id: number; title: string }[]; onMessage: Msg }) {
  const [competitionId, setCompetitionId] = useState('')
  const [judges, setJudges] = useState<{ id: number; full_name: string }[]>([])
  const [pickedJudge, setPickedJudge] = useState('')
  useEffect(() => {
    if (role === 'federation' || role === 'ministry') xapi<{ items: { id: number; full_name: string }[] }>('/api/judges').then(r => setJudges(r.items)).catch(() => setJudges([]))
  }, [role])
  async function download(path: string, filename: string) {
    try {
      const token = localStorage.getItem('fsp_token')
      const response = await fetch(path, { headers: token ? { Authorization: `Bearer ${token}` } : {} })
      if (!response.ok) { const p = await response.json().catch(() => ({})); throw new Error(p.error || `Ошибка выгрузки: ${response.status}`) }
      const url = URL.createObjectURL(await response.blob())
      const a = document.createElement('a'); a.href = url; a.download = filename; document.body.appendChild(a); a.click(); a.remove()
      window.setTimeout(() => URL.revokeObjectURL(url), 1000); onMessage(`Файл «${filename}» сформирован.`)
    } catch (e) { fail(onMessage)(e) }
  }
  const jid = role === 'judge' ? judgeId : Number(pickedJudge) || undefined
  return <div className="document-grid ex-docs">
    <div className="document-card"><h3>Справка о составе судейской коллегии</h3><p>Судьи соревнования, их должности и категории со сроками действия.</p>
      <label className="document-select-label">Соревнование<select value={competitionId} onChange={e => setCompetitionId(e.target.value)}><option value="">Выберите соревнование</option>{competitions.map(c => <option key={c.id} value={c.id}>{c.title}</option>)}</select></label>
      <div className="document-card-foot"><span className="doc-status">DOCX / XLSX</span><span className="ex-btns">
        <button disabled={!competitionId} onClick={() => void download(`/api/documents/judges/composition?competition_id=${competitionId}`, `judges-${competitionId}.docx`)}>DOCX</button>
        <button disabled={!competitionId} onClick={() => void download(`/api/documents/judges/composition?competition_id=${competitionId}&format=xlsx`, `judges-${competitionId}.xlsx`)}>XLSX</button></span></div></div>
    <div className="document-card"><h3>Отчёт о судействе</h3><p>Соревнования, на которых работал судья, с должностями.</p>
      {role !== 'judge' && <label className="document-select-label">Судья<select value={pickedJudge} onChange={e => setPickedJudge(e.target.value)}><option value="">Выберите судью</option>{judges.map(j => <option key={j.id} value={j.id}>{j.full_name}</option>)}</select></label>}
      <div className="document-card-foot"><span className="doc-status">DOCX / XLSX</span><span className="ex-btns">
        <button disabled={!jid} onClick={() => void download(`/api/documents/judges/${jid}/report`, `judge-report-${jid}.docx`)}>DOCX</button>
        <button disabled={!jid} onClick={() => void download(`/api/documents/judges/${jid}/report?format=xlsx`, `judge-report-${jid}.xlsx`)}>XLSX</button></span></div></div>
  </div>
}
