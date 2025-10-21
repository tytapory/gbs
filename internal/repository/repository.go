package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"gbs/internal/config"
	"gbs/internal/models"
	"gbs/pkg/logger"
)

const (
	notFoundCode         = "G0001"
	permissionDeniedCode = "G0002"
	badRequestCode       = "G0003"
	conflictCode         = "G0004"
	defaultConflictCode  = "23505"
)

var _ Repository = repositoryImplementation{}

type Repository interface {
	GetUserIDHash(username string) (int, string, error)
	RegisterUser(initiatorID int, allowDirectRegistration bool, username string, passwordHash string) (int, error)
	GetBalances(initiatorID, userID int) ([]models.Balance, error)
	TransferMoney(from int, to int, initiator int, currency string, amount int) error
	GetUserID(username string) (int, error)
	GetUsername(userID int) (string, error)
	GetUserPermissions(userID int) ([]int, error)
	GetTransactionCount(initiatorID, userID int) (int, error)
	GetTransactionsHistory(initiatorID, userID, limit, offset int) ([]models.Transaction, error)
	PrintMoney(receiverID, initiatorID, amount int, currency string) error
	SetPermission(initiatorID, userID, permissionID int) error
	UnsetPermission(initiatorID, userID, permissionID int) error
	ChangePassword(initiatorID, userID int, hash string) error
	DoesDefaultUsersInitialized() (bool, error)
	CreateRefreshToken(userID int, expiresAt time.Time) (string, error)
	InvalidateRefreshTokens(userID int) error
	GetUserByRefreshToken(token string) (int, error)
	CheckRegistrationPermissions(initiatorID int) (bool, error)
}

type repositoryImplementation struct {
	db         *sql.DB
	coreConfig config.CoreConfig
}

func NewRepositoryImplementation(databaseConfig config.DatabaseConfig, coreConfig config.CoreConfig) (
	Repository, error,
) {
	result := repositoryImplementation{coreConfig: coreConfig}
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		databaseConfig.Host, databaseConfig.Port, databaseConfig.User, databaseConfig.Password, databaseConfig.DBName,
		databaseConfig.SSLMode,
	)

	var err error
	result.db, err = sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database")
	}

	err = result.db.Ping()

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database")
	}

	logger.Info("Successfully connected to the database")

	return result, nil
}

func (r repositoryImplementation) GetUserIDHash(username string) (int, string, error) {
	var userID int
	var passwordHash string

	err := r.db.QueryRow("SELECT id, password_hash FROM users WHERE username = $1", username).Scan(
		&userID, &passwordHash,
	)

	return userID, passwordHash, r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) RegisterUser(
	initiatorID int, allowDirectRegistration bool, username string, passwordHash string,
) (int, error) {
	var userID int
	err := r.db.QueryRow(
		"SELECT register_user($1, $2, $3, $4)",
		initiatorID,
		username,
		passwordHash,
		allowDirectRegistration,
	).Scan(&userID)
	return userID, r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) GetBalances(initiatorID, userID int) ([]models.Balance, error) {
	rows, err := r.db.Query("SELECT * FROM get_balances($1, $2)", initiatorID, userID)

	var res []models.Balance
	if err != nil {
		return res, r.mapSQLErrorToGolangError(err)
	}

	defer rows.Close()
	for rows.Next() {
		var balance models.Balance

		err = rows.Scan(&balance.Currency, &balance.Amount)
		if err != nil {
			return res, r.mapSQLErrorToGolangError(err)
		}

		res = append(res, balance)
	}
	return res, nil
}

func (r repositoryImplementation) TransferMoney(from int, to int, initiator int, currency string, amount int) error {
	_, err := r.db.Exec(
		"SELECT proceed_transaction($1, $2, $3, $4, $5, $6)", from, to, initiator, currency, amount,
		r.coreConfig.CoreFee,
	)

	return r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) GetUserID(username string) (int, error) {
	var userID int
	err := r.db.QueryRow("SELECT id FROM users WHERE username = $1", username).Scan(&userID)

	return userID, r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) GetUsername(userID int) (string, error) {
	var username string
	err := r.db.QueryRow("SELECT username FROM users WHERE id = $1", userID).Scan(&username)

	return username, r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) GetUserPermissions(userID int) ([]int, error) {
	var permissions []int
	rows, err := r.db.Query("SELECT permission_id FROM user_permission WHERE user_id = $1", userID)

	if err != nil {
		return permissions, r.mapSQLErrorToGolangError(err)
	}

	defer rows.Close()
	for rows.Next() {
		var permission int
		err = rows.Scan(&permission)
		if err != nil {
			return permissions, r.mapSQLErrorToGolangError(err)
		}
		permissions = append(permissions, permission)
	}
	return permissions, nil
}

