package ledger

import "fmt"


type LedgerID uint32

const (
	LedgerNGN LedgerID = 1
	LedgerUSD LedgerID = 2
	LedgerEUR LedgerID = 3
	LedgerGBP LedgerID = 4
	LedgerJPY LedgerID = 5
	LedgerAUD LedgerID = 6
	LedgerCAD LedgerID = 7
	LedgerCHF LedgerID = 8
	LedgerCNY LedgerID = 9
	LedgerSEK LedgerID = 10
	LedgerNZD LedgerID = 11
)

var currencyLedgers = map[string]LedgerID{
	"NGN": LedgerNGN,
	"USD": LedgerUSD,
	"EUR": LedgerEUR,
	"GBP": LedgerGBP,
	"JPY": LedgerJPY,
	"AUD": LedgerAUD,
	"CAD": LedgerCAD,
	"CHF": LedgerCHF,
	"CNY": LedgerCNY,
	"SEK": LedgerSEK,
	"NZD": LedgerNZD,
}

func LedgerForCurrency(currency string) (LedgerID, error) {
	id, ok := currencyLedgers[currency]
	if !ok {
		return 0, fmt.Errorf("no ledger for currency %q", currency)
	}
	return id, nil
}

// AccountCode is the TigerBeetle account code: what kind of account it is.
type AccountCode uint16

const (
	AccountWallet     AccountCode = 1
	AccountSettlement AccountCode = 2
	AccountFees       AccountCode = 3
)

// TransferCode is the TigerBeetle transfer code: why the money moved.
type TransferCode uint16

const (
	TransferDeposit    TransferCode = 1
	TransferInternal   TransferCode = 2
	TransferWithdrawal TransferCode = 3
	TransferFee        TransferCode = 4
)
