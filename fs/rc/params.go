// Parameter parsing

package rc

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"github.com/rclone/rclone/fs"
)

// Params is the input and output type for the Func
type Params map[string]interface{}

// ErrParamNotFound - this is returned from the Get* functions if the
// parameter isn't found along with a zero value of the requested
// item.
//
// Returning an error of this type from an rc.Func will cause the http
// method to return http.StatusBadRequest
type ErrParamNotFound string

// Error turns this error into a string
func (e ErrParamNotFound) Error() string {
	return fmt.Sprintf("Didn't find key %q in input", string(e))
}

// IsErrParamNotFound returns whether err is ErrParamNotFound
func IsErrParamNotFound(err error) bool {
	_, isNotFound := err.(ErrParamNotFound)
	return isNotFound
}

// NotErrParamNotFound returns true if err != nil and
// !IsErrParamNotFound(err)
//
// This is for checking error returns of the Get* functions to ignore
// error not found returns and take the default value.
func NotErrParamNotFound(err error) bool {
	return err != nil && !IsErrParamNotFound(err)
}

// ErrParamInvalid - this is returned from the Get* functions if the
// parameter is invalid.
//
//
// Returning an error of this type from an rc.Func will cause the http
// method to return http.StatusBadRequest
type ErrParamInvalid struct {
	error
}

// IsErrParamInvalid returns whether err is ErrParamInvalid
func IsErrParamInvalid(err error) bool {
	_, isInvalid := err.(ErrParamInvalid)
	return isInvalid
}

// Reshape reshapes one blob of data into another via json serialization
//
// out should be a pointer type
//
// This isn't a very efficient way of dealing with this!
func Reshape(out interface{}, in interface{}) error {
	b, err := json.Marshal(in)
	if err != nil {
		return errors.Wrapf(err, "Reshape failed to Marshal")
	}
	err = json.Unmarshal(b, out)
	if err != nil {
		return errors.Wrapf(err, "Reshape failed to Unmarshal")
	}
	return nil
}

// Get gets a parameter from the input
//
// If the parameter isn't found then error will be of type
// ErrParamNotFound and the returned value will be nil.
func (p Params) Get(key string) (interface{}, error) {
	value, ok := p[key]
	if !ok {
		return nil, ErrParamNotFound(key)
	}
	return value, nil
}

// GetHTTPRequest gets a http.Request parameter associated with the request with the key "_request"
//
// If the parameter isn't found then error will be of type
// ErrParamNotFound and the returned value will be nil.
func (p Params) GetHTTPRequest() (*http.Request, error) {
	key := "_request"
	value, err := p.Get(key)
	if err != nil {
		return nil, err
	}
	request, ok := value.(*http.Request)
	if !ok {
		return nil, ErrParamInvalid{errors.Errorf("expecting http.request value for key %q (was %T)", key, value)}
	}
	return request, nil
}

// GetHTTPResponseWriter gets a http.ResponseWriter parameter associated with the request with the key "_response"
//
// If the parameter isn't found then error will be of type
// ErrParamNotFound and the returned value will be nil.
func (p Params) GetHTTPResponseWriter() (*http.ResponseWriter, error) {
	key := "_response"
	value, err := p.Get(key)
	if err != nil {
		return nil, err
	}
	request, ok := value.(*http.ResponseWriter)
	if !ok {
		return nil, ErrParamInvalid{errors.Errorf("expecting *http.ResponseWriter value for key %q (was %T)", key, value)}
	}
	return request, nil
}

// GetString gets a string parameter from the input
//
// If the parameter isn't found then error will be of type
// ErrParamNotFound and the returned value will be "".
func (p Params) GetString(key string) (string, error) {
	value, err := p.Get(key)
	if err != nil {
		return "", err
	}
	str, ok := value.(string)
	if !ok {
		return "", ErrParamInvalid{errors.Errorf("expecting string value for key %q (was %T)", key, value)}
	}
	return str, nil
}

