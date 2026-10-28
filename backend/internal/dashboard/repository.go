package dashboard

import (
	"context"
	"database/sql"
	"time"

	"orenkod/backend/internal/competitions"
	"orenkod/backend/internal/ratings"
)

type Stats struct {
	Athletes     int `json:"athletes"`
	Competitions int `json:"competitions"`
	Applications int `json:"applications"`
	Judges       int `json:"judges"`
	Ranks        int `json:"ranks"`
	Expiring     int `json:"expiring"`
}

type Snapshot struct {
	Stats             Stats                      `json:"stats"`
	CompetitionsTotal int                        `json:"competitions_total"`
	Competitions      []competitions.Competition `json:"competitions"`
	TopAthletes       []ratings.RatingEntry      `json:"top_athletes"`
	UpdatedAt         string                     `json:"updated_at"`
}

type Repository struct {
	db           *sql.DB
	competitions *competitions.Repository
	ratings      *ratings.Repository
}

func NewRepository(db *sql.DB, competitionsRepo *competitions.Repository, ratingsRepo *ratings.Repository) *Repository {
	return &Repository{db: db, competitions: competitionsRepo, ratings: ratingsRepo}
}

func (r *Repository) Snapshot(ctx context.Context) (Snapshot, error) {
	var athletes, totalCompetitions, activeCompetitions, pendingApps, judges, pendingRanks, expiring int
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM athletes`).Scan(&athletes)
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM competitions`).Scan(&totalCompetitions)
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM competitions WHERE status IN ('Регистрация','Планируется')`).Scan(&activeCompetitions)
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM applications WHERE status='pending'`).Scan(&pendingApps)
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM judges`).Scan(&judges)
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM rank_applications WHERE status='pending'`).Scan(&pendingRanks)
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM athletes WHERE rank_expires_at IS NOT NULL AND rank_expires_at <= CURRENT_DATE + INTERVAL '90 days'`).Scan(&expiring)
	items, err := r.competitions.List(ctx, true)
	if err != nil {
		return Snapshot{}, err
	}
	top, err := r.ratings.List(ctx, "", 5)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		Stats: Stats{
			Athletes:     athletes,
			Competitions: activeCompetitions,
			Applications: pendingApps,
			Judges:       judges,
			Ranks:        pendingRanks,
			Expiring:     expiring,
		},
		CompetitionsTotal: totalCompetitions,
		Competitions:      items,
		TopAthletes:       top,
		UpdatedAt:         time.Now().Format(time.RFC3339),
	}, nil
}
