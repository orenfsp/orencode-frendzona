package athletes

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context) ([]Athlete, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,COALESCE(user_id,0),full_name,organization,discipline,age_group,points,coach_name,status,
		municipality,birth_date,year_started,rank_title,rank_expires_at,fsp_id,disqualified,training_stage
		FROM athletes ORDER BY full_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Athlete{}
	for rows.Next() {
		var a Athlete
		var birth, expires sql.NullTime
		if err := rows.Scan(&a.ID, &a.UserID, &a.FullName, &a.Organization, &a.Discipline, &a.AgeGroup, &a.Points, &a.CoachName, &a.Status,
			&a.Municipality, &birth, &a.YearStarted, &a.RankTitle, &expires, &a.FSPID, &a.Disqualified, &a.TrainingStage); err != nil {
			return nil, err
		}
		if birth.Valid {
			a.BirthDate = birth.Time.Format("2006-01-02")
		}
		if expires.Valid {
			a.RankExpires = expires.Time.Format("2006-01-02")
		}
		discRows, err := r.db.QueryContext(ctx, `SELECT discipline FROM athlete_disciplines WHERE athlete_id=$1 ORDER BY discipline`, a.ID)
		if err == nil {
			for discRows.Next() {
				var d string
				_ = discRows.Scan(&d)
				a.Disciplines = append(a.Disciplines, d)
			}
			discRows.Close()
		}
		if len(a.Disciplines) == 0 {
			a.Disciplines = []string{a.Discipline}
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func Filter(items []Athlete, q, discipline, municipality, rank, age string) []Athlete {
	q = strings.ToLower(strings.TrimSpace(q))
	out := []Athlete{}
	for _, a := range items {
		if discipline != "" && a.Discipline != discipline && !contains(a.Disciplines, discipline) {
			continue
		}
		if municipality != "" && !strings.EqualFold(a.Municipality, municipality) {
			continue
		}
		if rank != "" && a.RankTitle != rank {
			continue
		}
		if age != "" && a.AgeGroup != age {
			continue
		}
		blob := strings.ToLower(a.FullName + " " + a.Organization + " " + a.CoachName + " " + a.FSPID)
		if q != "" && !strings.Contains(blob, q) {
			continue
		}
		out = append(out, a)
	}
	return out
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func AgeGroupForBirth(birth time.Time, at time.Time) string {
	age := at.Year() - birth.Year()
	if at.YearDay() < birth.YearDay() {
		age--
	}
	switch {
	case age <= 17:
		return "Юноши и девушки 14–17"
	case age <= 20:
		return "Юниоры и юниорки 18–20"
	default:
		return "Мужчины и женщины 21+"
	}
}
