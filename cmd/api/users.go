package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/Marsh-sudo/greenlight/internal/data"
	"github.com/Marsh-sudo/greenlight/internal/validator"
)

func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	//struct to hold expected data from request body
	var input struct {
		Name string `json:"name"`
		Email string `json:"email"`
		Password string `json:"password"`
	}

	//parse the request body into the struct
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r,err)
		return
	}

	// Copy the data from the request body into a new User struct. Notice also that we // set the Activated field to false, which isn't strictly necessary because the
// Activated field will have the zero-value of false by default. But setting this // explicitly helps to make our intentions clear to anyone reading the code.
	user := data.User{
		Name: input.Name,
		Email: input.Email,
		Activated: false,
	}

	// Use the Password.Set() method to generate and store the hashed and plaintext // passwords.
	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w,r,err)
		return
	}

	v := validator.New()

	//validate the user struct and return the error messages to the client if any of the checks fail
	if data.ValidateUser(v, &user); !v.Valid() {
		app.failedValidationResponse(w,r,v.Errors)
		return
	}

	err = app.models.Users.Insert(&user)
	if err != nil {
		switch {
			case errors.Is(err, data.ErrDuplicateEmail):
				v.AddError("email", "a user with this email address already exists")
				app.failedValidationResponse(w,r,v.Errors)
			default:
				app.serverErrorResponse(w,r,err)
		}
		return
	}

	token, err := app.models.Tokens.New(user.ID, 3*24*time.Hour, data.ScopeActivation)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}


	// err = app.mailer.Send(user.Email, "user_welcome.tmpl", user)
	// if err != nil {
	// 	app.serverErrorResponse(w,r,err)
	// 	return
	// }

	// //a goroutine that runs anonymous function that sends email
	// go func(){
	// 	// Run a deferred function which uses recover() to catch any panic, and log an // error message instead of terminating the application.
	// 	defer func() {
	// 		if err := recover(); err != nil {
	// 			app.logger.PrintError(fmt.Errorf("%s", err), nil)
	// 		}
	// 	}()

	// 	err = app.mailer.Send(user.Email,"user_welcome.tmpl", user)
	// 	if err != nil {
	// 		app.logger.PrintError(err,nil)
	// 	}
	// }()
	

	// Use the background helper to execute an anonymous function that sends the welcome // email.
	app.background(func() {

		data := map[string]interface{}{
			"activationToken":token.Plaintext,
			"userID":user.ID,
		}

		err = app.mailer.Send(user.Email,"user_welcome.tmpl", data)
		if err != nil {
			app.logger.PrintError(err, nil)
		}
	})
	

	//write a json response containing the user data with a 201 created status
	err = app.writeJSON(w, http.StatusAccepted, envelope{"user":user}, nil)
	if err != nil {
		app.serverErrorResponse(w,r,err)
	}

}


func (app *application) activateUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TokenPlaintext string  `json:"token"`
	}

	err := app.readJSON(w,r,&input)
	if err != nil {
		app.badRequestResponse(w,r,err)
		return
	}

	//validate the plaintext token provided by the client
	v := validator.New()

	if data.ValidateTokenPlaintext(v,input.TokenPlaintext); !v.Valid() {
		app.failedValidationResponse(w,r,v.Errors)
		return
	}

	// Retrieve the details of the user associated with the token using the
	// GetForToken() method (which we will create in a minute). If no matching record
	// is found, then we let the client know that the token they provided is not valid.
	user,err := app.models.Users.GetForToken(data.ScopeActivation, input.TokenPlaintext)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("token", "invalid or expired activation token")
			app.failedValidationResponse(w,r,v.Errors)
		default:
			app.serverErrorResponse(w,r,err)
		}
		return
	}

	user.Activated = true

	//save the updated user record in our database, checking for any edit conficts in
	//the same way that we did for our movie records
	err = app.models.Users.Update(user)
	if err != nil {
		switch {
			case errors.Is(err, data.ErrEditConflict):
				app.editConflictResponse(w,r)
			default:
				app.serverErrorResponse(w,r,err)
		}
		return
	}

	err = app.models.Tokens.DeleteAllForUser(data.ScopeActivation, user.ID)
	if err != nil {
		app.serverErrorResponse(w,r,err)
		return
	}

	// Send the updated user details to the client in a JSON response.
	err = app.writeJSON(w, http.StatusOK, envelope{"user":user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}