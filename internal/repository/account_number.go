package repository

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strconv"
)

const (

	accountNumberMin  = 1_000_000_000
	accountNumberSpan = 9_000_000_000
	AccountNumberLength = 10
	accountNumberAttempts = 5
)
var ErrAccountNumberExhausted = errors.New("could not allocate a unique account number")
func NewAccountNumber() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(accountNumberSpan))
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(n.Int64()+accountNumberMin, 10), nil
}
