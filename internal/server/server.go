package server

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	drynn "github.com/mdhender/drynn"
	"github.com/mdhender/drynn/internal/auth"
	"github.com/mdhender/drynn/internal/config"
	"github.com/mdhender/drynn/internal/email"
	"github.com/mdhender/drynn/internal/handler"
	hobomiddleware "github.com/mdhender/drynn/internal/middleware"
	"github.com/mdhender/drynn/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

type App struct {
	cfg  config.Config
	echo *echo.Echo
	db   *pgxpool.Pool
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	if err := db.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	mailgunCfg := email.MailgunConfig{
		APIKey:        cfg.Mailgun.APIKey,
		SendingDomain: cfg.Mailgun.SendingDomain,
		FromAddress:   cfg.Mailgun.FromAddress,
		FromName:      cfg.Mailgun.FromName,
	}

	userService := service.NewUserService(db)
	invitationService := service.NewInvitationService(db, mailgunCfg)
	passwordResetService := service.NewPasswordResetService(db, mailgunCfg)
	accessRequestService := service.NewAccessRequestService(mailgunCfg, cfg.AdminContactEmail)
	keyStore := auth.NewKeyStore(db)
	if err := keyStore.EnsureReady(ctx); err != nil {
		return nil, fmt.Errorf("jwt signing keys: %w", err)
	}

	renderer, err := newTemplateRenderer()
	if err != nil {
		return nil, err
	}

	jwtManager := auth.NewManager(
		keyStore,
		cfg.JWTAccessTTL,
		cfg.JWTRefreshTTL,
		cfg.CookieSecure,
	)

	e := echo.New()
	e.Renderer = renderer
	e.Use(skipPaths(hobomiddleware.RequestLogger(), "/healthz", "/readyz"))
	e.Use(middleware.Recover())
	e.Use(hobomiddleware.FetchMetadata())
	e.Static("/static", "web/static")

	siteFS := drynn.SiteFS()
	docsFS, _ := fs.Sub(siteFS, "docs")
	blogFS, _ := fs.Sub(siteFS, "blog")
	e.StaticFS("/docs", docsFS)
	e.StaticFS("/blog", blogFS)

	publicHandler := handler.NewPublicHandler()
	authHandler := handler.NewAuthHandler(
		userService,
		invitationService,
		passwordResetService,
		accessRequestService,
		jwtManager,
		cfg.RequestAccessEnabled && cfg.AdminContactEmail != "",
	)
	appHandler := handler.NewAppHandler(userService)
	adminHandler := handler.NewAdminHandler(userService, invitationService, passwordResetService)
	healthHandler := handler.NewHealthHandler(db)

	authRateLimiter := hobomiddleware.NewRateLimiter(hobomiddleware.DefaultAuthRate, hobomiddleware.DefaultAuthBurst)

	registerRoutes(e, publicHandler, authHandler, appHandler, adminHandler, healthHandler, jwtManager, userService, authRateLimiter)

	return &App{cfg: cfg, echo: e, db: db}, nil
}

func (a *App) Run(ctx context.Context) error {
	defer a.db.Close()

	startConfig := echo.StartConfig{
		Address:         a.cfg.AppAddr,
		HideBanner:      true,
		HidePort:        true,
		GracefulTimeout: 10 * time.Second,
	}

	return startConfig.Start(ctx, a.echo)
}

func registerRoutes(
	e *echo.Echo,
	publicHandler *handler.PublicHandler,
	authHandler *handler.AuthHandler,
	appHandler *handler.AppHandler,
	adminHandler *handler.AdminHandler,
	healthHandler *handler.HealthHandler,
	jwtManager *auth.Manager,
	userService *service.UserService,
	authRateLimiter *hobomiddleware.RateLimiter,
) {
	authRL := authRateLimiter.Middleware()

	e.GET("/healthz", healthHandler.Healthz)
	e.GET("/readyz", healthHandler.Readyz)

	e.GET("/", publicHandler.ShowHome)
	e.GET("/register", authHandler.ShowRegister)
	e.POST("/register", authHandler.Register)
	e.GET("/signin", authHandler.ShowSignIn)
	e.POST("/signin", authHandler.SignIn, authRL)
	e.GET("/forgot-password", authHandler.ShowForgotPassword)
	e.POST("/forgot-password", authHandler.ForgotPassword, authRL)
	e.GET("/request-access", authHandler.ShowRequestAccess)
	e.POST("/request-access", authHandler.RequestAccess, authRL)
	e.GET("/reset-password", authHandler.ShowResetPassword)
	e.POST("/reset-password", authHandler.ResetPassword, authRL)
	e.POST("/logout", authHandler.SignOut)
	e.POST("/refresh", authHandler.Refresh)

	appGroup := e.Group("/app")
	appGroup.Use(auth.RequireAuth(jwtManager))
	appGroup.Use(loadCurrentViewer(userService))

	appGroup.GET("", func(c *echo.Context) error {
		return c.Redirect(http.StatusSeeOther, "/app/profile")
	})
	appGroup.GET("/profile", appHandler.ShowProfile)
	appGroup.POST("/profile", appHandler.UpdateProfile)

	adminGroup := appGroup.Group("/admin")
	adminGroup.Use(requireRole(service.RoleAdmin))
	adminGroup.GET("/users", adminHandler.ListUsers)
	adminGroup.GET("/users/new", adminHandler.ShowCreateUser)
	adminGroup.POST("/users", adminHandler.CreateUser)
	adminGroup.GET("/users/:id/edit", adminHandler.ShowEditUser)
	adminGroup.POST("/users/:id", adminHandler.UpdateUser)
	adminGroup.POST("/users/:id/reset-password", adminHandler.SendPasswordReset)
	adminGroup.POST("/users/:id/delete", adminHandler.DeleteUser)

	adminGroup.GET("/invitations", adminHandler.ListInvitations)
	adminGroup.GET("/invitations/new", adminHandler.ShowInviteForm)
	adminGroup.POST("/invitations", adminHandler.SendInvitation)
	adminGroup.POST("/invitations/:id/resend", adminHandler.ResendInvitation)
	adminGroup.POST("/invitations/:id/archive", adminHandler.ArchiveInvitation)
}

func loadCurrentViewer(userService *service.UserService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			claims, ok := auth.ClaimsFromContext(c)
			if !ok {
				return c.Redirect(http.StatusSeeOther, "/signin")
			}

			userID, err := uuid.Parse(claims.Subject)
			if err != nil {
				return c.Redirect(http.StatusSeeOther, "/signin")
			}

			user, err := userService.GetUser(c.Request().Context(), userID)
			if err != nil || !user.IsActive {
				return c.Redirect(http.StatusSeeOther, "/signin")
			}

			auth.SetViewer(c, &auth.Viewer{
				ID:       user.ID,
				Handle:   user.Handle,
				Email:    user.Email,
				IsActive: user.IsActive,
				Roles:    user.Roles,
			})

			return next(c)
		}
	}
}

func skipPaths(mw echo.MiddlewareFunc, paths ...string) echo.MiddlewareFunc {
	skip := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		skip[p] = struct{}{}
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		wrapped := mw(next)
		return func(c *echo.Context) error {
			if _, ok := skip[c.Request().URL.Path]; ok {
				return next(c)
			}
			return wrapped(c)
		}
	}
}

func requireRole(role string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			viewer, ok := auth.CurrentViewer(c)
			if !ok || !viewer.HasRole(role) {
				return c.String(http.StatusForbidden, "forbidden")
			}

			return next(c)
		}
	}
}
