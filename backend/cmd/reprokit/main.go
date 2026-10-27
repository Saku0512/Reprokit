package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jphacks/sd_2601/internal/lab"
	"github.com/jphacks/sd_2601/internal/runtime"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "Reprokit Runner:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if len(os.Args) != 1 || os.Getenv("REPROKIT_BOOTSTRAP_FD") != "3" {
		return errors.New("start the local Web application with npm --prefix web start; Lab operation CLI is not available")
	}
	bootstrap := os.NewFile(3, "web-bootstrap")
	if bootstrap == nil {
		return errors.New("a private Web bootstrap pipe is required")
	}
	defer bootstrap.Close()
	repository := os.Getenv("REPROKIT_REPOSITORY_ROOT")
	app, err := newApplication(ctx, configuration{repository: repository, state: os.Getenv("REPROKIT_STATE_DIR"), organizationID: os.Getenv("REPROKIT_ORGANIZATION_ID"), administratorID: os.Getenv("REPROKIT_ADMINISTRATOR_ID"), administratorSubject: os.Getenv("REPROKIT_ADMIN_SUBJECT"), learnerID: os.Getenv("REPROKIT_LEARNER_ID"), oidcIssuer: os.Getenv("REPROKIT_OIDC_ISSUER"), oidcClientID: os.Getenv("REPROKIT_OIDC_CLIENT_ID"), oidcClientSecret: os.Getenv("REPROKIT_OIDC_CLIENT_SECRET"), oidcRedirectURL: os.Getenv("REPROKIT_OIDC_REDIRECT_URL")}, runtime.NewDockerComposeProvider(), lab.NewLocalMaterializer(repository).Materialize)
	if err != nil {
		return err
	}
	defer app.Close()
	server := &http.Server{Handler: app.server.Handler, BaseContext: func(net.Listener) context.Context { return ctx }, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, WriteTimeout: 2 * time.Minute, MaxHeaderBytes: 16 << 10}
	done := make(chan error, 1)
	go func() { done <- server.Serve(app.server.Listener) }()
	// トークンは端末出力やファイルへ出さず、親Webシェルだけが読む専用パイプへ渡す。
	if err := json.NewEncoder(bootstrap).Encode(map[string]any{"origin": "http://" + app.server.Listener.Addr().String(), "token": app.server.SessionToken, "managementToken": app.managementToken, "organizationId": app.organizationID, "administratorId": app.administratorID, "oidcEnabled": app.oidcEnabled}); err != nil {
		_ = server.Close()
		return errors.New("could not initialize the parent Web shell")
	}
	// 親Webシェルが消えた場合も、認証済みRunnerだけが孤立して残らないよう停止する。
	go func() { _, _ = io.Copy(io.Discard, bootstrap); cancel() }()
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
