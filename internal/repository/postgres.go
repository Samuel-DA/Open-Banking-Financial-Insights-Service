package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Samuel-DA/open-banking-financial-insights/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(ctx context.Context, url string) (*Postgres, error) {
	p, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := p.Ping(ctx); err != nil {
		p.Close()
		return nil, err
	}
	return &Postgres{pool: p}, nil
}
func (p *Postgres) Close()                         { p.pool.Close() }
func (p *Postgres) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

func (p *Postgres) Ingest(ctx context.Context, in domain.IngestRequest) ([]domain.Transaction, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	ct, err := tx.Exec(ctx, `INSERT INTO consents(id,customer_ref,permissions,status,granted_at,expires_at) VALUES($1,$2,$3,$4,$5,$6)
		ON CONFLICT(id) DO UPDATE SET permissions=EXCLUDED.permissions
		WHERE consents.status='active' AND consents.expires_at > now()`, in.Consent.ID, in.Consent.CustomerRef, in.Consent.Permissions, in.Consent.Status, in.Consent.GrantedAt, in.Consent.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("consent: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return nil, ErrConsentInactive
	}
	for _, a := range in.Accounts {
		_, err = tx.Exec(ctx, `INSERT INTO accounts(id,consent_id,external_id,display_name,currency,account_type,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT(external_id) DO UPDATE SET display_name=EXCLUDED.display_name`, a.ID, a.ConsentID, a.ExternalID, a.DisplayName, a.Currency, a.Type, a.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("account: %w", err)
		}
	}
	inserted := make([]domain.Transaction, 0, len(in.Transactions))
	for _, t := range in.Transactions {
		ct, err := tx.Exec(ctx, `INSERT INTO transactions(id,account_id,external_id,amount_minor,currency,description,booked_at,category) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(external_id) DO NOTHING`, t.ID, t.AccountID, t.ExternalID, t.Amount, t.Currency, t.Description, t.BookedAt, t.Category)
		if err != nil {
			return nil, fmt.Errorf("transaction: %w", err)
		}
		if ct.RowsAffected() > 0 {
			inserted = append(inserted, t)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return inserted, nil
}
func (p *Postgres) Consent(ctx context.Context, id string) (domain.Consent, error) {
	var c domain.Consent
	err := p.pool.QueryRow(ctx, `SELECT id,customer_ref,permissions,status,granted_at,expires_at,revoked_at FROM consents WHERE id=$1`, id).Scan(&c.ID, &c.CustomerRef, &c.Permissions, &c.Status, &c.GrantedAt, &c.ExpiresAt, &c.RevokedAt)
	if err == pgx.ErrNoRows {
		return c, ErrNotFound
	}
	return c, err
}
func (p *Postgres) RevokeConsent(ctx context.Context, id string, at time.Time) error {
	ct, err := p.pool.Exec(ctx, `UPDATE consents SET status='revoked',revoked_at=$2 WHERE id=$1 AND status='active'`, id, at)
	if err == nil && ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}
func (p *Postgres) ExpireConsents(ctx context.Context, now time.Time) (int64, error) {
	ct, err := p.pool.Exec(ctx, `UPDATE consents SET status='expired' WHERE status='active' AND expires_at <= $1`, now)
	return ct.RowsAffected(), err
}
func (p *Postgres) TransactionsForMonth(ctx context.Context, accountID string, month time.Time) ([]domain.Transaction, error) {
	end := month.AddDate(0, 1, 0)
	rows, err := p.pool.Query(ctx, `SELECT id,account_id,external_id,amount_minor,currency,description,booked_at,category FROM transactions WHERE account_id=$1 AND booked_at >= $2 AND booked_at < $3 ORDER BY booked_at`, accountID, month, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Transaction
	for rows.Next() {
		var t domain.Transaction
		if err := rows.Scan(&t.ID, &t.AccountID, &t.ExternalID, &t.Amount, &t.Currency, &t.Description, &t.BookedAt, &t.Category); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func (p *Postgres) SetCategory(ctx context.Context, id, cat string) error {
	_, err := p.pool.Exec(ctx, `UPDATE transactions SET category=$2 WHERE id=$1`, id, cat)
	return err
}
func (p *Postgres) UpsertInsight(ctx context.Context, i domain.Insight) error {
	cats, _ := json.Marshal(i.CategorySpend)
	_, err := p.pool.Exec(ctx, `INSERT INTO monthly_insights(id,account_id,month,total_income_minor,total_spending_minor,net_cashflow_minor,category_spend,transaction_count,generated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(account_id,month) DO UPDATE SET total_income_minor=EXCLUDED.total_income_minor,total_spending_minor=EXCLUDED.total_spending_minor,net_cashflow_minor=EXCLUDED.net_cashflow_minor,category_spend=EXCLUDED.category_spend,transaction_count=EXCLUDED.transaction_count,generated_at=EXCLUDED.generated_at`, i.ID, i.AccountID, i.Month+"-01", i.TotalIncome, i.TotalSpending, i.NetCashflow, cats, i.TransactionCount, i.GeneratedAt)
	return err
}
func (p *Postgres) Insights(ctx context.Context, accountID string) ([]domain.Insight, error) {
	rows, err := p.pool.Query(ctx, `SELECT id,account_id,to_char(month,'YYYY-MM'),total_income_minor,total_spending_minor,net_cashflow_minor,category_spend,transaction_count,generated_at FROM monthly_insights WHERE account_id=$1 ORDER BY month DESC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Insight
	for rows.Next() {
		var i domain.Insight
		var b []byte
		if err := rows.Scan(&i.ID, &i.AccountID, &i.Month, &i.TotalIncome, &i.TotalSpending, &i.NetCashflow, &b, &i.TransactionCount, &i.GeneratedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(b, &i.CategorySpend)
		out = append(out, i)
	}
	return out, rows.Err()
}
func (p *Postgres) Audit(ctx context.Context, action, resourceID, actor string, details map[string]any) error {
	b, _ := json.Marshal(details)
	_, err := p.pool.Exec(ctx, `INSERT INTO audit_log(action,resource_id,actor,details) VALUES($1,$2,$3,$4)`, action, resourceID, actor, b)
	return err
}
