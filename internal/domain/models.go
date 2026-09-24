package domain

import "time"

type ConsentStatus string

const (
	ConsentActive  ConsentStatus = "active"
	ConsentRevoked ConsentStatus = "revoked"
	ConsentExpired ConsentStatus = "expired"
)

type Consent struct {
	ID          string        `json:"id"`
	CustomerRef string        `json:"customer_ref"`
	Permissions []string      `json:"permissions"`
	Status      ConsentStatus `json:"status"`
	GrantedAt   time.Time     `json:"granted_at"`
	ExpiresAt   time.Time     `json:"expires_at"`
	RevokedAt   *time.Time    `json:"revoked_at,omitempty"`
}

func (c Consent) IsUsable(now time.Time) bool {
	return c.Status == ConsentActive && now.Before(c.ExpiresAt)
}

type Account struct {
	ID          string    `json:"id"`
	ConsentID   string    `json:"consent_id"`
	ExternalID  string    `json:"external_id"`
	DisplayName string    `json:"display_name"`
	Currency    string    `json:"currency"`
	Type        string    `json:"type"`
	CreatedAt   time.Time `json:"created_at"`
}

type Transaction struct {
	ID          string    `json:"id"`
	AccountID   string    `json:"account_id"`
	ExternalID  string    `json:"external_id"`
	Amount      int64     `json:"amount_minor"`
	Currency    string    `json:"currency"`
	Description string    `json:"description"`
	BookedAt    time.Time `json:"booked_at"`
	Category    string    `json:"category,omitempty"`
}

type Insight struct {
	ID               string           `json:"id"`
	AccountID        string           `json:"account_id"`
	Month            string           `json:"month"`
	TotalIncome      int64            `json:"total_income_minor"`
	TotalSpending    int64            `json:"total_spending_minor"`
	NetCashflow      int64            `json:"net_cashflow_minor"`
	CategorySpend    map[string]int64 `json:"category_spend_minor"`
	TransactionCount int              `json:"transaction_count"`
	GeneratedAt      time.Time        `json:"generated_at"`
}

type IngestRequest struct {
	Consent      Consent       `json:"consent"`
	Accounts     []Account     `json:"accounts"`
	Transactions []Transaction `json:"transactions"`
}
