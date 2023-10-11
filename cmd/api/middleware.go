package main

import (
	"errors"
	"strings"
	"fmt"
	"net"
	"sync"
	"net/http"
	"time"
	"golang.org/x/time/rate"
	"github.com/Marsh-sudo/greenlight/internal/data"
	"github.com/Marsh-sudo/greenlight/internal/validator"
)

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func ()  {
			// Use the builtin recover function to check if there has been a panic or // not.
			if err := recover(); err != nil {
				// If there was a panic, set a "Connection: close" header on the
				// response. This acts as a trigger to make Go's HTTP server
				// automatically close the current connection after a response has been // sent.
				w.Header().Set("Connection","close")
				// The value returned by recover() has the type interface{}, so we use // fmt.Errorf() to normalize it into an error and call our
				// serverErrorResponse() helper. In turn, this will log the error using // our custom Logger type at the ERROR level and send the client a 500 
				// Internal Server Error response.
				app.serverErrorResponse(w,r,fmt.Errorf("%s",err))
			}
		}()

		next.ServeHTTP(w,r)
	})
}

func (app *application) ratelimit(next http.Handler) http.Handler {

	type client struct {
		limiter *rate.Limiter
		lastSeen time.Time
	}

	var (
		mu sync.Mutex
		clients = make(map[string]*client)
	)

	go func ()  {
		for {
			time.Sleep(time.Minute)
			mu.Lock()

			// Loop through all clients. If they haven't been seen within the last three // minutes, delete the corresponding entry from the map.
			for ip,client := range clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(clients,ip)
				}
			}

			mu.Unlock()
		}
	}()
	

	// The function we are returning is a closure, which 'closes over' the limiter // variable.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
		if app.config.limiter.enabled {
			//extract the client's IP address from the request
			ip,_,err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				app.serverErrorResponse(w,r,err)
				return
			}

			mu.Lock()
			// Check to see if the IP address already exists in the map. If it doesn't, then // initialize a new rate limiter and add the IP address and limiter to the map.
			if _,found := clients[ip]; !found {
				clients[ip] = &client{
					limiter: rate.NewLimiter(rate.Limit(app.config.limiter.rps), app.config.limiter.burst),
				}
			}

			clients[ip].lastSeen = time.Now()

			if !clients[ip].limiter.Allow() {
				mu.Unlock()
				app.rateLimitExceededResponse(w,r)
				return
			}

			mu.Unlock()
		}

		next.ServeHTTP(w,r)
	})
}

func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary","Authorization")

		// Retrieve the value of the Authorization header from the request. This will // return the empty string "" if there is no such header found.
		authorizationHeader := r.Header.Get("Authorization")

		// If there is no Authorization header found, use the contextSetUser() helper // that we just made to add the AnonymousUser to the request context. Then we // call the next handler in the chain and return without executing any of the // code below
		if authorizationHeader == "" {
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w,r)
			return
		}

		headerParts := strings.Split(authorizationHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			app.invalidAuthenticationTokenResponse(w,r)
			return
		}

		//extract the actual authentication token from the header parts
		token := headerParts[1]

		//validate the token to make sure it is in a sensible format
		v := validator.New()

		if data.ValidateTokenPlaintext(v,token); !v.Valid() {
			app.invalidAuthenticationTokenResponse(w,r)
			return
		}

		// Retrieve the details of the user associated with the authentication token, // again calling the invalidAuthenticationTokenResponse() helper if no
// matching record was found. IMPORTANT: Notice that we are using
// ScopeAuthentication as the first parameter here.
		user, err := app.models.Users.GetForToken(data.ScopeAuthentication, token)
		if err != nil {
			switch {
			case errors.Is(err,data.ErrRecordNotFound):
				app.invalidAuthenticationTokenResponse(w,r)
			default:
				app.serverErrorResponse(w,r,err)
			}
			return
		}

		// Call the contextSetUser() helper to add the user information to the request // context.
		r = app.contextSetUser(r,user)

		next.ServeHTTP(w,r)
	})
}