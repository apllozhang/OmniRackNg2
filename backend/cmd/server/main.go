package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"omnirack-replica/backend/internal/config"
	"omnirack-replica/backend/internal/handler"
	"omnirack-replica/backend/internal/repository"
	"omnirack-replica/backend/internal/router"
	appserver "omnirack-replica/backend/internal/server"
	"omnirack-replica/backend/internal/service"
)

// Populated by the verified build; development builds remain explicit.
var sourceSHA256 = "development"

func main() {
	showBuildInfo := flag.Bool("build-info", false, "print build identity without connecting to a database")
	flag.Parse()
	if *showBuildInfo {
		if err := json.NewEncoder(os.Stdout).Encode(map[string]string{
			"sourceSHA256": sourceSHA256, "goVersion": runtime.Version(),
			"module": "omnirack-replica/backend",
		}); err != nil {
			log.Fatal(err)
		}
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	db, err := repository.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("database pool unavailable")
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	defer sqlDB.Close()
	startupCtx, cancelStartup := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelStartup()
	if err := repository.Migrate(db.WithContext(startupCtx)); err != nil {
		log.Fatalf("migrate database: %v", err)
	}
	if err := repository.SeedRoles(startupCtx, db); err != nil {
		log.Fatalf("seed roles: %v", err)
	}
	if err := repository.SeedAdmin(startupCtx, db, cfg.AdminUser, cfg.AdminPass); err != nil {
		log.Fatalf("seed admin: %v", err)
	}
	if err := repository.SeedRackTemplates(startupCtx, db); err != nil {
		log.Fatalf("seed rack templates: %v", err)
	}
	if err := repository.SeedDeviceCatalog(startupCtx, db); err != nil {
		log.Fatalf("seed device catalog: %v", err)
	}

	userStore := repository.NewUserStore(db)
	ldapService := service.NewLDAPService(userStore)
	authService := service.NewAuthService(db, cfg, ldapService)
	resourceStore := repository.NewResourceStore(db)
	templateStore := repository.NewRackTemplateStore(db)
	deviceStore := repository.NewDeviceStore(db)
	resourceService := service.NewResourceService(resourceStore, templateStore)
	templateService := service.NewRackTemplateService(templateStore)
	deviceService := service.NewDeviceService(deviceStore)
	pduStore := repository.NewPDUStore(db)
	pduService := service.NewPDUService(pduStore)

	healthHandler := handler.NewHealthHandler(db)
	authHandler := handler.NewAuthHandler(authService)
	resourceHandler := handler.NewResourceHandler(resourceService)
	templateHandler := handler.NewRackTemplateHandler(templateService)
	deviceHandler := handler.NewDeviceHandler(deviceService)
	pduHandler := handler.NewPDUHandler(pduService)
	userService := service.NewUserService(userStore)
	adminHandler := handler.NewAdminHandler(userService, ldapService, deviceService)
	assetOps := service.NewAssetOperations(db)
	assetHandler := handler.NewAssetOperationsHandler(assetOps)
	engine := router.New(authHandler, healthHandler, resourceHandler, templateHandler, deviceHandler, pduHandler, adminHandler, cfg.JWTSecret, assetHandler.LegacyAccess())
	router.RegisterAssetOperations(engine, assetHandler, cfg.JWTSecret, authHandler.AuthMiddleware())
	notifications := service.NewNotificationService(db, cfg.JWTSecret)
	router.RegisterNotifications(engine, handler.NewNotificationHandler(notifications), assetHandler, cfg.JWTSecret, authHandler.AuthMiddleware())
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	var workers sync.WaitGroup
	appserver.Periodic(ctx, &workers, time.Minute, 50*time.Second, func(ctx context.Context) {
		if err := notifications.DispatchApprovals(ctx); err != nil && ctx.Err() == nil {
			log.Printf("approval notification dispatch failed")
		}
	})
	appserver.Periodic(ctx, &workers, 15*time.Minute, 5*time.Minute, func(ctx context.Context) {
		if err := assetOps.RefreshReminders(ctx); err != nil {
			if ctx.Err() == nil {
				log.Printf("asset reminder refresh failed")
			}
		} else if err := notifications.Dispatch(ctx); err != nil {
			if ctx.Err() == nil {
				log.Printf("asset notification dispatch failed")
			}
		}
	})
	log.Printf("omnirack-replica listening on %s", cfg.HTTPAddr)
	serveErr := appserver.Serve(ctx, appserver.New(cfg.HTTPAddr, engine), listener)
	stop()
	workerDone := make(chan struct{})
	go func() { workers.Wait(); close(workerDone) }()
	select {
	case <-workerDone:
	case <-time.After(10 * time.Second):
		log.Printf("background shutdown deadline reached")
	}
	if serveErr != nil {
		_ = sqlDB.Close()
		log.Printf("HTTP server stopped: %v", serveErr)
		os.Exit(1)
	}
}
