import { useCallback, useEffect, useMemo, useState, type CSSProperties, type FormEvent } from 'react'
import {
  Activity, ArrowDownRight, ArrowRight, ArrowUp, ArrowUpRight, Bell, CalendarDays,
  CheckCircle2, ChevronDown, ChevronRight, CircleHelp, ClipboardList, Code2,
  FileBadge, FileText, Filter, Flag, LayoutDashboard, Menu, Plus, Search,
  ShieldCheck, Users, UserRound, Trophy, X, LogOut, Sun, Moon, Monitor, Send,
} from 'lucide-react'
import { BonusesView, NormsView, PathView } from './Extras2'
import { AppealsView, JudgeDocs, OrganizationsView } from './Extras'
import { ReadinessView } from './Readiness'
import { JudgingView, LinksView, RanksView, ResultsView } from './Workflows'
import fspLogo from './assets/fsp-logo.svg'
import fspLogoDark from './assets/fsp-logo-dark.svg'

type PageKey = 'overview' | 'competitions' | 'participants' | 'rating' | 'team' | 'documents' | 'notifications' | 'appeals' | 'organizations' | 'norms' | 'bonuses' | 'path' | 'links' | 'results' | 'judging' | 'ranks' | 'readiness'
type Competition = {
  id: number; title: string; discipline: string; competition_type: string; age_group: string;
  starts_at: string; location: string; status: string; participants: number
}
type Athlete = {
  id: number; full_name: string; organization: string; discipline: string; age_group: string; rank_title?: string;
  points: number; coach_name: string; status: string
}
type RatingEntry = {
  athlete_id?: number; rank: number; athlete: string; organization: string; discipline: string; age_group: string; rank_title?: string; points: number
}
type Notice = { id: number; kind: string; title: string; message: string; when: string }
type ApplicationItem = { id: number; competition_id: number; competition: string; athlete_id: number; athlete: string; discipline: string; age_group: string; status: string; reject_reason: string; created_at: string }
type TelegramStatus = { configured: boolean; default_chat_configured: boolean; polling_enabled: boolean; mode: string }
type TelegramLinkStatus = { linked: boolean }
type DashboardData = { stats: { athletes: number; competitions: number; applications: number; judges: number }; competitions_total: number; competitions: Competition[]; top_athletes: RatingEntry[] }

type Role = 'Федерация' | 'Тренер' | 'Спортсмен' | 'Судья' | 'Министерство'
type RoleCode = 'federation' | 'coach' | 'athlete' | 'judge' | 'ministry'
type AuthUser = { id: number; email: string; full_name: string; roles: RoleCode[]; active_role: RoleCode; organization: string; municipality: string; athlete_id?: number; coach_id?: number; judge_id?: number }
const roleLabels: Record<RoleCode, Role> = { federation: 'Федерация', coach: 'Тренер', athlete: 'Спортсмен', judge: 'Судья', ministry: 'Министерство' }
const roleCodes: Record<Role, RoleCode> = { 'Федерация': 'federation', 'Тренер': 'coach', 'Спортсмен': 'athlete', 'Судья': 'judge', 'Министерство': 'ministry' }

const fallbackCompetitions: Competition[] = [
  { id: 1, title: 'Кубок Оренбуржья по спортивному программированию', discipline: 'Программирование алгоритмическое', competition_type: 'Региональные соревнования', age_group: 'Юноши и девушки 14–17', starts_at: '2026-11-08T10:00:00+05:00', location: 'Оренбург · ОГУ', status: 'Регистрация', participants: 86 },
  { id: 2, title: 'Осенний алгоритмический спринт', discipline: 'Программирование продуктовое', competition_type: 'Открытые соревнования', age_group: 'Все группы', starts_at: '2026-11-16T11:00:00+05:00', location: 'Онлайн', status: 'Регистрация', participants: 124 },
  { id: 3, title: 'Отбор в сборную области', discipline: 'Программирование систем искусственного интеллекта', competition_type: 'Отборочные соревнования', age_group: 'Юниоры и юниорки 18–20', starts_at: '2026-11-29T09:30:00+05:00', location: 'Оренбург · Точка кипения', status: 'Планируется', participants: 0 },
]
const fallbackAthletes: Athlete[] = [
  { id: 1, full_name: 'Алексей Морозов', organization: 'Лицей № 1', discipline: 'Программирование алгоритмическое', age_group: 'Юноши и девушки 14–17', rank_title: 'КМС', points: 968, coach_name: 'Мария Крылова', status: 'Активен' },
  { id: 2, full_name: 'Дарья Соколова', organization: 'ОГУ', discipline: 'Программирование продуктовое', age_group: 'Мужчины и женщины 21+', rank_title: '1 разряд', points: 914, coach_name: 'Илья Волков', status: 'Активен' },
  { id: 3, full_name: 'Максим Орлов', organization: 'Гимназия № 2', discipline: 'Программирование робототехники', age_group: 'Юниоры и юниорки 18–20', rank_title: '2 разряд', points: 887, coach_name: 'Мария Крылова', status: 'Активен' },
  { id: 4, full_name: 'Полина Васильева', organization: 'ОГУ', discipline: 'Программирование систем информационной безопасности', age_group: 'Мужчины и женщины 21+', rank_title: '1 разряд', points: 841, coach_name: 'Илья Волков', status: 'Активен' },
  { id: 5, full_name: 'Артём Кузнецов', organization: 'Лицей № 1', discipline: 'Программирование беспилотных авиационных систем', age_group: 'Юниоры и юниорки 18–20', rank_title: '3 разряд', points: 812, coach_name: 'Мария Крылова', status: 'Активен' },
  { id: 6, full_name: 'София Попова', organization: 'IT-центр Орск', discipline: 'Программирование продуктовое', age_group: 'Юноши и девушки 14–17', rank_title: '3 разряд', points: 779, coach_name: 'Никита Егоров', status: 'Активен' },
  { id: 7, full_name: 'Иван Лебедев', organization: 'ОГУ', discipline: 'Программирование алгоритмическое', age_group: 'Мужчины и женщины 21+', rank_title: 'КМС', points: 745, coach_name: 'Илья Волков', status: 'Активен' },
  { id: 8, full_name: 'Ева Фёдорова', organization: 'Гимназия № 2', discipline: 'Программирование робототехники', age_group: 'Юниоры и юниорки 18–20', rank_title: '2 разряд', points: 702, coach_name: 'Мария Крылова', status: 'Активен' },
]
const fallbackRating: RatingEntry[] = fallbackAthletes.map((a, i) => ({ athlete_id: a.id, rank: i + 1, athlete: a.full_name, organization: a.organization, discipline: a.discipline, age_group: a.age_group, rank_title: a.rank_title, points: a.points }))
const fallbackNotices: Notice[] = [
  { id: 1, kind: 'warning', title: 'Проверить судейские категории', message: 'У 3 судей скоро истекает срок категории.', when: 'Сегодня' },
  { id: 2, kind: 'info', title: 'Новые заявки на проверке', message: '12 заявок ожидают модерации Федерации.', when: '2 ч назад' },
  { id: 3, kind: 'success', title: 'Протокол готов к публикации', message: 'По последнему турниру сформирован черновик протокола.', when: 'Вчера' },
]

const navItems: { key: PageKey; label: string; icon: typeof LayoutDashboard; tag?: string }[] = [
  { key: 'overview', label: 'Главная', icon: LayoutDashboard },
  { key: 'competitions', label: 'Соревнования', icon: CalendarDays },
  { key: 'participants', label: 'Участники', icon: Users },
  { key: 'rating', label: 'Рейтинг', icon: Trophy },
  { key: 'team', label: 'Сборная', icon: Flag },
  { key: 'documents', label: 'Документы', icon: FileText },
  { key: 'appeals', label: 'Апелляции', icon: Flag },
  { key: 'organizations', label: 'Организации', icon: Users },
  { key: 'norms', label: 'Нормативы', icon: Activity },
  { key: 'bonuses', label: 'Премии и показатели', icon: Trophy },
  { key: 'path', label: 'Путь и справочник', icon: CircleHelp },
  { key: 'links', label: 'Подопечные', icon: Users },
  { key: 'results', label: 'Результаты', icon: ClipboardList },
  { key: 'judging', label: 'Судейство', icon: ShieldCheck },
  { key: 'ranks', label: 'Разряды', icon: FileBadge },
  { key: 'readiness', label: 'Готовность к старту', icon: CheckCircle2 },
  { key: 'notifications', label: 'Уведомления', icon: Bell },
]
const roleDescriptions: Record<Role, string> = {
  Федерация: 'Региональное управление', Тренер: 'Кабинет тренера', Спортсмен: 'Кабинет спортсмена', Судья: 'Судейская коллегия', Министерство: 'Только просмотр (Министерство спорта)',
}
const titles: Record<PageKey, string> = {
  overview: 'Обзор системы', competitions: 'Соревнования', participants: 'Реестр участников', rating: 'Рейтинг спортсменов', team: 'Сборная области', documents: 'Документы и отчёты', notifications: 'Центр уведомлений', appeals: 'Апелляции и нарушения', organizations: 'Организации и группы', norms: 'Нормативы и аттестация', bonuses: 'Премии и показатели развития', path: 'Личный путь и справочник', readiness: 'Готовность к старту', links: 'Привязка тренера и спортсмена', results: 'Результаты и протоколы', judging: 'Судейская коллегия и категории', ranks: 'Заявки на разряд',
}

function formatFullDate(raw: string) {
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return raw
  return new Intl.DateTimeFormat('ru-RU', { day: '2-digit', month: 'long', year: 'numeric' }).format(d)
}
function greeting() {
  const hour = new Date().getHours()
  return hour < 5 ? 'Доброй ночи' : hour < 12 ? 'Доброе утро' : hour < 18 ? 'Добрый день' : 'Добрый вечер'
}
function initialsOf(name: string) { return name.split(' ').map(x => x[0]).slice(0, 2).join('') }
const allDisciplines = ['Программирование алгоритмическое', 'Программирование продуктовое', 'Программирование систем информационной безопасности', 'Программирование систем искусственного интеллекта', 'Программирование робототехники', 'Программирование беспилотных авиационных систем']

