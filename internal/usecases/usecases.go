package usecases

import (
	"fmt"

	"gbs/internal/auth"
	"gbs/internal/models"
	"gbs/internal/repository"
)

var _ UseCases = useCasesImplementation{}

type UseCases interface {
	GetTransactionsHistory(targetUserID, initiatorID, limit, offset int) ([]models.Transaction, error)
	RegisterUser(login, password string, initiatorID int) (models.AuthResponse, error)
	Login(login, password string) (models.AuthResponse, error)
	GetTransactionCount(initiatorID, userID int) (int, error)
	GetUserPermissions(userID int) ([]int, error)
	GetUserID(username string) (int, error)
	GetUsername(userID int) (string, error)
	GetBalances(initiatorID, userID int) ([]models.Balance, error)
	TransferMoney(from int, to int, initiator int, currency string, amount int) error
	PrintMoney(receiverID, initiatorID, amount int, currency string) error
	RefreshJWT(refreshToken string) (string, error)
	TogglePermission(initiatorID, userID, permissionID int, enable bool) error
	ChangePassword(initiatorID, userID int, password string) error
	GetUserIDFromJWT(tokenString string) (int, error)
}

type useCasesImplementation struct {
	repo repository.Repository
	auth auth.AuthService
}

func NewUseCasesImplementation(repo repository.Repository, auth auth.AuthService) UseCases {
	return useCasesImplementation{repo: repo, auth: auth}
}

func (u useCasesImplementation) GetTransactionsHistory(targetUserID, initiatorID, limit, offset int) (
	[]models.Transaction, error,
) {
	history, err := u.repo.GetTransactionsHistory(initiatorID, targetUserID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf(
			"GetTransactionsHistory: Failed to get transactions history for targetUserID=%d, initiatorID=%d, limit=%d, offset=%d: %w",
			targetUserID, initiatorID, err.Error(),
		)
	}

	return history, nil
}

func (u useCasesImplementation) RegisterUser(login, password string, initiatorID int) (models.AuthResponse, error) {
	return u.auth.RegisterUser(login, password, initiatorID)
}

func (u useCasesImplementation) Login(login, password string) (models.AuthResponse, error) {
	return u.auth.Login(login, password)
}

func (u useCasesImplementation) GetTransactionCount(initiatorID, userID int) (int, error) {
	return u.repo.GetTransactionCount(initiatorID, userID)
}

func (u useCasesImplementation) GetUserPermissions(userID int) ([]int, error) {
	return u.repo.GetUserPermissions(userID)
}

func (u useCasesImplementation) GetUserID(username string) (int, error) {
	return u.repo.GetUserID(username)
}

func (u useCasesImplementation) GetUsername(userID int) (string, error) {
	return u.repo.GetUsername(userID)
}

func (u useCasesImplementation) GetBalances(initiatorID, userID int) ([]models.Balance, error) {
	return u.repo.GetBalances(initiatorID, userID)
}

func (u useCasesImplementation) TransferMoney(from int, to int, initiator int, currency string, amount int) error {
	return u.repo.TransferMoney(from, to, initiator, currency, amount)
}

func (u useCasesImplementation) PrintMoney(receiverID, initiatorID, amount int, currency string) error {
	return u.repo.PrintMoney(receiverID, initiatorID, amount, currency)
}

func (u useCasesImplementation) RefreshJWT(refreshToken string) (string, error) {
	return u.auth.RefreshJWT(refreshToken)
}

func (u useCasesImplementation) TogglePermission(initiatorID, userID, permissionID int, enable bool) error {
	if enable {
		return u.repo.SetPermission(initiatorID, userID, permissionID)
	} else {
		return u.repo.UnsetPermission(initiatorID, userID, permissionID)
	}
}

func (u useCasesImplementation) ChangePassword(initiatorID, userID int, password string) error {
	return u.auth.ChangePassword(initiatorID, userID, password)
}

func (u useCasesImplementation) GetUserIDFromJWT(tokenString string) (int, error) {
	return u.auth.GetUserIDFromJWT(tokenString)
}
