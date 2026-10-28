package competitions

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

const competitionCols = `id,title,discipline,competition_type,age_group,starts_at,location,status,participants,
sport_status,event_format,calendar_plan,calendar_number,regulation,applications_until`

func (r *Repository) List(ctx context.Context, upcomingOnly bool) ([]Competition, error) {
	query := `SELECT ` + competitionCols + ` FROM competitions ORDER BY (starts_at < NOW()) ASC, starts_at ASC`
	if upcomingOnly {
		query = `SELECT ` + competitionCols + ` FROM competitions WHERE starts_at >= NOW() - INTERVAL '1 day' ORDER BY starts_at ASC LIMIT 6`
	}
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Competition{}
	for rows.Next() {
		c, err := scanCompetition(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (r *Repository) Get(ctx context.Context, id int64) (Competition, error) {
	return scanCompetition(r.db.QueryRowContext(ctx, `SELECT `+competitionCols+` FROM competitions WHERE id=$1`, id))
}

func (r *Repository) Create(ctx context.Context, input CreateInput, starts time.Time) (Competition, error) {
	var until any
	if strings.TrimSpace(input.ApplicationsUntil) != "" {
		until = input.ApplicationsUntil
	}
	return scanCompetition(r.db.QueryRowContext(ctx, `INSERT INTO competitions(title,discipline,competition_type,age_group,starts_at,location,status,participants,sport_status,event_format,calendar_plan,calendar_number,regulation,applications_until)
		VALUES($1,$2,$3,$4,$5,$6,'Черновик',0,$7,$8,$9,$10,$11,$12) RETURNING `+competitionCols,
		input.Title, input.Discipline, input.CompetitionType, input.AgeGroup, starts, input.Location,
		input.SportStatus, input.EventFormat, input.CalendarPlan, input.CalendarNumber, input.Regulation, until))
}

func IsDuplicateTitle(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate key")
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCompetition(row rowScanner) (Competition, error) {
	var c Competition
	var starts time.Time
	var until sql.NullTime
	if err := row.Scan(&c.ID, &c.Title, &c.Discipline, &c.CompetitionType, &c.AgeGroup, &starts, &c.Location, &c.Status, &c.Participants,
		&c.SportStatus, &c.EventFormat, &c.CalendarPlan, &c.CalendarNumber, &c.Regulation, &until); err != nil {
		return Competition{}, err
	}
	c.StartsAt = starts.Format(time.RFC3339)
	if until.Valid {
		c.ApplicationsUntil = until.Time.Format(time.RFC3339)
	}
	return c, nil
}
