package routes

import (
	"net/http"

	"github/marveldo/eda-monolith/internal/api/services"

	"github.com/go-chi/chi/v5"
)

func (rt *Routes) InitiateTransfer(w http.ResponseWriter, r *http.Request) {
	log := rt.RequestLogger("transfer.initiate")
	ctx, span := rt.GetServiceCtx(r.Context(), "transfer.initiate")
	if span != nil {
		defer span.End()
	}

	var body TransferRequest
	if !rt.DecodeAndValidate(w, r, log, span, &body) {
		return
	}

	transfer, appErr := rt.Services.InitiateTransfer(ctx, &services.InitiateTransferParam{
		SenderWalletID:         body.SenderWalletID,
		RecipientAccountNumber: body.RecipientAccountNumber,
		Amount:                 body.Amount,
	})
	if appErr != nil {
		rt.WriteAppError(w, log, span, appErr)
		return
	}

	rt.WriteJSON(w, http.StatusAccepted, mapTransfer(*transfer))
}

func (rt *Routes) GetTransfer(w http.ResponseWriter, r *http.Request) {
	log := rt.RequestLogger("transfer.get")
	ctx, span := rt.GetServiceCtx(r.Context(), "transfer.get")
	if span != nil {
		defer span.End()
	}

	transfer, appErr := rt.Services.GetTransfer(ctx, chi.URLParam(r, "id"))
	if appErr != nil {
		rt.WriteAppError(w, log, span, appErr)
		return
	}

	rt.WriteJSON(w, http.StatusOK, mapTransfer(*transfer))
}

func mapTransfer(transfer services.Transfer) TransferResponse {
	return TransferResponse{
		ID:                transfer.ID,
		SenderWalletID:    transfer.SenderWalletID,
		RecipientWalletID: transfer.RecipientWalletID,
		Amount:            transfer.Amount.Major(),
		Currency:          transfer.Currency,
		Status:            transfer.Status,
		FailureReason:     transfer.FailureReason,
		CreatedAt:         transfer.CreatedAt,
	}
}
