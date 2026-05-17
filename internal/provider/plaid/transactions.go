package plaid

import (
	"context"
	"strconv"
	"time"

	plaid "github.com/plaid/plaid-go/v25/plaid"

	"github.com/kdubb1337/fin-cli/internal/provider"
)

func (c *Client) ListTransactions(ctx context.Context, accessToken string, opts provider.TxOptions) (provider.TxPage, error) {
	since := opts.Since
	if since.IsZero() {
		since = time.Now().AddDate(0, -1, 0)
	}
	until := opts.Until
	if until.IsZero() {
		until = time.Now()
	}

	limit := int32(opts.Limit)
	if limit <= 0 {
		limit = 25
	}

	req := plaid.NewTransactionsGetRequest(accessToken, since.Format("2006-01-02"), until.Format("2006-01-02"))
	o := plaid.NewTransactionsGetRequestOptions()
	o.SetCount(limit)
	if opts.AccountID != "" {
		o.SetAccountIds([]string{opts.AccountID})
	}
	offset := offsetFromCursor(opts.Cursor)
	if offset > 0 {
		o.SetOffset(int32(offset))
	}
	req.SetOptions(*o)

	resp, _, err := c.api.PlaidApi.TransactionsGet(ctx).TransactionsGetRequest(*req).Execute()
	if err != nil {
		return provider.TxPage{}, translateErr(err)
	}

	txs := resp.GetTransactions()
	out := make([]provider.Transaction, 0, len(txs))
	for _, t := range txs {
		d, _ := time.Parse("2006-01-02", t.GetDate())
		out = append(out, provider.Transaction{
			ID:           t.GetTransactionId(),
			AccountID:    t.GetAccountId(),
			Date:         d,
			Amount:       t.GetAmount(),
			Currency:     t.GetIsoCurrencyCode(),
			Name:         t.GetName(),
			MerchantName: t.GetMerchantName(),
			Pending:      t.GetPending(),
			Category:     t.GetCategory(),
		})
	}

	nextCursor := ""
	total := int(resp.GetTotalTransactions())
	consumed := int(limit) + offset
	if consumed < total {
		nextCursor = strconv.Itoa(consumed)
	}
	return provider.TxPage{Transactions: out, NextCursor: nextCursor}, nil
}

// SyncTransactions wraps Plaid /transactions/sync. Plaid returns up to ~500
// rows per call; the caller is expected to loop while HasMore is true and
// persist NextCursor after each successful page so a crash mid-loop resumes.
func (c *Client) SyncTransactions(ctx context.Context, accessToken, cursor string) (provider.TxSyncPage, error) {
	req := plaid.NewTransactionsSyncRequest(accessToken)
	if cursor != "" {
		req.SetCursor(cursor)
	}
	req.SetCount(500)

	resp, _, err := c.api.PlaidApi.TransactionsSync(ctx).TransactionsSyncRequest(*req).Execute()
	if err != nil {
		return provider.TxSyncPage{}, translateErr(err)
	}

	conv := func(in []plaid.Transaction) []provider.Transaction {
		out := make([]provider.Transaction, 0, len(in))
		for _, t := range in {
			d, _ := time.Parse("2006-01-02", t.GetDate())
			out = append(out, provider.Transaction{
				ID:           t.GetTransactionId(),
				AccountID:    t.GetAccountId(),
				Date:         d,
				Amount:       t.GetAmount(),
				Currency:     t.GetIsoCurrencyCode(),
				Name:         t.GetName(),
				MerchantName: t.GetMerchantName(),
				Pending:      t.GetPending(),
				Category:     t.GetCategory(),
			})
		}
		return out
	}
	removed := make([]string, 0, len(resp.GetRemoved()))
	for _, r := range resp.GetRemoved() {
		removed = append(removed, r.GetTransactionId())
	}
	return provider.TxSyncPage{
		Added:      conv(resp.GetAdded()),
		Modified:   conv(resp.GetModified()),
		Removed:    removed,
		NextCursor: resp.GetNextCursor(),
		HasMore:    resp.GetHasMore(),
	}, nil
}

func (c *Client) Health(ctx context.Context, accessToken string) error {
	req := plaid.NewItemGetRequest(accessToken)
	_, _, err := c.api.PlaidApi.ItemGet(ctx).ItemGetRequest(*req).Execute()
	return translateErr(err)
}

func offsetFromCursor(s string) int {
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// Compile-time assertion that *Client satisfies provider.Provider.
var _ provider.Provider = (*Client)(nil)
