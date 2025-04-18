package jsonHelper

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New(validator.WithRequiredStructEnabled())
}

func DecodeJSON(r io.Reader, obj interface{}) error {
	decoder := json.NewDecoder(r)
	// decoder.DisallowUnknownFields()
	if err := decoder.Decode(obj); err != nil {
		return err
	}
	return nil
}

func DecodeJSONAndValidate(r io.Reader, obj interface{}, ignoreUnknown bool) error {

	// Decode
	decoder := json.NewDecoder(r)

	if !ignoreUnknown {
		decoder.DisallowUnknownFields()
	}

	if err := decoder.Decode(obj); err != nil {
		return err
	}

	// Validate
	val := reflect.ValueOf(obj).Elem()
	switch val.Kind() {
	case reflect.Array, reflect.Slice:
		for i := 0; i < val.Len(); i++ {
			validationError := validate.Struct(val.Index(i))
			if validationError != nil {
				return validationError
			}
		}
	default:
		validationError := validate.Struct(obj)
		if validationError != nil {
			return validationError
		}
	}
	return nil
}

func ServeJson(data interface{}) []byte {
	value, _ := json.Marshal(data)
	if value != nil {
		return value
	}
	return nil
}

func HttpResponse(data interface{}, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	if error := json.NewEncoder(w).Encode(&data); error != nil {
		println(error.Error())
	}
}

func HttpErrorResponse(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	httpErr := HTTPError{
		Code:    status,
		Message: err.Error(),
	}
	if error := json.NewEncoder(w).Encode(&httpErr); error != nil {
		println(error.Error())
	}
}

// HTTPError
type HTTPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
