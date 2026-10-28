package platform

import (
	"testing"

	"orenkod/backend/internal/rules"
)

func TestNextStageFollowsTrainingStages(t *testing.T) {
	st := rules.TrainingStages
	for i := 0; i < len(st)-1; i++ {
		if got := nextStage(st[i]); got != st[i+1] {
			t.Errorf("после %q ожидали %q, получили %q", st[i], st[i+1], got)
		}
	}
	if got := nextStage(st[len(st)-1]); got != "" {
		t.Errorf("после последнего этапа перевода нет, получили %q", got)
	}
	if got := nextStage("несуществующий этап"); got != "" {
		t.Errorf("неизвестный этап не должен переводиться, получили %q", got)
	}
}

func TestStageAndDisciplineValidation(t *testing.T) {
	for _, s := range rules.TrainingStages {
		if !validStage(s) {
			t.Errorf("этап %q должен считаться допустимым", s)
		}
	}
	for _, d := range rules.Disciplines {
		if !validDiscipline(d) {
			t.Errorf("дисциплина %q должна считаться допустимой", d)
		}
	}
	if validStage("") || validDiscipline("Шахматы") {
		t.Error("пустой этап и чужая дисциплина недопустимы")
	}
}

func TestParseIDAndItoa(t *testing.T) {
	if id, err := parseID(" 42 "); err != nil || id != 42 {
		t.Fatalf("parseID: %d %v", id, err)
	}
	if _, err := parseID("abc"); err == nil {
		t.Fatal("нечисловой id должен давать ошибку")
	}
	if itoaInt(-3) != "-3" {
		t.Fatal("itoaInt")
	}
}