/** Число, которое плавно «набегает» от нуля. Уважает prefers-reduced-motion. */
function CountUp({ value, pad = 0 }: { value: number; pad?: number }) {
  const [shown, setShown] = useState(0)
  useEffect(() => {
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) { setShown(value); return }
    let frame = 0
    const startedAt = performance.now()
    const tick = (now: number) => {
      const t = Math.min(1, (now - startedAt) / 1200)
      setShown(Math.round(value * (1 - Math.pow(1 - t, 3))))
      if (t < 1) frame = requestAnimationFrame(tick)
    }
    frame = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(frame)
  }, [value])
  return <>{pad ? String(shown).padStart(pad, '0') : shown.toLocaleString('ru-RU')}</>
}

function RoleSelect({ user, role, onChange, className = '' }: { user: AuthUser; role: Role; onChange: (role: Role) => void; className?: string }) {
  return <label className={`role-select-wrap ${className}`}><ShieldCheck size={16} /><select aria-label="Активная роль" value={role} onChange={e => onChange(e.target.value as Role)}>{user.roles.map(code => <option key={code} value={roleLabels[code]}>{roleLabels[code]}</option>)}</select><ChevronDown size={14} /></label>
}

function SiteFooter({ items, onNavigate, logo }: { items: { key: PageKey; label: string }[]; onNavigate: (page: PageKey) => void; logo: string }) {
  return <footer className="site-footer">
    <div className="footer-stripe" aria-hidden="true" />
    <div className="footer-inner">
      <div className="footer-brand"><img className="fsp-logo fsp-logo-footer" src={logo} alt="ФСП 56 — Оренбургская область" /><p>Региональная платформа Федерации спортивного программирования Оренбургской области: соревнования, заявки, рейтинг и сборная — в одном месте.</p></div>
      <div className="footer-col"><h4>Разделы</h4>{items.map(item => <button key={item.key} onClick={() => onNavigate(item.key)}>{item.label}</button>)}</div>
      <div className="footer-col"><h4>На связи</h4><a href="https://t.me/FSP56OrenburgBot" target="_blank" rel="noreferrer">Telegram-бот ФСП 56</a><a href="https://fsp-russia.ru/" target="_blank" rel="noreferrer">Сайт ФСП России</a></div>
      <div className="footer-col"><h4>Интеграции</h4><a href="/oidc/demo">Войти через ФСП ID (демо)</a><a href="/oidc/admin">ФСП ID: клиенты (Федерация)</a><a href="/api/integrations/console">Вебхуки и API-токены (Федерация)</a><a href="/api/v1/openapi.yaml">Внешний API (OpenAPI)</a></div>
    </div>
    <div className="footer-bottom"><span>© 2026 ФСП · Оренбургская область</span><span>Сделано на хакатоне ОренКод</span><button className="to-top" onClick={() => window.scrollTo({ top: 0, behavior: 'smooth' })}><ArrowUp size={14} /> Наверх</button></div>
  </footer>
}

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const token = localStorage.getItem('fsp_token')
  const response = await fetch(path, { ...init, headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}), ...(init?.headers || {}) } })
  if (!response.ok) {
    const payload = await response.json().catch(() => ({}))
    throw new Error(payload.error || `Ошибка запроса: ${response.status}`)
  }
  return response.json() as Promise<T>
}

type ThemePreference = 'system' | 'light' | 'dark'

function ThemeControl({ preference, effectiveTheme, onToggle }: { preference: ThemePreference; effectiveTheme: 'light' | 'dark'; onToggle: () => void }) {
  const Icon = preference === 'system' ? Monitor : effectiveTheme === 'dark' ? Moon : Sun
  const label = preference === 'system' ? `Тема: системная (${effectiveTheme === 'dark' ? 'тёмная' : 'светлая'})` : `Тема: ${effectiveTheme === 'dark' ? 'тёмная' : 'светлая'}`
  return <button type="button" className="top-icon theme-control" onClick={onToggle} aria-label={`${label}. Нажмите, чтобы изменить`} title={`${label} · переключить тему`}><Icon size={19} /><span className="theme-control-label">{preference === 'system' ? 'Авто' : effectiveTheme === 'dark' ? 'Тёмная' : 'Светлая'}</span></button>
}

