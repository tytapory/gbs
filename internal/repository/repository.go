package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"gbs/internal/config"
	"gbs/internal/models"
	"gbs/pkg/logger"
)

const (
	PgUniqueViolation     = "23505"
	PgForeignKeyViolation = "23503"
	PgCheckViolation      = "23514"
	PgNotNullViolation    = "23502"
)

var _ Repository = repositoryImplementation{}

type Repository interface {
	GetUserPermissions(q Querier, userID uuid.UUID) ([]models.Permission, error)
	GetUserIDAndPasswordHash(q Querier, username string) (uuid.UUID, string, error)
	RegisterUser(q Querier, username string, passwordHash string) (uuid.UUID, error)
	GetBalances(q Querier, userID uuid.UUID) ([]models.Balance, error)
	GetBalanceByCurrencyAndLock(q Querier, userID uuid.UUID, currency string) (models.Balance, error)
	AddBalanceAndReturnNew(q Querier, userID uuid.UUID, currency string, amount int64) (int64, error)
	SetBalance(q Querier, userID uuid.UUID, currency string, amount int64) error
	LogTransaction(q Querier, log models.Transaction) error
	GetUserID(q Querier, username string) (uuid.UUID, error)
	GetUsername(q Querier, userID uuid.UUID) (string, error)
	GetTransactionCount(q Querier, userID uuid.UUID) (int64, error)
	GetTransactionsHistory(q Querier, userID uuid.UUID, limit, offset int) ([]models.Transaction, error)
	SetPermission(q Querier, userID uuid.UUID, permission models.Permission) error
	UnsetPermission(q Querier, userID uuid.UUID, permission models.Permission) error
	ChangePassword(q Querier, userID uuid.UUID, hash string) error
	DoesDefaultUsersInitialized(q Querier) (bool, error)
	CreateRefreshToken(q Querier, userID uuid.UUID, expiresAt time.Time) (uuid.UUID, error)
	InvalidateRefreshTokens(q Querier, userID uuid.UUID) error
	GetUserByRefreshToken(q Querier, token uuid.UUID) (uuid.UUID, error)
	NewTransaction() Querier
	CommitTransaction(q Querier) error
	RollbackTransaction(q Querier) error
	NewSingleQuery() Querier
	Close() error
}

type Querier interface {
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
	Exec(query string, args ...any) (sql.Result, error)
}

type repositoryImplementation struct {
	db *sql.DB
}

func NewRepositoryImplementation(databaseConfig config.DatabaseConfig) (Repository, error) {
	result := repositoryImplementation{}
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

func (r repositoryImplementation) Close() error {
	logger.Info("Closing database connections")
	return r.db.Close()
}

func (r repositoryImplementation) GetUserPermissions(q Querier, userID uuid.UUID) ([]models.Permission, error) {
	rows, err := q.Query(`SELECT permission_id FROM user_permission WHERE user_id = $1`, userID)
	if err != nil {
		return nil, r.mapSQLErrorToGolangError(err)
	}
	defer rows.Close()

	permissions := make([]models.Permission, 0)
	for rows.Next() {
		var permission models.Permission

		if err := rows.Scan(&permission); err != nil {
			return nil, r.mapSQLErrorToGolangError(err)
		}

		permissions = append(permissions, permission)
	}

	if err = rows.Err(); err != nil {
		return nil, r.mapSQLErrorToGolangError(err)
	}

	return permissions, nil
}

func (r repositoryImplementation) GetUserIDAndPasswordHash(q Querier, username string) (uuid.UUID, string, error) {
	var userID uuid.UUID
	var passwordHash string

	err := q.QueryRow(`SELECT id, password_hash FROM users WHERE username = $1`, username).Scan(
		&userID, &passwordHash,
	)

	return userID, passwordHash, r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) RegisterUser(q Querier, username string, passwordHash string) (uuid.UUID, error) {
	newUserID, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, err
	}

	_, err = q.Exec(
		`INSERT INTO users (id, username, password_hash) VALUES ($1, $2, $3)`,
		newUserID, username, passwordHash,
	)

	if err != nil {
		return uuid.Nil, r.mapSQLErrorToGolangError(err)
	}

	return newUserID, nil
}

