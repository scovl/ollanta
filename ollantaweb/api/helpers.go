package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// jsonOK writes a JSON response with the given status code.
func jsonOK(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// jsonError writes a JSON error response.
func jsonError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// routeParam extracts a URL path parameter by name.
func routeParam(r *http.Request, name string) string {
	return chi.URLParam(r, name)
}

// parseID extracts and parses an int64 route parameter.
func parseID(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(routeParam(r, name), 10, 64)
}