export default function App() {
  const [themePreference, setThemePreference] = useState<ThemePreference>(() => { const saved = localStorage.getItem('fsp_theme'); return saved === 'light' || saved === 'dark' ? saved : 'system' })
  const [systemTheme, setSystemTheme] = useState<'light' | 'dark'>(() => window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
  const effectiveTheme: 'light' | 'dark' = themePreference === 'system' ? systemTheme : themePreference
  useEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)')
    const update = (event: MediaQueryListEvent) => setSystemTheme(event.matches ? 'dark' : 'light')
    setSystemTheme(media.matches ? 'dark' : 'light')
    media.addEventListener?.('change', update)
    return () => media.removeEventListener?.('change', update)
  }, [])
  useEffect(() => { document.documentElement.dataset.theme = effectiveTheme; document.documentElement.style.colorScheme = effectiveTheme }, [effectiveTheme])
  function toggleTheme() { const next: ThemePreference = themePreference === 'system' ? 'light' : themePreference === 'light' ? 'dark' : 'system'; setThemePreference(next); if (next === 'system') localStorage.removeItem('fsp_theme'); else localStorage.setItem('fsp_theme', next) }
  const [page, setPage] = useState<PageKey>('overview')
  const [role, setRole] = useState<Role>('Федерация')
  const [user, setUser] = useState<AuthUser | null>(null)
  const [authChecking, setAuthChecking] = useState(true)
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [dashboard, setDashboard] = useState<DashboardData>({
    stats: { athletes: 248, competitions: 6, applications: 312, judges: 24 }, competitions_total: 12, competitions: fallbackCompetitions, top_athletes: fallbackRating.slice(0, 5),
  })
  const [competitions, setCompetitions] = useState<Competition[]>(fallbackCompetitions)
  const [athletes, setAthletes] = useState<Athlete[]>(fallbackAthletes)
  const [rating, setRating] = useState<RatingEntry[]>(fallbackRating)
  const [team, setTeam] = useState<RatingEntry[]>(fallbackRating.slice(0, 5))
  const [notices, setNotices] = useState<Notice[]>(fallbackNotices)
  const [applications, setApplications] = useState<ApplicationItem[]>([])
  const [telegramStatus, setTelegramStatus] = useState<TelegramStatus>({ configured: false, default_chat_configured: false, polling_enabled: false, mode: 'long_polling' })
  const [telegramLinkStatus, setTelegramLinkStatus] = useState<TelegramLinkStatus>({ linked: false })
  const [telegramLinkCode, setTelegramLinkCode] = useState('')
  const [telegramLinkExpiresAt, setTelegramLinkExpiresAt] = useState<number | null>(null)
  const [telegramLinkBusy, setTelegramLinkBusy] = useState(false)
  const [notice, setNotice] = useState('')
  const [search, setSearch] = useState('')
  const [discipline, setDiscipline] = useState('Все дисциплины')
  const [ageGroup, setAgeGroup] = useState('Все группы')
  const [modalOpen, setModalOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const [dataSource, setDataSource] = useState<'demo' | 'api'>('demo')

  useEffect(() => {
    const token = localStorage.getItem('fsp_token')
    if (!token) { setAuthChecking(false); return }
    api<AuthUser>('/api/auth/me').then(u => { setUser(u); setRole(roleLabels[u.active_role] || 'Спортсмен') }).catch(() => { localStorage.removeItem('fsp_token'); setUser(null) }).finally(() => setAuthChecking(false))
  }, [])

  const loadData = useCallback(async () => {
    if (!user) return
    try {
      setLoading(true)
      const [d, c, a, r, t, n, nTelegram, nTelegramLink] = await Promise.all([
        api<DashboardData>('/api/dashboard'), api<{ items: Competition[] }>('/api/competitions'),
        (user.active_role === 'federation' || user.active_role === 'coach' ? api<{ items: Athlete[] }>('/api/athletes') : Promise.resolve({ items: [] as Athlete[] })), api<{ items: RatingEntry[] }>('/api/rating'),
        api<{ items: RatingEntry[] }>('/api/team'), api<{ items: Notice[] }>('/api/notifications'),
        api<TelegramStatus>('/api/integrations/telegram/status'), api<TelegramLinkStatus>('/api/integrations/telegram/link-status'),
      ])
      setDashboard(d); setCompetitions(c.items); setAthletes(a.items); setRating(r.items); setTeam(t.items); setNotices(n.items); setTelegramStatus(nTelegram); setTelegramLinkStatus(nTelegramLink); if (nTelegramLink.linked) { setTelegramLinkCode(''); setTelegramLinkExpiresAt(null) }; setDataSource('api')
      if (user.active_role === 'federation') {
        try { const apps = await api<{ items: ApplicationItem[] }>('/api/applications'); setApplications(apps.items) } catch { setApplications([]) }
      } else { setApplications([]) }
    } catch {
      setNotice('API пока недоступно — показаны демонстрационные данные. Запустите проект через Docker Compose.')
    } finally { setLoading(false) }
  }, [user])

  useEffect(() => { if (user) void loadData() }, [user, loadData])
  useEffect(() => {
    if (!user || !telegramStatus.configured || telegramStatus.polling_enabled) return
    let cancelled = false
    const refreshTelegramStatus = async () => {
      try {
        const status = await api<TelegramStatus>('/api/integrations/telegram/status')
        if (!cancelled) setTelegramStatus(status)
      } catch { /* status polling is best-effort while the bot starts */ }
    }
    const timer = window.setInterval(() => void refreshTelegramStatus(), 5000)
    void refreshTelegramStatus()
    return () => { cancelled = true; window.clearInterval(timer) }
  }, [user, telegramStatus.configured, telegramStatus.polling_enabled])
  useEffect(() => {
    if (!user || page !== 'notifications' || !telegramLinkCode || !telegramLinkExpiresAt || telegramLinkStatus.linked) return
    let checking = false
    const checkLinkStatus = async () => {
      if (checking) return
      if (Date.now() >= telegramLinkExpiresAt) {
        setTelegramLinkCode(''); setTelegramLinkExpiresAt(null)
        setNotice('Код подключения Telegram истёк. Создайте новый код.')
        return
      }
      checking = true
      try {
        const status = await api<TelegramLinkStatus>('/api/integrations/telegram/link-status')
        if (status.linked) {
          setTelegramLinkStatus(status); setTelegramLinkCode(''); setTelegramLinkExpiresAt(null)
          setNotice('Аккаунт Telegram успешно подключён. Персональные уведомления активны.')
        }
      } catch { /* временная недоступность статуса не должна сбрасывать введённый код */ }
      finally { checking = false }
    }
    const timer = window.setInterval(() => void checkLinkStatus(), 2500)
    void checkLinkStatus()
    return () => window.clearInterval(timer)
  }, [user, page, telegramLinkCode, telegramLinkExpiresAt, telegramLinkStatus.linked])
  useEffect(() => { if (!notice) return; const timer = window.setTimeout(() => setNotice(''), 4800); return () => window.clearTimeout(timer) }, [notice])

  const visibleCompetitions = useMemo(() => competitions.filter(c => `${c.title} ${c.discipline} ${c.location} ${c.status}`.toLowerCase().includes(search.toLowerCase())), [competitions, search])
  const visibleAthletes = useMemo(() => athletes.filter(a => `${a.full_name} ${a.organization} ${a.discipline} ${a.coach_name}`.toLowerCase().includes(search.toLowerCase())), [athletes, search])
  const visibleRating = useMemo(() => rating.filter(a => (discipline === 'Все дисциплины' || a.discipline === discipline) && (ageGroup === 'Все группы' || a.age_group === ageGroup) && `${a.athlete} ${a.organization} ${a.discipline}`.toLowerCase().includes(search.toLowerCase())).map((a, i) => ({ ...a, rank: i + 1 })), [rating, discipline, ageGroup, search])

  function openPage(next: PageKey) { setPage(next); setSearch(''); setSidebarOpen(false); window.scrollTo({ top: 0 }) }
  async function changeRole(nextRole: Role) {
    try { const updated = await api<AuthUser>('/api/auth/switch-role', { method: 'POST', body: JSON.stringify({ role: roleCodes[nextRole] }) }); setUser(updated); setRole(nextRole); setNotice(`Переключено на роль «${nextRole}»`); openPage('overview') }
    catch (error) { setNotice(error instanceof Error ? error.message : 'Не удалось сменить роль') }
  }
  async function logout() { try { await api('/api/auth/logout', { method: 'POST' }) } catch { /* локальная сессия всё равно очищается */ } localStorage.removeItem('fsp_token'); setUser(null); setPage('overview') }
  async function generateTelegramLinkCode() {
    setTelegramLinkBusy(true)
    try { const result = await api<{ code: string; expires_in_seconds: number }>('/api/integrations/telegram/link-code', { method: 'POST', body: JSON.stringify({}) }); setTelegramLinkCode(result.code); setTelegramLinkExpiresAt(Date.now() + result.expires_in_seconds * 1000); setNotice('Код Telegram создан и действует 10 минут.') }
    catch (error) { setNotice(error instanceof Error ? error.message : 'Не удалось создать код Telegram') }
    finally { setTelegramLinkBusy(false) }
  }
  async function unlinkTelegram() {
    setTelegramLinkBusy(true)
    try { await api('/api/integrations/telegram/link', { method: 'DELETE' }); setTelegramLinkStatus({ linked: false }); setTelegramLinkCode(''); setTelegramLinkExpiresAt(null); setNotice('Персональные уведомления Telegram отключены.') }
    catch (error) { setNotice(error instanceof Error ? error.message : 'Не удалось отключить Telegram') }
    finally { setTelegramLinkBusy(false) }
  }
  if (authChecking) return <div className="auth-loading"><img className="fsp-logo fsp-logo-loading" src={effectiveTheme === 'light' ? fspLogoDark : fspLogo} alt="ФСП 56 — Оренбургская область" /><p>Проверяем сессию…</p></div>
  if (!user) return <AuthScreen preference={themePreference} effectiveTheme={effectiveTheme} onToggleTheme={toggleTheme} onAuthenticated={(token, signedInUser) => { localStorage.setItem('fsp_token', token); setUser(signedInUser); setRole(roleLabels[signedInUser.active_role] || 'Спортсмен') }} />

  const visibleNav = navItems.filter(item => {
    if (role === 'Министерство') return ['overview', 'competitions', 'rating', 'team', 'documents', 'organizations', 'bonuses', 'path', 'notifications'].includes(item.key)
    if (role === 'Федерация') return item.key !== 'path'
    if (role === 'Тренер') return !['bonuses', 'path', 'judging'].includes(item.key)
    if (role === 'Спортсмен') return !['participants', 'norms', 'bonuses', 'judging'].includes(item.key)
    return ['overview', 'competitions', 'results', 'judging', 'documents', 'appeals', 'organizations', 'path', 'notifications'].includes(item.key)
  })
  const initials = initialsOf(user.full_name)
  const logoSrc = effectiveTheme === 'light' ? fspLogoDark : fspLogo
  const pageTitle = page === 'overview'
    ? (user.active_role === 'federation' ? 'Управление спортивным программированием' : user.active_role === 'coach' ? 'Кабинет тренера' : user.active_role === 'athlete' ? 'Кабинет спортсмена' : user.active_role === 'ministry' ? 'Кабинет министерства спорта' : 'Рабочее место судьи')
    : titles[page]

  return <div className="app-shell">
    <header className="site-header">
      <div className="header-inner">
        <button className="icon-button mobile-menu" onClick={() => setSidebarOpen(true)} aria-label="Открыть меню"><Menu size={22} /></button>
        <button className="brand-lockup" onClick={() => openPage('overview')} aria-label="ФСП 56 — на главную"><img className="fsp-logo fsp-logo-header" src={logoSrc} alt="ФСП 56 — Оренбургская область" /></button>
        <nav className="top-nav" aria-label="Основные разделы">{visibleNav.map(item => <button key={item.key} onClick={() => openPage(item.key)} className={`nav-item ${page === item.key ? 'active' : ''}`} aria-current={page === item.key ? 'page' : undefined}><span>{item.label}</span></button>)}</nav>
        <div className="header-actions">
          <ThemeControl preference={themePreference} effectiveTheme={effectiveTheme} onToggle={toggleTheme} />
          <button className="top-icon notification-top" aria-label="Уведомления" onClick={() => openPage('notifications')}><Bell size={19} /><i /></button>
          <RoleSelect className="header-role" user={user} role={role} onChange={next => void changeRole(next)} />
          <div className="header-profile" title={`${user.full_name} · ${roleLabels[user.active_role]}`}><div className="avatar">{initials}</div><button className="profile-logout" onClick={() => void logout()} aria-label="Выйти" title="Выйти"><LogOut size={17} /></button></div>
        </div>
      </div>
    </header>

    <div className={`drawer-layer ${sidebarOpen ? 'open' : ''}`} inert={!sidebarOpen}>
      <button className="mobile-overlay" aria-label="Закрыть меню" onClick={() => setSidebarOpen(false)} />
      <aside className="drawer">
        <div className="drawer-head"><img className="fsp-logo fsp-logo-drawer" src={logoSrc} alt="ФСП 56 — Оренбургская область" /><button className="icon-button" onClick={() => setSidebarOpen(false)} aria-label="Закрыть меню"><X size={20} /></button></div>
        <div className="season-chip"><span className="pulse-dot" /> СЕЗОН 2026 / 27</div>
        <nav className="drawer-nav">{visibleNav.map(item => <button key={item.key} onClick={() => openPage(item.key)} className={`nav-item ${page === item.key ? 'active' : ''}`}><item.icon size={18} strokeWidth={1.9} /><span>{item.label}</span></button>)}</nav>
        <RoleSelect className="drawer-role" user={user} role={role} onChange={next => void changeRole(next)} />
        <div className="drawer-profile"><div className="avatar">{initials}</div><div className="profile-copy"><b>{user.full_name}</b><small>{roleLabels[user.active_role]} · {user.organization || user.municipality}</small></div><button className="profile-logout" onClick={() => void logout()} aria-label="Выйти" title="Выйти"><LogOut size={17} /></button></div>
      </aside>
    </div>

    <main className="main-area">
      <div className="page-content" key={page}>
        <section className="welcome-row">
          <div><div className="eyebrow"><span className="eyebrow-line" /> ФСП · ОРЕНБУРГСКАЯ ОБЛАСТЬ</div><h1>{page === 'overview' ? `${greeting()}!` : titles[page]}</h1><p>{page === 'overview' ? `${pageTitle} · ${roleDescriptions[role]}` : `${roleDescriptions[role]} · Оренбургская область`}</p></div>
          <div className="welcome-actions"><div className="last-updated"><span className={`refresh-dot ${loading ? 'loading' : ''}`} />{loading ? 'Обновляем данные…' : dataSource === 'api' ? 'Данные актуальны' : 'Показаны примеры данных'}</div>{user.active_role === 'federation' && (page === 'overview' || page === 'competitions') && <button className="button button-primary" onClick={() => setModalOpen(true)}><Plus size={17} /> Создать соревнование</button>}</div>
        </section>

        {page === 'overview' && <Dashboard dashboard={dashboard} competitions={competitions} rating={rating} notices={notices} telegramStatus={telegramStatus} onNavigate={openPage} role={user.active_role} />}
        {page === 'competitions' && <><CompetitionsView items={visibleCompetitions} search={search} setSearch={setSearch} canCreate={user.active_role === 'federation'} role={user.active_role} onCreate={() => setModalOpen(true)} onApply={async id => { try { await api(`/api/competitions/${id}/apply`, { method: 'POST', body: JSON.stringify({}) }); setNotice('Заявка отправлена на рассмотрение Федерации.'); await loadData() } catch (e) { setNotice(e instanceof Error ? e.message : 'Не удалось подать заявку') } }} onPublish={async id => { try { await api(`/api/competitions/${id}/publish`, { method: 'POST', body: JSON.stringify({ open: true }) }); setNotice('Соревнование опубликовано, приём заявок открыт.'); await loadData() } catch (e) { setNotice(e instanceof Error ? e.message : 'Не удалось опубликовать соревнование') } }} />{user.active_role === 'federation' && <ApplicationsPanel items={applications} onDecide={async (id, status) => { try { await api(`/api/applications/${id}/decide`, { method: 'POST', body: JSON.stringify({ status, reason: status === 'rejected' ? 'Не соответствует требованиям соревнования' : '' }) }); setNotice(status === 'approved' ? 'Заявка одобрена.' : 'Заявка отклонена.'); await loadData() } catch (e) { setNotice(e instanceof Error ? e.message : 'Не удалось обработать заявку') } }} />}</>}
        {page === 'participants' && <AthletesView items={visibleAthletes} search={search} setSearch={setSearch} canApprove={user.active_role === 'federation'} onApprove={async id => { try { await api(`/api/athletes/${id}/approve`, { method: 'POST', body: JSON.stringify({}) }); setNotice('Профиль спортсмена подтверждён.'); await loadData() } catch (e) { setNotice(e instanceof Error ? e.message : 'Не удалось подтвердить профиль') } }} />}
        {page === 'rating' && <RatingView items={visibleRating} discipline={discipline} setDiscipline={setDiscipline} ageGroup={ageGroup} setAgeGroup={setAgeGroup} search={search} setSearch={setSearch} />}
        {page === 'team' && <TeamView items={team} rating={rating} canEdit={user.active_role === 'federation'} onSave={async ids => { try { const selected = ids.map(id => { const athlete = rating.find(a => a.athlete_id === id); return { athlete_id: id, discipline: athlete?.discipline || '', age_group: athlete?.age_group || '' } }); await api('/api/team', { method: 'POST', body: JSON.stringify({ items: selected }) }); setNotice('Состав сборной сохранён.'); await loadData() } catch (error) { setNotice(error instanceof Error ? error.message : 'Не удалось сохранить состав сборной') } }} />}
        {page === 'documents' && <><DocumentsView onMessage={setNotice} competitions={competitions} athletes={athletes} user={user} role={user.active_role} />{(user.active_role === 'federation' || user.active_role === 'judge' || user.active_role === 'ministry') && <JudgeDocs role={user.active_role} judgeId={user.judge_id} competitions={competitions} onMessage={setNotice} />}</>}
        {page === 'norms' && <NormsView role={user.active_role} athletes={athletes} onMessage={setNotice} />}
        {page === 'bonuses' && <BonusesView role={user.active_role} onMessage={setNotice} />}
        {page === 'readiness' && <ReadinessView role={user.active_role} competitions={competitions} onMessage={setNotice} />}
        {page === 'path' && <PathView role={user.active_role} onMessage={setNotice} />}
        {page === 'links' && <LinksView role={user.active_role} onMessage={setNotice} />}
        {page === 'results' && <ResultsView role={user.active_role} competitions={competitions} onMessage={setNotice} />}
        {page === 'judging' && <JudgingView role={user.active_role} user={user} competitions={competitions} onMessage={setNotice} />}
        {page === 'ranks' && <RanksView role={user.active_role} user={user} onMessage={setNotice} />}
        {page === 'appeals' && <AppealsView role={user.active_role} onMessage={setNotice} />}
        {page === 'organizations' && <OrganizationsView role={user.active_role} athletes={athletes} onMessage={setNotice} />}
        {page === 'notifications' && <NotificationsView items={notices} telegramStatus={telegramStatus} telegramLinkStatus={telegramLinkStatus} telegramLinkCode={telegramLinkCode} telegramLinkBusy={telegramLinkBusy} onGenerateCode={() => void generateTelegramLinkCode()} onUnlink={() => void unlinkTelegram()} />}
      </div>
    </main>

    <SiteFooter items={visibleNav} onNavigate={openPage} logo={fspLogo} />

    {modalOpen && user.active_role === 'federation' && <CreateCompetitionModal onClose={() => setModalOpen(false)} onCreated={async () => { setModalOpen(false); setNotice('Соревнование сохранено в базе данных.'); await loadData() }} onError={setNotice} />}
    {notice && <div className="toast" role="status"><CheckCircle2 size={18} /><span>{notice}</span><button onClick={() => setNotice('')} aria-label="Закрыть уведомление"><X size={15} /></button></div>}
  </div>
}

