package data

import (
	"database/sql"
	"time"

	"github.com/Marsh-sudo/greenlight/internal/validator"
	"github.com/lib/pq" // New import
)

type Movie struct {
	ID int64 `json:"id"`
	CreatedAt time.Time `json:"-"`
	Title string `json:"title"`
	Year int32  `json:"year,omitempty"`
	Runtime Runtime  `json:"runtime,omitempty"`
	Genres []string `json:"genres,omitempty"`// Slice of genres for the movie (romance, comedy, etc.)
	Version int32  `json:"version"`
}

type MovieModel struct {
	DB*sql.DB
}

// Define a MovieModel struct type which wraps a sql.DB connection pool.
func (m MovieModel) Insert(movie *Movie) error{
	query := `
	    INSERT INTO movies (title,year,runtime,genres)
		VALUES ($1,$2,$3,$4)
		RETURNING id,created_at,version`

	args := []interface{}{movie.Title,movie.Year,movie.Runtime,pq.Array(movie.Genres)}

	//use the QueryRow() method to execute the SQL query on our connection pool,
	//passing in the args slice as a variadic parameter and scanning the system-generated
	// id,vreated_at and version values into the movie struct

	return m.DB.QueryRow(query,args...).Scan(&movie.ID,&movie.CreatedAt,&movie.Version)
}

// Add a placeholder method for fetching a specific record from the movies table.
func (m MovieModel) Get(id int64) (*Movie, error) {
	return nil, nil
}

// Add a placeholder method for updating a specific record in the movies table.
func (m MovieModel) Update(movie *Movie) error {
	return nil
}

func (m MovieModel) Delete(id int64) error {
	return nil
}

func ValidateMovie(v *validator.Validator, movie *Movie) {
	v.Check(movie.Title != "", "title", "must be provided")
	v.Check(len(movie.Title) <= 500, "title", "must not be more than 500 bytes long")


	v.Check(movie.Year != 0, "year", "must be provided")
	v.Check(movie.Year >= 1888, "year", "must be greater than 1888")
	v.Check(movie.Year <= int32(time.Now().Year()), "year", "must not be in the future")

	v.Check(movie.Runtime != 0, "runtime", "must be provided") 
	v.Check(movie.Runtime > 0, "runtime", "must be a positive integer")
	
	v.Check(movie.Genres != nil, "genres", "must be provided")
	v.Check(len(movie.Genres) >= 1, "genres", "must contain at least 1 genre") 
	v.Check(len(movie.Genres) <= 5, "genres", "must not contain more than 5 genres") 
	v.Check(validator.Unique(movie.Genres), "genres", "must not contain duplicate values")
}