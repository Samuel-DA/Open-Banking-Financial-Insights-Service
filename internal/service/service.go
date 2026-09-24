package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Samuel-DA/open-banking-financial-insights/internal/domain"
	"github.com/Samuel-DA/open-banking-financial-insights/internal/events"
	"github.com/Samuel-DA/open-banking-financial-insights/internal/repository"
	"github.com/google/uuid"
)

var ErrInvalidConsent = errors.New("consent is inactive or expired")

type Service struct {
	repo      repository.Repository
	publisher events.Publisher
	now       func() time.Time
}

func New(repo repository.Repository, publisher events.Publisher) *Service {
	return &Service{repo: repo, publisher: publisher, now: time.Now}
}
func (s *Service) Ingest(ctx context.Context, in domain.IngestRequest, actor string) (int, error) {
	now := s.now()
	if !in.Consent.IsUsable(now) {
		return 0, ErrInvalidConsent
	}
	stored, err := s.repo.Consent(ctx, in.Consent.ID)
	if err == nil && !stored.IsUsable(now) {
		return 0, ErrInvalidConsent
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return 0, fmt.Errorf("load consent: %w", err)
	}
	for _, a := range in.Accounts {
		if a.ConsentID != in.Consent.ID {
			return 0, fmt.Errorf("account %s has mismatched consent", a.ID)
		}
	}
	inserted, err := s.repo.Ingest(ctx, in)
	if errors.Is(err, repository.ErrConsentInactive) {
		return 0, ErrInvalidConsent
	}
	if err != nil {
		return 0, err
	}
	for _, t := range inserted {
		if err := s.publisher.PublishTransaction(ctx, t); err != nil {
			return len(inserted), fmt.Errorf("publish transaction %s: %w", t.ID, err)
		}
	}
	_ = s.repo.Audit(ctx, "data.ingested", in.Consent.ID, actor, map[string]any{"transactions": len(inserted)})
	return len(inserted), nil
}
func (s *Service) Revoke(ctx context.Context, id, actor string) error {
	if err := s.repo.RevokeConsent(ctx, id, s.now()); err != nil {
		return err
	}
	return s.repo.Audit(ctx, "consent.revoked", id, actor, nil)
}
func (s *Service) Expire(ctx context.Context) (int64, error) {
	return s.repo.ExpireConsents(ctx, s.now())
}
func (s *Service) Insights(ctx context.Context, accountID string) ([]domain.Insight, error) {
	return s.repo.Insights(ctx, accountID)
}
func (s *Service) ProcessTransaction(ctx context.Context, t domain.Transaction) error {
	cat := Categorise(t.Description, t.Amount)
	if err := s.repo.SetCategory(ctx, t.ID, cat); err != nil {
		return err
	}
	month := time.Date(t.BookedAt.Year(), t.BookedAt.Month(), 1, 0, 0, 0, 0, time.UTC)
	txns, err := s.repo.TransactionsForMonth(ctx, t.AccountID, month)
	if err != nil {
		return err
	}
	i := Summarise(t.AccountID, month, txns)
	if err := s.repo.UpsertInsight(ctx, i); err != nil {
		return err
	}
	return s.repo.Audit(ctx, "insight.generated", t.AccountID, "worker", map[string]any{"month": i.Month})
}
func Categorise(description string, amount int64) string {
	d := strings.ToLower(description)
	rules := map[string][]string{"groceries": {"tesco", "sainsbury", "aldi", "lidl", "waitrose"}, "transport": {"tfl", "uber", "train", "rail"}, "housing": {"rent", "mortgage"}, "utilities": {"energy", "water", "broadband", "mobile"}, "dining": {"restaurant", "cafe", "deliveroo"}, "entertainment": {"netflix", "spotify", "cinema"}, "income": {"salary", "payroll"}}
	for cat, terms := range rules {
		for _, term := range terms {
			if strings.Contains(d, term) {
				return cat
			}
		}
	}
	if amount > 0 {
		return "income"
	}
	return "other"
}
func Summarise(accountID string, month time.Time, txns []domain.Transaction) domain.Insight {
	i := domain.Insight{ID: uuid.NewString(), AccountID: accountID, Month: month.Format("2006-01"), CategorySpend: map[string]int64{}, TransactionCount: len(txns), GeneratedAt: time.Now().UTC()}
	for _, t := range txns {
		cat := t.Category
		if cat == "" {
			cat = Categorise(t.Description, t.Amount)
		}
		if t.Amount >= 0 {
			i.TotalIncome += t.Amount
		} else {
			spent := -t.Amount
			i.TotalSpending += spent
			i.CategorySpend[cat] += spent
		}
		i.NetCashflow += t.Amount
	}
	return i
}
