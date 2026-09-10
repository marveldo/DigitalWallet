package shared

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const DefaultOTPLength = 6

func GenerateOTP(length int) (string, error) {
	if length <= 0 {
		length = DefaultOTPLength
	}
	digits := make([]byte, length)
	for i := range digits {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", fmt.Errorf("generating otp: %w", err)
		}
		digits[i] = byte('0' + n.Int64())
	}
	return string(digits), nil
}
