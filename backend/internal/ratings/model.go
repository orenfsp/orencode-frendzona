package ratings

type RatingEntry struct {
	Rank         int    `json:"rank"`
	Athlete      string `json:"athlete"`
	Organization string `json:"organization"`
	Discipline   string `json:"discipline"`
	AgeGroup     string `json:"age_group"`
	Points       int    `json:"points"`
}
