package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Samuel-DA/open-banking-financial-insights/internal/domain"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrConsentInactive = errors.New("consent is inactive or expired")
)

type Repository interface {
	Ping(context.Context) error
	Ingest(context.Context, domain.IngestRequest) ([]domain.Transaction, error)
	Consent(context.Context, string) (domain.Consent, error)
	RevokeConsent(context.Context, string, time.Time) error
	ExpireConsents(context.Context, time.Time) (int64, error)
	TransactionsForMonth(context.Context, string, time.Time) ([]domain.Transaction, error)
	SetCategory(context.Context, string, string) error
	UpsertInsight(context.Context, domain.Insight) error
	Insights(context.Context, string) ([]domain.Insight, error)
	Audit(context.Context, string, string, string, map[string]any) error
}