function AuthScreen({ onAuthenticated, preference, effectiveTheme, onToggleTheme }: { onAuthenticated: (token: string, user: AuthUser) => void; preference: ThemePreference; effectiveTheme: 'light' | 'dark'; onToggleTheme: () => void }) {
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [form, setForm] = useState({ email: '', password: '', full_name: '', role: 'athlete' as RoleCode, birth_date: '', municipality: 'Оренбург', organization: '', phone: '', discipline: 'Программирование алгоритмическое', age_group: 'Юноши и девушки 14–17', consent_personal: false, guardian_name: '', guardian_consent: false, email_code: '' })
  const [codeSent, setCodeSent] = useState(false)
  function update<K extends keyof typeof form>(key: K, value: (typeof form)[K]) { setForm(current => ({ ...current, [key]: value })) }
  const athleteAge = form.birth_date ? Math.floor((Date.now() - new Date(form.birth_date).getTime()) / 31557600000) : 99
  async function sendCode() {
    setBusy(true); setError('')
    try { const r = await api<{ delivered: boolean }>('/api/auth/send-code', { method: 'POST', body: JSON.stringify({ email: form.email }) }); setCodeSent(true); if (!r.delivered) setError('Почта не настроена на сервере (пустой SMTP_HOST). Код записан в логи api: docker compose logs api. Задайте SMTP_* в .env рядом с docker-compose.yml и выполните docker compose up -d --force-recreate api.') }
    catch (e) { setError(e instanceof Error ? e.message : 'Не удалось отправить код') }
    finally { setBusy(false) }
  }
  async function submit(event: FormEvent) {
    event.preventDefault(); setBusy(true); setError('')
    try {
      const path = mode === 'login' ? '/api/auth/login' : '/api/auth/register'
      const payload = mode === 'login' ? { email: form.email, password: form.password } : form
      const result = await api<{ token: string; user: AuthUser }>(path, { method: 'POST', body: JSON.stringify(payload) })
      localStorage.setItem('fsp_token', result.token)
      onAuthenticated(result.token, result.user)
    } catch (e) { setError(e instanceof Error ? e.message : 'Не удалось выполнить запрос') }
    finally { setBusy(false) }
  }
  return <div className="auth-screen"><div className="auth-backdrop-grid" /><div className="auth-theme-control"><ThemeControl preference={preference} effectiveTheme={effectiveTheme} onToggle={onToggleTheme} /></div><section className="auth-card"><div className="auth-brand"><img className="fsp-logo fsp-logo-auth" src={effectiveTheme === 'light' ? fspLogoDark : fspLogo} alt="ФСП 56 — Оренбургская область" /></div><div className="auth-kicker"><ShieldCheck size={14} /> ПЛАТФОРМА ФСП 56</div><h1>{mode === 'login' ? 'С возвращением!' : 'Создайте аккаунт'}</h1><p className="auth-intro">{mode === 'login' ? 'Войдите, чтобы подать заявку, посмотреть результаты или управлять соревнованиями.' : 'Это займёт пару минут. Права Федерации назначает администратор отдельно.'}</p><div className="auth-tabs"><button className={mode === 'login' ? 'active' : ''} onClick={() => { setMode('login'); setError('') }}>Вход</button><button className={mode === 'register' ? 'active' : ''} onClick={() => { setMode('register'); setError('') }}>Регистрация</button></div><form onSubmit={submit}>
    {mode === 'register' && <><label className="auth-label">ФИО<input required autoComplete="name" value={form.full_name} onChange={e => update('full_name', e.target.value)} placeholder="Иван Иванов" /></label><label className="auth-label">Роль в системе<select value={form.role} onChange={e => update('role', e.target.value as RoleCode)}><option value="athlete">Спортсмен</option><option value="coach">Тренер</option><option value="judge">Судья</option></select></label></>}
    <label className="auth-label">Email<input type="email" required autoComplete="email" value={form.email} onChange={e => update('email', e.target.value)} placeholder="you@example.ru" /></label>{mode === 'register' && <div className="auth-form-row"><label className="auth-label">Код из письма<input required inputMode="numeric" maxLength={6} value={form.email_code} onChange={e => update('email_code', e.target.value)} placeholder="6 цифр" /></label><button type="button" className="button button-light" style={{ alignSelf: "end", marginBottom: 14 }} disabled={busy || !form.email} onClick={sendCode}>{codeSent ? 'Отправить ещё раз' : 'Получить код'}</button></div>}<label className="auth-label">Пароль<input type="password" required minLength={mode === 'register' ? 8 : undefined} autoComplete={mode === 'login' ? 'current-password' : 'new-password'} value={form.password} onChange={e => update('password', e.target.value)} placeholder={mode === 'login' ? 'Ваш пароль' : 'Минимум 8 символов'} /></label>
    {mode === 'register' && <><div className="auth-form-row"><label className="auth-label">Муниципалитет<input required value={form.municipality} onChange={e => update('municipality', e.target.value)} /></label><label className="auth-label">Организация<input required value={form.organization} onChange={e => update('organization', e.target.value)} placeholder="Школа / вуз / клуб" /></label></div><label className="auth-label">Телефон <span>(необязательно)</span><input type="tel" value={form.phone} onChange={e => update('phone', e.target.value)} placeholder="+7 ..." /></label>{form.role === 'athlete' && <><div className="auth-form-row"><label className="auth-label">Дата рождения<input required type="date" value={form.birth_date} onChange={e => update('birth_date', e.target.value)} /></label><label className="auth-label">Возрастная группа<select value={form.age_group} onChange={e => update('age_group', e.target.value)}><option>Юноши и девушки 14–17</option><option>Юниоры и юниорки 18–20</option><option>Мужчины и женщины 21+</option></select></label></div><label className="auth-label">Дисциплина<select value={form.discipline} onChange={e => update('discipline', e.target.value)}>{['Программирование алгоритмическое', 'Программирование продуктовое', 'Программирование систем информационной безопасности', 'Программирование систем искусственного интеллекта', 'Программирование робототехники', 'Программирование беспилотных авиационных систем'].map(d => <option key={d}>{d}</option>)}</select></label>{athleteAge < 18 && <><label className="auth-label">ФИО законного представителя<input required value={form.guardian_name} onChange={e => update('guardian_name', e.target.value)} placeholder="ФИО родителя / опекуна" /></label><label className="auth-consent"><input type="checkbox" checked={form.guardian_consent} onChange={e => update('guardian_consent', e.target.checked)} required /><span>Законный представитель согласен на обработку данных несовершеннолетнего.</span></label></>}</> }<label className="auth-consent"><input type="checkbox" checked={form.consent_personal} onChange={e => update('consent_personal', e.target.checked)} required /><span>Согласен(на) на обработку персональных данных для работы платформы.</span></label></>}
    {error && <div className="auth-error" role="alert">{error}</div>}<button className="auth-submit" disabled={busy}>{busy ? 'Подождите…' : mode === 'login' ? 'Войти в платформу' : 'Создать аккаунт'}<ArrowRight size={16} /></button></form>{mode === 'login' && <div className="auth-demo"><b>Попробовать без регистрации</b><div className="demo-buttons"><button type="button" className="demo-fill" onClick={() => { update('email', 'federation@fsp56.ru'); update('password', 'demo') }}>Войти как Федерация</button><button type="button" className="demo-fill" onClick={() => { update('email', 'morozov@fsp56.ru'); update('password', 'demo') }}>Войти как спортсмен</button><button type="button" className="demo-fill" onClick={() => { update('email', 'ministry@fsp56.ru'); update('password', 'demo') }}>Войти как Министерство</button></div><small><a href="/api/public/navigator">Навигатор для новичка</a> — публичный раздел без входа</small><small>Кнопки подставляют демо-аккаунт, пароль для всех: <b>demo</b></small></div>}<div className="auth-foot"><ShieldCheck size={13} /> Защищённая сессия · разграничение прав по ролям</div></section><div className="auth-side-note"><div className="auth-shapes" aria-hidden="true"><i /><i /><i /></div><span>ФСП / 56</span><h2>Спортивное<br/>программирование<br/>региона — в одной системе.</h2><p>Подавайте заявки, следите за результатами и растите в рейтинге Оренбургской области.</p><div className="auth-tags"><span>Соревнования</span><span>Заявки</span><span>Рейтинг</span><span>Сборная</span></div></div></div>
}

