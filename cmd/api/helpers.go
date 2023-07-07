package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
)

type envelope map[string]interface{}

func (app *application) readIDParam(r *http.Request) (int64,error) {
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"),10,64)
	if err != nil || id < 1 {
		return 0, errors.New("Invalid id parameter")
	}

	return id,nil
}

func (app *application) writeJSON(w http.ResponseWriter,status int,data envelope,headers http.Header) error {
	js, err := json.MarshalIndent(data,"","\t")
	if err != nil {
		return err
	}
	js = append(js, '\n')
	for key, value := range headers {
		w.Header()[key] = value
	}

	w.Header().Set("Content-Type","application/json")
	w.WriteHeader(status)
	w.Write(js)

	return nil
}

func (app *application) readJSON(w http.ResponseWriter, r *http.Request,dst interface{}) error {
	// decode the request body into the target dest.
	maxBytes := 1_048_576
	r.Body = http.MaxBytesReader(w,r.Body,int64(maxBytes))

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	if err != nil {
		//if there is an error during decoding start the triage
		var syntaxError *json.SyntaxError
		var unmarshallTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError

		switch {
			// Use the errors.As() function to check whether the error has the type
            // *json.SyntaxError. If it does, then return a plain-english error message // which includes the location of the problem.
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON(at character %d)",syntaxError.Offset)
		// In some circumstances Decode() may also return an io.ErrUnexpectedEOF error // for syntax errors in the JSON. So we check for this using errors.Is() and // return a generic error message. There is an open issue regarding this at
		// https://github.com/golang/go/issues/25956.

		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly formed JSON")

		case errors.As(err,&unmarshallTypeError):
			if unmarshallTypeError.Field != "" {
				return fmt.Errorf("body contains incorretc JSON type for field %q",unmarshallTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)",unmarshallTypeError.Offset)

		case errors.Is(err,io.EOF):
			return errors.New("BODY MUST NOT BE empty")

		case strings.HasPrefix(err.Error(),"json: unknown field"):
			fieldName := strings.TrimPrefix(err.Error(),"json: unknown field")
			return fmt.Errorf("body contains unknown key %s",fieldName)

		case err.Error() == "http: request body too large":
			return fmt.Errorf("body must not be larger than %d bytes",maxBytes)

		case errors.As(err,&invalidUnmarshalError):
			panic(err)
		
		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must only contain a single JSON value")
	}

	return nil
}