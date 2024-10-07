package httphandler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/romashorodok/test-task-bank-account/account/pkg/query"
	"github.com/romashorodok/test-task-bank-account/contrib/cqrs"
	"github.com/romashorodok/test-task-bank-account/contrib/httputil"
)

type ErrorResponse struct {
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, statusCode int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	json.NewEncoder(w).Encode(ErrorResponse{
		Message: err.Error(),
	})
}

type AccountHandler struct {
	queryBus   *cqrs.BusContext
	commandBus *cqrs.BusRabbitMQ
}

func (a *AccountHandler) createAccount(w http.ResponseWriter, r *http.Request) {
}

func (a *AccountHandler) getAccounts(w http.ResponseWriter, r *http.Request) {
	result, err := cqrs.DispatchQuery(a.queryBus, r.Context(), query.NewGetAccountsQuery())
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}

	json.NewEncoder(w).Encode(result)
}

func (a *AccountHandler) withdraw(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_ = id
}

func (a *AccountHandler) deposit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	log.Println("deposit route", id)
	_ = id
}

func (h *AccountHandler) RegisterHandler() {
	handlers := map[string]http.HandlerFunc{
		"POST /account":               h.createAccount,
		"GET /account":                h.getAccounts,
		"POST /account/{id}/withdraw": h.withdraw,
		"POST /account/{id}/deposit":  h.deposit,
	}

	for route, handler := range handlers {
		http.HandleFunc(route, httputil.JSONResponse(handler))
	}
}

func NewAccountHandler(queryBus *cqrs.BusContext, commandBus *cqrs.BusRabbitMQ) *AccountHandler {
	return &AccountHandler{
		queryBus:   queryBus,
		commandBus: commandBus,
	}
}
