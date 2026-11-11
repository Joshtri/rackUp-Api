package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// WriteJSON sends v as JSON with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		http.Error(w, `{"error":{"code":"internal_error","message":"something went wrong"}}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(bytes.TrimRight(buf.Bytes(), "\n"))
}

// DecodeJSON reads the request body into dst.
//
//   - 415 when Content-Type isn't JSON
//   - 413 when the body is over the limit
//   - 400 when the body isn't valid JSON
//   - 422 when it is JSON but has the wrong shape (missing field, wrong type)
func DecodeJSON(r *http.Request, dst any) error {
	if !isJSONContentType(r.Header.Get("Content-Type")) {
		return BadRequest(http.StatusUnsupportedMediaType, "Expected request with `Content-Type: application/json`")
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return BadRequest(http.StatusRequestEntityTooLarge, "Failed to buffer the request body: length limit exceeded")
		}
		return BadRequest(http.StatusBadRequest, "Failed to read the request body")
	}
	if err := json.Unmarshal(body, dst); err != nil {
		msg := strings.TrimPrefix(err.Error(), "json: ")
		var syntax *json.SyntaxError
		if errors.As(err, &syntax) {
			return BadRequest(http.StatusBadRequest, "Failed to parse the request body as JSON: "+msg)
		}
		return BadRequest(http.StatusUnprocessableEntity, "Failed to deserialize the JSON body into the target type: "+msg)
	}
	return nil
}

func isJSONContentType(header string) bool {
	mt, _, err := mime.ParseMediaType(header)
	if err != nil {
		return false
	}
	return mt == "application/json" ||
		(strings.HasPrefix(mt, "application/") && strings.HasSuffix(mt, "+json"))
}

// RequireFields checks that a JSON object has every key, and none of them is null.
// Go's decoder silently leaves missing fields at their zero value; call this
// from UnmarshalJSON for fields the client must send.
func RequireFields(data []byte, keys ...string) error {
	obj, err := objectKeys(data)
	if err != nil {
		return err
	}
	for _, k := range keys {
		v, ok := obj[k]
		if !ok {
			return fmt.Errorf("missing field `%s`", k)
		}
		if string(bytes.TrimSpace(v)) == "null" {
			return fmt.Errorf("invalid type: null for field `%s`", k)
		}
	}
	return nil
}

// RequirePresent checks that a JSON object has every key. null is allowed.
func RequirePresent(data []byte, keys ...string) error {
	obj, err := objectKeys(data)
	if err != nil {
		return err
	}
	for _, k := range keys {
		if _, ok := obj[k]; !ok {
			return fmt.Errorf("missing field `%s`", k)
		}
	}
	return nil
}

func objectKeys(data []byte) (map[string]json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, errors.New("invalid type: null, expected an object")
	}
	return obj, nil
}

// PathUUID reads a {name} path segment as a UUID.
func PathUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		return uuid.Nil, BadRequest(http.StatusBadRequest, "Invalid URL: "+name+" is not a valid UUID")
	}
	return id, nil
}
