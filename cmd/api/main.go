package main

import (
	"context"
	"database/sql"
	"fmt"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

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
}

// struct  to hold the dependencies for our HTTP handlers, helpers, // and middleware.
type application struct {
	config config
	logger *log.Logger
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


	flag.Parse()

	logger := log.New(os.Stdout, "",log.Ldate | log.Ltime)
	
	db,err := OpenDB(cfg)
	if err != nil {
		logger.Fatal(err)
	}
	defer db.Close()

	logger.Printf("database connection pool established")

	app := &application{
		config: cfg,
		logger: logger,
	}

	
	srv := &http.Server{
		Addr: fmt.Sprintf(":%d",cfg.port),
		Handler: app.routes(),
		IdleTimeout: time.Minute,
		ReadTimeout: 10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	logger.Printf("starting %s server on %s", cfg.env, srv.Addr)
	err = srv.ListenAndServe()
	logger.Fatal(err)
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