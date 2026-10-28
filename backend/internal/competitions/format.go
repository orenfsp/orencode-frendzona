package competitions

import (
	"fmt"
	"strings"
)

func FormatUpcomingMessage(items []Competition) string {
	if len(items) == 0 {
		return "Сейчас нет ближайших соревнований."
	}
	var b strings.Builder
	b.WriteString("Ближайшие соревнования ФСП:\n")
	for _, c := range items {
		dateLabel := c.StartsAt
		if t, err := parseStartsAt(c.StartsAt); err == nil {
			dateLabel = t.Format("02.01.2006")
		}
		b.WriteString(fmt.Sprintf("• %s — %s, %s\n", c.Title, dateLabel, c.Discipline))
	}
	return strings.TrimSpace(b.String())
}
