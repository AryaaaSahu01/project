package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

const (
	MaxEntryBytes int64 = 1 << 20
	MaxSealBytes  int64 = 4 << 10
)

func DecodeJSON(
	w http.ResponseWriter,
	r *http.Request,
	dst any,
	maxBytes int64,
) (int, error) {
	mediaType, _, err := mime.ParseMediaType(
		r.Header.Get("Content-Type"),
	)

	if err != nil || mediaType != "application/json" {
		return http.StatusUnsupportedMediaType,
			errors.New("Content-Type must be application/json")
	}

	reader := http.MaxBytesReader(w, r.Body, maxBytes)

	defer reader.Close()

	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return decodeError(err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return http.StatusBadRequest,
				errors.New("request must contain exactly one JSON value")
		}

		return decodeError(err)
	}
	return 0, nil
}

func decodeError(err error) (int, error) {
	var tooLarge *http.MaxBytesError

	if errors.As(err, &tooLarge) {
		return http.StatusRequestEntityTooLarge,
			errors.New("request body  too large")
	}

	return http.StatusBadRequest,
		errors.New("invalid JSON request body")
}
