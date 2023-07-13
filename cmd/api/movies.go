package main

import (
	"errors"
	"fmt"
	"net/http" 
	
	"github.com/Marsh-sudo/greenlight/internal/data"
	"github.com/Marsh-sudo/greenlight/internal/validator"

)

func (app *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {

	var input struct {
		Title string `json:"title"`
		Year int32 `json:"year"`
		Runtime data.Runtime `json:"runtime"`
		Genres []string `json:"genres"`
	}

	err := app.readJSON(w,r,&input)
	if err != nil {
		app.badRequestResponse(w,r,err)
		return
	}

	movie := &data.Movie{
		Title: input.Title, 
		Year: input.Year, 
		Runtime: input.Runtime, 
		Genres: input.Genres,
	}
	// initialize a new validator instance
	v := validator.New()
	
	// Call the ValidateMovie() function and return a response containing the errors if // any of the checks fail.
	if data.ValidateMovie(v,movie); !v.Valid() {
		app.failedValidationResponse(w,r,v.Errors)
		return
	}

	// Call the Insert() method on our movies model, passing in a pointer to the
	// validated movie struct. This will create a record in the database and update the 
	// movie struct with the system-generated information.
	err = app.models.Movie.Insert(movie)
	if err != nil {
		app.serverErrorResponse(w,r,err)
		return
	}

	// When sending a HTTP response, we want to include a Location header to let the
	// client know which URL they can find the newly-created resource at. We make an
	// empty http.Header map and then use the Set() method to add a new Location header, 
	// interpolating the system-generated ID for our new movie in the URL.
	headers := make(http.Header)
	headers.Set("Location",fmt.Sprintf("/v1/movies/%d",movie.ID))

	//write a JSON response with a 201 created status code,the movie data in the
	//response body and the Location header
	err = app.writeJSON(w,http.StatusCreated,envelope{"movie":movie},headers)
	if err != nil {
		app.serverErrorResponse(w,r,err)
	}
	

	fmt.Fprintf(w, "%+v\n", input) 
}

func(app *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w,r)
		return
	}

	// cll the get() methos to fetch the data for a specific movie and also
	// use the errors.Is() function to check if it returns a data.ErrRecordNotFound
	movie,err := app.models.Movie.Get(id)
	if err != nil {
		switch {
		case errors.Is(err,data.ErrRecordNotFound):
			app.notFoundResponse(w,r)
		default:
			app.serverErrorResponse(w,r,err)
		}
		return
	}



	err = app.writeJSON(w,http.StatusOK,envelope{"movie":movie},nil)
	if err != nil {
		app.serverErrorResponse(w,r,err)
	}

	fmt.Fprintf(w, "show the details of movie %d\n",id)

}