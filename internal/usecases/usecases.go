package usecases

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gbs/internal/auth"
	"gbs/internal/config"
	"gbs/internal/models"
	"gbs/internal/repository"
)

const feesUserID = 2

var _ UseCases = useCasesImplementation{}

type UseCases interface {
	GetTransactionsHistory(initiatorID, userID, limit, offset int) ([]models.Transaction, error)
	Register(login, password string, initiatorID int) (models.AuthResponse, error)
	Login(login, password string) (models.AuthResponse, error)
	GetTransactionCount(initiatorID, userID int) (int64, error)
	GetUserPermissions(userID int) ([]models.Permission, error)
	GetUserID(username string) (int, error)
	GetUsername(userID int) (string, error)
	GetBalances(initiatorID, userID int) ([]models.Balance, error)
	TransferMoney(senderID, receiverID, initiatorID int, currency string, amount int64) error
	PrintMoney(receiverID, initiatorID int, amount int64, currency string) error
	RefreshJWT(refreshToken string) (string, error)
	TogglePermission(initiatorID, userID int, permissionID models.Permission, enable bool) error
	ChangePassword(initiatorID, userID int, password string) error
	GetUserIDFromJWT(tokenString string) (int, error)
}

type useCasesImplementation struct {
	repo           repository.Repository
	auth           auth.AuthService
	securityConfig config.SecurityConfig
	coreConfig     config.CoreConfig
}

func NewUseCasesImplementation(
	repo repository.Repository, auth auth.AuthService, securityConfig config.SecurityConfig,
	coreConfig config.CoreConfig,
) UseCases {
	return useCasesImplementation{repo: repo, auth: auth, securityConfig: securityConfig, coreConfig: coreConfig}
}

func (u useCasesImplementation) GetTransactionsHistory(initiatorID, userID, limit, offset int) (
	[]models.Transaction, error,
) {
	q := u.repo.NewSingleQuery()

	if initiatorID != userID {
		initiatorPerms, err := u.repo.GetUserPermissions(q, initiatorID)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to get user permissions for initiatorID=%d: %w",
				initiatorID, err,
			)
		}

		if !u.hasPermission(initiatorPerms, []models.Permission{models.Administrator, models.AuditFunds}) {
			return nil, models.InitiatorPermissionError{Message: "insufficient permissions"}
		}
	}

	history, err := u.repo.GetTransactionsHistory(q, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get transactions history for userID=%d, initiatorID=%d, limit=%d, offset=%d: %w",
			userID, initiatorID, limit, offset, err,
		)
	}

	return history, nil
}

func (u useCasesImplementation) Register(login, password string, initiatorID int) (
	resp models.AuthResponse, err error,
) {
	q := u.repo.NewTransaction()

	defer func() {
		if err != nil {
			_ = u.repo.RollbackTransaction(q)
		} else {
			err = u.repo.CommitTransaction(q)
		}
	}()

	if !u.securityConfig.AllowDirectRegistration {
		if err = u.checkRegistrationPermission(q, initiatorID); err != nil {
			return
		}
	}

	if !u.auth.ValidateUsername(login) {
		err = &models.InvalidUsernameError{Message: "username is invalid"}

		return
	}

	if !u.auth.ValidatePassword(password) {
		err = &models.InvalidPasswordError{Message: "password is invalid"}

		return
	}

	hash, err := u.auth.GeneratePasswordHash(password)
	if err != nil {
		return
	}

	userID, err := u.repo.RegisterUser(q, login, hash)
	if err != nil {
		return
	}

	token, err := u.auth.GenerateJWT(userID)
	if err != nil {
		return
	}

	refreshToken, err := u.generateRefreshToken(q, userID)
	if err != nil {
		return
	}

	resp = models.AuthResponse{
		Token: token, TokenExpiry: u.securityConfig.TokenExpiry, RefreshToken: refreshToken,
		RefreshTokenExpiry: u.securityConfig.RefreshTokenExpiry,
	}

	return
}

