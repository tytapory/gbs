package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"gbs/internal/config"
	"gbs/internal/models"
	"gbs/pkg/logger"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

var db *sql.DB

func InitDB() {
	cfg := config.GetConfig()

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode,
	)

	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to open database: %s %s", dsn, err.Error()))
	}

	if err = db.Ping(); err != nil {
		logger.Fatal(fmt.Sprintf("Failed to connect to database: %s %s", dsn, err.Error()))
	}

	logger.Info("Successfully connected to the database")
}

var GetUserIDHash = func(username string) (int, string, error) {
	var userID int
	var passwordHash string

	err := db.QueryRow("SELECT id, password_hash FROM users WHERE username = $1", username).Scan(&userID, &passwordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn(fmt.Sprintf("User not found: %s", username))
			return 0, "", fmt.Errorf("User not found: %s", username)
		}
		logger.Error(fmt.Sprintf("Database error: %s", err.Error()))
		return 0, "", err
	}

	return userID, passwordHash, nil
}

var RegisterUser = func(username string, passwordHash string) (int, error) {
	var userID int

	err := db.QueryRow("SELECT register_user($1, $2)", username, passwordHash).Scan(&userID)
	if err != nil {
		logger.Error(fmt.Sprintf("Database error: %s", err.Error()))
		return 0, err
	}

	if userID == 0 {
		logger.Warn(fmt.Sprintf("User already exists: %s", username))
		return 0, fmt.Errorf("User already exists: %s", username)
	}

	return userID, nil
}

var GetBalances = func(initiatorID, userID int) ([]models.Balance, error) {
	rows, err := db.Query("SELECT * FROM get_balances($1, $2)", initiatorID, userID)
	var res []models.Balance
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			return nil, fmt.Errorf(pqErr.Message)
		}
		if errors.Is(err, sql.ErrNoRows) {
			return res, nil
		}
		logger.Error(fmt.Sprintf("Database error: %s", err.Error()))
		return res, err
	}
	defer rows.Close()
	for rows.Next() {
		var balance models.Balance
		err = rows.Scan(&balance.Currency, &balance.Amount)
		if err != nil {
			logger.Error(fmt.Sprintf("Database error: %s", err.Error()))
			return res, err
		}
		res = append(res, balance)
	}
	return res, nil
}

var TransferMoney = func(from int, to int, initiator int, currency string, amount int) error {
	_, err := db.Exec("SELECT proceed_transaction($1, $2, $3, $4, $5, $6)", from, to, initiator, currency, amount, config.GetConfig().Core.CoreFee)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			return fmt.Errorf(pqErr.Message)
		} else {
			logger.Error(fmt.Sprintf("Database error: %s", err.Error()))
			return fmt.Errorf("internal database error")
		}
	}
	return nil
}

var GetUserID = func(username string) (int, error) {
	var userID int
	err := db.QueryRow("SELECT id FROM users WHERE username = $1", username).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("User not found: %s", username)
		}
		return 0, err
	}
	return userID, nil
}

var GetUserPermissions = func(userID int) ([]int, error) {
	var permissions []int
	rows, err := db.Query("SELECT permission_id FROM user_permission WHERE user_id = $1", userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return permissions, nil
		}
		logger.Error(fmt.Sprintf("Database error: %s", err.Error()))
		return permissions, err
	}
	defer rows.Close()
	for rows.Next() {
		var permission int
		err = rows.Scan(&permission)
		if err != nil {
			logger.Error(fmt.Sprintf("Database error: %s", err.Error()))
			return permissions, err
		}
		permissions = append(permissions, permission)
	}
	return permissions, nil
}

var GetTransactionCount = func(initiatorID, userID int) (int, error) {
	var amount int
	err := db.QueryRow("SELECT * FROM get_amount_of_user_transactions($1, $2)", initiatorID, userID).Scan(&amount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("User do not have any transactions: %s", userID)
		}
		return 0, fmt.Errorf("Internal server error", err.Error())
	}
	return amount, nil
}

var GetTransactionsHistory = func(initiatorID, userID, limit, offset int) ([]models.Transaction, error) {
	var transactions []models.Transaction
	rows, err := db.Query("SELECT * FROM get_transaction_history($1, $2, $3, $4)", initiatorID, userID, limit, offset)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("User do not have any transactions: %s", userID)
		}
		return nil, fmt.Errorf("Internal server error", err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		var transaction models.Transaction
		err = rows.Scan(
			&transaction.SenderID,
			&transaction.ReceiverID,
			&transaction.Initiator,
			&transaction.Currency,
			&transaction.Amount,
			&transaction.Fee,
			&transaction.CreatedAt,
		)
		if err != nil {
			logger.Error(fmt.Sprintf("Database error: %s", err.Error()))
			return transactions, fmt.Errorf("Internal server error", err.Error())
		}
		transactions = append(transactions, transaction)
	}
	return transactions, nil
}

var PrintMoney = func(receiver_id, initiator_id, amount int, currency string) error {
	_, err := db.Exec("SELECT print_money($1, $2, $3, $4)", receiver_id, initiator_id, currency, amount)
	if pqErr, ok := err.(*pq.Error); ok {
		return fmt.Errorf(pqErr.Message)
	} else if err != nil {
		logger.Error(fmt.Sprintf("Database error: %s", err.Error()))
		return fmt.Errorf("internal database error")
	}
	return nil
}

var SetPermission = func(initiator_id, user_id, permission_id int) error {
	_, err := db.Exec("SELECT set_permission($1, $2, $3)", initiator_id, user_id, permission_id)
	if pqErr, ok := err.(*pq.Error); ok {
		return fmt.Errorf(pqErr.Message)
	} else if err != nil {
		logger.Error(fmt.Sprintf("Database error: %s", err.Error()))
		return fmt.Errorf("internal database error")
	}
	return nil
}

var UnsetPermission = func(initiator_id, user_id, permission_id int) error {
	_, err := db.Exec("SELECT unset_permission($1, $2, $3)", initiator_id, user_id, permission_id)
	if pqErr, ok := err.(*pq.Error); ok {
		return fmt.Errorf(pqErr.Message)
	} else if err != nil {
		logger.Error(fmt.Sprintf("Database error: %s", err.Error()))
		return fmt.Errorf("internal database error")
	}
	return nil
}

var CheckRegistrationPermissions = func(initiatorID int) bool {
	var allowed bool

	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM user_permission 
			WHERE user_id = $1 
			AND permission_id IN (1, 4)
		)
	`, initiatorID).Scan(&allowed)

	if err != nil {
		logger.Error(fmt.Sprintf("Database error: %s", err.Error()))
		return false
	}
	return allowed
}

var ChangePassword = func(initiatorID, userID int, hash string) error {
	_, err := db.Exec("SELECT reset_user_password($1, $2, $3)", initiatorID, userID, hash)
	if pqErr, ok := err.(*pq.Error); ok {
		return fmt.Errorf(pqErr.Message)
	} else if err != nil {
		logger.Error(fmt.Sprintf("Database error: %s", err.Error()))
		return fmt.Errorf("internal database error")
	}
	return nil
}

var DoesDefaultUsersInitialized = func() bool {
	row := db.QueryRow("SELECT password_hash FROM users WHERE id = 1")
	var hash sql.NullString
	if err := row.Scan(&hash); err != nil {
		logger.Fatal(fmt.Sprintf("Database error: %s", err.Error()))
		return false
	}
	return hash.Valid && hash.String != ""
}
