package platform

import (
	"context"
	"fmt"
	"log"
	"time"
)

// RunDeadlineWatcher периодически проверяет сроки разрядов и судейских категорий
// и рассылает предупреждения (личный кабинет + Telegram привязанным пользователям).
func (h *Handler) RunDeadlineWatcher(ctx context.Context, every time.Duration, days int) {
	h.checkDeadlines(ctx, days)
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.checkDeadlines(ctx, days)
		}
	}
}

// CheckDeadlines запускает одну проверку и возвращает число созданных уведомлений.
func (h *Handler) CheckDeadlines(ctx context.Context, days int) int {
	return h.checkDeadlines(ctx, days)
}

func (h *Handler) checkDeadlines(ctx context.Context, days int) int {
	sent := 0
	// Разряды: спортсмен, его активные тренеры и федерация.
	rows, err := h.db.QueryContext(ctx, `SELECT a.id, a.full_name, a.rank_title, a.rank_expires_at, COALESCE(a.user_id,0)
		FROM athletes a WHERE a.rank_expires_at IS NOT NULL AND a.rank_expires_at <= CURRENT_DATE + $1::int AND a.rank_expires_at >= CURRENT_DATE - 30`, days)
	if err != nil {
		log.Printf("deadlines ranks: %v", err)
	} else {
		type item struct {
			id, uid    int64
			name, rank string
			exp        time.Time
		}
		var items []item
		for rows.Next() {
			var it item
			if rows.Scan(&it.id, &it.name, &it.rank, &it.exp, &it.uid) == nil {
				items = append(items, it)
			}
		}
		rows.Close()
		for _, it := range items {
			msg := fmt.Sprintf("Срок действия разряда «%s» спортсмена %s истекает %s. Подайте заявку на подтверждение.", it.rank, it.name, it.exp.Format("02.01.2006"))
			targets := []int64{}
			if it.uid > 0 {
				targets = append(targets, it.uid)
			}
			cr, err := h.db.QueryContext(ctx, `SELECT c.user_id FROM coach_links l JOIN coaches c ON c.id=l.coach_id WHERE l.athlete_id=$1 AND l.status='active'`, it.id)
			if err == nil {
				for cr.Next() {
					var u int64
					if cr.Scan(&u) == nil {
						targets = append(targets, u)
					}
				}
				cr.Close()
			}
			sent += h.warnOnce(ctx, "Истекает разряд", msg, targets, true)
		}
	}
	// Судейские категории: судья и федерация.
	jr, err := h.db.QueryContext(ctx, `SELECT j.user_id, u.full_name, j.category, j.category_expires_at FROM judges j JOIN users u ON u.id=j.user_id
		WHERE j.category_expires_at IS NOT NULL AND j.category_expires_at <= CURRENT_DATE + $1::int AND j.category_expires_at >= CURRENT_DATE - 30`, days)
	if err != nil {
		log.Printf("deadlines judges: %v", err)
		return sent
	}
	type jitem struct {
		uid       int64
		name, cat string
		exp       time.Time
	}
	var js []jitem
	for jr.Next() {
		var it jitem
		if jr.Scan(&it.uid, &it.name, &it.cat, &it.exp) == nil {
			js = append(js, it)
		}
	}
	jr.Close()
	for _, it := range js {
		msg := fmt.Sprintf("Срок действия судейской категории «%s» судьи %s истекает %s. Подайте заявку на подтверждение.", it.cat, it.name, it.exp.Format("02.01.2006"))
		sent += h.warnOnce(ctx, "Истекает судейская категория", msg, []int64{it.uid}, true)
	}
	return sent
}

// warnOnce не дублирует одинаковое предупреждение чаще раза в 7 суток.
func (h *Handler) warnOnce(ctx context.Context, title, msg string, users []int64, toFederation bool) int {
	var n int
	_ = h.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM inbox WHERE title=$1 AND message=$2 AND created_at > NOW() - INTERVAL '7 days'`, title, msg).Scan(&n)
	if n > 0 {
		return 0
	}
	seen := map[int64]bool{}
	for _, u := range users {
		if u <= 0 || seen[u] {
			continue
		}
		seen[u] = true
		_, _ = h.db.ExecContext(ctx, `INSERT INTO inbox(user_id,kind,title,message) VALUES($1,'warning',$2,$3)`, u, title, msg)
		var tg *int64
		if h.notify != nil && h.db.QueryRowContext(ctx, `SELECT telegram_user_id FROM users WHERE id=$1`, u).Scan(&tg) == nil && tg != nil {
			h.notify.NotifyUser(*tg, "⚠️ "+title+"\n"+msg)
		}
	}
	if toFederation {
		_, _ = h.db.ExecContext(ctx, `INSERT INTO inbox(role_scope,kind,title,message) VALUES('federation','warning',$1,$2)`, title, msg)
	}
	return 1
}
