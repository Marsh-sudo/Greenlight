package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

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


func (app *application) updateMovieHandler(w http.ResponseWriter, r *http.Request) {
	// EXTRACT the movie ID from the URL
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w,r)
		return
	}

	// fetch the existing movie record from the database sending a 404 NOT found
	movie, err := app.models.Movie.Get(id)
	if err != nil {
		switch {
		case errors.Is(err,data.ErrRecordNotFound):
			app.notFoundResponse(w,r)
		default:
			app.serverErrorResponse(w,r,err)
		}
		return
	}

	if r.Header.Get("X-Expected-Version") != "" {
		if strconv.FormatInt(int64(movie.Version),32) != r.Header.Get("X-Expected-Version") {
			app.editConflictResponse(w,r)
			return
		}
	}

	// declare an input struct to hold the expected data
	//using pointer for the Title,Year and Runtime
	var input struct{
		Title *string `json:"title"`
		Year *int32 `json:"year"`
		Runtime *data.Runtime `json:"runtime"`
		Genres []string `json:"genres"` 
	}

	//read the JSON request body data into the input struct
	err = app.readJSON(w,r,&input)
	if err != nil {
		app.badRequestResponse(w,r,err)
		return
	}

	//copy the values from the request body to the appropriate fields of the movie
	if input.Title != nil {
		movie.Title = *input.Title
	}

	if input.Year != nil {
		movie.Year = *input.Year
	}
	if input.Runtime != nil {
		movie.Runtime = *input.Runtime
	}
	if input.Genres != nil {
		movie.Genres = input.Genres
	}

	//validate the updated movie record sendiing the client a 422 Unprocessable Entity
	v := validator.New()
	if data.ValidateMovie(v,movie); !v.Valid() {
		app.failedValidationResponse(w,r,v.Errors)
		return
	}

	// pass the updated movie record to our new Update method
	err = app.models.Movie.Update(movie)
	if err != nil {
		switch {
		case errors.Is(err,data.ErrEditConflict):
			app.editConflictResponse(w,r)
		default:
			app.serverErrorResponse(w, r,err)
		}
		return
	}

	//write the updated movie record in a JSON response
	err = app.writeJSON(w,http.StatusOK,envelope{"movie":movie},nil)
	if err != nil {
		app.serverErrorResponse(w,r,err)
	}
}

func (app *application) deleteMovieHandler(w http.ResponseWriter, r *http.Request) {
	//extract the movie ID from the URL
	id,err := app.readIDParam(r)
	if err!= nil{
		app.notFoundResponse(w,r)
		return
	}

	//delete the movie from the database,sending a 404 Not found response to the client
	err = app.models.Movie.Delete(id)
	if err != nil {
		switch {
		case errors.Is(err,data.ErrRecordNotFound):
			app.notFoundResponse(w,r)
		default:
			app.serverErrorResponse(w,r,err)
		}
		return
	}

	//return a 200 OK status code along with a success message
	err = app.writeJSON(w,http.StatusOK,envelope{"message":"movie successfully deleted"},nil)
	if err != nil {
		app.serverErrorResponse(w,r,err)
	}
}


func (app *application) listMoviesHandler(w http.ResponseWriter,r *http.Request) {
	// To keep things consistent with our other handlers, we'll define an input struct // to hold the expected values from the request query string
	var input struct {
		Title string
		Genres []string
		data.Filters
	}

	v := validator.New()

	// Call r.URL.Query() to get the url.Values map containing the query string data.
	qs := r.URL.Query()

	// Use our helpers to extract the title and genres query string values, falling back // to defaults of an empty string and an empty slice respectively if they are not
	// provided by the client.
	input.Title = app.readString(qs,"title", "")
	input.Genres = app.readCSV(qs,"genres",[]string{})

	// Get the page and page_size query string values as integers. Notice that we set // the default page value to 1 and default page_size to 20, and that we pass the // validator instance as the final argument here.
	input.Filters.Page = app.readInt(qs,"page",1,v)
	input.Filters.PageSize = app.readInt(qs, "page_size",20,v)

	// Extract the sort query string value, falling back to "id" if it is not provided // by the client (which will imply a ascending sort on movie ID).
	input.Filters.Sort = app.readString(qs,"sort","id")

	input.Filters.SortSafelist = []string{"id", "title", "year", "runtime", "-id", "-title", "-year", "-runtime"}


	// Check the Validator instance for any errors and use the failedValidationResponse() // helper to send the client a response if necessary.
	if data.ValidateFilters(v,input.Filters); !v.Valid() {
		app.failedValidationResponse(w,r,v.Errors)
		return
	}

	movies,metadata,err := app.models.Movie.GetAll(input.Title,input.Genres,input.Filters)
	if err != nil {
		app.serverErrorResponse(w,r,err)
		return
	}

	err = app.writeJSON(w,http.StatusOK,envelope{"movies":movies,"metadata":metadata},nil)
	if err != nil {
		app.serverErrorResponse(w,r,err)
	}


// Dump the contents of the input struct in a HTTP response.
	fmt.Fprintf(w, "%+v\n",input)

}