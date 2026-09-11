// Command kkt-monitor runs the KKT (cash register) fleet monitoring service:
// a password-protected web UI for uploading CSV exports and managing
// Telegram notification settings, plus a background scheduler that warns
// about upcoming ОФД service and ФН expiry dates.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kkt-monitor/internal/config"
	"kkt-monitor/internal/db"
	"kkt-monitor/internal/notify"
	"kkt-monitor/internal/web"
)

func boolLabel(v bool, yes, no string) string {
	if v {
		return yes
	}
	return no
}

func main() {
	cfg := config.Load()
	if cfg.AdminPassword == "" {
		log.Fatal("ADMIN_PASSWORD не задан: установите переменную окружения с паролем для входа в веб-интерфейс")
	}
	if cfg.ViewerPassword != "" && cfg.ViewerPassword == cfg.AdminPassword {
		log.Fatal("VIEWER_PASSWORD совпадает с ADMIN_PASSWORD: задайте разные пароли или оставьте VIEWER_PASSWORD пустым")
	}
	log.Printf("конфигурация: адрес=%s, база=%s, COOKIE_SECURE=%v, VIEWER_PASSWORD %s",
		cfg.ListenAddr, cfg.DBPath, cfg.CookieSecure, boolLabel(cfg.ViewerPassword != "", "задан", "не задан"))
	if cfg.CookieSecure {
		log.Println("ВНИМАНИЕ: COOKIE_SECURE=true — вход сработает, только если сервис открыт по HTTPS. Если это обычный http://, поставьте COOKIE_SECURE=false, иначе после ввода пароля вас будет возвращать обратно на страницу входа.")
	}

	store, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("не удалось открыть базу данных: %v", err)
	}
	defer store.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	scheduler := notify.New(store)
	go scheduler.Run(ctx)

	server := web.New(cfg, store)
	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           server,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("kkt-monitor слушает на %s", cfg.ListenAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("остановка сервиса...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("ошибка остановки http server: %v", err)
	}
}
