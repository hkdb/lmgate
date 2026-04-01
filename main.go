package main

import (
	"crypto/rand"
	"crypto/tls"
	"context"
	"database/sql"
	"embed"
	"encoding/base64"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/utils"

	"github.com/hkdb/lmgate/internal/admin"
	"github.com/hkdb/lmgate/internal/auth"
	"github.com/hkdb/lmgate/internal/config"
	"github.com/hkdb/lmgate/internal/database"
	"github.com/hkdb/lmgate/internal/metrics"
	"github.com/hkdb/lmgate/internal/middleware"
	"github.com/hkdb/lmgate/internal/models"
	"github.com/hkdb/lmgate/internal/proxy"
	tlsutil "github.com/hkdb/lmgate/internal/tls"
)

// Version of LM Gate.
var Version = "v0.2.0"

//go:embed all:web/build
var webAssets embed.FS

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	tlsDisabled := flag.Bool("tls-disabled", false, "disable TLS (plain HTTP)")
	devMode := flag.Bool("dev", false, "bind to localhost instead of all interfaces")
	showVersion := flag.Bool("v", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("LM Gate " + Version)
		return
	}

	printBanner()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	if *tlsDisabled {
		cfg.Server.TLS.Disabled = true
	}

	// Determine default port if LMGATE_LISTEN is not set
	if cfg.Server.Listen == "" && *devMode && cfg.Server.TLS.Disabled {
		cfg.Server.Listen = "8080"
	}
	if cfg.Server.Listen == "" && *devMode {
		cfg.Server.Listen = "8443"
	}
	if cfg.Server.Listen == "" && cfg.Server.TLS.Disabled {
		cfg.Server.Listen = "80"
	}
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = "443"
	}

	// Normalize listen address: accept bare port (e.g. "8080") or ":port"
	if !strings.Contains(cfg.Server.Listen, ":") {
		host := "0.0.0.0"
		if *devMode {
			host = "localhost"
		}
		cfg.Server.Listen = host + ":" + cfg.Server.Listen
	}

	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	bootstrapAdminUser(db)

	// Load persisted settings and apply to config
	generalDefaults := models.GeneralSettings{
		RateLimitEnabled:         cfg.RateLimit.Enabled,
		RateLimitDefaultRPM:      cfg.RateLimit.DefaultRPM,
		APILogEnabled:            cfg.Logging.APILogEnabled,
		APILogRetentionDays:      cfg.Logging.APILogRetentionDays,
		AdminLogEnabled:          cfg.Logging.AdminLogEnabled,
		AdminLogRetentionDays:    cfg.Logging.AdminLogRetentionDays,
		SecurityLogEnabled:       cfg.Logging.SecurityLogEnabled,
		SecurityLogRetentionDays: cfg.Logging.SecurityLogRetentionDays,
		AuditFlushInterval:       cfg.Logging.AuditFlushInterval,
		MaxFailedLogins:          cfg.Security.MaxFailedLogins,
		PasswordMinLength:        cfg.Security.PasswordMinLength,
		PasswordRequireSpecial:   cfg.Security.PasswordRequireSpecial,
		PasswordRequireNumber:    cfg.Security.PasswordRequireNumber,
		UserCacheTTL:             cfg.Security.UserCacheTTL,
		Enforce2FA:               cfg.Security.Enforce2FA,
		PasswordExpiryDays:       cfg.Security.PasswordExpiryDays,
		AdminAllowedNetworks:     cfg.Security.AdminAllowedNetworks,
		GatewayAllowedNetworks:   cfg.Security.GatewayAllowedNetworks,
	}
	savedSettings, err := models.GetGeneralSettings(db, generalDefaults)
	if err != nil {
		log.Printf("warning: failed to load saved settings: %v", err)
	}
	cfg.RateLimit.Enabled = savedSettings.RateLimitEnabled
	cfg.RateLimit.DefaultRPM = savedSettings.RateLimitDefaultRPM
	cfg.Logging.APILogEnabled = savedSettings.APILogEnabled
	cfg.Logging.APILogRetentionDays = savedSettings.APILogRetentionDays
	cfg.Logging.AdminLogEnabled = savedSettings.AdminLogEnabled
	cfg.Logging.AdminLogRetentionDays = savedSettings.AdminLogRetentionDays
	cfg.Logging.SecurityLogEnabled = savedSettings.SecurityLogEnabled
	cfg.Logging.SecurityLogRetentionDays = savedSettings.SecurityLogRetentionDays
	cfg.Logging.AuditFlushInterval = savedSettings.AuditFlushInterval
	cfg.Security.MaxFailedLogins = savedSettings.MaxFailedLogins
	cfg.Security.PasswordMinLength = savedSettings.PasswordMinLength
	cfg.Security.PasswordRequireSpecial = savedSettings.PasswordRequireSpecial
	cfg.Security.PasswordRequireNumber = savedSettings.PasswordRequireNumber
	cfg.Security.UserCacheTTL = savedSettings.UserCacheTTL
	cfg.Security.Enforce2FA = savedSettings.Enforce2FA
	cfg.Security.PasswordExpiryDays = savedSettings.PasswordExpiryDays
	cfg.Security.AdminAllowedNetworks = savedSettings.AdminAllowedNetworks
	cfg.Security.GatewayAllowedNetworks = savedSettings.GatewayAllowedNetworks
	middleware.ParseGatewayACL(cfg.Security.GatewayAllowedNetworks)
	auth.SetUserCacheTTL(time.Duration(savedSettings.UserCacheTTL) * time.Second)

	go sendTelemetry(cfg, db)

	collector := metrics.NewCollector(db, cfg.Metrics.FlushInterval)
	defer collector.Stop()

	middleware.StartAuditWorker(db, time.Duration(cfg.Logging.AuditFlushInterval)*time.Second)

	proxyHandler := proxy.New(cfg.Upstream.URL, cfg.Upstream.Timeout, collector, cfg.Security.ResponseBodyLimit*1024*1024, db)

	app := fiber.New(fiber.Config{
		ReadTimeout:    cfg.Server.ReadTimeout,
		WriteTimeout:   cfg.Server.WriteTimeout,
		ReadBufferSize: 8192,
		BodyLimit:      cfg.Security.RequestBodyLimit * 1024 * 1024,
		AppName:        "LM Gate",
		// Disable startup message for clean logs
		DisableStartupMessage: true,
	})

	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(securityHeaders(cfg))

	// Only enable CORS if origins are explicitly configured
	corsOrigins := cfg.Server.AllowedOrigins
	app.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-CSRF-Token, X-TwoFA-Token",
		AllowCredentials: corsOrigins != "" && corsOrigins != "*",
	}))

	// Initialize WebAuthn if configured
	if err := auth.InitWebAuthn(cfg); err != nil {
		log.Printf("warning: WebAuthn initialization failed: %v", err)
	}

	// Admin network restriction (before admin routes)
	app.Use("/admin", middleware.AdminNetworkRestrict(cfg))

	// Admin API logging (path-scoped, never touches proxy chain)
	app.Use("/admin/api", middleware.AdminLog(cfg))

	// CSRF protection for admin API (browser sessions only)
	app.Use("/admin/api", csrf.New(csrf.Config{
		KeyLookup:      "header:X-CSRF-Token",
		CookieName:     "lmgate_csrf",
		CookiePath:     "/admin",
		CookieSecure:   !cfg.Server.TLS.Disabled,
		CookieHTTPOnly: false, // JS must read the token
		CookieSameSite: "Lax",
		Expiration:     1 * time.Hour,
		KeyGenerator:   utils.UUID,
		Next: func(c *fiber.Ctx) bool {
			// Skip CSRF for API-key-authenticated requests (non-browser clients)
			if c.Get("Authorization") != "" {
				return true
			}
			// Skip CSRF for SSE stream endpoints (GET-only, read-only)
			return strings.HasSuffix(c.Path(), "/stream")
		},
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "CSRF token missing or invalid",
			})
		},
	}))

	// Admin API + dashboard
	adm := admin.New(db, cfg, collector)
	adm.Version = Version
	adm.SecurityLogger = middleware.NewSecurityLogger(cfg)
	middleware.OnFlush = func() {
		adm.Notifier.Notify()
		adm.MetricsNotifier.Notify()
	}
	metrics.OnFlush = adm.MetricsNotifier.Notify
	adm.RegisterRoutes(app, cfg.Auth.JWTSecret)
	adm.LoadOIDCProviders()

	// Serve embedded SvelteKit build
	webFS, err := fs.Sub(webAssets, "web/build")
	if err != nil {
		log.Printf("warning: embedded web assets not found (build web/ first): %v", err)
	}
	if err == nil {
		app.Use("/admin", filesystem.New(filesystem.Config{
			Root:         http.FS(webFS),
			Index:        "index.html",
			NotFoundFile: "index.html", // SPA fallback
		}))
	}

	// Proxied routes: everything not under /admin/
	app.Use(
		middleware.GatewayNetworkRestrict(),
		auth.Middleware(db, cfg.Auth.JWTSecret, middleware.NewSecurityLogger(cfg)),
		middleware.RateLimit(cfg, middleware.NewRateLimitLogger(cfg)),
		middleware.ModelACL(db),
		middleware.Audit(collector, cfg),
	)
	app.All("/*", proxyHandler.Handle)

	// Periodic audit log pruning (daily)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			adm.PruneAuditLogs()
		}
	}()

	// Start server
	go startServer(app, cfg)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	collector.Flush()
	adm.Notifier.Close()
	adm.MetricsNotifier.Close()
	if err := app.Shutdown(); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

