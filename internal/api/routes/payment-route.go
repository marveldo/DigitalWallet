package routes

import (
	"io"
	"net/http"

	"github/marveldo/eda-monolith/internal/api/providers"
	"github/marveldo/eda-monolith/internal/api/services"

	"github.com/go-chi/chi/v5"
)

const MaxWebhookBodyBytes = 1 << 20

func (rt *Routes) InitializeDeposit(w http.ResponseWriter, r *http.Request) {
	log := rt.RequestLogger("payment.deposit.initialize")
	ctx, span := rt.GetServiceCtx(r.Context(), "payment.deposit.initialize")
	if span != nil {
		defer span.End()
	}

	var body InitializeDepositRequest
	if !rt.DecodeAndValidate(w, r, log, span, &body) {
		return
	}

	result, appErr := rt.Services.InitializeDeposit(ctx, &services.InitializeDepositParam{
		Amount:   body.Amount,
		Currency: body.Currency,
	})
	if appErr != nil {
		rt.WriteAppError(w, log, span, appErr)
		return
	}

	rt.WriteJSON(w, http.StatusCreated, DepositResponse{
		Reference:        result.Reference,
		AuthorizationURL: result.AuthorizationURL,
		AccessCode:       result.AccessCode,
		Amount:           result.Amount.Major(),
		Currency:         result.Currency,
		Status:           result.Status,
	})
}

func (rt *Routes) PaymentWebhook(w http.ResponseWriter, r *http.Request) {
	log := rt.RequestLogger("payment.webhook")
	ctx, span := rt.GetServiceCtx(r.Context(), "payment.webhook")
	if span != nil {
		defer span.End()
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, MaxWebhookBodyBytes))
	if err != nil {
		rt.WriteError(w, log, span, http.StatusBadRequest, "Could Not Read Webhook Body", err)
		return
	}

	signature := r.Header.Get(providers.PaystackSignatureHeader)
	if appErr := rt.Services.HandlePaymentWebhook(ctx, signature, body); appErr != nil {
		rt.WriteAppError(w, log, span, appErr)
		return
	}

	rt.WriteJSON(w, http.StatusOK, MessageResponse{Message: "Webhook Received"})
}

func (rt *Routes) GetTransaction(w http.ResponseWriter, r *http.Request) {
	log := rt.RequestLogger("payment.transaction.get")
	ctx, span := rt.GetServiceCtx(r.Context(), "payment.transaction.get")
	if span != nil {
		defer span.End()
	}

	transaction, appErr := rt.Services.GetTransaction(ctx, chi.URLParam(r, "reference"))
	if appErr != nil {
		rt.WriteAppError(w, log, span, appErr)
		return
	}

	rt.WriteJSON(w, http.StatusOK, mapTransaction(*transaction))
}

func (rt *Routes) ListMyTransactions(w http.ResponseWriter, r *http.Request) {
	log := rt.RequestLogger("payment.transaction.list")
	ctx, span := rt.GetServiceCtx(r.Context(), "payment.transaction.list")
	if span != nil {
		defer span.End()
	}

	limit, err := rt.QueryInt(r, "limit")
	if err != nil {
		rt.WriteError(w, log, span, http.StatusBadRequest, "Invalid limit", err)
		return
	}
	offset, err := rt.QueryInt(r, "offset")
	if err != nil {
		rt.WriteError(w, log, span, http.StatusBadRequest, "Invalid offset", err)
		return
	}

	result, appErr := rt.Services.ListMyTransactions(ctx, &services.ActivityPageParam{Limit: limit, Offset: offset})
	if appErr != nil {
		rt.WriteAppError(w, log, span, appErr)
		return
	}

	transactions := make([]TransactionResponse, 0, len(result.Transactions))
	for _, transaction := range result.Transactions {
		transactions = append(transactions, mapTransaction(transaction))
	}

	rt.WriteJSON(w, http.StatusOK, TransactionListResponse{
		Transactions: transactions,
		Total:        result.Total,
		Limit:        result.Limit,
		Offset:       result.Offset,
	})
}

func mapTransaction(transaction services.Transaction) TransactionResponse {
	return TransactionResponse{
		Reference: transaction.Reference,
		WalletID:  transaction.WalletID,
		Amount:    transaction.Amount.Major(),
		Status:    transaction.Status,
		Provider:  transaction.Provider,
		CreatedAt: transaction.CreatedAt,
	}
}
