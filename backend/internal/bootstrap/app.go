package bootstrap

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	httpadapter "mini-messenger/internal/adapter/http"
	"mini-messenger/internal/adapter/observability"
	"mini-messenger/internal/adapter/postgres"
	"mini-messenger/internal/adapter/realtime"
	"mini-messenger/internal/adapter/security"
	"mini-messenger/internal/service/auth"
	"mini-messenger/internal/service/chat"
	"mini-messenger/internal/service/user"
	"mini-messenger/migrations"
	"net/http"
	"time"
)

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

type errorReporter struct{}

func (errorReporter) Report(ctx context.Context, err error) {
	observability.Report(ctx, "realtime publication", err)
}
func Run(ctx context.Context, c Config) error {
	if err := validateTransport(c); err != nil {
		return err
	}
	startup, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	db, err := postgres.Open(startup, c.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	tokens, err := security.NewJWT(c.JWTKey, "mini-messenger", "mini-messenger-web")
	if err != nil {
		return err
	}
	passwords, err := security.NewPassword()
	if err != nil {
		return err
	}
	users := postgres.NewUserRepository(db)
	sessions := postgres.NewSessionRepository(db)
	messages := postgres.NewMessageRepository(db)
	hub := realtime.NewHub()
	defer hub.Close()
	authenticate := auth.NewAuthenticate(sessions, tokens, systemClock{})
	login := auth.NewLogin(users, sessions, tokens, passwords, systemClock{}, security.IDs{}, security.MaxTokenLifetime)
	logout := auth.NewLogout(sessions, hub)
	refresh := auth.NewRefresh(sessions, security.RefreshSecrets{}, tokens, hub, systemClock{})
	currentUser := user.NewGetUser(users)
	listUsers := user.NewListUsers(users)
	sendMessage := chat.NewSendMessage(users, messages, hub, errorReporter{}, systemClock{}, security.IDs{})
	listMessages := chat.NewListMessages(users, messages)
	readStatus := chat.NewReadStatus(postgres.NewReadRepository(db), hub, errorReporter{})
	listConversations := chat.NewListConversations(postgres.NewConversationRepository(db))

	router, err := httpadapter.NewRouter(
		httpadapter.Config{Origin: c.Origin, SecureCookie: c.SecureCookie},
		login,
		authenticate,
		logout,
		currentUser,
		listUsers,
		sendMessage,
		listMessages,
		readStatus,
		listConversations,
		refresh,
	)
	if err != nil {
		return err
	}
	if err = migrations.Apply(startup, db); err != nil {
		return err
	}
	if c.SeedDemo {
		if err = seedDemo(startup, db, passwords, c.DemoPassword); err != nil {
			return err
		}
	}
	mux := http.NewServeMux()
	mux.Handle("/api/v1/ws", realtime.NewHandler(hub, authenticate, c.Origin))
	mux.Handle("/api/", router)
	mux.Handle("/", frontendHandler(c.FrontendDir))
	server := &http.Server{
		Addr:              c.Address,
		Handler:           securityHeaders(observability.Middleware(mux), c.Origin),
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS12},
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	result := make(chan error, 1)
	go func() {
		if c.TLSCertFile != "" {
			result <- server.ListenAndServeTLS(c.TLSCertFile, c.TLSKeyFile)
		} else {
			result <- server.ListenAndServe()
		}
	}()
	slog.Info("HTTP server starting", "address", c.Address)
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		hub.Close()
		shutdown, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if err = server.Shutdown(shutdown); err != nil {
			server.Close()
			return err
		}
		return nil
	}
}