func (u useCasesImplementation) Login(login, password string) (resp models.AuthResponse, err error) {
	q := u.repo.NewTransaction()

	defer func() {
		if err != nil {
			_ = u.repo.RollbackTransaction(q)
		} else {
			err = u.repo.CommitTransaction(q)
		}
	}()

	id, hash, err := u.repo.GetUserIDAndPasswordHash(q, login)

	if err != nil || !u.auth.CompareHashes(hash, password) {
		var notFound *models.NotFoundError
		if err != nil && !errors.As(err, &notFound) {
			return
		}

		err = &models.PermissionError{Message: "invalid username or password"}

		return
	}

	token, err := u.auth.GenerateJWT(id)
	if err != nil {
		return
	}

	refreshToken, err := u.generateRefreshToken(q, id)
	if err != nil {
		return
	}

	resp = models.AuthResponse{
		Token: token, TokenExpiry: u.securityConfig.TokenExpiry, RefreshToken: refreshToken,
		RefreshTokenExpiry: u.securityConfig.RefreshTokenExpiry,
	}

	return
}

func (u useCasesImplementation) GetTransactionCount(initiatorID, userID int) (int64, error) {
	q := u.repo.NewSingleQuery()

	if initiatorID != userID {
		initiatorPerms, err := u.repo.GetUserPermissions(q, initiatorID)
		if err != nil {
			return 0, fmt.Errorf(
				"failed to get user permissions for initiatorID=%d: %w",
				initiatorID, err,
			)
		}

		if !u.hasPermission(initiatorPerms, []models.Permission{models.Administrator, models.AuditFunds}) {
			return 0, models.InitiatorPermissionError{Message: "insufficient permissions"}
		}
	}

	count, err := u.repo.GetTransactionCount(q, userID)
	if err != nil {
		return 0, fmt.Errorf(
			"failed to get transactions count for userID=%d, initiatorID=%d: %w",
			userID, initiatorID, err,
		)
	}

	return count, nil
}

func (u useCasesImplementation) GetUserPermissions(userID int) ([]models.Permission, error) {
	q := u.repo.NewSingleQuery()

	permissions, err := u.repo.GetUserPermissions(q, userID)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get permissions for userID=%d: %w",
			userID, err,
		)
	}

	return permissions, nil
}

func (u useCasesImplementation) GetUserID(username string) (int, error) {
	q := u.repo.NewSingleQuery()

	userID, err := u.repo.GetUserID(q, username)
	if err != nil {
		return 0, fmt.Errorf(
			"failed to get userID for username=%s: %w",
			username, err,
		)
	}

	return userID, nil
}

func (u useCasesImplementation) GetUsername(userID int) (string, error) {
	q := u.repo.NewSingleQuery()

	username, err := u.repo.GetUsername(q, userID)
	if err != nil {
		return "", fmt.Errorf(
			"failed to get username for userID=%d: %w",
			userID, err,
		)
	}

	return username, nil
}

func (u useCasesImplementation) GetBalances(initiatorID, userID int) ([]models.Balance, error) {
	q := u.repo.NewSingleQuery()

	if initiatorID != userID {
		initiatorPerms, err := u.repo.GetUserPermissions(q, initiatorID)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to get user permissions for initiatorID=%d: %w",
				initiatorID, err,
			)
		}

		if !u.hasPermission(initiatorPerms, []models.Permission{models.Administrator, models.AuditFunds}) {
			return nil, models.InitiatorPermissionError{Message: "Insufficient permissions"}
		}
	}

	balances, err := u.repo.GetBalances(q, userID)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get balances for initiatorID=%d, userID=%d: %w",
			initiatorID, userID, err,
		)
	}

	return balances, nil
}

