package ratings

import (
	"context"
	"database/sql"
	"strconv"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context, discipline string, limit int) ([]RatingEntry, error) {
	query := `SELECT full_name, organization, discipline, age_group, points FROM athletes`
	args := []any{}
	if discipline != "" {
		query += ` WHERE discipline=$1`
		args = append(args, discipline)
	}
	query += ` ORDER BY points DESC, full_name ASC`
	if limit > 0 {
		query += ` LIMIT $` + strconv.Itoa(len(args)+1)
		args = append(args, limit)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []RatingEntry{}
	for rows.Next() {
		var item RatingEntry
		if err := rows.Scan(&item.Athlete, &item.Organization, &item.Discipline, &item.AgeGroup, &item.Points); err != nil {
			return nil, err
		}
		item.Rank = len(items) + 1
		items = append(items, item)
	}
	return items, rows.Err()
}
