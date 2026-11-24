package main

import (
	"fmt"
	"os"

	"github.com/mitdo-garden/mdg-tempo-svc/internal/config"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	gdb, err := db.Connect(cfg.MySQLDSN)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer sqlDB.Close()
	stmt, err := os.ReadFile("migrations/003_user_telegram.sql")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := gdb.Exec(string(stmt)).Error; err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("user_telegram ready")
}
