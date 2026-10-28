package rules

import (
	"testing"
	"time"
)

func TestPlacePointsByLevel(t *testing.T) {
	cases := []struct {
		status, typ string
		place, want int
	}{
		{"региональное", "Чемпионат", 1, 40},
		{"региональное", "Первенство", 3, 20},
		{"межрегиональное", "Кубок", 2, 45},
		{"всероссийское", "Чемпионат", 1, 100},
		{"всероссийское", "Первенство", 2, 60},
		{"региональное", "Чемпионат", 0, 0},
		{"региональное", "Чемпионат", 11, 0},
	}
	for _, c := range cases {
		if got := PlacePoints(c.status, c.typ, c.place); got != c.want {
			t.Errorf("PlacePoints(%q,%q,%d)=%d, ожидали %d", c.status, c.typ, c.place, got, c.want)
		}
	}
	// места 4–10 дают убывающие, но положительные баллы (не меньше 4)
	for p := 4; p <= 10; p++ {
		if PlacePoints("региональное", "Чемпионат", p) < 4 {
			t.Errorf("место %d должно давать не меньше 4 баллов", p)
		}
	}
}

func TestStabilityMultiplier(t *testing.T) {
	if StabilityMultiplier(0) != 1 || StabilityMultiplier(1) != 1 {
		t.Fatal("меньше двух призовых мест — коэффициент 1")
	}
	if StabilityMultiplier(2) != 1.2 || StabilityMultiplier(5) != 1.2 {
		t.Fatal("два и более призовых места — коэффициент 1,2")
	}
}

func TestAgeGroups(t *testing.T) {
	cases := []struct {
		group string
		age   int
		want  bool
	}{
		{"Юноши и девушки 14–17", 13, false},
		{"Юноши и девушки 14–17", 14, true},
		{"Юноши и девушки 14–17", 17, true},
		{"Юноши и девушки 14–17", 18, false},
		{"Юниоры и юниорки 18–20", 18, true},
		{"Юниоры и юниорки 18–20", 21, false},
		{"Мужчины и женщины 21+", 20, false},
		{"Мужчины и женщины 21+", 40, true},
		{"Все группы", 5, true},
		{"", 5, true},
	}
	for _, c := range cases {
		if got := MatchesAgeGroup(c.group, c.age); got != c.want {
			t.Errorf("MatchesAgeGroup(%q,%d)=%v, ожидали %v", c.group, c.age, got, c.want)
		}
	}
}

func TestAgeAtBirthdayBoundary(t *testing.T) {
	birth := time.Date(2008, 6, 15, 0, 0, 0, 0, time.UTC)
	if AgeAt(birth, time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)) != 17 {
		t.Fatal("за день до 18-летия возраст 17")
	}
	if AgeAt(birth, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)) != 18 {
		t.Fatal("в день 18-летия возраст 18")
	}
}

func TestSixDisciplinesAndStages(t *testing.T) {
	if len(Disciplines) != 6 {
		t.Fatalf("в кейсе шесть дисциплин, в коде %d", len(Disciplines))
	}
	seen := map[string]bool{}
	for _, d := range Disciplines {
		if seen[d] {
			t.Fatalf("дисциплина %q повторяется", d)
		}
		seen[d] = true
	}
	if len(TrainingStages) < 2 {
		t.Fatal("нужны как минимум два этапа подготовки для перевода")
	}
}

func TestMatchesDisciplineIgnoresCase(t *testing.T) {
	if !MatchesDiscipline([]string{"  программирование РОБОТОТЕХНИКИ "}, "Программирование робототехники") {
		t.Fatal("сравнение должно игнорировать регистр и пробелы")
	}
	if MatchesDiscipline(nil, Disciplines[0]) {
		t.Fatal("без дисциплин спортсмен не подходит под конкретную дисциплину")
	}
	if !MatchesDiscipline(nil, "") {
		t.Fatal("соревнование без дисциплины подходит всем")
	}
}
