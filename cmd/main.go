package main

import (
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/lib/pq"
	"log"
	"net/http"
	"os"
	"social-media/internal/handler"
	"social-media/internal/service"
	"time"
)

type Config struct {
	dsn       string
	port      int
	jwtSecret string
}

func main() {
	var config Config
	flag.StringVar(&config.dsn, "db-dsn", "", "Database source name")
	flag.IntVar(&config.port, "port", 6001, "Server port")
	flag.StringVar(&config.jwtSecret, "jwt-secret", "", "JWT secret")
	flag.Parse()

	db, err := sql.Open("postgres", config.dsn)
	if err != nil {
		log.Fatalf("could not open db connection: %w", err)
	}

	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatalf("could not ping to db: %w", err)
	}

	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime)
	s := service.New(db, config.jwtSecret, fmt.Sprintf("http://localhost:%v/img/avatars/", config.port))
	h := handler.New(s, logger)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%v", config.port),
		Handler:           h,
		ReadHeaderTimeout: time.Second * 10,
		ReadTimeout:       time.Second * 30,
	}

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		logger.Fatalf("could not listen and serve: %w", err)
	}
}
