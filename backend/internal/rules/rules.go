package rules

import (
	"strings"
	"time"
)

const (
	RoleFederation = "federation"
	RoleAthlete    = "athlete"
	RoleCoach      = "coach"
	RoleJudge      = "judge"
	RoleMinistry   = "ministry"
)

var Disciplines = []string{
	"Программирование алгоритмическое",
	"Программирование продуктовое",
	"Программирование систем информационной безопасности",
	"Программирование систем искусственного интеллекта",
	"Программирование робототехники",
	"Программирование беспилотных авиационных систем",
}

var AgeGroups = []string{
	"Юноши и девушки 14–17",
	"Юниоры и юниорки 18–20",
	"Мужчины и женщины 21+",
	"Все группы",
}

var SportStatuses = []string{"региональное", "межрегиональное", "всероссийское"}
var EventFormats = []string{"очно", "онлайн", "смешанный"}

func AgeAt(birth, at time.Time) int {
	age := at.Year() - birth.Year()
	// сравниваем месяц и день, а не номер дня в году: иначе в високосные годы возраст сбивается на день
	if at.Month() < birth.Month() || (at.Month() == birth.Month() && at.Day() < birth.Day()) {
		age--
	}
	return age
}

func MatchesAgeGroup(group string, age int) bool {
	switch strings.TrimSpace(group) {
	case "", "Все группы":
		return true
	case "Юноши и девушки 14–17", "Юниоры":
		return age >= 14 && age <= 17
	case "Юниоры и юниорки 18–20":
		return age >= 18 && age <= 20
	case "Мужчины и женщины 21+", "Взрослые":
		return age >= 21
	case "Юниоры и взрослые":
		return age >= 14
	default:
		return true
	}
}

func MatchesDiscipline(athleteDisciplines []string, competitionDiscipline string) bool {
	want := strings.TrimSpace(competitionDiscipline)
	if want == "" {
		return true
	}
	for _, item := range athleteDisciplines {
		if strings.EqualFold(strings.TrimSpace(item), want) {
			return true
		}
	}
	return false
}

func PlacePoints(sportStatus, competitionType string, place int) int {
	if place < 1 || place > 3 {
		if place > 3 && place <= 10 {
			base := levelBase(sportStatus, competitionType)
			return max(4, base[2]/place)
		}
		return 0
	}
	table := levelBase(sportStatus, competitionType)
	return table[place-1]
}

func levelBase(sportStatus, competitionType string) [3]int {
	kind := strings.ToLower(competitionType + " " + sportStatus)
	switch {
	case strings.Contains(kind, "чемпионат россии") || (strings.Contains(kind, "всероссийск") && strings.Contains(kind, "чемпионат")):
		return [3]int{100, 80, 60}
	case strings.Contains(kind, "первенство россии") || (strings.Contains(kind, "всероссийск") && strings.Contains(kind, "первенство")):
		return [3]int{80, 60, 40}
	case strings.Contains(kind, "межрегиональн") || strings.Contains(kind, "окружн"):
		return [3]int{60, 45, 30}
	default:
		return [3]int{40, 30, 20}
	}
}

func StabilityMultiplier(prizeCount int) float64 {
	if prizeCount >= 2 {
		return 1.2
	}
	return 1
}

func RoleLabel(role string) string {
	switch role {
	case RoleFederation:
		return "Федерация"
	case RoleCoach:
		return "Тренер"
	case RoleAthlete:
		return "Спортсмен"
	case RoleJudge:
		return "Судья"
	case RoleMinistry:
		return "Министерство спорта"
	default:
		return role
	}
}

// Этапы спортивной подготовки по ФССП.
var TrainingStages = []string{
	"Начальная подготовка",
	"Учебно-тренировочный этап",
	"Совершенствование спортивного мастерства",
	"Высшее спортивное мастерство",
}
