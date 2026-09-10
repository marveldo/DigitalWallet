package services

import "golang.org/x/crypto/bcrypt"

var cost = 12

func HashPassword(password []byte) ([]byte, error) {

	hashed_password, err := bcrypt.GenerateFromPassword(password, cost)
	if err != nil {
		return nil, err
	}
	return hashed_password, nil
}

func CompareHash(hashedPassword []byte, password []byte) error {
	return bcrypt.CompareHashAndPassword(hashedPassword, password)
}