func (u useCasesImplementation) TransferMoney(
	senderID, receiverID, initiatorID int, currency string, amount int64,
) (err error) {
	q := u.repo.NewTransaction()

	defer func() {
		if err != nil {
			_ = u.repo.RollbackTransaction(q)
		} else {
			err = u.repo.CommitTransaction(q)
		}
	}()

	if err = u.checkTransactionPermission(q, senderID, receiverID, initiatorID); err != nil {
		return
	}

	senderBalance, err := u.repo.GetBalanceByCurrencyAndLock(q, senderID, currency)
	if err != nil {
		err = fmt.Errorf(
			"failed to get balance for senderID=%d, currency=%s: %w",
			senderID, currency, err,
		)

		return
	}

	if senderBalance.Amount < amount {
		return models.NotEnoughFundsError{Message: "sender balance is too low"}
	}

	senderBalance.Amount -= amount

	err = u.repo.SetBalance(q, senderID, currency, senderBalance.Amount)
	if err != nil {
		err = fmt.Errorf(
			"failed to update balance for senderID=%d, currency=%s: %w",
			senderID, currency, err,
		)

		return
	}

	commissionAmount := (amount*u.coreConfig.CoreFee + 9999) / 10000
	newReceiverBalance, err := u.repo.AddBalanceAndReturnNew(q, receiverID, currency, amount-commissionAmount)
	if err != nil {
		err = fmt.Errorf(
			"failed to update balance for receiverID=%d, currency=%s: %w",
			receiverID, currency, err,
		)

		return
	}

	_, err = u.repo.AddBalanceAndReturnNew(q, feesUserID, currency, amount-commissionAmount)
	if err != nil {
		err = fmt.Errorf(
			"failed to update balance for receiverID=%d, currency=%s: %w",
			feesUserID, currency, err,
		)

		return
	}

	transaction := models.Transaction{
		SenderID:             &senderID,
		ReceiverID:           receiverID,
		InitiatorID:          initiatorID,
		SenderBalanceAfter:   &senderBalance.Amount,
		ReceiverBalanceAfter: newReceiverBalance,
		Currency:             currency,
		Amount:               amount,
		Fee:                  commissionAmount,
		CreatedAt:            time.Now(),
	}

	err = u.repo.LogTransaction(q, transaction)
	if err != nil {
		transactionJSON, marshalErr := json.Marshal(transaction)
		if marshalErr != nil {
			transactionJSON = []byte(fmt.Sprintf("failed to marshal transaction: %v", marshalErr))
		}

		err = fmt.Errorf(
			"failed to log transaction %s: %w",
			string(transactionJSON), err,
		)
	}

	return
}

func (u useCasesImplementation) PrintMoney(receiverID, initiatorID int, amount int64, currency string) (err error) {
	q := u.repo.NewSingleQuery()

	initiatorPermissions, err := u.repo.GetUserPermissions(q, initiatorID)
	if err != nil {
		err = fmt.Errorf(
			"failed to get permissions for initiatorID=%d: %w",
			initiatorID, err,
		)

		return
	}

	if !u.hasPermission(initiatorPermissions, []models.Permission{models.Administrator, models.PrintMoney}) {
		err = models.InitiatorPermissionError{Message: "insufficient permissions"}

		return
	}

	q = u.repo.NewTransaction()

	defer func() {
		if err != nil {
			_ = u.repo.RollbackTransaction(q)
		} else {
			err = u.repo.CommitTransaction(q)
		}
	}()

	newReceiverBalance, err := u.repo.AddBalanceAndReturnNew(q, receiverID, currency, amount)
	if err != nil {
		err = fmt.Errorf(
			"failed to update balance for receiverID=%d, currency=%s: %w",
			receiverID, currency, err,
		)

		return
	}

	transaction := models.Transaction{
		ReceiverID:           receiverID,
		InitiatorID:          initiatorID,
		ReceiverBalanceAfter: newReceiverBalance,
		Currency:             currency,
		Amount:               amount,
		Fee:                  0,
		CreatedAt:            time.Now(),
	}

	err = u.repo.LogTransaction(q, transaction)
	if err != nil {
		transactionJSON, marshalErr := json.Marshal(transaction)
		if marshalErr != nil {
			transactionJSON = []byte(fmt.Sprintf("failed to marshal transaction: %v", marshalErr))
		}

		err = fmt.Errorf(
			"failed to log transaction %s: %w",
			string(transactionJSON), err,
		)
	}

	return
}

