package athletes

type Athlete struct {
	ID            int64    `json:"id"`
	UserID        int64    `json:"user_id,omitempty"`
	FullName      string   `json:"full_name"`
	Organization  string   `json:"organization"`
	Discipline    string   `json:"discipline"`
	Disciplines   []string `json:"disciplines,omitempty"`
	AgeGroup      string   `json:"age_group"`
	Points        int      `json:"points"`
	CoachName     string   `json:"coach_name"`
	Status        string   `json:"status"`
	Municipality  string   `json:"municipality,omitempty"`
	BirthDate     string   `json:"birth_date,omitempty"`
	YearStarted   int      `json:"year_started,omitempty"`
	RankTitle     string   `json:"rank_title,omitempty"`
	RankExpires   string   `json:"rank_expires_at,omitempty"`
	FSPID         string   `json:"fsp_id,omitempty"`
	Disqualified  bool     `json:"disqualified,omitempty"`
	TrainingStage string   `json:"training_stage,omitempty"`
}
