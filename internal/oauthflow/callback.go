package oauthflow

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"sync"
	"time"
)

type CallbackResult struct {
	PublicToken     string
	InstitutionID   string
	InstitutionName string
}

type Listener struct {
	srv       *http.Server
	port      int
	linkToken string
	done      chan struct{}
	once      sync.Once
	result    CallbackResult
	err       error
	ctx       context.Context
}

// Start binds a loopback listener on the requested port (0 = ephemeral) and
// serves two routes:
//
//	GET /          — an HTML shell that loads Plaid's link-initialize.js,
//	                 invokes Plaid.create({token: linkToken, ...}).open(),
//	                 and on onSuccess navigates the browser to /callback
//	                 with the public_token and institution metadata.
//	GET /callback  — captures public_token / institution_* and unblocks Wait().
//
// linkToken may be empty when the caller intends to drive the flow externally
// (e.g. tests posting directly to /callback).
func Start(ctx context.Context, port int, linkToken string) (*Listener, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	l := &Listener{
		port:      ln.Addr().(*net.TCPAddr).Port,
		linkToken: linkToken,
		done:      make(chan struct{}),
		ctx:       ctx,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", l.handleRoot)
	mux.HandleFunc("/callback", l.handle)
	l.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	go func() { _ = l.srv.Serve(ln) }()
	return l, nil
}

// URL returns the loopback URL using `localhost` rather than the IP literal.
// Plaid's allowed-redirect-URIs list is exact-match and only `localhost` is
// accepted; the JS SDK page itself is happy at either, but we standardise.
func (l *Listener) URL() string { return fmt.Sprintf("http://localhost:%d", l.port) }

var linkPage = template.Must(template.New("link").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>fin: linking…</title>
<style>body{font-family:system-ui,sans-serif;padding:2rem;color:#333}</style>
</head><body>
<h1>fin</h1><p id="msg">Opening Plaid Link…</p>
<script src="https://cdn.plaid.com/link/v2/stable/link-initialize.js"></script>
<script>
const handler = Plaid.create({
  token: {{.Token}},
  onSuccess: (public_token, metadata) => {
    const q = new URLSearchParams({
      public_token,
      institution_id:   (metadata && metadata.institution && metadata.institution.institution_id) || "",
      institution_name: (metadata && metadata.institution && metadata.institution.name) || "",
    });
    window.location = "/callback?" + q.toString();
  },
  onExit: (err) => {
    document.getElementById("msg").textContent =
      err ? ("Link exited with error: " + (err.error_message || err.error_code || "unknown")) :
            "Link closed. You can return to your terminal.";
  },
});
handler.open();
</script>
</body></html>`))

func (l *Listener) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if l.linkToken == "" {
		http.Error(w, "no link token configured", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tokJSON, _ := json.Marshal(l.linkToken)
	_ = linkPage.Execute(w, map[string]any{"Token": template.JS(tokJSON)})
}

func (l *Listener) handle(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	pt := q.Get("public_token")
	if pt == "" {
		http.Error(w, "missing public_token", http.StatusBadRequest)
		l.finish(CallbackResult{}, fmt.Errorf("missing public_token in callback"))
		return
	}
	res := CallbackResult{
		PublicToken:     pt,
		InstitutionID:   q.Get("institution_id"),
		InstitutionName: q.Get("institution_name"),
	}
	w.Header().Set("Content-Type", "text/html")
	// Best-effort auto-close: modern browsers only honour window.close() on
	// tabs opened by script, so this works in some contexts (e.g. fresh tabs
	// from `open`) and silently no-ops elsewhere — the countdown still gives
	// the user a clear "you can close this" cue either way.
	fmt.Fprint(w, `<!doctype html><html><body style="font-family:system-ui,sans-serif;padding:2rem;color:#333">
<h1>fin: linked.</h1>
<p>Closing this tab in <span id="c">5</span>s. <a href="#" id="close-now">Close now</a></p>
<script>
let n = 5;
const span = document.getElementById("c");
const tick = () => {
  n -= 1;
  if (n <= 0) { window.close(); return; }
  span.textContent = n;
  setTimeout(tick, 1000);
};
setTimeout(tick, 1000);
document.getElementById("close-now").addEventListener("click", (e) => { e.preventDefault(); window.close(); });
</script></body></html>`)
	l.finish(res, nil)
}

func (l *Listener) finish(res CallbackResult, err error) {
	l.once.Do(func() {
		l.result = res
		l.err = err
		close(l.done)
	})
}

func (l *Listener) Wait() (CallbackResult, error) {
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = l.srv.Shutdown(shutdownCtx)
	}()
	select {
	case <-l.done:
		return l.result, l.err
	case <-l.ctx.Done():
		return CallbackResult{}, fmt.Errorf("oauth callback timeout: %w", l.ctx.Err())
	}
}