func (r repositoryImplementation) GetBalances(q Querier, userID uuid.UUID) ([]models.Balance, error) {
	rows, err := q.Query(`SELECT balances.currency, balances.amount FROM balances WHERE user_id = $1`, userID)
	if err != nil {
		return nil, r.mapSQLErrorToGolangError(err)
	}
	defer rows.Close()

	var balances []models.Balance
	for rows.Next() {
		var balance models.Balance

		if err = rows.Scan(&balance.Currency, &balance.Amount); err != nil {
			return nil, r.mapSQLErrorToGolangError(err)
		}

		balances = append(balances, balance)
	}

	if err = rows.Err(); err != nil {
		return nil, r.mapSQLErrorToGolangError(err)
	}

	return balances, nil
}

func (r repositoryImplementation) GetBalanceByCurrencyAndLock(q Querier, userID uuid.UUID, currency string) (
	models.Balance, error,
) {
	var balance models.Balance
	err := q.QueryRow(
		`SELECT currency, amount FROM balances 
        WHERE user_id = $1 AND currency = $2 
        FOR UPDATE`,
		userID, currency,
	).Scan(&balance.Currency, &balance.Amount)

	return balance, r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) SetBalance(q Querier, userID uuid.UUID, currency string, newAmount int64) error {
	_, err := q.Exec(
		`UPDATE balances 
        SET amount = $1 
        WHERE user_id = $2 AND currency = $3`,
		newAmount, userID, currency,
	)
	return r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) AddBalanceAndReturnNew(q Querier, userID uuid.UUID, currency string, amount int64) (
	int64, error,
) {
	var newBalance int64
	err := q.QueryRow(
		`INSERT INTO balances(user_id, currency, amount) VALUES ($1, $2, $3)
             ON CONFLICT (user_id, currency) 
             DO UPDATE SET amount = balances.amount + EXCLUDED.amount 
             RETURNING amount`,
		userID, currency, amount,
	).Scan(&newBalance)

	return newBalance, r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) LogTransaction(q Querier, log models.Transaction) error {
	newLogID, err := uuid.NewV7()
	if err != nil {
		return err
	}

	_, err = q.Exec(
		`
    INSERT INTO transaction_logs(
        id,
        sender_id, receiver_id, initiator_id, sender_balance_after, receiver_balance_after, currency,
        amount, fee
    )
    VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		newLogID,
		log.SenderID,
		log.ReceiverID,
		log.InitiatorID,
		log.SenderBalanceAfter,
		log.ReceiverBalanceAfter,
		log.Currency, log.Amount, log.Fee,
	)

	return r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) GetUserID(q Querier, username string) (uuid.UUID, error) {
	var userID uuid.UUID
	err := q.QueryRow(`SELECT id FROM users WHERE username = $1`, username).Scan(&userID)

	return userID, r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) GetUsername(q Querier, userID uuid.UUID) (string, error) {
	var username string
	err := q.QueryRow(`SELECT username FROM users WHERE id = $1`, userID).Scan(&username)

	return username, r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) GetTransactionCount(q Querier, userID uuid.UUID) (int64, error) {
	var amount int64
	err := q.QueryRow(
		`
	    SELECT COUNT(*)
	    FROM transaction_logs
	    WHERE sender_id = $1 OR receiver_id = $1 OR initiator_id = $1`,
		userID,
	).Scan(&amount)

	return amount, r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) GetTransactionsHistory(q Querier, userID uuid.UUID, limit, offset int) (
	[]models.Transaction, error,
) {
	rows, err := q.Query(
		`
	SELECT
	    transaction_logs.sender_id,
	    transaction_logs.receiver_id,
	    transaction_logs.initiator_id,
	    transaction_logs.currency,
	    transaction_logs.amount,
	    transaction_logs.fee,
	    transaction_logs.created_at
	FROM transaction_logs
	WHERE transaction_logs.sender_id = $1 OR transaction_logs.receiver_id = $1 OR transaction_logs.initiator_id = $1
	
	ORDER BY created_at DESC
	OFFSET $2 LIMIT $3`, userID, offset, limit,
	)

	if err != nil {
		return nil, r.mapSQLErrorToGolangError(err)
	}
	defer rows.Close()

	transactions := make([]models.Transaction, 0, limit)
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
			return nil, r.mapSQLErrorToGolangError(err)
		}

		transactions = append(transactions, transaction)
	}

	if err = rows.Err(); err != nil {
		return nil, r.mapSQLErrorToGolangError(err)
	}

	return transactions, nil
}

func (r repositoryImplementation) SetPermission(q Querier, userID uuid.UUID, permission models.Permission) error {
	_, err := q.Exec(
		`
	INSERT INTO user_permission (user_id, permission_id)
	VALUES ($1, $2)
    ON CONFLICT DO NOTHING`, userID, permission,
	)

	return r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) UnsetPermission(q Querier, userID uuid.UUID, permission models.Permission) error {
	_, err := q.Exec(
		`
	DELETE FROM user_permission
	WHERE user_id = $1
	  AND permission_id = $2`, userID, permission,
	)

	return r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) ChangePassword(q Querier, userID uuid.UUID, hash string) error {
	_, err := q.Exec(
		`
	UPDATE users
	SET password_hash = $1
	WHERE id = $2`, hash, userID,
	)

	return r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) DoesDefaultUsersInitialized(q Querier) (bool, error) {
	var hash sql.NullString

	row := q.QueryRow("SELECT password_hash FROM users WHERE username = 'adm'")
	err := row.Scan(&hash)

	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}

	return hash.Valid && hash.String != "", r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) CreateRefreshToken(q Querier, userID uuid.UUID, expiresAt time.Time) (
	uuid.UUID, error,
) {
	newToken, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, err
	}

	_, err = q.Exec(
		`INSERT INTO refresh_tokens(token, user_id, expires_at) VALUES ($1, $2, $3)`,
		newToken, userID, expiresAt,
	)
	if err != nil {
		return uuid.Nil, r.mapSQLErrorToGolangError(err)
	}

	return newToken, nil
}
func (r repositoryImplementation) InvalidateRefreshTokens(q Querier, userID uuid.UUID) error {
	_, err := q.Exec(
		`
	UPDATE refresh_tokens
	SET revoked = true
	WHERE user_id = $1
	  AND revoked = false
	  AND expires_at > now()`, userID,
	)

	return r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) GetUserByRefreshToken(q Querier, token uuid.UUID) (uuid.UUID, error) {
	var userID uuid.UUID
	err := q.QueryRow(
		`
	SELECT user_id 
    FROM refresh_tokens
    WHERE token = $1
      AND revoked = false
      AND expires_at > now()
        LIMIT 1`, token,
	).Scan(&userID)

	return userID, r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) NewTransaction() Querier {
	tx, err := r.db.Begin()
	if err != nil {
		logger.Error("Failed to begin transaction: " + err.Error())
		return nil
	}

	return tx
}

func (r repositoryImplementation) CommitTransaction(q Querier) error {
	tx, ok := q.(*sql.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	err := tx.Commit()
	return r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) RollbackTransaction(q Querier) error {
	tx, ok := q.(*sql.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	err := tx.Rollback()
	return r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation) NewSingleQuery() Querier {
	return r.db
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
		case PgUniqueViolation:
			return &models.ConflictError{Message: "Conflict: " + pgErr.Message}

		case PgForeignKeyViolation:
			return &models.NotFoundError{Message: "Related entity not found: " + pgErr.Message}

		case PgCheckViolation:
			return &models.BadRequestError{Message: "Check constraint violation: " + pgErr.Message}

		case PgNotNullViolation:
			return &models.BadRequestError{Message: "Not null violation: " + pgErr.Message}

		default:
			return &models.ServerFaultError{Message: "Unhandled DB error (code " + string(pgErr.Code) + "): " + pgErr.Message}
		}
	}

	return &models.ServerFaultError{Message: "Server Fault: " + err.Error()}
}
