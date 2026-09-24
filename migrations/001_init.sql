CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE TABLE IF NOT EXISTS consents (
  id UUID PRIMARY KEY, customer_ref TEXT NOT NULL, permissions TEXT[] NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('active','revoked','expired')),
  granted_at TIMESTAMPTZ NOT NULL, expires_at TIMESTAMPTZ NOT NULL, revoked_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS consents_expiry_idx ON consents(status, expires_at);
CREATE TABLE IF NOT EXISTS accounts (
  id UUID PRIMARY KEY, consent_id UUID NOT NULL REFERENCES consents(id), external_id TEXT UNIQUE NOT NULL,
  display_name TEXT NOT NULL, currency CHAR(3) NOT NULL, account_type TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE IF NOT EXISTS transactions (
  id UUID PRIMARY KEY, account_id UUID NOT NULL REFERENCES accounts(id), external_id TEXT UNIQUE NOT NULL,
  amount_minor BIGINT NOT NULL, currency CHAR(3) NOT NULL, description TEXT NOT NULL,
  booked_at TIMESTAMPTZ NOT NULL, category TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS transactions_account_booked_idx ON transactions(account_id, booked_at);
CREATE TABLE IF NOT EXISTS monthly_insights (
  id UUID PRIMARY KEY, account_id UUID NOT NULL REFERENCES accounts(id), month DATE NOT NULL,
  total_income_minor BIGINT NOT NULL, total_spending_minor BIGINT NOT NULL, net_cashflow_minor BIGINT NOT NULL,
  category_spend JSONB NOT NULL, transaction_count INTEGER NOT NULL, generated_at TIMESTAMPTZ NOT NULL,
  UNIQUE(account_id, month)
);
CREATE TABLE IF NOT EXISTS audit_log (
  id BIGSERIAL PRIMARY KEY, occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(), action TEXT NOT NULL,
  resource_id TEXT NOT NULL, actor TEXT NOT NULL, details JSONB NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS audit_log_resource_idx ON audit_log(resource_id, occurred_at DESC);
