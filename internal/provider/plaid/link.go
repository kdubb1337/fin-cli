package plaid

import (
	"context"

	plaid "github.com/plaid/plaid-go/v25/plaid"

	"github.com/kdubb1337/fin-cli/internal/provider"
)

func (c *Client) StartLink(ctx context.Context, redirectURI string) (provider.LinkSession, error) {
	user := plaid.LinkTokenCreateRequestUser{ClientUserId: "fin-cli-user"}
	req := plaid.NewLinkTokenCreateRequest(
		"fin",
		"en",
		[]plaid.CountryCode{plaid.COUNTRYCODE_CA, plaid.COUNTRYCODE_US},
		user,
	)
	req.SetProducts([]plaid.Products{plaid.PRODUCTS_TRANSACTIONS})
	if redirectURI != "" {
		req.SetRedirectUri(redirectURI)
	}

	resp, _, err := c.api.PlaidApi.LinkTokenCreate(ctx).LinkTokenCreateRequest(*req).Execute()
	if err != nil {
		return provider.LinkSession{}, translateErr(err)
	}
	return provider.LinkSession{Token: resp.GetLinkToken()}, nil
}

func (c *Client) ExchangePublicToken(ctx context.Context, publicToken string) (provider.ExchangeResult, error) {
	req := plaid.NewItemPublicTokenExchangeRequest(publicToken)
	resp, _, err := c.api.PlaidApi.ItemPublicTokenExchange(ctx).ItemPublicTokenExchangeRequest(*req).Execute()
	if err != nil {
		return provider.ExchangeResult{}, translateErr(err)
	}

	// Resolve institution name via /item/get + /institutions/get_by_id
	itemReq := plaid.NewItemGetRequest(resp.GetAccessToken())
	itemResp, _, err := c.api.PlaidApi.ItemGet(ctx).ItemGetRequest(*itemReq).Execute()
	if err != nil {
		return provider.ExchangeResult{}, translateErr(err)
	}
	item := itemResp.GetItem()
	instID := item.GetInstitutionId()

	var instName string
	if instID != "" {
		instReq := plaid.NewInstitutionsGetByIdRequest(instID, []plaid.CountryCode{plaid.COUNTRYCODE_CA, plaid.COUNTRYCODE_US})
		instResp, _, ierr := c.api.PlaidApi.InstitutionsGetById(ctx).InstitutionsGetByIdRequest(*instReq).Execute()
		if ierr == nil {
			instName = instResp.GetInstitution().Name
		}
	}

	return provider.ExchangeResult{
		AccessToken:     resp.GetAccessToken(),
		ItemID:          resp.GetItemId(),
		InstitutionID:   instID,
		InstitutionName: instName,
	}, nil
}
