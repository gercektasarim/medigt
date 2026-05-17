package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: migrate up|down|version|force <n>")
		os.Exit(2)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Println("DATABASE_URL not set")
		os.Exit(1)
	}

	m, err := migrate.New("file://migrations", dbURL)
	if err != nil {
		fmt.Println("migrate init failed:", err)
		os.Exit(1)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			fmt.Println("migrate close source error:", srcErr)
		}
		if dbErr != nil {
			fmt.Println("migrate close db error:", dbErr)
		}
	}()

	switch os.Args[1] {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("migrate up failed:", err)
			os.Exit(1)
		}
		fmt.Println("OK migrations applied")
	case "down":
		if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("migrate down failed:", err)
			os.Exit(1)
		}
		fmt.Println("OK migrations rolled back")
	case "version":
		v, dirty, err := m.Version()
		if err != nil {
			fmt.Println("migrate version failed:", err)
			os.Exit(1)
		}
		fmt.Printf("version=%d dirty=%v\n", v, dirty)
	case "force":
		if len(os.Args) < 3 {
			fmt.Println("usage: migrate force <n>")
			os.Exit(2)
		}
		var n int
		if _, err := fmt.Sscanf(os.Args[2], "%d", &n); err != nil {
			fmt.Println("invalid version:", err)
			os.Exit(2)
		}
		if err := m.Force(n); err != nil {
			fmt.Println("migrate force failed:", err)
			os.Exit(1)
		}
		fmt.Printf("OK forced to version %d\n", n)
	default:
		fmt.Println("unknown command:", os.Args[1])
		os.Exit(2)
	}
}
