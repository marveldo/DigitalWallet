package routes

import (
	"net/http"

	"github/marveldo/eda-monolith/internal/api/services"

	"github.com/go-chi/chi/v5"
)

func (rt *Routes) CreateWallet(w http.ResponseWriter, r *http.Request) {
	log := rt.RequestLogger("wallet.create")
	ctx, span := rt.GetServiceCtx(r.Context(), "wallet.create")
	if span != nil {
		defer span.End()
	}

	var body CreateWalletRequest
	if !rt.DecodeAndValidate(w, r, log, span, &body) {
		return
	}

	wallet, appErr := rt.Services.CreateWallet(ctx, body.Currency)
	if appErr != nil {
		rt.WriteAppError(w, log, span, appErr)
		return
	}

	rt.WriteJSON(w, http.StatusCreated, mapWallet(*wallet))
}

func (rt *Routes) ListMyWallets(w http.ResponseWriter, r *http.Request) {
	log := rt.RequestLogger("wallet.list")
	ctx, span := rt.GetServiceCtx(r.Context(), "wallet.list")
	if span != nil {
		defer span.End()
	}

	result, appErr := rt.Services.ListMyWallets(ctx)
	if appErr != nil {
		rt.WriteAppError(w, log, span, appErr)
		return
	}

	wallets := make([]WalletResponse, 0, len(result))
	for _, wallet := range result {
		wallets = append(wallets, mapWallet(wallet))
	}
	rt.WriteJSON(w, http.StatusOK, WalletListResponse{Wallets: wallets})
}

func (rt *Routes) GetWallet(w http.ResponseWriter, r *http.Request) {
	log := rt.RequestLogger("wallet.get")
	ctx, span := rt.GetServiceCtx(r.Context(), "wallet.get")
	if span != nil {
		defer span.End()
	}

	wallet, appErr := rt.Services.GetWallet(ctx, chi.URLParam(r, "id"))
	if appErr != nil {
		rt.WriteAppError(w, log, span, appErr)
		return
	}

	rt.WriteJSON(w, http.StatusOK, mapWallet(*wallet))
}

func mapWallet(wallet services.Wallet) WalletResponse {
	return WalletResponse{
		ID:       wallet.ID,
		Currency: wallet.Currency,
		Status:   wallet.Status,
		Balance:  wallet.Balance.Major(),
	}
}
