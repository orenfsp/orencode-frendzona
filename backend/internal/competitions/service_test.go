package competitions

import (
	"testing"
	"time"
)

func TestNormalizeCreateRequired(t *testing.T) {
	_, _, err := NormalizeCreate(CreateInput{})
	if err != ErrRequired {
		t.Fatalf("got %v want %v", err, ErrRequired)
	}
}

func TestNormalizeCreateInvalidDate(t *testing.T) {
	_, _, err := NormalizeCreate(CreateInput{Title: "Кубок", Discipline: "Программирование", StartsAt: "21.11.2026"})
	if err != ErrInvalidDate {
		t.Fatalf("got %v want %v", err, ErrInvalidDate)
	}
}

func TestNormalizeCreateDefaultsAndRFC3339(t *testing.T) {
	input, starts, err := NormalizeCreate(CreateInput{
		Title:      "  Кубок  ",
		Discipline: "Программирование",
		StartsAt:   "2026-11-21T10:00:00+05:00",
	})
	if err != nil {
		t.Fatal(err)
	}
	if input.Title != "Кубок" {
		t.Fatalf("title: %q", input.Title)
	}
	if input.CompetitionType != "Региональный" || input.AgeGroup != "Все группы" || input.Location != "Оренбург" {
		t.Fatalf("defaults: %+v", input)
	}
	if !starts.Equal(time.Date(2026, 11, 21, 10, 0, 0, 0, starts.Location())) {
		t.Fatalf("starts: %v", starts)
	}
}

func TestNormalizeCreateDateOnly(t *testing.T) {
	_, starts, err := NormalizeCreate(CreateInput{
		Title:      "Спринт",
		Discipline: "Алгоритмическое программирование",
		StartsAt:   "2026-11-21",
	})
	if err != nil {
		t.Fatal(err)
	}
	if starts.Year() != 2026 || starts.Month() != 11 || starts.Day() != 21 {
		t.Fatalf("starts: %v", starts)
	}
}

func TestFormatUpcomingMessageEmpty(t *testing.T) {
	if got := FormatUpcomingMessage(nil); got != "Сейчас нет ближайших соревнований." {
		t.Fatalf("got %q", got)
	}
}
