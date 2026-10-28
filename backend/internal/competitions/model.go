package competitions

type Competition struct {
	ID                int64  `json:"id"`
	Title             string `json:"title"`
	Discipline        string `json:"discipline"`
	CompetitionType   string `json:"competition_type"`
	AgeGroup          string `json:"age_group"`
	StartsAt          string `json:"starts_at"`
	Location          string `json:"location"`
	Status            string `json:"status"`
	Participants      int    `json:"participants"`
	SportStatus       string `json:"sport_status"`
	EventFormat       string `json:"event_format"`
	CalendarPlan      string `json:"calendar_plan"`
	CalendarNumber    string `json:"calendar_number"`
	Regulation        string `json:"regulation"`
	ApplicationsUntil string `json:"applications_until,omitempty"`
}

type CreateInput struct {
	Title             string `json:"title"`
	Discipline        string `json:"discipline"`
	CompetitionType   string `json:"competition_type"`
	AgeGroup          string `json:"age_group"`
	StartsAt          string `json:"starts_at"`
	Location          string `json:"location"`
	SportStatus       string `json:"sport_status"`
	EventFormat       string `json:"event_format"`
	CalendarPlan      string `json:"calendar_plan"`
	CalendarNumber    string `json:"calendar_number"`
	Regulation        string `json:"regulation"`
	ApplicationsUntil string `json:"applications_until"`
}