// GetInt64 gets an int64 parameter from the input
//
// If the parameter isn't found then error will be of type
// ErrParamNotFound and the returned value will be 0.
func (p Params) GetInt64(key string) (int64, error) {
	value, err := p.Get(key)
	if err != nil {
		return 0, err
	}
	switch x := value.(type) {
	case int:
		return int64(x), nil
	case int64:
		return x, nil
	case float64:
		if x > math.MaxInt64 || x < math.MinInt64 {
			return 0, ErrParamInvalid{errors.Errorf("key %q (%v) overflows int64 ", key, value)}
		}
		return int64(x), nil
	case string:
		i, err := strconv.ParseInt(x, 10, 0)
		if err != nil {
			return 0, ErrParamInvalid{errors.Wrapf(err, "couldn't parse key %q (%v) as int64", key, value)}
		}
		return i, nil
	}
	return 0, ErrParamInvalid{errors.Errorf("expecting int64 value for key %q (was %T)", key, value)}
}

// GetFloat64 gets a float64 parameter from the input
//
// If the parameter isn't found then error will be of type
// ErrParamNotFound and the returned value will be 0.
func (p Params) GetFloat64(key string) (float64, error) {
	value, err := p.Get(key)
	if err != nil {
		return 0, err
	}
	switch x := value.(type) {
	case float64:
		return x, nil
	case int:
		return float64(x), nil
	case int64:
		return float64(x), nil
	case string:
		f, err := strconv.ParseFloat(x, 64)
		if err != nil {
			return 0, ErrParamInvalid{errors.Wrapf(err, "couldn't parse key %q (%v) as float64", key, value)}
		}
		return f, nil
	}
	return 0, ErrParamInvalid{errors.Errorf("expecting float64 value for key %q (was %T)", key, value)}
}

// GetBool gets a boolean parameter from the input
//
// If the parameter isn't found then error will be of type
// ErrParamNotFound and the returned value will be false.
func (p Params) GetBool(key string) (bool, error) {
	value, err := p.Get(key)
	if err != nil {
		return false, err
	}
	switch x := value.(type) {
	case int:
		return x != 0, nil
	case int64:
		return x != 0, nil
	case float64:
		return x != 0, nil
	case bool:
		return x, nil
	case string:
		b, err := strconv.ParseBool(x)
		if err != nil {
			return false, ErrParamInvalid{errors.Wrapf(err, "couldn't parse key %q (%v) as bool", key, value)}
		}
		return b, nil
	}
	return false, ErrParamInvalid{errors.Errorf("expecting bool value for key %q (was %T)", key, value)}
}

// GetStruct gets a struct from key from the input into the struct
// pointed to by out. out must be a pointer type.
//
// If the parameter isn't found then error will be of type
// ErrParamNotFound and out will be unchanged.
func (p Params) GetStruct(key string, out interface{}) error {
	value, err := p.Get(key)
	if err != nil {
		return err
	}
	err = Reshape(out, value)
	if err != nil {
		if valueStr, ok := value.(string); ok {
			// try to unmarshal as JSON if string
			err = json.Unmarshal([]byte(valueStr), out)
			if err == nil {
				return nil
			}
		}
		return ErrParamInvalid{errors.Wrapf(err, "key %q", key)}
	}
	return nil
}

// GetStructMissingOK works like GetStruct but doesn't return an error
// if the key is missing
func (p Params) GetStructMissingOK(key string, out interface{}) error {
	_, ok := p[key]
	if !ok {
		return nil
	}
	return p.GetStruct(key, out)
}

// GetDuration get the duration parameters from in
func (p Params) GetDuration(key string) (time.Duration, error) {
	s, err := p.GetString(key)
	if err != nil {
		return 0, err
	}
	duration, err := fs.ParseDuration(s)
	if err != nil {
		return 0, ErrParamInvalid{errors.Wrap(err, "parse duration")}
	}
	return duration, nil
}
