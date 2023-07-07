package data

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrInvalidRuntimeFormat = errors.New("invalid runtime format")

type Runtime int32

func (r Runtime) MarshalJSON() ([]byte,error) {
	jsonValue := fmt.Sprintf("%d mins",r)

	quotedJSONValue := strconv.Quote(jsonValue)

	return []byte(quotedJSONValue), nil
}

func (r *Runtime) UnmarshalJSON(jsonValue []byte) error {
	unquotedJsonValue, err := strconv.Unquote(string(jsonValue))
	if err!= nil {
		return ErrInvalidRuntimeFormat
	}

	parts := strings.Split(unquotedJsonValue," ")

	// Sanity check the parts of the string to make sure it was in the expected format. // If it isn't, we return the ErrInvalidRuntimeFormat error again.
	if len(parts)!= 2 || parts[1]!="mins"{
		return ErrInvalidRuntimeFormat
	}

	i, err := strconv.ParseInt(parts[0],10,32)
	if err != nil {
		return ErrInvalidRuntimeFormat
	}

	// Convert the int32 to a Runtime type and assign this to the receiver. Note that we // use the * operator to deference the receiver (which is a pointer to a Runtime
    // type) in order to set the underlying value of the pointer.
	*r = Runtime(i)
	return nil
}