func startServer(app *fiber.App, cfg *config.Config) {
	if cfg.Server.TLS.Disabled {
		log.Printf("Starting LM Gate %s on %s", Version, listenURL(cfg.Server.Listen, false))
		if err := app.Listen(cfg.Server.Listen); err != nil {
			log.Fatalf("server error: %v", err)
		}
		return
	}

	result, err := tlsutil.Resolve(cfg.Server.TLS)
	if err != nil {
		log.Fatalf("TLS setup error: %v", err)
	}

	// Start HTTP->HTTPS redirect on :80
	go startHTTPRedirect(cfg)

	log.Printf("Starting LM Gate %s on %s", Version, listenURL(cfg.Server.Listen, true))

	if result.TLSConfig != nil {
		// autocert mode: use custom TLS listener
		ln, err := net.Listen("tcp", cfg.Server.Listen)
		if err != nil {
			log.Fatalf("listen error: %v", err)
		}
		tlsLn := tls.NewListener(ln, result.TLSConfig)
		if err := app.Listener(tlsLn); err != nil {
			log.Fatalf("server error: %v", err)
		}
		return
	}

	// cert file mode
	if err := app.ListenTLS(cfg.Server.Listen, result.CertFile, result.KeyFile); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func bootstrapAdminUser(db *sql.DB) {
	hasAdmin, err := models.HasAdminUser(db)
	if err != nil {
		log.Fatalf("checking for admin user: %v", err)
	}
	if hasAdmin {
		return
	}

	email := os.Getenv("LMGATE_AUTH_ADMIN_EMAIL")
	if email == "" {
		email = "admin@lmgate.local"
	}

	password, err := generatePassword(16)
	if err != nil {
		log.Fatalf("generating admin password: %v", err)
	}

	_, err = models.CreateUser(db, email, "Admin", password, "admin", "local", "", true)
	if err != nil {
		log.Fatalf("creating bootstrap admin user: %v", err)
	}

	fmt.Fprintln(os.Stderr, strings.Repeat("=", 60))
	fmt.Fprintln(os.Stderr, "BOOTSTRAP: Admin user created")
	fmt.Fprintf(os.Stderr, "  Email:    %s\n", email)
	fmt.Fprintf(os.Stderr, "  Password: %s\n", password)
	fmt.Fprintln(os.Stderr, "You will be required to change this password on first login.")
	fmt.Fprintln(os.Stderr, strings.Repeat("=", 60))
}

func generatePassword(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf)[:length], nil
}

