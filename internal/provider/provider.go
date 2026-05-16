package provider

import (
	"context"
	"time"
)

type Account struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	OfficialName     string   `json:"official_name,omitempty"`
	Mask             string   `json:"mask,omitempty"`
	Type             string   `json:"type"`
	Subtype          string   `json:"subtype,omitempty"`
	Currency         string   `json:"currency"`
	Balance          float64  `json:"balance"`
	AvailableBalance *float64 `json:"available_balance,omitempty"`
	InstitutionName  string   `json:"institution_name"`
	ItemID           string   `json:"item_id"`
}

type Transaction struct {
	ID           string    `json:"id"`
	AccountID    string    `json:"account_id"`
	Date         time.Time `json:"date"`
	Amount       float64   `json:"amount"`
	Currency     string    `json:"currency"`
	Name         string    `json:"name"`
	MerchantName string    `json:"merchant_name,omitempty"`
	Pending      bool      `json:"pending"`
	Category     []string  `json:"category,omitempty"`
}

type LinkSession struct {
	URL       string
	Token     string
	SessionID string
}

type ExchangeResult struct {
	AccessToken     string
	ItemID          string
	InstitutionID   string
	InstitutionName string
}

type TxOptions struct {
	Since     time.Time
	Until     time.Time
	AccountID string
	Limit     int
	Cursor    string
}

type TxPage struct {
	Transactions []Transaction
	NextCursor   string
}

type Provider interface {
	Name() string
	StartLink(ctx context.Context, redirectURI string) (LinkSession, error)
	ExchangePublicToken(ctx context.Context, publicToken string) (ExchangeResult, error)
	ListAccounts(ctx context.Context, accessToken string) ([]Account, error)
	GetAccount(ctx context.Context, accessToken, accountID string) (Account, error)
	ListTransactions(ctx context.Context, accessToken string, opts TxOptions) (TxPage, error)
	Health(ctx context.Context, accessToken string) error
}
