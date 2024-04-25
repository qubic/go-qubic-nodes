package web

import (
	"encoding/json"
	"github.com/qubic/go-qubic-nodes/node"

	"log"
	"net/http"
)

type RequestHandler struct {
	Container *node.Container `json:"data"`
}

func (handler *RequestHandler) HandleStatus(writer http.ResponseWriter, request *http.Request) {

	data, err := json.Marshal(handler)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		writer.Write([]byte(err.Error()))
		return
	}

	writer.Header().Add("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, err = writer.Write(data)
	if err != nil {
		log.Printf("Failed to write response for status request. Err: %v\n", err)
	}

}