function Dashboard({ dashboard, competitions, rating, notices, telegramStatus, onNavigate, role }: { dashboard: DashboardData; competitions: Competition[]; rating: RatingEntry[]; notices: Notice[]; telegramStatus: TelegramStatus; onNavigate: (page: PageKey) => void; role: RoleCode }) {
  useEffect(() => {
    const root = document.querySelector('.dashboard-experience')
    if (!root) return
    const targets = Array.from(root.querySelectorAll<HTMLElement>('[data-reveal]'))
    if (!targets.length) return
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches || !('IntersectionObserver' in window)) {
      targets.forEach(target => target.classList.add('is-revealed'))
      return
    }
    root.classList.add('reveal-enabled')
    const observer = new IntersectionObserver(entries => {
      entries.forEach(entry => {
        if (entry.isIntersecting) {
          entry.target.classList.add('is-revealed')
          observer.unobserve(entry.target)
        }
      })
    }, { threshold: 0.12, rootMargin: '0px 0px -7% 0px' })
    targets.forEach(target => observer.observe(target))
    return () => observer.disconnect()
  }, [])

  const stats = [
    { label: 'Спортсменов в реестре', tag: 'Реестр', value: dashboard.stats.athletes, pad: 0, icon: Users, cls: 'stat-indigo', foot: 'учётные записи спортсменов' },
    { label: 'Соревнований в календаре', tag: 'Календарь', value: dashboard.stats.competitions, pad: 2, icon: CalendarDays, cls: 'stat-mint', foot: 'события платформы' },
    { label: 'Заявок от участников', tag: 'Заявки', value: dashboard.stats.applications, pad: 0, icon: ClipboardList, cls: 'stat-orange', foot: 'поступило за всё время' },
    { label: 'Судей в системе', tag: 'Судьи', value: dashboard.stats.judges, pad: 2, icon: ShieldCheck, cls: 'stat-blue', foot: 'учётные записи судей' },
  ]
  const heroLabel = role === 'federation' ? 'ФСП · ОРЕНБУРГСКАЯ ОБЛАСТЬ' : role === 'coach' ? 'ЛИЧНЫЙ КАБИНЕТ ТРЕНЕРА' : role === 'athlete' ? 'ЛИЧНЫЙ КАБИНЕТ СПОРТСМЕНА' : 'СУДЕЙСКАЯ КОЛЛЕГИЯ'
  const topPoints = rating[0]?.points || 1
  const delay = (i: number) => ({ '--i': i }) as CSSProperties
  return <div className="dashboard-experience">
    <section className="hero-banner">
      <div className="hero-grid" aria-hidden="true" />
      <div className="hero-copy">
        <div className="hero-label"><span className="hero-label-mark" /> {heroLabel}</div>
        <h2>Спорт начинается<br /><span>с возможностей</span></h2>
        <p>Соревнования, люди и результаты регионального спортивного программирования — в одном понятном месте. Выбирайте старт, подавайте заявку и следите за своим местом в рейтинге.</p>
        <div className="hero-buttons">
          <button onClick={() => onNavigate('competitions')} className="hero-cta">Открыть календарь <ArrowRight size={17} /></button>
          {role !== 'judge' && <button onClick={() => onNavigate('rating')} className="hero-cta hero-cta-ghost">Смотреть рейтинг</button>}
        </div>
      </div>
      <div className="hero-visual" aria-hidden="true">
        <span className="hero-number">56</span>
        <span className="hero-block block-red" />
        <span className="hero-block block-white" />
        <div className="hero-terminal">
          <div className="hero-terminal-dots"><i /><i /><i /></div>
          <div><em>$</em> сезон <b>2026/27</b></div>
          <div><em>&gt;</em> спортсменов: <b>{dashboard.stats.athletes}</b></div>
          <div><em>&gt;</em> заявок: <b>{dashboard.stats.applications}</b></div>
        </div>
      </div>
    </section>

    <div className="marquee" aria-label="Дисциплины спортивного программирования">
      <div className="marquee-track" aria-hidden="true">{[...allDisciplines, ...allDisciplines, ...allDisciplines, ...allDisciplines].map((d, i) => <span className="marquee-item" key={i}>{d}</span>)}</div>
    </div>

    <div className="section-heading" data-reveal="heading"><div><h2>Платформа в цифрах</h2><p>Актуальные данные региона</p></div><button className="text-button" onClick={() => onNavigate('participants')}>Открыть реестр <ArrowRight size={15} /></button></div>
    <section className="stats-grid">
      {stats.map((s, i) => <div className="stat-card" data-reveal="stat" style={delay(i)} key={s.label}>
        <div className="stat-card-top"><div className={`stat-icon ${s.cls}`}><s.icon size={21} /></div><span className="stat-tag">{s.tag}</span></div>
        <div className="stat-meta"><b><CountUp value={s.value} pad={s.pad} /></b><span>{s.label}</span><small>{s.foot}</small></div>
      </div>)}
    </section>

    <section className="content-grid main-dashboard-grid">
      <div className="panel competition-panel" data-reveal="calendar"><div className="panel-heading"><div><div className="panel-kicker">Календарь</div><h3>Ближайшие соревнования</h3></div><button className="panel-action" onClick={() => onNavigate('competitions')}>Все события <ArrowRight size={14} /></button></div>
        <div className="event-list">{competitions.slice(0, 3).map(c => <div className="event-row" key={c.id}><div className="event-date"><b>{new Date(c.starts_at).toLocaleDateString('ru-RU', { day: '2-digit' })}</b><span>{new Date(c.starts_at).toLocaleDateString('ru-RU', { month: 'short' }).replace('.', '')}</span></div><div className="event-copy"><b>{c.title}</b><span><Code2 size={13} /> {c.discipline} <i /> {c.location}</span></div><div className={`status-pill ${c.status === 'Регистрация' ? 'status-open' : c.status === 'Завершено' ? 'status-done' : 'status-planned'}`}>{c.status}</div><button className="row-arrow" aria-label={`Открыть ${c.title}`} onClick={() => onNavigate('competitions')}><ChevronRight size={18} /></button></div>)}</div>
        <button className="panel-bottom-link" onClick={() => onNavigate('competitions')}><CalendarDays size={16} /> Перейти в календарь <ArrowRight size={15} /></button>
      </div>
      <div className="panel rating-panel" data-reveal="rating" style={delay(1)}><div className="panel-heading"><div><div className="panel-kicker">Лидеры</div><h3>Топ рейтинга региона</h3></div><button className="round-action" onClick={() => onNavigate('rating')} aria-label="Открыть рейтинг"><ArrowUpRight size={18} /></button></div><div className="rating-list">{rating.slice(0, 5).map((a, i) => <div className="rating-row" key={a.athlete}><div className={`rank-num ${i === 0 ? 'rank-gold' : i === 1 ? 'rank-silver' : i === 2 ? 'rank-bronze' : ''}`}>{String(i + 1).padStart(2, '0')}</div><div className={`athlete-avatar avatar-${i % 4}`}>{initialsOf(a.athlete)}</div><div className="rating-athlete"><b>{a.athlete}</b><span>{a.organization}</span></div><div className="rating-points"><b>{a.points}</b><span>очков</span></div><i className="rating-bar" style={{ '--w': `${Math.round(a.points / topPoints * 100)}%` } as CSSProperties} /></div>)}</div><button className="panel-bottom-link" onClick={() => onNavigate('rating')}><Trophy size={16} /> Полный рейтинг <ArrowRight size={15} /></button></div>
    </section>

    <section className="content-grid lower-grid"><div className="panel workflow-panel" data-reveal="workflow"><div className="panel-heading"><div><div className="panel-kicker">Требуют внимания</div><h3>Рабочие задачи</h3></div><span className="count-badge">03</span></div><div className="workflow-item"><div className="workflow-icon workflow-yellow"><ClipboardList size={18} /></div><div><b>Модерация заявок</b><span>12 заявок ждут решения Федерации</span></div><button onClick={() => onNavigate('competitions')} aria-label="Проверить заявки"><ArrowRight size={16} /></button></div><div className="workflow-item"><div className="workflow-icon workflow-red"><ShieldCheck size={18} /></div><div><b>Срок действия категорий</b><span>3 судейские категории требуют проверки</span></div><button onClick={() => onNavigate('participants')} aria-label="Проверить категории"><ArrowRight size={16} /></button></div><div className="workflow-item"><div className="workflow-icon workflow-green"><FileBadge size={18} /></div><div><b>Протокол к публикации</b><span>Один документ готов к согласованию</span></div><button onClick={() => onNavigate('documents')} aria-label="Открыть документы"><ArrowRight size={16} /></button></div></div>
      <div className="panel notices-panel" data-reveal="notices" style={delay(1)}><div className="panel-heading"><div><div className="panel-kicker">Система</div><h3>Последние уведомления</h3></div><button className="panel-action" onClick={() => onNavigate('notifications')}>Все <ArrowRight size={14} /></button></div><div className="notice-list">{notices.slice(0, 3).map(n => <div className="notice-row" key={n.id}><div className={`notice-mark notice-${n.kind}`}>{n.kind === 'warning' ? <Activity size={16} /> : n.kind === 'success' ? <CheckCircle2 size={16} /> : <Bell size={16} />}</div><div><b>{n.title}</b><p>{n.message}</p><small>{n.when}</small></div></div>)}</div></div></section>
    <section className="telegram-integration-card" data-reveal="integration"><div className="telegram-brand-block"><span className="telegram-logo"><Send size={22} /></span><div><span className="panel-kicker">Telegram</span><h3>Новости прямо в мессенджере</h3><p>Решения по заявкам, обновления соревнований и результаты — бот пришлёт всё сам, заходить на сайт не придётся.</p></div></div><div className="telegram-status-block"><span className={`telegram-connection ${telegramStatus.configured && telegramStatus.polling_enabled ? 'telegram-connected' : ''}`}><span />{telegramStatus.configured && telegramStatus.polling_enabled ? 'Бот на связи' : telegramStatus.configured ? 'Нет связи — проверьте логи' : 'Ждёт подключения'}</span><div className="telegram-meta"><span><CheckCircle2 size={14} /> Заявки и решения</span><span><CheckCircle2 size={14} /> Результаты и рейтинг</span></div><button className="button button-light" onClick={() => onNavigate('notifications')}>Подключить бота <ArrowRight size={15} /></button></div></section>
  </div>
}

