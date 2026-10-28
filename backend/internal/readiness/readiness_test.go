package readiness

import (
	"strings"
	"testing"
	"time"

	"orenkod/backend/internal/rules"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 12, 0, 0, 0, time.UTC) }
func ptr(t time.Time) *time.Time               { return &t }

func base() Input {
	return Input{
		Now:                   day(2026, 10, 28),
		StartsAt:              day(2026, 11, 5),
		CompetitionAgeGroup:   "Мужчины и женщины 21+",
		CompetitionDiscipline: rules.Disciplines[0],
		Birth:                 ptr(day(2000, 1, 1)),
		AthleteDisciplines:    []string{rules.Disciplines[0]},
		ApplicationStatus:     "approved",
		RusadaCompleted:       true,
		HasCoach:              true,
	}
}

func find(r Report, code string) Check {
	for _, c := range r.Checks {
		if c.Code == code {
			return c
		}
	}
	return Check{}
}

func TestFullyReadyAthlete(t *testing.T) {
	r := Evaluate(base())
	if !r.Ready || r.Blockers != 0 || r.Score != 100 {
		t.Fatalf("ожидали полную готовность, получили %+v", r)
	}
	if r.DaysLeft != 8 {
		t.Fatalf("до старта 8 суток, получили %d", r.DaysLeft)
	}
}

func TestBlockers(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Input)
		code string
	}{
		{"дисквалификация", func(i *Input) { i.Disqualified = true }, "sanctions"},
		{"возраст не подходит", func(i *Input) { i.Birth = ptr(day(2012, 1, 1)) }, "age"},
		{"чужая дисциплина", func(i *Input) { i.AthleteDisciplines = []string{rules.Disciplines[4]} }, "discipline"},
		{"заявка отклонена", func(i *Input) { i.ApplicationStatus = "rejected" }, "application"},
		{"заявки нет", func(i *Input) { i.ApplicationStatus = "" }, "application"},
		{"нет согласия представителя", func(i *Input) {
			i.Birth = ptr(day(2010, 1, 1))
			i.CompetitionAgeGroup = ""
		}, "guardian"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := base()
			tc.mut(&in)
			r := Evaluate(in)
			c := find(r, tc.code)
			if c.Status != Fail || !c.Blocking {
				t.Fatalf("%s: ожидали блокирующий fail, получили %+v", tc.code, c)
			}
			if r.Ready {
				t.Fatal("с блокером участник не может быть готов")
			}
		})
	}
}

func TestWarningsDoNotBlock(t *testing.T) {
	in := base()
	in.RusadaCompleted = false
	in.HasCoach = false
	in.ApplicationStatus = "pending"
	in.HasRank = true
	in.RankExpiresAt = ptr(day(2026, 11, 1)) // истекает до старта
	r := Evaluate(in)
	if !r.Ready {
		t.Fatalf("предупреждения не должны блокировать допуск: %+v", r)
	}
	for _, code := range []string{"antidoping", "coach", "application", "rank"} {
		if find(r, code).Status != Warn {
			t.Errorf("%s должен быть warn, получили %+v", code, find(r, code))
		}
	}
	if r.Score >= 100 || r.Score <= 50 {
		t.Fatalf("ожидали промежуточный процент, получили %d", r.Score)
	}
	if len(r.Problems()) != 4 {
		t.Fatalf("ожидали 4 замечания, получили %v", r.Problems())
	}
}

func TestUnknownBirthDateIsWarning(t *testing.T) {
	in := base()
	in.Birth = nil
	if c := find(Evaluate(in), "age"); c.Status != Warn || c.Blocking {
		t.Fatalf("без даты рождения возраст — предупреждение, получили %+v", c)
	}
}

func TestApplicationsClosed(t *testing.T) {
	in := base()
	in.ApplicationStatus = ""
	in.ApplicationsUntil = ptr(day(2026, 10, 20))
	c := find(Evaluate(in), "application")
	if c.Status != Fail || !strings.Contains(c.Hint, "закрыт") {
		t.Fatalf("приём закрыт, получили %+v", c)
	}
	in.ApplicationsUntil = ptr(day(2026, 11, 1))
	c = find(Evaluate(in), "application")
	if !strings.Contains(c.Hint, "01.11.2026") {
		t.Fatalf("подсказка должна называть срок приёма: %+v", c)
	}
}

func TestDaysUntilAndBucket(t *testing.T) {
	now := day(2026, 10, 28)
	if DaysUntil(now, day(2026, 10, 28)) != 0 || DaysUntil(now, day(2026, 10, 27)) != -1 {
		t.Fatal("DaysUntil считает сутки неверно")
	}
	// время суток не должно влиять на количество календарных дней
	late := time.Date(2026, 10, 28, 23, 59, 0, 0, time.UTC)
	if DaysUntil(late, time.Date(2026, 10, 29, 0, 1, 0, 0, time.UTC)) != 1 {
		t.Fatal("23:59 → 00:01 следующего дня — это 1 сутки")
	}
	want := map[int]int{-1: 0, 0: 0, 1: 1, 2: 3, 3: 3, 4: 7, 7: 7, 8: 0, 30: 0}
	for days, bucket := range want {
		if got := Bucket(days); got != bucket {
			t.Errorf("Bucket(%d)=%d, ожидали %d", days, got, bucket)
		}
	}
}
