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

// New builds a client using the config's default env (cfg.Plaid.Env). Use this
// only for operations not bound to a specific item (e.g. starting a new link).
// For per-item operations (tx, accounts, item health), use NewForEnv with the
// item's stored env — a single config can hold items from both sandbox and
// production and they require separate API endpoints.
func New(cfg *config.Config) (*Client, error) {
	return NewForEnv(cfg, cfg.Plaid.Env)
}

func NewForEnv(cfg *config.Config, env string) (*Client, error) {
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
	switch env {
	case "sandbox":
		pc.UseEnvironment(plaid.Sandbox)
	case "production":
		pc.UseEnvironment(plaid.Production)
	default:
		return nil, fmt.Errorf("unknown plaid env %q", env)
	}
	return &Client{api: plaid.NewAPIClient(pc), env: env}, nil
}

func (c *Client) Name() string { return "plaid" }