function PageToolbar({ search, setSearch, placeholder = 'Поиск по реестру...' }: { search: string; setSearch: (value: string) => void; placeholder?: string }) {
  return <div className="table-toolbar"><label className="search-box"><Search size={17} /><input id="global-search" value={search} onChange={e => setSearch(e.target.value)} placeholder={placeholder} /></label><button className="button button-light" onClick={() => setSearch('')}><Filter size={15} /> Сбросить</button></div>
}

function CompetitionsView({ items, search, setSearch, onCreate, canCreate, role, onApply, onPublish }: { items: Competition[]; search: string; setSearch: (value: string) => void; onCreate: () => void; canCreate: boolean; role: RoleCode; onApply: (id: number) => void; onPublish: (id: number) => void }) {
  return <div className="panel table-panel"><div className="table-header-copy"><div><h3>Календарь соревнований</h3><p>События, публикация и приём заявок</p></div><span className="total-chip">{items.length} событий</span></div><PageToolbar search={search} setSearch={setSearch} placeholder="Название, дисциплина или место..." /><div className="table-scroll"><table><thead><tr><th>Соревнование</th><th>Дата</th><th>Дисциплина</th><th>Участники</th><th>Статус</th><th>Действие</th></tr></thead><tbody>{items.map(c => <tr key={c.id}><td><div className="table-primary"><span className="table-icon"><CalendarDays size={16} /></span><div><b>{c.title}</b><small>{c.competition_type} · {c.age_group}</small></div></div></td><td><span className="date-cell">{formatFullDate(c.starts_at)}</span><small className="cell-sub">{c.location}</small></td><td>{c.discipline}</td><td><span className="participants-number">{c.participants || '—'}</span></td><td><span className={`status-pill ${c.status === 'Регистрация' ? 'status-open' : c.status === 'Завершено' ? 'status-done' : c.status === 'Черновик' ? 'status-draft' : 'status-planned'}`}>{c.status}</span></td><td>{role === 'athlete' && c.status === 'Регистрация' ? <button className="action-small" onClick={() => onApply(c.id)}>Подать заявку</button> : canCreate && (c.status === 'Черновик' || c.status === 'Планируется') ? <button className="action-small" onClick={() => onPublish(c.id)}>Опубликовать</button> : <span className="cell-sub">—</span>}</td></tr>)}</tbody></table>{items.length === 0 && <EmptyState label="Ничего не найдено" detail="Попробуйте изменить запрос." />}</div><div className="table-footer"><span>Показано {items.length} событий</span>{canCreate && <button className="button button-primary" onClick={onCreate}><Plus size={16} /> Создать соревнование</button>}</div></div>
}

function ApplicationsPanel({ items, onDecide }: { items: ApplicationItem[]; onDecide: (id: number, status: 'approved' | 'rejected') => void }) {
  const pending = items.filter(a => a.status === 'pending')
  return <div className="panel table-panel applications-panel"><div className="table-header-copy"><div><h3>Заявки на рассмотрении</h3><p>Федерация проверяет соответствие участников условиям соревнования</p></div><span className="total-chip">{pending.length} ожидают</span></div><div className="table-scroll"><table><thead><tr><th>Спортсмен</th><th>Соревнование</th><th>Дисциплина</th><th>Возрастная группа</th><th>Статус</th><th>Решение</th></tr></thead><tbody>{items.map(a => <tr key={a.id}><td><b>{a.athlete}</b></td><td>{a.competition}</td><td>{a.discipline}</td><td>{a.age_group}</td><td><span className={`status-pill ${a.status === 'approved' ? 'status-open' : a.status === 'rejected' ? 'status-done' : 'status-planned'}`}>{a.status === 'approved' ? 'Одобрена' : a.status === 'rejected' ? 'Отклонена' : 'На проверке'}</span></td><td>{a.status === 'pending' ? <div className="application-actions"><button className="action-small action-approve" onClick={() => onDecide(a.id, 'approved')}>Одобрить</button><button className="action-small action-reject" onClick={() => onDecide(a.id, 'rejected')}>Отклонить</button></div> : <span className="cell-sub">Решение принято</span>}</td></tr>)}</tbody></table>{items.length === 0 && <EmptyState label="Заявок пока нет" detail="Когда спортсмены подадут заявки, они появятся здесь." />}</div></div>
}

function AthletesView({ items, search, setSearch, canApprove, onApprove }: { items: Athlete[]; search: string; setSearch: (value: string) => void; canApprove: boolean; onApprove: (id: number) => void }) {
  return <div className="panel table-panel"><div className="table-header-copy"><div><h3>Реестр участников</h3><p>Спортсмены, организации и подтверждение профилей</p></div><span className="total-chip">{items.length} записей</span></div><PageToolbar search={search} setSearch={setSearch} placeholder="ФИО, организация, тренер..." /><div className="table-scroll"><table><thead><tr><th>Спортсмен</th><th>Организация</th><th>Дисциплина</th><th>Возрастная группа</th><th>Очки</th><th>Статус</th>{canApprove && <th>Профиль</th>}</tr></thead><tbody>{items.map((a, i) => <tr key={a.id}><td><div className="table-primary"><span className={`athlete-avatar avatar-${i % 4}`}>{a.full_name.split(' ').map(x => x[0]).slice(0, 2).join('')}</span><div><b>{a.full_name}</b><small>Тренер: {a.coach_name || 'не указан'}</small></div></div></td><td>{a.organization}</td><td>{a.discipline}</td><td>{a.age_group}</td><td><b>{a.points}</b></td><td><span className={`status-pill ${a.status === 'Активен' ? 'status-open' : 'status-planned'}`}>{a.status}</span></td>{canApprove && <td>{a.status === 'На проверке' ? <button className="action-small action-approve" onClick={() => onApprove(a.id)}>Подтвердить</button> : <span className="cell-sub">—</span>}</td>}</tr>)}</tbody></table>{items.length === 0 && <EmptyState label="Участники не найдены" detail="Попробуйте другой поисковый запрос." />}</div><div className="table-footer"><span>Профили новых спортсменов подтверждает Федерация</span><span className="table-foot-note"><ShieldCheck size={14} /> Реестр ФСП</span></div></div>
}

function RatingView({ items, discipline, setDiscipline, ageGroup, setAgeGroup, search, setSearch }: { items: RatingEntry[]; discipline: string; setDiscipline: (value: string) => void; ageGroup: string; setAgeGroup: (value: string) => void; search: string; setSearch: (value: string) => void }) {
  const disciplines = ['Все дисциплины', 'Программирование алгоритмическое', 'Программирование продуктовое', 'Программирование систем информационной безопасности', 'Программирование систем искусственного интеллекта', 'Программирование робототехники', 'Программирование беспилотных авиационных систем']
  const ageGroups = ['Все группы', 'Юноши и девушки 14–17', 'Юниоры и юниорки 18–20', 'Мужчины и женщины 21+']
  return <><section className="rating-overview-grid"><div className="rating-highlight"><div className="highlight-top"><span>РЕГИОНАЛЬНЫЙ РЕЙТИНГ</span><Trophy size={20} /></div><b>{items.length.toLocaleString('ru-RU')}</b><p>спортсменов в выбранной выборке</p><div className="highlight-rule"><span /><span /><span /><span /><span /><span /><span /><span /><span /></div><small>Сортировка по сумме набранных очков</small></div><div className="rating-note"><div className="rating-note-icon"><Activity size={19} /></div><div><b>Прозрачный расчёт</b><p>Рейтинг рассчитывается по результатам соревнований с учётом уровня и места. Перед официальным использованием необходимо сверить регламент начисления с Федерацией.</p></div></div></section><div className="panel table-panel rating-table-panel"><div className="table-header-copy"><div><h3>Таблица рейтинга</h3><p>Фильтры по дисциплине и возрастной группе</p></div><div className="rating-filters"><label className="select-box"><span>Дисциплина</span><select value={discipline} onChange={e => setDiscipline(e.target.value)}>{disciplines.map(d => <option key={d}>{d}</option>)}</select><ChevronDown size={14} /></label><label className="select-box"><span>Возраст</span><select value={ageGroup} onChange={e => setAgeGroup(e.target.value)}>{ageGroups.map(g => <option key={g}>{g}</option>)}</select><ChevronDown size={14} /></label></div></div><PageToolbar search={search} setSearch={setSearch} placeholder="Поиск спортсмена..." /><div className="table-scroll"><table><thead><tr><th>Место</th><th>Спортсмен</th><th>Дисциплина</th><th>Группа</th><th>Очки</th><th /></tr></thead><tbody>{items.map(a => <tr key={a.athlete}><td><span className={`rank-num table-rank ${a.rank <= 3 ? `rank-${a.rank === 1 ? 'gold' : a.rank === 2 ? 'silver' : 'bronze'}` : ''}`}>{String(a.rank).padStart(2, '0')}</span></td><td><div className="table-primary"><div className="rating-athlete"><b>{a.athlete}</b><small>{a.organization}</small></div></div></td><td>{a.discipline}</td><td>{a.age_group}</td><td><b>{a.points}</b></td><td><button className="row-arrow" aria-label="Карточка спортсмена"><ChevronRight size={17} /></button></td></tr>)}</tbody></table>{items.length === 0 && <EmptyState label="Нет результатов" detail="В этой выборке пока нет спортсменов." />}</div></div></>
}

