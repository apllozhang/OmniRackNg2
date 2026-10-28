package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"runtime"
	"time"

	"omnirack-replica/backend/internal/config"
	"omnirack-replica/backend/internal/handler"
	"omnirack-replica/backend/internal/repository"
	"omnirack-replica/backend/internal/router"
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
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	db, err := repository.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	if err := repository.Migrate(db); err != nil {
		log.Fatalf("migrate database: %v", err)
	}
	ctx := context.Background()
	if err := repository.SeedRoles(ctx, db); err != nil {
		log.Fatalf("seed roles: %v", err)
	}
	if err := repository.SeedAdmin(ctx, db, cfg.AdminUser, cfg.AdminPass); err != nil {
		log.Fatalf("seed admin: %v", err)
	}
	if err := repository.SeedRackTemplates(ctx, db); err != nil {
		log.Fatalf("seed rack templates: %v", err)
	}
	if err := repository.SeedDeviceCatalog(ctx, db); err != nil {
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
	router.RegisterAssetOperations(engine, assetHandler, cfg.JWTSecret)
	notifications := service.NewNotificationService(db, cfg.JWTSecret)
	router.RegisterNotifications(engine, handler.NewNotificationHandler(notifications), assetHandler, cfg.JWTSecret)
	go func() {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
			if err := notifications.DispatchApprovals(ctx); err != nil {
				log.Printf("approval notification dispatch failed")
			}
			cancel()
			time.Sleep(time.Minute)
		}
	}()
	go func() {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			if err := assetOps.RefreshReminders(ctx); err != nil {
				log.Printf("asset reminder refresh failed: %v", err)
			} else if err := notifications.Dispatch(ctx); err != nil {
				log.Printf("asset notification dispatch failed")
			}
			cancel()
			time.Sleep(15 * time.Minute)
		}
	}()
	log.Printf("omnirack-replica listening on %s", cfg.HTTPAddr)
	if err := engine.Run(cfg.HTTPAddr); err != nil {
		log.Fatal(err)
	}
}
