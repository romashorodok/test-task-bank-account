package httphandler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/romashorodok/test-task-bank-account/account/pkg/model/account"

	// "github.com/romashorodok/test-task-bank-account/account/pkg/query"
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
	// log.Println(("Create account handler run"))
	// if err := cqrs.Dispatch(a.commandBus, r.Context(), command.NewCreateAccountCommand()); err != nil {
	// 	writeError(w, http.StatusInternalServerError, err)
	// 	return
	// }
}

func (a *AccountHandler) getAccounts(w http.ResponseWriter, r *http.Request) {
	// result, err := cqrs.DispatchQuery(a.queryBus, r.Context(), query.NewGetAccountsQuery())
	// if err != nil {
	// 	writeError(w, http.StatusNotFound, err)
	// 	return
	// }
	// json.NewEncoder(w).Encode(result)
}

func (a *AccountHandler) withdraw(w http.ResponseWriter, r *http.Request) {
	// id := chi.URLParam(r, "id")
	// log.Println(("Withdraw handler run"))
	//
	// if err := cqrs.Dispatch(a.commandBus, r.Context(), command.NewWithdrawAccountCommand(id, 20)); err != nil {
	// 	writeError(w, http.StatusBadRequest, err)
	// 	return
	// }
}

func (a *AccountHandler) deposit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	log.Println(("Deposit handler run"))

	if err := cqrs.Dispatch(a.commandBus, r.Context(), account.NewDepositAccountEvent(id, 20)); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	// json.NewEncoder(w).Encode(v any)
}

func (h *AccountHandler) RegisterHandler(router *chi.Mux) {
	router.Route("/account", func(r chi.Router) {
		r.Use(httputil.JSONResponse)

		r.Get("/", h.getAccounts)
		r.Post("/", h.createAccount)

		r.Post("/{id}/withdraw", h.withdraw)
		r.Post("/{id}/deposit", h.deposit)
	})
}

func NewAccountHandler(queryBus *cqrs.BusContext, commandBus *cqrs.BusRabbitMQ) *AccountHandler {
	return &AccountHandler{
		queryBus:   queryBus,
		commandBus: commandBus,
	}
}
