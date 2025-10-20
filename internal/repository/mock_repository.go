package repository

import (
	"time"

	"gbs/internal/models"
	"gbs/internal/tests"
)

var _ Repository = repositoryMock{}

type repositoryMock struct{}

func (r repositoryMock) GetUserIDHash(username string) (int, string, error) {
	switch username {
	case tests.GoodUsername:
		return tests.GoodUserID, tests.GoodPasswordHash, nil
	case tests.NonExistingUser:
		return 0, "", &models.NotFoundError{Message: "Not Found"}
	}

	panic("unreachable")
}

func (r repositoryMock) RegisterUser(
	initiatorID int, allowDirectRegistration bool, username string, passwordHash string,
) (int, error) {
	switch {
	case initiatorID == tests.NoInitiatorID && allowDirectRegistration && username == tests.GoodUsername && passwordHash == tests.GoodPasswordHash:
		return tests.GoodUserID, nil
	case initiatorID == tests.NoInitiatorID && !allowDirectRegistration && username == tests.GoodUsername && passwordHash == tests.GoodPasswordHash:
		return 0, &models.PermissionError{Message: "Registration: Public registration is disabled"}
	case initiatorID == tests.WithoutPermissionsID && !allowDirectRegistration && username == tests.GoodUsername && passwordHash == tests.GoodPasswordHash:
		return 0, &models.PermissionError{Message: "Registration: Insufficient permissions"}
	case initiatorID == tests.GoodUserID && !allowDirectRegistration && username == tests.DuplicateUsername && passwordHash == tests.DuplicatePassword:
		return 0, &models.ConflictError{Message: "Conflict (duplicate key): mock details"}
	}

	panic("unreachable")
}

func (r repositoryMock) GetBalances(initiatorID, userID int) ([]models.Balance, error) {
	switch {
	case initiatorID == tests.GoodUserID && userID == tests.GoodUserID:
		return tests.GoodBalances, nil
	case initiatorID == tests.WithoutPermissionsID && userID == tests.GoodUserID:
		return nil, &models.PermissionError{Message: "Get balance: Insufficient permissions"}
	case initiatorID == tests.GoodUserID && userID == tests.NonExistingUserID:
		return tests.EmptyBalances, nil
	case initiatorID == tests.GoodUserID && userID == tests.NoMoneyUserID:
		return tests.EmptyBalances, nil
	}

	panic("unreachable")
}

func (r repositoryMock) TransferMoney(from int, to int, initiator int, currency string, amount int) error {
	//TODO implement me
	panic("implement me")
}

func (r repositoryMock) GetUserID(username string) (int, error) {
	//TODO implement me
	panic("implement me")
}

func (r repositoryMock) GetUsername(userID int) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (r repositoryMock) GetUserPermissions(userID int) ([]int, error) {
	//TODO implement me
	panic("implement me")
}

func (r repositoryMock) GetTransactionCount(initiatorID, userID int) (int, error) {
	//TODO implement me
	panic("implement me")
}

func (r repositoryMock) GetTransactionsHistory(initiatorID, userID, limit, offset int) ([]models.Transaction, error) {
	//TODO implement me
	panic("implement me")
}

func (r repositoryMock) PrintMoney(receiverID, initiatorID, amount int, currency string) error {
	//TODO implement me
	panic("implement me")
}

func (r repositoryMock) SetPermission(initiatorID, userID, permissionID int) error {
	//TODO implement me
	panic("implement me")
}

func (r repositoryMock) UnsetPermission(initiatorID, userID, permissionID int) error {
	//TODO implement me
	panic("implement me")
}

func (r repositoryMock) ChangePassword(initiatorID, userID int, hash string) error {
	//TODO implement me
	panic("implement me")
}

func (r repositoryMock) DoesDefaultUsersInitialized() (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (r repositoryMock) CreateRefreshToken(userID int, expiresAt time.Time) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (r repositoryMock) InvalidateRefreshTokens(userID int) error {
	//TODO implement me
	panic("implement me")
}

func (r repositoryMock) GetUserByRefreshToken(token string) (int, error) {
	//TODO implement me
	panic("implement me")
}

func (r repositoryMock) CheckRegistrationPermissions(initiatorID int) (bool, error) {
	//TODO implement me
	panic("implement me")
}
