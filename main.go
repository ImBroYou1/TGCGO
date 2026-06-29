package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"TGCGO/config"
	"TGCGO/internal/bot"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	config.Init()

	log.Printf("✅ Config loaded: server=%s, lang=%s, password=%v",
		config.Load().ServerName,
		config.Load().Language,
		config.Load().UsePassword,
	)

	b, err := bot.New(config.Load().Token)
	if err != nil {
		log.Fatal(err)
	}

	b.Start()
	log.Println("🚀 Bot started on Go")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	b.Stop()
	log.Println("🛑 Bot stopped")
}
