package main

import (
	"context"
	"database/sql"
	"fmt"
	"flag"
	"net/http"
	"sync"
	"os"
	"time"
	"github.com/Marsh-sudo/greenlight/internal/data"
	"github.com/Marsh-sudo/greenlight/internal/mailer"
	"github.com/Marsh-sudo/greenlight/internal/jsonlog"

	 _"github.com/lib/pq"
)

const version = "1.0.0"

type config struct {
	port int
	env string
	db struct {
		dsn string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime string
	}

	limiter struct {
		rps float64
		burst int
		enabled bool
	}
	smtp struct{
		host string
		port int
		username string
		password string
		sender string
	}
}

// struct  to hold the dependencies for our HTTP handlers, helpers, // and middleware.
type application struct {
	config config
	logger *jsonlog.Logger
	models data.Models
	mailer mailer.Mailer
	wg sync.WaitGroup
}

func main() {
	var cfg config

	flag.IntVar(&cfg.port, "port", 4000,"API server port")
	flag.StringVar(&cfg.env, "env","development","Environment(development|staging|production)")

	// Read the DSN value from the db-dsn command-line flag into the config struct. We
	// default to using our development DSN if no flag is provided.
	flag.StringVar(&cfg.db.dsn,"db-dsn","postgres://greenlight:123456@localhost/greenlight?sslmode=disable","PostgreSQL DSN")

	flag.IntVar(&cfg.db.maxOpenConns,"db-max-open-conns",25,"PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections") 
	flag.StringVar(&cfg.db.maxIdleTime, "db-max-idle-time", "15m", "PostgreSQL max connection idle time")

	flag.Float64Var(&cfg.limiter.rps, "limiter-rps",2,"Rate limiter maximum request per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst") 
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")

	flag.StringVar(&cfg.smtp.host, "smtp-host", "smtp.mailtrap.io","SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", 25,"SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username","a74ca27c74d3c9","SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", "4a473708dc73dd", "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", "Greenlight <no-reply@greenlight.marshkelvin.net>", "SMTP sender")



	flag.Parse()

	logger := jsonlog.New(os.Stdout, jsonlog.LevelInfo)
	
	db,err := OpenDB(cfg)
	if err != nil {
		logger.PrintFatal(err,nil)
	}
	defer db.Close()

	logger.PrintInfo("database connection pool established",nil)

	app := &application{
		config: cfg,
		logger: logger,
		models: data.NewModels(db),
		mailer: mailer.New(cfg.smtp.host,cfg.smtp.port, cfg.smtp.username, cfg.smtp.password,cfg.smtp.sender),
	}

	// Call app.serve() to start the server.
	err = app.serve()
	if err != nil {
		logger.PrintFatal(err,nil)
	}
	
	srv := &http.Server{
		Addr: fmt.Sprintf(":%d",cfg.port),
		Handler: app.routes(),
		IdleTimeout: time.Minute,
		ReadTimeout: 10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	logger.PrintInfo("starting server", map[string]string{
		"addr":srv.Addr,
		"env":cfg.env,
	})
	err = srv.ListenAndServe()
	logger.PrintFatal(err,nil)
}

func OpenDB(cfg config) (*sql.DB,error) {
	// Use sql.Open() to create an empty connection pool, using the DSN from the config // struct.
	db,err := sql.Open("postgres",cfg.db.dsn)
	if err != nil {
		return nil, err
	}

	// Set the maximum number of open (in-use + idle) connections in the pool. Note that // passing a value less than or equal to 0 will mean there is no limit.
	db.SetMaxOpenConns(cfg.db.maxOpenConns)

	// Set the maximum number of idle connections in the pool. Again, passing a value // less than or equal to 0 will mean there is no limit.
	db.SetMaxIdleConns(cfg.db.maxIdleConns)

	// Use the time.ParseDuration() function to convert the idle timeout duration string // to a time.Duration type.
	duration,err := time.ParseDuration(cfg.db.maxIdleTime)
	if err != nil {
		return nil,err
	}

	//set the max idle timeout
	db.SetConnMaxIdleTime(duration)

	
	// Create a context with a 5-second timeout deadline.
	ctx,cancel := context.WithTimeout(context.Background(),5*time.Second)
	defer cancel()

	// Use PingContext() to establish a new connection to the database, passing in the // context we created above as a parameter. If the connection couldn't be
	// established successfully within the 5 second deadline, then this will return an // error.
	err = db.PingContext(ctx)
	if err != nil {
		return nil,err
	}

	// return the sql.DB connection pool
	return db,nil
}