func startHTTPRedirect(cfg *config.Config) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if !isAllowedHost(host, cfg.Server.AllowedHosts) {
			http.Error(w, "invalid host", http.StatusBadRequest)
			return
		}
		target := fmt.Sprintf("https://%s%s", host, r.RequestURI)
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})

	log.Printf("starting HTTP->HTTPS redirect on :80")
	if err := http.ListenAndServe(":80", mux); err != nil {
		log.Printf("HTTP redirect server error: %v", err)
	}
}

func isAllowedHost(host string, allowed []string) bool {
	if len(allowed) == 0 {
		return true // no restriction configured
	}
	// Strip port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	for _, h := range allowed {
		if strings.EqualFold(host, h) {
			return true
		}
	}
	return false
}

func listenURL(listen string, tlsEnabled bool) string {
	scheme := "http"
	defaultPort := "80"
	if tlsEnabled {
		scheme = "https"
		defaultPort = "443"
	}

	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return fmt.Sprintf("%s://%s", scheme, listen)
	}
	if port == defaultPort {
		return fmt.Sprintf("%s://%s", scheme, host)
	}
	return fmt.Sprintf("%s://%s:%s", scheme, host, port)
}

func printBanner() {
	const green = "\033[32m"
	const reset = "\033[0m"
	banner := `
░▒▓█▓▒░      ░▒▓██████████████▓▒░        ░▒▓██████▓▒░ ░▒▓██████▓▒░▒▓████████▓▒░▒▓████████▓▒░
░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░ ░▒▓█▓▒░   ░▒▓█▓▒░
░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░      ░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░ ░▒▓█▓▒░   ░▒▓█▓▒░
░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░      ░▒▓█▓▒▒▓███▓▒░▒▓████████▓▒░ ░▒▓█▓▒░   ░▒▓██████▓▒░
░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░ ░▒▓█▓▒░   ░▒▓█▓▒░
░▒▓████████▓▒░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░       ░▒▓██████▓▒░░▒▓█▓▒░░▒▓█▓▒░ ░▒▓█▓▒░   ░▒▓████████▓▒░
`
	fmt.Print(green + banner)
	bannerWidth := 90
	versionLine := fmt.Sprintf("%*s", bannerWidth, Version)
	fmt.Println(versionLine + reset)
	fmt.Println()
}