func (u useCasesImplementation) RefreshJWT(refreshToken string) (string, error) {
	q := u.repo.NewSingleQuery()

	userID, err := u.repo.GetUserByRefreshToken(q, refreshToken)
	if err != nil {
		return "", err
	}

	token, err := u.auth.GenerateJWT(userID)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (u useCasesImplementation) TogglePermission(
	initiatorID, userID int, permissionID models.Permission, enable bool,
) error {
	q := u.repo.NewSingleQuery()

	initiatorPermissions, err := u.repo.GetUserPermissions(q, initiatorID)
	if err != nil {
		return fmt.Errorf(
			"failed to get permissions for initiatorID=%d: %w",
			initiatorID, err,
		)
	}

	if !u.hasPermission(initiatorPermissions, []models.Permission{models.Administrator, models.ManagePermissions}) {
		return models.InitiatorPermissionError{Message: "insufficient permissions"}
	}

	if enable {
		return u.repo.SetPermission(q, userID, permissionID)
	} else {
		return u.repo.UnsetPermission(q, userID, permissionID)
	}
}

func (u useCasesImplementation) ChangePassword(initiatorID, userID int, password string) (err error) {
	if initiatorID != userID {
		var initiatorPerms []models.Permission

		initiatorPerms, err = u.repo.GetUserPermissions(u.repo.NewSingleQuery(), initiatorID)
		if err != nil {
			err = fmt.Errorf(
				"failed to get user permissions for initiatorID=%d: %w",
				initiatorID, err,
			)

			return
		}

		if !u.hasPermission(initiatorPerms, []models.Permission{models.Administrator, models.ControlAccounts}) {
			err = models.InitiatorPermissionError{Message: "insufficient permissions"}

			return
		}
	}

	if !u.auth.ValidatePassword(password) {
		err = &models.InvalidPasswordError{Message: "password is invalid"}

		return
	}
	hash, err := u.auth.GeneratePasswordHash(password)
	if err != nil {
		err = &models.ServerFaultError{Message: "unexpected error during generate password hash: " + err.Error()}

		return
	}

	q := u.repo.NewTransaction()

	defer func() {
		if err != nil {
			_ = u.repo.RollbackTransaction(q)
		} else {
			err = u.repo.CommitTransaction(q)
		}
	}()

	err = u.repo.ChangePassword(q, userID, hash)
	if err != nil {
		return
	}

	err = u.repo.InvalidateRefreshTokens(q, userID)

	return
}

func (u useCasesImplementation) GetUserIDFromJWT(tokenString string) (int, error) {
	return u.auth.GetUserIDFromJWT(tokenString)
}

func (u useCasesImplementation) checkRegistrationPermission(q repository.Querier, initiatorID int) error {
	initiatorPerms, err := u.repo.GetUserPermissions(q, initiatorID)
	if err != nil {
		err = fmt.Errorf("failed to get user permissions for initiatorID=%d: %w", initiatorID, err)

		return err
	}

	if !u.hasPermission(initiatorPerms, []models.Permission{models.Administrator, models.ControlAccounts}) {
		err = models.InitiatorPermissionError{
			Message: "direct registration is disabled and initiator don't have registration permissions",
		}
	}

	return err
}

func (u useCasesImplementation) checkTransactionPermission(
	q repository.Querier, senderID int, receiverID int, initiatorID int,
) error {
	senderPerms, err := u.repo.GetUserPermissions(q, senderID)
	if err != nil {
		return fmt.Errorf("failed to get sender permissions for senderID=%d: %w", senderID, err)
	}

	if u.hasPermission(senderPerms, []models.Permission{models.Administrator, models.ManageFunds}) {
		return nil
	}

	if !u.hasPermission(senderPerms, []models.Permission{models.SendFunds}) {
		return models.SenderPermissionError{
			Message: "insufficient permissions",
		}
	}

	receiverPerms, err := u.repo.GetUserPermissions(q, receiverID)
	if err != nil {
		return fmt.Errorf("failed to get receiver permissions for receiverID=%d: %w", receiverID, err)
	}

	if !u.hasPermission(receiverPerms, []models.Permission{models.ReceiveFunds}) {
		return models.RecipientPermissionError{
			Message: "insufficient permissions",
		}
	}

	return nil
}

func (u useCasesImplementation) hasPermission(
	userPermissions []models.Permission, appropriatePermissions []models.Permission,
) bool {
	for _, userPerm := range userPermissions {
		for _, appropriatePerm := range appropriatePermissions {
			if userPerm == appropriatePerm {
				return true
			}
		}
	}

	return false
}

func (u useCasesImplementation) generateRefreshToken(q repository.Querier, userID int) (string, error) {
	duration, err := time.ParseDuration(u.securityConfig.RefreshTokenExpiry)
	if err != nil {
		return "", &models.ServerFaultError{Message: "invalid refresh token lifespan"}
	}

	newRefreshToken, err := u.repo.CreateRefreshToken(q, userID, time.Now().Add(duration))
	if err != nil {
		return "", err
	}

	return newRefreshToken, nil
}
