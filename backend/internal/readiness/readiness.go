// Package readiness — собственный сценарий команды «Готовность к старту».
// Чистая логика без базы данных: по данным спортсмена и соревнования строит чек-лист допуска,
// процент готовности и список блокеров. Вызывается из platform (API) и напоминаний в Telegram.
package readiness

import (
	"math"
	"time"

	"orenkod/backend/internal/rules"
)

const (
	OK   = "ok"
	Warn = "warn"
	Fail = "fail"
)

// ReminderDays — за сколько дней до старта напоминаем неготовым участникам.
var ReminderDays = []int{7, 3, 1}

type Input struct {
	Now                   time.Time
	StartsAt              time.Time
	ApplicationsUntil     *time.Time
	CompetitionAgeGroup   string
	CompetitionDiscipline string
	Birth                 *time.Time
	AthleteDisciplines    []string
	ApplicationStatus     string // "", pending, approved, rejected
	HasRank               bool
	RankExpiresAt         *time.Time
	RusadaCompleted       bool
	GuardianConsent       bool
	Disqualified          bool
	HasCoach              bool
}

type Check struct {
	Code     string `json:"code"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Hint     string `json:"hint"`
	Blocking bool   `json:"blocking"`
}

type Report struct {
	Checks   []Check `json:"checks"`
	Score    int     `json:"score"`
	Ready    bool    `json:"ready"`
	Blockers int     `json:"blockers"`
	DaysLeft int     `json:"days_left"`
}

// DaysUntil — сколько полных календарных суток до старта (0 — старт сегодня, <0 — уже прошёл).
func DaysUntil(now, start time.Time) int {
	a := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	return int(math.Round(b.Sub(a).Hours() / 24))
}

// Bucket возвращает порог напоминания (7, 3 или 1) для оставшихся дней или 0, если напоминать рано/поздно.
func Bucket(daysLeft int) int {
	if daysLeft < 1 {
		return 0
	}
	for i := len(ReminderDays) - 1; i >= 0; i-- {
		if daysLeft <= ReminderDays[i] {
			return ReminderDays[i]
		}
	}
	return 0
}

func Evaluate(in Input) Report {
	var cs []Check
	add := func(code, title, status, hint string, blocking bool) {
		cs = append(cs, Check{Code: code, Title: title, Status: status, Hint: hint, Blocking: blocking && status == Fail})
	}

	if in.Disqualified {
		add("sanctions", "Нет действующих санкций", Fail, "Действует дисквалификация или отстранение — обратитесь в федерацию.", true)
	} else {
		add("sanctions", "Нет действующих санкций", OK, "", true)
	}

	age := -1
	if in.Birth != nil {
		age = rules.AgeAt(*in.Birth, in.StartsAt)
	}
	switch {
	case in.CompetitionAgeGroup == "":
		add("age", "Возрастная группа", OK, "Возрастных ограничений нет.", true)
	case age < 0:
		add("age", "Возрастная группа", Warn, "В профиле не указана дата рождения — платформа не может проверить возраст.", false)
	case rules.MatchesAgeGroup(in.CompetitionAgeGroup, age):
		add("age", "Возрастная группа", OK, "", true)
	default:
		add("age", "Возрастная группа", Fail, "Возраст на дату старта не подходит под группу «"+in.CompetitionAgeGroup+"».", true)
	}

	if rules.MatchesDiscipline(in.AthleteDisciplines, in.CompetitionDiscipline) {
		add("discipline", "Дисциплина", OK, "", true)
	} else {
		add("discipline", "Дисциплина", Fail, "Дисциплина «"+in.CompetitionDiscipline+"» не указана в профиле спортсмена.", true)
	}

	switch in.ApplicationStatus {
	case "approved":
		add("application", "Заявка одобрена", OK, "", true)
	case "pending":
		add("application", "Заявка одобрена", Warn, "Заявка подана и ждёт модерации федерации.", false)
	case "rejected":
		add("application", "Заявка одобрена", Fail, "Заявка отклонена — уточните причину в кабинете.", true)
	default:
		if in.ApplicationsUntil != nil && in.Now.After(*in.ApplicationsUntil) {
			add("application", "Заявка одобрена", Fail, "Заявка не подана, приём заявок закрыт.", true)
		} else {
			hint := "Подайте заявку на соревнование."
			if in.ApplicationsUntil != nil {
				hint += " Приём до " + in.ApplicationsUntil.Format("02.01.2006") + "."
			}
			add("application", "Заявка одобрена", Fail, hint, true)
		}
	}

	if in.RusadaCompleted {
		add("antidoping", "Антидопинговый курс РУСАДА", OK, "", false)
	} else {
		add("antidoping", "Антидопинговый курс РУСАДА", Warn, "Пройдите онлайн-курс на rusada.ru и приложите сертификат.", false)
	}

	if age >= 0 && age < 18 {
		if in.GuardianConsent {
			add("guardian", "Согласие законного представителя", OK, "", true)
		} else {
			add("guardian", "Согласие законного представителя", Fail, "Для несовершеннолетнего нужно согласие законного представителя на обработку данных.", true)
		}
	} else {
		add("guardian", "Согласие законного представителя", OK, "Не требуется.", true)
	}

	switch {
	case !in.HasRank:
		add("rank", "Срок действия разряда", OK, "Разряд не заявлен.", false)
	case in.RankExpiresAt == nil:
		add("rank", "Срок действия разряда", OK, "", false)
	case in.RankExpiresAt.Before(in.StartsAt):
		add("rank", "Срок действия разряда", Warn, "Разряд действует до "+in.RankExpiresAt.Format("02.01.2006")+" — раньше старта. Подайте заявку на подтверждение.", false)
	default:
		add("rank", "Срок действия разряда", OK, "", false)
	}

	if in.HasCoach {
		add("coach", "Тренер привязан", OK, "", false)
	} else {
		add("coach", "Тренер привязан", Warn, "Нет активной привязки к тренеру — заявки на разряд подаёте сами.", false)
	}

	score, blockers := 0.0, 0
	for _, c := range cs {
		switch c.Status {
		case OK:
			score++
		case Warn:
			score += 0.5
		}
		if c.Blocking {
			blockers++
		}
	}
	return Report{Checks: cs, Score: int(math.Round(100 * score / float64(len(cs)))), Ready: blockers == 0, Blockers: blockers, DaysLeft: DaysUntil(in.Now, in.StartsAt)}
}

// Problems возвращает названия непройденных (fail и warn) пунктов — для текста напоминания.
func (r Report) Problems() []string {
	var out []string
	for _, c := range r.Checks {
		if c.Status != OK {
			out = append(out, c.Title+": "+c.Hint)
		}
	}
	return out
}