func sendTelemetry(cfg *config.Config, db *sql.DB) {
	if cfg.Telemetry.Disabled {
		return
	}

	val, _ := models.GetAppSetting(db, "telemetry_sent")
	if val == "true" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://static.scarf.sh/a.png?x-pxid=02f34f61-ad3d-4c55-b01e-b9e56b00e482&v="+Version, nil)
	if err != nil {
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Install telemetry ping failed: %v", err)
		return
	}
	resp.Body.Close()

	log.Println("Install telemetry ping sent successfully")
	_ = models.SetAppSetting(db, "telemetry_sent", "true")
}

func securityHeaders(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !cfg.Security.SecurityHeadersEnabled {
			return c.Next()
		}
		c.Set("X-Content-Type-Options", "nosniff")
		xfo := strings.ToUpper(cfg.Security.HeaderXFrameOptions)
		switch xfo {
		case "DENY", "SAMEORIGIN":
			c.Set("X-Frame-Options", xfo)
		default:
			c.Set("X-Frame-Options", "DENY")
		}
		c.Set("Referrer-Policy", cfg.Security.HeaderReferrerPolicy)
		c.Set("X-XSS-Protection", cfg.Security.HeaderXSSProtection)
		c.Set("Content-Security-Policy", cfg.Security.HeaderCSP)
		if c.Protocol() == "https" {
			c.Set("Strict-Transport-Security", "max-age="+strconv.Itoa(cfg.Security.HSTSMaxAge)+"; includeSubDomains")
		}
		return c.Next()
	}
}
