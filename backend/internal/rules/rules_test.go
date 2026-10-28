package rules

import (
	"testing"
	"time"
)

func TestAgeAndDiscipline(t *testing.T) {
	birth := time.Date(2008, 3, 1, 0, 0, 0, 0, time.UTC)
	event := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
	age := AgeAt(birth, event)
	if age != 18 {
		t.Fatalf("age %d", age)
	}
	if MatchesAgeGroup("Юноши и девушки 14–17", age) {
		t.Fatal("18 must not pass 14-17")
	}
	if !MatchesAgeGroup("Юниоры и юниорки 18–20", age) {
		t.Fatal("18 must pass juniors")
	}
	if !MatchesDiscipline([]string{"Программирование алгоритмическое", "Программирование продуктовое"}, "Программирование алгоритмическое") {
		t.Fatal("discipline should match")
	}
	if MatchesDiscipline([]string{"Программирование робототехники"}, "Программирование алгоритмическое") {
		t.Fatal("discipline should reject")
	}
}

func TestRatingFormula(t *testing.T) {
	if PlacePoints("всероссийское", "Чемпионат России", 1) != 100 {
		t.Fatal("russia gold")
	}
	if PlacePoints("региональное", "Чемпионат области", 2) != 30 {
		t.Fatal("regional silver")
	}
	if StabilityMultiplier(2) != 1.2 || StabilityMultiplier(1) != 1 {
		t.Fatal("stability")
	}
}