function TeamView({ items, rating, canEdit, onSave }: { items: RatingEntry[]; rating: RatingEntry[]; canEdit: boolean; onSave: (ids: number[]) => void }) {
  const [editing, setEditing] = useState(false)
  const [selected, setSelected] = useState<number[]>([])
  const [teamAgeGroup, setTeamAgeGroup] = useState('Все группы')
  const [teamDiscipline, setTeamDiscipline] = useState('Все дисциплины')
  const [minimumRank, setMinimumRank] = useState('Без ограничения')
  const teamDisciplines = ['Все дисциплины', 'Программирование алгоритмическое', 'Программирование продуктовое', 'Программирование систем информационной безопасности', 'Программирование систем искусственного интеллекта', 'Программирование робототехники', 'Программирование беспилотных авиационных систем']
  const teamAgeGroups = ['Все группы', 'Юноши и девушки 14–17', 'Юниоры и юниорки 18–20', 'Мужчины и женщины 21+']
  const rankOrder: Record<string, number> = { 'Без разряда': 0, '3 разряд': 1, '2 разряд': 2, '1 разряд': 3, 'КМС': 4, 'МС': 5, 'МСМК': 6 }
  const minimumRankValue = rankOrder[minimumRank] ?? 0
  const candidateRows = rating.filter(a => (teamAgeGroup === 'Все группы' || a.age_group === teamAgeGroup) && (teamDiscipline === 'Все дисциплины' || a.discipline === teamDiscipline) && (minimumRankValue === 0 || (rankOrder[a.rank_title || 'Без разряда'] ?? 0) >= minimumRankValue))
  useEffect(() => { setSelected(items.map(a => a.athlete_id).filter((id): id is number => typeof id === 'number')) }, [items])
  function toggle(id: number) { setSelected(current => current.includes(id) ? current.filter(value => value !== id) : [...current, id]) }
  return <><div className="team-banner"><div className="team-banner-mark"><Flag size={28} /></div><div><div className="panel-kicker">СОСТАВ СБОРНОЙ · 2026</div><h2>Сборная Оренбургской области</h2><p>Состав хранится в платформе; Федерация может выбрать спортсменов из рейтинга.</p></div><span className="team-count">{items.length} <small>участников</small></span></div>
    {canEdit && <div className="team-editor panel"><div><h3>Формирование состава</h3><p>Выберите спортсменов из рейтинга. Сохранённый состав используется при выгрузке проекта приказа.</p></div><button className="button button-primary" onClick={() => setEditing(value => !value)}>{editing ? 'Скрыть выбор' : 'Изменить состав'}</button>{editing && <><div className="team-select-filters"><label className="select-box"><span>Дисциплина</span><select value={teamDiscipline} onChange={e => setTeamDiscipline(e.target.value)}>{teamDisciplines.map(value => <option key={value}>{value}</option>)}</select><ChevronDown size={14} /></label><label className="select-box"><span>Возраст</span><select value={teamAgeGroup} onChange={e => setTeamAgeGroup(e.target.value)}>{teamAgeGroups.map(value => <option key={value}>{value}</option>)}</select><ChevronDown size={14} /></label><label className="select-box"><span>Минимальный разряд</span><select value={minimumRank} onChange={e => setMinimumRank(e.target.value)}>{['Без ограничения', '3 разряд', '2 разряд', '1 разряд', 'КМС', 'МС', 'МСМК'].map(value => <option key={value}>{value}</option>)}</select><ChevronDown size={14} /></label></div><div className="team-select-list">{candidateRows.map(a => <label key={a.athlete_id ?? a.athlete} className="team-select-row"><input type="checkbox" disabled={typeof a.athlete_id !== 'number'} checked={typeof a.athlete_id === 'number' && selected.includes(a.athlete_id)} onChange={() => { if (typeof a.athlete_id === 'number') toggle(a.athlete_id) }} /><span><b>{a.athlete}</b><small>{a.discipline} · {a.age_group} · {a.rank_title || 'Без разряда'}</small></span><strong>{a.points} очк.</strong></label>)}</div><div className="team-editor-actions"><span>Выбрано: {selected.length}</span><button className="button button-primary" onClick={() => onSave(selected)} disabled={selected.length === 0}>Сохранить состав</button></div></>}</div>}
    {items.length > 0 ? <div className="team-grid">{items.map((a, i) => <div className="team-card" key={a.athlete_id ?? a.athlete}><div className="team-card-top"><div className={`athlete-avatar avatar-${i % 4}`}>{a.athlete.split(' ').map(x => x[0]).slice(0, 2).join('')}</div><span className={`rank-num ${i === 0 ? 'rank-gold' : i === 1 ? 'rank-silver' : i === 2 ? 'rank-bronze' : ''}`}>#{i + 1}</span></div><h3>{a.athlete}</h3><p>{a.organization}</p><div className="team-card-discipline"><Code2 size={13} /> {a.discipline}</div><div className="team-score"><span>Рейтинг</span><b>{a.points} <small>очков</small></b></div><div className="team-progress"><span style={{ width: `${Math.max(14, Math.round(a.points / (items[0]?.points || 968) * 100))}%` }} /></div></div>)}</div> : <div className="panel team-empty"><EmptyState label="Состав сборной пока не сохранён" detail={canEdit ? 'Откройте «Изменить состав», выберите спортсменов из рейтинга и сохраните.' : 'Федерация формирует и утверждает состав сборной.'} /></div>}
    <div className="prototype-note"><CircleHelp size={17} /><span>Список и проект приказа формируются из сохранённого состава. Окончательные ограничения по дисциплинам, возрасту и разрядам необходимо сверить с регламентом Федерации.</span></div></>
}

function DocumentsView({ onMessage, competitions, athletes, user, role }: { onMessage: (message: string) => void; competitions: Competition[]; athletes: Athlete[]; user: AuthUser; role: RoleCode }) {
  const [competitionId, setCompetitionId] = useState<number | ''>('')
  const [athleteId, setAthleteId] = useState<number | ''>('')
  const canExportRegistry = role === 'federation' || role === 'ministry'
  const reportAthleteId = role === 'athlete' ? user.athlete_id : (athleteId || undefined)
  const selectableAthletes = role === 'athlete' ? [{ id: user.athlete_id || 0, full_name: user.full_name }] : athletes
  async function download(path: string, filename: string) {
    try {
      const token = localStorage.getItem('fsp_token')
      const response = await fetch(path, { headers: token ? { Authorization: `Bearer ${token}` } : {} })
      if (!response.ok) {
        const payload = await response.json().catch(() => ({}))
        throw new Error(payload.error || `Ошибка выгрузки: ${response.status}`)
      }
      const blob = await response.blob()
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url; link.download = filename; document.body.appendChild(link); link.click(); link.remove()
      window.setTimeout(() => URL.revokeObjectURL(url), 1000)
      onMessage(`Файл «${filename}» сформирован.`)
    } catch (error) { onMessage(error instanceof Error ? error.message : 'Не удалось сформировать документ') }
  }
  return <><div className="doc-intro"><div className="doc-intro-icon"><FileText size={24} /></div><div><h3>Библиотека документов</h3><p>Файлы формируются из данных платформы; доступные выгрузки зависят от роли.</p></div></div><div className="document-grid">
    <div className="document-card"><div className="document-card-head"><div className="document-icon doc-purple"><FileBadge size={20} /></div><span className="file-type">DOCX / XLSX</span></div><h3>Протокол соревнования</h3><p>Итоговые места и результаты участников выбранного соревнования.</p><label className="document-select-label">Соревнование<select value={competitionId} onChange={e => setCompetitionId(e.target.value ? Number(e.target.value) : '')}><option value="">Выберите соревнование</option>{competitions.map(c => <option key={c.id} value={c.id}>{c.title}</option>)}</select></label><div className="document-card-foot"><span className="doc-status">Из данных платформы</span><div className="document-actions"><button disabled={!competitionId} onClick={() => void download(`/api/competitions/${competitionId}/protocol?format=docx`, `protocol-${competitionId}.docx`)} title="Скачать DOCX">DOCX</button><button disabled={!competitionId} onClick={() => void download(`/api/competitions/${competitionId}/protocol`, `protocol-${competitionId}.xlsx`)} title="Скачать XLSX">XLSX</button></div></div></div>
    <div className="document-card"><div className="document-card-head"><div className="document-icon doc-blue"><UserRound size={20} /></div><span className="file-type">DOCX</span></div><h3>Отчёт по спортсмену</h3><p>Организация, дисциплина, разряд и история выступлений.</p>{role !== 'athlete' && <label className="document-select-label">Спортсмен<select value={athleteId} onChange={e => setAthleteId(e.target.value ? Number(e.target.value) : '')}><option value="">{athletes.length ? 'Выберите спортсмена' : 'Реестр не загружен'}</option>{selectableAthletes.map(a => <option key={a.id} value={a.id}>{a.full_name}</option>)}</select></label>}<div className="document-card-foot"><span className="doc-status">Из данных платформы</span><button disabled={!reportAthleteId} onClick={() => void download(`/api/documents/athletes/${reportAthleteId}`, `athlete-${reportAthleteId}.docx`)} aria-label="Скачать отчёт по спортсмену"><ArrowDownRight size={17} /></button></div></div>
    <div className="document-card"><div className="document-card-head"><div className="document-icon doc-green"><Users size={20} /></div><span className="file-type">XLSX</span></div><h3>Реестр участников</h3><p>ФИО, ФСП ID, организация, муниципалитет, дисциплина, группа и разряд.</p><div className="document-card-foot"><span className="doc-status">{canExportRegistry ? 'Доступно Федерации' : 'Только Федерация'}</span><button disabled={!canExportRegistry} onClick={() => void download('/api/documents/registry.xlsx', 'registry.xlsx')} aria-label="Скачать реестр участников"><ArrowDownRight size={17} /></button></div></div>
    <div className="document-card"><div className="document-card-head"><div className="document-icon doc-orange"><Activity size={20} /></div><span className="file-type">XLSX</span></div><h3>Отчёт о численности</h3><p>Динамика по годам, муниципалитетам, организациям, дисциплинам и возрастным группам.</p><div className="document-card-foot"><span className="doc-status">{canExportRegistry ? 'Доступно Федерации' : 'Только Федерация'}</span><button disabled={!canExportRegistry} onClick={() => void download('/api/documents/enrollment?format=xlsx', 'enrollment.xlsx')} aria-label="Скачать отчёт о численности"><ArrowDownRight size={17} /></button></div></div>
    <div className="document-card"><div className="document-card-head"><div className="document-icon doc-purple"><FileText size={20} /></div><span className="file-type">DOCX</span></div><h3>Проект приказа о сборной</h3><p>Проект приказа на основе сохранённого состава сборной региона.</p><div className="document-card-foot"><span className="doc-status">{canExportRegistry ? 'Доступно Федерации' : 'Только Федерация'}</span><button disabled={!canExportRegistry} onClick={() => void download('/api/team/order', 'team-order-2026.docx')} aria-label="Скачать проект приказа"><ArrowDownRight size={17} /></button></div></div>
  </div><div className="prototype-note"><CircleHelp size={17} /><span>Экспорт подключён к API. Проверьте, что у выбранного соревнования уже есть результаты, а у спортсмена — история выступлений: пустые документы будут содержать только доступные записи.</span></div></>
}

function NotificationsView({ items, telegramStatus, telegramLinkStatus, telegramLinkCode, telegramLinkBusy, onGenerateCode, onUnlink }: { items: Notice[]; telegramStatus: TelegramStatus; telegramLinkStatus: TelegramLinkStatus; telegramLinkCode: string; telegramLinkBusy: boolean; onGenerateCode: () => void; onUnlink: () => void }) {
  const telegramReady = telegramStatus.configured && telegramStatus.polling_enabled
  return <div className="notifications-page-stack">
    <section className="panel telegram-link-panel">
      <div className="telegram-link-mark"><Send size={20} /></div>
      <div className="telegram-link-copy"><div className="panel-kicker">ВНЕШНИЙ КАНАЛ УВЕДОМЛЕНИЙ</div><h3>Подключение Telegram</h3>
        <p>{telegramLinkStatus.linked ? 'Аккаунт Telegram связан с вашей учётной записью ФСП 56. Персональные уведомления о решениях по заявкам, разрядам, категориям и публикации результатов будут приходить в мессенджер.' : 'Свяжите свой аккаунт Telegram с платформой, чтобы получать персональные сообщения о решениях по заявкам, разрядам и судейским категориям, а также о публикации результатов.'}</p>
        {!telegramStatus.configured && <div className="prototype-note"><CircleHelp size={16} /><span>Telegram-бот пока не настроен. Добавьте TELEGRAM_BOT_TOKEN в .env и перезапустите API; публичный URL для локальной проверки не нужен.</span></div>}
        {telegramStatus.configured && !telegramStatus.polling_enabled && <div className="prototype-note"><CircleHelp size={16} /><span>Токен задан, но long polling не запущен. Проверьте журналы API и правильность токена.</span></div>}
        {!telegramLinkStatus.linked && telegramLinkCode && <div className="telegram-pairing-code"><span>ОДНОРАЗОВЫЙ КОД · 10 МИНУТ</span><b>{telegramLinkCode}</b><code>/link {telegramLinkCode}</code><small>Откройте бота Telegram, нажмите «Запустить» и отправьте ему команду выше. Код можно использовать один раз.</small></div>}
      </div>
      <div className="telegram-link-actions">{telegramLinkStatus.linked ? <><span className="telegram-linked-state"><CheckCircle2 size={14} /> Аккаунт подключён</span><button className="button button-light" disabled={telegramLinkBusy} onClick={onUnlink}>Отключить</button></> : <button className="button button-primary" disabled={telegramLinkBusy || !telegramReady} onClick={onGenerateCode} title={!telegramReady ? 'Сначала настройте Telegram на сервере' : undefined}>{telegramLinkBusy ? 'Создаём код…' : telegramLinkCode ? 'Создать новый код' : 'Получить код подключения'}</button>}</div>
    </section>
    <div className="panel notifications-full"><div className="table-header-copy"><div><h3>Входящие уведомления</h3><p>Системные задачи и напоминания для Федерации</p></div><span className="total-chip">{items.length} сообщения</span></div><div className="notifications-full-list">{items.map(n => <div className="notifications-full-row" key={n.id}><div className={`notice-mark notice-${n.kind}`}>{n.kind === 'warning' ? <Activity size={17} /> : n.kind === 'success' ? <CheckCircle2 size={17} /> : <Bell size={17} />}</div><div className="notifications-full-copy"><div><b>{n.title}</b><span>{n.when}</span></div><p>{n.message}</p></div><span className={`notice-label label-${n.kind}`}>{n.kind === 'warning' ? 'Внимание' : n.kind === 'success' ? 'Готово' : 'Информация'}</span></div>)}</div></div>
  </div>
}

function CreateCompetitionModal({ onClose, onCreated, onError }: { onClose: () => void; onCreated: () => Promise<void>; onError: (message: string) => void }) {
  const [saving, setSaving] = useState(false)
  const [form, setForm] = useState({
    title: '', discipline: 'Программирование алгоритмическое', competition_type: 'Региональные соревнования',
    sport_status: 'региональное', event_format: 'очно', age_group: 'Все группы',
    starts_at: '2026-11-21', applications_until: '2026-11-14', location: 'Оренбург',
    calendar_plan: 'Календарный план Оренбургской области', calendar_number: '', regulation: '',
  })
  const [catalogs, setCatalogs] = useState({
    disciplines: ['Программирование алгоритмическое', 'Программирование продуктовое', 'Программирование систем информационной безопасности', 'Программирование систем искусственного интеллекта', 'Программирование робототехники', 'Программирование беспилотных авиационных систем'],
    age_groups: ['Юноши и девушки 14–17', 'Юниоры и юниорки 18–20', 'Мужчины и женщины 21+', 'Все группы'],
    types: ['Чемпионат', 'Первенство', 'Кубок', 'Чемпионат России', 'Первенство России', 'Всероссийские соревнования', 'Межрегиональные соревнования', 'Региональные соревнования'],
    statuses: ['региональное', 'межрегиональное', 'всероссийское'], formats: ['очно', 'онлайн', 'смешанный'],
  })
  useEffect(() => {
    api<{ disciplines: string[]; age_groups: string[]; types: string[]; statuses: string[]; formats: string[] }>('/api/public/catalogs').then(result => {
      setCatalogs(current => ({ disciplines: result.disciplines?.length ? result.disciplines : current.disciplines, age_groups: result.age_groups?.length ? result.age_groups : current.age_groups, types: result.types?.length ? result.types : current.types, statuses: result.statuses?.length ? result.statuses : current.statuses, formats: result.formats?.length ? result.formats : current.formats }))
    }).catch(() => {})
  }, [])
  async function submit(e: FormEvent) {
    e.preventDefault(); setSaving(true)
    try {
      const payload = { ...form, starts_at: `${form.starts_at}T10:00:00+05:00`, applications_until: form.applications_until ? `${form.applications_until}T23:59:59+05:00` : '' }
      await api<Competition>('/api/competitions', { method: 'POST', body: JSON.stringify(payload) })
      await onCreated()
    } catch (error) { onError(error instanceof Error ? error.message : 'Не удалось создать соревнование') }
    finally { setSaving(false) }
  }
  function update(key: keyof typeof form, value: string) { setForm(current => ({ ...current, [key]: value })) }
  return <div className="modal-backdrop" onMouseDown={e => { if (e.target === e.currentTarget) onClose() }}><section className="modal-card competition-modal"><div className="modal-header"><div><div className="panel-kicker">НОВОЕ СОБЫТИЕ</div><h2>Создать соревнование</h2><p>Заполните обязательные поля календарного плана и положения.</p></div><button className="icon-button" onClick={onClose} aria-label="Закрыть окно"><X size={20} /></button></div><form onSubmit={submit}>
    <label className="field-label">Название соревнования<input autoFocus required value={form.title} onChange={e => update('title', e.target.value)} placeholder="Например, Кубок Оренбуржья" /></label>
    <div className="form-row"><label className="field-label">Дисциплина<select value={form.discipline} onChange={e => update('discipline', e.target.value)}>{catalogs.disciplines.map(x => <option key={x}>{x}</option>)}</select></label><label className="field-label">Тип соревнования<select value={form.competition_type} onChange={e => update('competition_type', e.target.value)}>{catalogs.types.map(x => <option key={x}>{x}</option>)}</select></label></div>
    <div className="form-row"><label className="field-label">Уровень соревнования<select value={form.sport_status} onChange={e => update('sport_status', e.target.value)}>{catalogs.statuses.map(x => <option key={x} value={x}>{x[0].toUpperCase() + x.slice(1)}</option>)}</select></label><label className="field-label">Формат проведения<select value={form.event_format} onChange={e => update('event_format', e.target.value)}>{catalogs.formats.map(x => <option key={x} value={x}>{x[0].toUpperCase() + x.slice(1)}</option>)}</select></label></div>
    <div className="form-row"><label className="field-label">Возрастная группа<select value={form.age_group} onChange={e => update('age_group', e.target.value)}>{catalogs.age_groups.map(x => <option key={x}>{x}</option>)}</select></label><label className="field-label">Дата проведения<input required type="date" value={form.starts_at} onChange={e => update('starts_at', e.target.value)} /></label></div>
    <div className="form-row"><label className="field-label">Окончание приёма заявок<input type="date" value={form.applications_until} onChange={e => update('applications_until', e.target.value)} /></label><label className="field-label">Место проведения<input required value={form.location} onChange={e => update('location', e.target.value)} placeholder="Оренбург / Онлайн" /></label></div>
    <div className="form-row"><label className="field-label">Календарный план<input required value={form.calendar_plan} onChange={e => update('calendar_plan', e.target.value)} placeholder="Календарный план Оренбургской области" /></label><label className="field-label">Номер в календарном плане <span>(если присвоен)</span><input value={form.calendar_number} onChange={e => update('calendar_number', e.target.value)} placeholder="Например, 24-ОБЛ" /></label></div>
    <label className="field-label">Положение соревнования<input required value={form.regulation} onChange={e => update('regulation', e.target.value)} placeholder="Ссылка или название документа положения" /></label>
    <div className="modal-note"><ShieldCheck size={16} /><span>Сохраняются уровень, формат, календарный план, положение и срок заявок. После создания черновик можно опубликовать отдельно.</span></div><div className="modal-actions"><button className="button button-light" type="button" onClick={onClose}>Отмена</button><button className="button button-primary" type="submit" disabled={saving}>{saving ? 'Сохраняем…' : <><Plus size={16} /> Создать черновик</>}</button></div></form></section></div>
}

function EmptyState({ label, detail }: { label: string; detail: string }) { return <div className="empty-state"><div><Search size={20} /></div><b>{label}</b><span>{detail}</span></div> }
