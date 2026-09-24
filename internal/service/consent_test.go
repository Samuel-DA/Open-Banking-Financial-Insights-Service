package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Samuel-DA/open-banking-financial-insights/internal/domain"
	"github.com/Samuel-DA/open-banking-financial-insights/internal/repository"
)

type consentRepo struct {
	consent      domain.Consent
	ingestCalled bool
}

func (r *consentRepo) Ping(context.Context) error { return nil }
func (r *consentRepo) Ingest(context.Context, domain.IngestRequest) ([]domain.Transaction, error) {
	r.ingestCalled = true
	return nil, nil
}
func (r *consentRepo) Consent(context.Context, string) (domain.Consent, error) {
	return r.consent, nil
}
func (r *consentRepo) RevokeConsent(context.Context, string, time.Time) error   { return nil }
func (r *consentRepo) ExpireConsents(context.Context, time.Time) (int64, error) { return 0, nil }
func (r *consentRepo) TransactionsForMonth(context.Context, string, time.Time) ([]domain.Transaction, error) {
	return nil, nil
}
func (r *consentRepo) SetCategory(context.Context, string, string) error          { return nil }
func (r *consentRepo) UpsertInsight(context.Context, domain.Insight) error        { return nil }
func (r *consentRepo) Insights(context.Context, string) ([]domain.Insight, error) { return nil, nil }
func (r *consentRepo) Audit(context.Context, string, string, string, map[string]any) error {
	return nil
}

func TestIngestRejectsPersistedRevokedConsent(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	repo := &consentRepo{consent: domain.Consent{
		ID:        "consent-1",
		Status:    domain.ConsentRevoked,
		ExpiresAt: now.Add(time.Hour),
	}}
	svc := New(repo, nil)
	svc.now = func() time.Time { return now }

	_, err := svc.Ingest(context.Background(), domain.IngestRequest{Consent: domain.Consent{
		ID:        "consent-1",
		Status:    domain.ConsentActive,
		ExpiresAt: now.Add(time.Hour),
	}}, "test")

	if !errors.Is(err, ErrInvalidConsent) {
		t.Fatalf("expected ErrInvalidConsent, got %v", err)
	}
	if repo.ingestCalled {
		t.Fatal("repository ingestion must not run for a revoked consent")
	}
}

var _ repository.Repository = (*consentRepo)(nil)