func (r repositoryImplementation) GetTransactionCount(initiatorID, userID int) (int, error) {
	var amount int
	err := r.db.QueryRow("SELECT * FROM get_amount_of_user_transactions($1, $2)", initiatorID, userID).Scan(&amount)

	return amount, r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) GetTransactionsHistory(initiatorID, userID, limit, offset int) (
	[]models.Transaction, error,
) {
	var transactions []models.Transaction
	rows, err := r.db.Query("SELECT * FROM get_transaction_history($1, $2, $3, $4)", initiatorID, userID, limit, offset)

	if err != nil {
		return nil, r.mapSQLErrorToGolangError(err)
	}

	defer rows.Close()
	for rows.Next() {
		var transaction models.Transaction
		err = rows.Scan(
			&transaction.SenderID,
			&transaction.ReceiverID,
			&transaction.InitiatorID,
			&transaction.Currency,
			&transaction.Amount,
			&transaction.Fee,
			&transaction.CreatedAt,
		)
		if err != nil {
			return transactions, r.mapSQLErrorToGolangError(err)
		}
		transactions = append(transactions, transaction)
	}

	return transactions, nil
}

func (r repositoryImplementation) PrintMoney(receiverID, initiatorID, amount int, currency string) error {
	_, err := r.db.Exec("SELECT print_money($1, $2, $3, $4)", receiverID, initiatorID, currency, amount)

	return r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) SetPermission(initiatorID, userID, permissionID int) error {
	_, err := r.db.Exec("SELECT set_permission($1, $2, $3)", initiatorID, userID, permissionID)

	return r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) UnsetPermission(initiatorID, userID, permissionID int) error {
	_, err := r.db.Exec("SELECT unset_permission($1, $2, $3)", initiatorID, userID, permissionID)

	return r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) CheckRegistrationPermissions(initiatorID int) (bool, error) {
	var allowed bool
	err := r.db.QueryRow(
		`
		SELECT EXISTS (
			SELECT 1 FROM user_permission 
			WHERE user_id = $1 
			AND permission_id IN (1, 4)
		)
	`, initiatorID,
	).Scan(&allowed)

	return allowed, r.mapSQLErrorToGolangError(err)

}

func (r repositoryImplementation) ChangePassword(initiatorID, userID int, hash string) error {
	_, err := r.db.Exec("SELECT reset_user_password($1, $2, $3)", initiatorID, userID, hash)

	return r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) DoesDefaultUsersInitialized() (bool, error) {
	var hash sql.NullString

	row := r.db.QueryRow("SELECT password_hash FROM users WHERE id = 1")
	err := row.Scan(&hash)

	return hash.Valid && hash.String != "", r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) CreateRefreshToken(userID int, expiresAt time.Time) (string, error) {
	var token string
	err := r.db.QueryRow("SELECT create_refresh_token($1, $2)", userID, expiresAt).Scan(&token)

	return token, r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) InvalidateRefreshTokens(userID int) error {
	_, err := r.db.Exec("SELECT invalidate_refresh_tokens($1)", userID)

	return r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) GetUserByRefreshToken(token string) (int, error) {
	var userID int
	err := r.db.QueryRow("SELECT is_refresh_token_valid($1)", token).Scan(&userID)
	return userID, r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) mapSQLErrorToGolangError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return &models.NotFoundError{Message: "Not Found"}
	}

	var pgErr *pq.Error
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case notFoundCode:
			return &models.NotFoundError{Message: pgErr.Message}
		case permissionDeniedCode:
			return &models.PermissionError{Message: pgErr.Message}
		case badRequestCode:
			return &models.BadRequestError{Message: pgErr.Message}
		case conflictCode:
			return &models.ConflictError{Message: pgErr.Message}
		case defaultConflictCode:
			return &models.ConflictError{Message: "Conflict (duplicate key): " + pgErr.Detail}
		default:
			return &models.ServerFaultError{Message: "Server Fault (db code " + string(pgErr.Code) + "): " + pgErr.Message}
		}
	}

	return &models.ServerFaultError{Message: "Server Fault: " + err.Error()}
}
