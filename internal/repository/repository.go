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
	"time"
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

func GetUserIDHash(username string) (int, string, error) {
	var userID int
	var passwordHash string

	err := db.QueryRow("SELECT id, password_hash FROM users WHERE username = $1", username).Scan(&userID, &passwordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn(fmt.Sprintf("User not found: %s", username))
			return 0, "", fmt.Errorf("user not found: %s", username)
		}
		logger.Error(fmt.Sprintf("Database error: %s", err.Error()))
		return 0, "", err
	}
	return userID, passwordHash, nil
}

func RegisterUser(username string, passwordHash string) (int, error) {
	var userID int

	err := db.QueryRow("SELECT register_user($1, $2)", username, passwordHash).Scan(&userID)
	if err != nil {
		logger.Error(fmt.Sprintf("Database error: %s", err.Error()))
		return 0, err
	}

	if userID == 0 {
		logger.Warn(fmt.Sprintf("User already exists: %s", username))
		return 0, fmt.Errorf("user already exists: %s", username)
	}

	return userID, nil
}

func GetBalances(initiatorID, userID int) ([]models.Balance, error) {
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

func TransferMoney(from int, to int, initiator int, currency string, amount int) error {
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

func GetUserID(username string) (int, error) {
	var userID int
	err := db.QueryRow("SELECT id FROM users WHERE username = $1", username).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("user not found: %s", username)
		}
		return 0, err
	}
	return userID, nil
}

func GetUsername(userID int) (string, error) {
	var username string
	err := db.QueryRow("SELECT username FROM users WHERE id = $1", userID).Scan(&username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("user not found: %d", userID)
		}
		return "", err
	}
	return username, nil
}

func GetUserPermissions(userID int) ([]int, error) {
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

func GetTransactionCount(initiatorID, userID int) (int, error) {
	var amount int
	err := db.QueryRow("SELECT * FROM get_amount_of_user_transactions($1, $2)", initiatorID, userID).Scan(&amount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("user does not have any transactions: %d", userID)
		}
		return 0, fmt.Errorf("internal server error: %s", err.Error())
	}
	return amount, nil
}

func GetTransactionsHistory(initiatorID, userID, limit, offset int) ([]models.Transaction, error) {
	var transactions []models.Transaction
	rows, err := db.Query("SELECT * FROM get_transaction_history($1, $2, $3, $4)", initiatorID, userID, limit, offset)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user does not have any transactions: %d", userID)
		}
		return nil, fmt.Errorf("internal server error: %s", err.Error())
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
			return transactions, fmt.Errorf("internal server error: %s", err.Error())
		}
		transactions = append(transactions, transaction)
	}
	return transactions, nil
}

func PrintMoney(receiverID, initiatorID, amount int, currency string) error {
	_, err := db.Exec("SELECT print_money($1, $2, $3, $4)", receiverID, initiatorID, currency, amount)
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

func SetPermission(initiatorID, userID, permissionID int) error {
	_, err := db.Exec("SELECT set_permission($1, $2, $3)", initiatorID, userID, permissionID)
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

func UnsetPermission(initiatorID, userID, permissionID int) error {
	_, err := db.Exec("SELECT unset_permission($1, $2, $3)", initiatorID, userID, permissionID)
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

func CheckRegistrationPermissions(initiatorID int) bool {
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

func ChangePassword(initiatorID, userID int, hash string) error {
	_, err := db.Exec("SELECT reset_user_password($1, $2, $3)", initiatorID, userID, hash)
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

func DoesDefaultUsersInitialized() bool {
	row := db.QueryRow("SELECT password_hash FROM users WHERE id = 1")
	var hash sql.NullString
	if err := row.Scan(&hash); err != nil {
		logger.Fatal(fmt.Sprintf("Database error: %s", err.Error()))
		return false
	}
	return hash.Valid && hash.String != ""
}

func CreateRefreshToken(userID int, expiresAt time.Time) (string, error) {
	var token string
	err := db.QueryRow("SELECT create_refresh_token($1, $2)", userID, expiresAt).Scan(&token)
	if err != nil {
		logger.Error(fmt.Sprintf("Database error (create_refresh_token): %s", err.Error()))
		return "", err
	}
	return token, nil
}

func InvalidateRefreshTokens(userID int) error {
	_, err := db.Exec("SELECT invalidate_refresh_tokens($1)", userID)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			return fmt.Errorf(pqErr.Message)
		}
		logger.Error(fmt.Sprintf("Database error (invalidate_refresh_tokens): %s", err.Error()))
		return fmt.Errorf("internal database error")
	}
	return nil
}

func GetUserByRefreshToken(token string) (int, error) {
	var userID int
	err := db.QueryRow("SELECT is_refresh_token_valid($1)", token).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return -1, nil
		}
		logger.Error(fmt.Sprintf("Database error (is_refresh_token_valid): %s", err.Error()))
		return -1, err
	}
	return userID, nil
}
