package competitions

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidJSON  = errors.New("Проверьте формат JSON")
	ErrRequired     = errors.New("Название, дисциплина и дата обязательны")
	ErrInvalidDate  = errors.New("Дата должна быть в формате ISO: YYYY-MM-DD или RFC3339")
	ErrDuplicate    = errors.New("Соревнование с таким названием уже существует")
	ErrCreateFailed = errors.New("Не удалось сохранить соревнование")
	ErrListFailed   = errors.New("Не удалось получить соревнования")
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context) ([]Competition, error) {
	items, err := s.repo.List(ctx, false)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Service) ListUpcoming(ctx context.Context) ([]Competition, error) {
	return s.repo.List(ctx, true)
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Competition, error) {
	normalized, starts, err := NormalizeCreate(input)
	if err != nil {
		return Competition{}, err
	}
	c, err := s.repo.Create(ctx, normalized, starts)
	if err != nil {
		if IsDuplicateTitle(err) {
			return Competition{}, ErrDuplicate
		}
		return Competition{}, err
	}
	return c, nil
}

func NormalizeCreate(input CreateInput) (CreateInput, time.Time, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Discipline = strings.TrimSpace(input.Discipline)
	input.Location = strings.TrimSpace(input.Location)
	input.CompetitionType = strings.TrimSpace(input.CompetitionType)
	input.AgeGroup = strings.TrimSpace(input.AgeGroup)
	input.StartsAt = strings.TrimSpace(input.StartsAt)
	if input.Title == "" || input.Discipline == "" || input.StartsAt == "" {
		return CreateInput{}, time.Time{}, ErrRequired
	}
	starts, err := parseStartsAt(input.StartsAt)
	if err != nil {
		return CreateInput{}, time.Time{}, ErrInvalidDate
	}
	if input.CompetitionType == "" {
		input.CompetitionType = "Региональный"
	}
	if input.AgeGroup == "" {
		input.AgeGroup = "Все группы"
	}
	if input.Location == "" {
		input.Location = "Оренбург"
	}
	if input.SportStatus == "" {
		input.SportStatus = "региональное"
	}
	if input.EventFormat == "" {
		input.EventFormat = "очно"
	}
	if input.CalendarPlan == "" {
		if input.SportStatus == "региональное" {
			input.CalendarPlan = "Календарный план Оренбургской области"
		} else {
			input.CalendarPlan = "ЕКП Минспорта России"
		}
	}
	return input, starts, nil
}

func parseStartsAt(value string) (time.Time, error) {
	starts, err := time.Parse(time.RFC3339, value)
	if err != nil {
		starts, err = time.Parse("2006-01-02", value)
	}
	return starts, err
}
