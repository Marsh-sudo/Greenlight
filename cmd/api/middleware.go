package main

import (
	"fmt"
	"net"
	"sync"
	"net/http"
	"golang.org/x/time/rate"
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

	var (
		mu sync.Mutex
		clients = make(map[string]*rate.Limiter)
	)
	

	// The function we are returning is a closure, which 'closes over' the limiter // variable.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
		//extract the client's IP address from the request
		ip,_,err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			app.serverErrorResponse(w,r,err)
			return
		}

		//lock the mutex to prevent this code from being executed concurrently
		mu.Lock()

		// Check to see if the IP address already exists in the map. If it doesn't, then // initialize a new rate limiter and add the IP address and limiter to the map.
		if _, found := clients[ip]; !found {
			clients[ip] = rate.NewLimiter(2,4)
		}

		// Call the Allow() method on the rate limiter for the current IP address. If // the request isn't allowed, unlock the mutex and send a 429 Too Many Requests // response, just like before.
		if !clients[ip].Allow() {
			mu.Unlock()
			app.rateLimitExceededResponse(w,r)
			return
		}

		mu.Unlock()

		next.ServeHTTP(w,r)
	})
}