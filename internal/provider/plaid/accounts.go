package plaid

import (
	"context"

	plaid "github.com/plaid/plaid-go/v25/plaid"

	finerr "github.com/kdubb1337/fin-cli/internal/errors"
	"github.com/kdubb1337/fin-cli/internal/provider"
)

func (c *Client) ListAccounts(ctx context.Context, accessToken string) ([]provider.Account, error) {
	req := plaid.NewAccountsGetRequest(accessToken)
	resp, _, err := c.api.PlaidApi.AccountsGet(ctx).AccountsGetRequest(*req).Execute()
	if err != nil {
		return nil, translateErr(err)
	}

	out := make([]provider.Account, 0, len(resp.GetAccounts()))
	for _, a := range resp.GetAccounts() {
		bal := a.GetBalances()
		cur := bal.GetIsoCurrencyCode()
		if cur == "" {
			cur = bal.GetUnofficialCurrencyCode()
		}
		acct := provider.Account{
			ID:           a.GetAccountId(),
			Name:         a.GetName(),
			OfficialName: a.GetOfficialName(),
			Mask:         a.GetMask(),
			Type:         string(a.GetType()),
			Subtype:      string(a.GetSubtype()),
			Currency:     cur,
			Balance:      bal.GetCurrent(),
			// InstitutionName + ItemID filled by caller from config.
		}
		if av, ok := bal.GetAvailableOk(); ok && av != nil {
			v := *av
			acct.AvailableBalance = &v
		}
		out = append(out, acct)
	}
	return out, nil
}

func (c *Client) GetAccount(ctx context.Context, accessToken, accountID string) (provider.Account, error) {
	all, err := c.ListAccounts(ctx, accessToken)
	if err != nil {
		return provider.Account{}, err
	}
	for _, a := range all {
		if a.ID == accountID {
			return a, nil
		}
	}
	return provider.Account{}, finerr.New(finerr.CodeNotFound, "account %q not found", accountID)
}
