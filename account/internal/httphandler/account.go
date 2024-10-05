package httphandler

import (
	"encoding/json"
	"net/http"
)

type AccountHandler struct{}

func (a *AccountHandler) createAccount(w http.ResponseWriter, r *http.Request) {
}

type getAccountResult struct {
	ID string `json:"id"`
}

func (a *AccountHandler) getAccount(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(&getAccountResult{
		"ID",
	})
}

func (a *AccountHandler) withdraw(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_ = id
}

func (a *AccountHandler) deposit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_ = id
}

func (h *AccountHandler) RegisterHandler() {
	handlers := map[string]http.HandlerFunc{
		"POST /account":               h.createAccount,
		"GET /account":                h.getAccount,
		"POST /account/{id}/withdraw": h.withdraw,
		"POST /account/{id}/deposit":  h.deposit,
	}

	for route, handler := range handlers {
		http.HandleFunc(route, handler)
	}
}

func NewAccountHandler() *AccountHandler {
	return nil
}
