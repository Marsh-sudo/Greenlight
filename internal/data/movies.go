package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

	ctx, cancel := context.WithTimeout(context.Background(),3*time.Second)
	defer cancel()

	//use the QueryRow() method to execute the SQL query on our connection pool,
	//passing in the args slice as a variadic parameter and scanning the system-generated
	// id,vreated_at and version values into the movie struct

	return m.DB.QueryRowContext(ctx,query,args...).Scan(&movie.ID,&movie.CreatedAt,&movie.Version)
}

// Add a placeholder method for fetching a specific record from the movies table.
func (m MovieModel) Get(id int64) (*Movie, error) {

	if id < 1 {
		return nil ,ErrRecordNotFound
	}
	//define the sql query for retrieving the movie data
	query := `
		SELECT id, created_at,title,year,runtime,genres,version
		FROM movies
		WHERE id = $1`

		// Declare a Movie struct to hold the data returned by the query
		var movie Movie

		//use the context.WithTimeout() function to create a context.Context which carries a 
		// 3-second timeout deadline.
		ctx, cancel := context.WithTimeout(context.Background(),3*time.Second)

		defer cancel()

		// Use the QueryRowContext() method to execute the query, passing in the context 
		// with the deadline as the first argument.

		err := m.DB.QueryRowContext(ctx,query,id).Scan(
			&movie.ID, 
			&movie.CreatedAt,
			&movie.Title,
			&movie.Year,
			&movie.Runtime,
			pq.Array(&movie.Genres),
			&movie.Version,
		)

		//if there was no matching movie found, Scan() will return
		// a sql.ErrNoRows error. We check for this and return our custom ErrRecordNotFound
		if err != nil {
			switch {
			case errors.Is(err,sql.ErrNoRows):
				return nil, ErrRecordNotFound
			default:
				return nil,err
			}
		}

		//otherwise return a pointer to the Movie struct
		return &movie, nil
}

// Add a placeholder method for updating a specific record in the movies table.
func (m MovieModel) Update(movie *Movie) error {
	// declare the sql query for updating the record and returning the new versio
	query := `
		UPDATE movies
		set title = $1, year = $2,runtime = $3, genres = $4, version = version + 1
		WHERE id = $5 AND version = $6
		RETURNING version`

		// create an args slice containing the values for the placeholder parameters
		args := []interface{}{
			movie.Title,
			movie.Year,
			movie.Runtime,
			pq.Array(movie.Genres),
			movie.ID,
			movie.Version,
		}

		ctx,cancel := context.WithTimeout(context.Background(),3*time.Second)
		defer cancel()

		//executte the SQL query. If no matching row could be found we know the movie
		// movie version has changed
		err := m.DB.QueryRowContext(ctx,query,args...).Scan(&movie.Version)
		if err != nil {
			switch {
			case errors.Is(err,sql.ErrNoRows): // if no rows were updated, then the version 
				return ErrEditConflict
			default:
				return err
			}
		}

		return nil



		// USE THE QUERYRow() method to execute the query, passing in the args slice as a 
		// vardiac parameter and scanning the new version value into the movie struct

	// return m.DB.QueryRow(query,args...).Scan(&movie.Version)
}

func (m MovieModel) Delete(id int64) error {
	//return an ErrRecordNotFound error if the movie ID is less than 1
	if id < 1 {
		return ErrRecordNotFound
	}

	//construct the sql query to delete the record
	query := `
		DELETE FROM movies
		WHERE id = $1`
	
	ctx,cancel := context.WithTimeout(context.Background(),3*time.Second)
	defer cancel()

	//execute the SQL query using the Exec() method, passing in the id variable as
	// the value for the placehoder parameter. The Exec method return a sql.Result object
	result,err := m.DB.ExecContext(ctx,query,id)
	if err != nil {
		return err
	}

	//call the RowsAffected() method on the sql.Result object to get the number of rows
	//affected by the query
	rowsAffected,err := result.RowsAffected()
	if err != nil {
		return err
	}

	// If no rows were affected, we know that the movies table didn't contain a record // with the provided ID at the moment we tried to delete it. In that case we
	// return an ErrRecordNotFound error.
	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	
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

func (m MovieModel) GetAll(title string, genres []string,filters Filters) ([]*Movie,Metadata, error) {
	query := fmt.Sprintf(`
	SELECT count(*) OVER(), id, created_at,title,year,runtime,genres,version
	FROM movies
	WHERE (to_tsvector('simple', title) @@ plainto_tsquery('simple', $1) OR $1 = '')
	AND (genres @> $2 OR $2 = '{}')
	ORDER BY %s %s, id ASC 
	LIMIT $3 OFFSET $4` ,filters.sortColumn(),filters.sortDirection())

	ctx,cancel := context.WithTimeout(context.Background(),3*time.Second)
	defer cancel()

	// As our SQL query now has quite a few placeholder parameters, let's collect the // values for the placeholders in a slice. Notice here how we call the limit() and // offset() methods on the Filters struct to get the appropriate values for the
	// LIMIT and OFFSET clauses.
	args := []interface{}{title,pq.Array(genres),filters.limit(),filters.offset()}

	// Use QueryContext() to execute the query. This returns a sql.Rows resultset // containing the result.
	rows,err := m.DB.QueryContext(ctx,query,args...)
	if err != nil {
		return nil,Metadata{},err
	}

	// Importantly, defer a call to rows.Close() to ensure that the resultset is closed // before GetAll() returns.
	defer rows.Close()


	// Initialize an empty slice to hold the movie data.
	totalRecords := 0
	movies := []*Movie{}

	for rows.Next() {
		var movie Movie

		// Scan the values from the row into the Movie struct. Again, note that we're // using the pq.Array() adapter on the genres field here.
		err := rows.Scan(
			&totalRecords,
			&movie.ID,
			&movie.CreatedAt,
			&movie.Title,
			&movie.Year,
			&movie.Runtime,
			pq.Array(&movie.Genres),
			&movie.Version,
		)
		if err != nil {
			return nil,Metadata{},err
		}

		//add the movie struct to the slice
		movies = append(movies, &movie)
	}

	// When the rows.Next() loop has finished, call rows.Err() to retrieve any error // that was encountered during the iteration.
	if err = rows.Err();err != nil {
		return nil,Metadata{},err
	}

	// Generate a Metadata struct, passing in the total record count and pagination // parameters from the client.
	metadata := calculateMetadata(totalRecords,filters.Page,filters.PageSize)

	return movies,metadata,nil


}