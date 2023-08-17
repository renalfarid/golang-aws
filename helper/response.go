// helper/response.go

package helper

import (
	"encoding/json"
	"net/http"
)

// Response represents the JSON response format
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// SuccessResponse sends a JSON success response
func SuccessResponse(w http.ResponseWriter, code int, message string, data interface{}) {
	response := Response{
		Code:    code,
		Message: message,
		Data:    data,
	}

	sendJSONResponse(w, code, response)
}

// ErrorResponse sends a JSON error response
func ErrorResponse(w http.ResponseWriter, code int, message string) {
	response := Response{
		Code:    code,
		Message: message,
	}

	sendJSONResponse(w, code, response)
}

func sendJSONResponse(w http.ResponseWriter, code int, response Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
