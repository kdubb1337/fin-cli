package plaid

import (
	"fmt"

	plaid "github.com/plaid/plaid-go/v25/plaid"

	"github.com/kdubb1337/fin-cli/internal/config"
)

type Client struct {
	api *plaid.APIClient
	env string
}

func New(cfg *config.Config) (*Client, error) {
	if cfg.Plaid.ClientID == "" {
		return nil, fmt.Errorf("plaid not configured; run `fin auth setup`")
	}
	secret, err := config.GetSecret("plaid:client_secret")
	if err != nil {
		return nil, err
	}

	pc := plaid.NewConfiguration()
	pc.AddDefaultHeader("PLAID-CLIENT-ID", cfg.Plaid.ClientID)
	pc.AddDefaultHeader("PLAID-SECRET", secret)
	switch cfg.Plaid.Env {
	case "sandbox":
		pc.UseEnvironment(plaid.Sandbox)
	case "production":
		pc.UseEnvironment(plaid.Production)
	default:
		return nil, fmt.Errorf("unknown plaid env %q", cfg.Plaid.Env)
	}
	return &Client{api: plaid.NewAPIClient(pc), env: cfg.Plaid.Env}, nil
}

func (c *Client) Name() string { return "plaid" }
