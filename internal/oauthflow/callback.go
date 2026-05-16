package oauthflow

import (
	"context"
	"fmt"
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
	srv    *http.Server
	addr   string
	done   chan struct{}
	once   sync.Once
	result CallbackResult
	err    error
	ctx    context.Context
}

func Start(ctx context.Context, port int) (*Listener, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	l := &Listener{addr: ln.Addr().String(), done: make(chan struct{}), ctx: ctx}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", l.handle)
	l.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	go func() { _ = l.srv.Serve(ln) }()
	return l, nil
}

func (l *Listener) URL() string { return "http://" + l.addr }

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
	fmt.Fprint(w, `<html><body style="font-family:sans-serif;padding:2rem"><h1>fin: linked.</h1><p>You can close this tab.</p></body></html>`)
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
