package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"

	"gbs/internal/config"
	"gbs/internal/models"
	"gbs/pkg/logger"
)

var _ Repository2 = repositoryImplementation2{}

type Repository2 interface {
	GetUserPermissions(q Querier, userID int) ([]models.Permission, error)
	GetUserIDAndPasswordHash(q Querier, username string) (int, string, error)
	RegisterUser(q Querier, username string, passwordHash string) (int, error)
	GetBalances(q Querier, userID int) ([]models.Balance, error)
	GetBalanceByCurrencyAndLock(q Querier, userID int, currency string) (models.Balance, error)
	AddBalanceAndReturnNew(q Querier, userID int, currency string, amount int) (int, error)
	SetBalance(q Querier, userID int, currency string, amount int) error
	LogTransaction(q Querier, log models.Transaction) error
	GetUserID(q Querier, username string) (int, error)
	GetUsername(q Querier, userID int) (string, error)
	GetTransactionCount(q Querier, userID int) (int, error)
	GetTransactionsHistory(q Querier, userID, limit, offset int) ([]models.Transaction, error)
}

type Querier interface {
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
	Exec(query string, args ...any) (sql.Result, error)
}

type repositoryImplementation2 struct {
	db *sql.DB
}

func NewRepositoryImplementation2(databaseConfig config.DatabaseConfig) (Repository2, error) {
	result := repositoryImplementation2{}
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

func (r repositoryImplementation2) GetUserPermissions(q Querier, userID int) ([]models.Permission, error) {
	rows, err := q.Query(`SELECT permission_id FROM user_permission WHERE user_id = $1`, userID)
	if err != nil {
		return nil, r.mapSQLErrorToGolangError(err)
	}
	defer rows.Close()

	var permissions []models.Permission
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

func (r repositoryImplementation2) GetUserIDAndPasswordHash(q Querier, username string) (int, string, error) {
	var userID int
	var passwordHash string

	err := q.QueryRow(`SELECT id, password_hash FROM users WHERE username = $1`, username).Scan(
		&userID, &passwordHash,
	)

	return userID, passwordHash, r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation2) RegisterUser(q Querier, username string, passwordHash string) (int, error) {
	var userID int
	err := q.QueryRow(
		`INSERT INTO users (username, password_hash) VALUES ($1, $2) RETURNING id`,
		username, passwordHash,
	).Scan(&userID)
	return userID, r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation2) GetBalances(q Querier, userID int) ([]models.Balance, error) {
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

func (r repositoryImplementation2) GetBalanceByCurrencyAndLock(q Querier, userID int, currency string) (
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

func (r repositoryImplementation2) SetBalance(q Querier, userID int, currency string, amount int) error {
	_, err := q.Exec(
		`INSERT INTO balances(user_id, currency, amount) VALUES ($1, $2, $3)
             ON CONFLICT (user_id, currency) 
             DO UPDATE SET amount = EXCLUDED.amount`,
		userID, currency, amount,
	)

	return r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation2) AddBalanceAndReturnNew(q Querier, userID int, currency string, amount int) (
	int, error,
) {
	var newBalance int
	err := q.QueryRow(
		`INSERT INTO balances(user_id, currency, amount) VALUES ($1, $2, $3)
             ON CONFLICT (user_id, currency) 
             DO UPDATE SET amount = balances.amount + EXCLUDED.amount 
             RETURNING amount`,
		userID, currency, amount,
	).Scan(&newBalance)

	return newBalance, r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation2) LogTransaction(q Querier, log models.Transaction) error {
	_, err := q.Exec(
		`
	INSERT INTO transaction_logs(
	    sender_id, receiver_id, initiator_id,
	    transaction_status, sender_balance_after, receiver_balance_after, currency,
	    amount, fee
    )
	VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		log.SenderID,
		log.ReceiverID,
		log.InitiatorID,
		log.TransactionStatus,
		log.SenderBalanceAfter,
		log.ReceiverBalanceAfter,
		log.Currency, log.Amount, log.Fee,
	)

	return r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation2) GetUserID(q Querier, username string) (int, error) {
	var userID int
	err := q.QueryRow(`SELECT id FROM users WHERE username = $1`, username).Scan(&userID)

	return userID, r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation2) GetUsername(q Querier, userID int) (string, error) {
	var username string
	err := q.QueryRow(`SELECT username FROM users WHERE id = $1`, userID).Scan(&username)

	return username, r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation2) GetTransactionCount(q Querier, userID int) (int, error) {
	var amount int
	err := q.QueryRow(
		`
		SELECT COUNT(*) FROM (
	    SELECT 1 
	    FROM transaction_logs
	    WHERE (sender_id = $1 OR receiver_id = $1)
	      AND transaction_status = 100
	
	    UNION ALL
	
	    SELECT 1
	    FROM print_money_logs
	    WHERE (initiator_id = $1 OR receiver_id = $1)
	      AND print_status = 200) AS transactions`,
		userID,
	).Scan(&amount)

	return amount, r.mapSQLErrorToGolangError(err)
}

func (r repositoryImplementation2) GetTransactionsHistory(q Querier, userID, limit, offset int) (
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
	WHERE (transaction_logs.sender_id = $1 OR transaction_logs.receiver_id = $1)
	  AND transaction_logs.transaction_status = 100
	
	UNION ALL

	SELECT
		-1 AS sender_id,
	    print_money_logs.receiver_id,
	    print_money_logs.initiator_id,
	    print_money_logs.currency,
	    print_money_logs.amount,
	    0 AS fee,
	    print_money_logs.created_at
	FROM print_money_logs
	WHERE print_money_logs.receiver_id = $1 OR print_money_logs.initiator_id = $1
	  AND print_money_logs.print_status = 200
	
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

func (r repositoryImplementation2) mapSQLErrorToGolangError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return &models.NotFoundError{Message: "Not Found"}
	}

	var pgErr *pq.Error
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case defaultConflictCode:
			return &models.ConflictError{Message: "Conflict (duplicate key): " + pgErr.Detail}
		default:
			return &models.ServerFaultError{Message: "Server Fault (db code " + string(pgErr.Code) + "): " + pgErr.Message}
		}
	}

	return &models.ServerFaultError{Message: "Server Fault: " + err.Error()}
}
