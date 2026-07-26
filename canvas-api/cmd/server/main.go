package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/blsnt8586/canvas-api/internal/config"
	"github.com/blsnt8586/canvas-api/internal/handler"
	"github.com/blsnt8586/canvas-api/internal/repository"
	"github.com/blsnt8586/canvas-api/internal/server"
	"github.com/blsnt8586/canvas-api/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := repository.Open(cfg.DB.DSN())
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// 只创建/维护 canvas_* 表，幂等，可安全重复运行。
	migCtx, migCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := db.Migrate(migCtx); err != nil {
		migCancel()
		log.Fatalf("migrate: %v", err)
	}
	migCancel()

	// S3 客户端：Wasabi 未配置时为 nil，media 上传接口返回 503，其余接口正常。
	// 用接口变量而非 *storage.Client，避免 typed-nil 导致 h.s3 == nil 判断失效。
	var objStore handler.ObjectStore
	if cfg.Wasabi.Enabled() {
		s3Client, err := storage.New(context.Background(), storage.Config{
			Endpoint:  cfg.Wasabi.Endpoint,
			Region:    cfg.Wasabi.Region,
			Bucket:    cfg.Wasabi.Bucket,
			AccessKey: cfg.Wasabi.AccessKey,
			SecretKey: cfg.Wasabi.SecretKey,
		})
		if err != nil {
			log.Fatalf("init s3 client: %v", err)
		}
		objStore = s3Client
		log.Printf("object storage: bucket=%s endpoint=%s", cfg.Wasabi.Bucket, cfg.Wasabi.Endpoint)
	} else {
		log.Println("object storage: disabled (WASABI_BUCKET/ACCESS_KEY/SECRET_KEY not set)")
	}

	engine := server.New(cfg, db, objStore)
	srv := &http.Server{
		Addr:              cfg.Host + ":" + cfg.Port,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("canvas-api listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down canvas-api...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
}
