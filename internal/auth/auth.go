package auth

import (
	"gbs/pkg/logger"

	"golang.org/x/crypto/bcrypt"
)

var registerUser = func() {

}

var generateAuthJWT = func() {

}

var generateRefreshJWT = func() {

}

var refreshJWT = func() {

}

var getUserIDFromJWT = func() {

}

var validateUsername = func() {

}

var validatePassword = func() {

}

var generatePasswordHash = func(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("Hash generation error")
		return "", err
	}
	return hashedPassword, nil
}

var compareHashes = func(hash string, password string) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err != nil
}
