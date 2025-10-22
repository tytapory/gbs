package models

import "time"

type Permission int

const (
	Administrator     Permission = 1
	ManagePermissions Permission = 2
	ManageFunds       Permission = 3
	ControlAccounts   Permission = 4
	PrintMoney        Permission = 5
	AuditFunds        Permission = 6
	ReceiveFunds      Permission = 7
	SendFunds         Permission = 8
)

type ErrorResponse struct {
	Message string `json:"message"`
}

type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token              string `json:"token"`
	TokenExpiry        string `json:"token_expiry"`
	RefreshToken       string `json:"refresh_token"`
	RefreshTokenExpiry string `json:"refresh_token_expiry"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshResponse struct {
	Token string `json:"token"`
}

type BalanceResponse struct {
	Balances []Balance `json:"balances"`
}

type Balance struct {
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
}

type TransactionRequest struct {
	From     int    `json:"from"`
	To       int    `json:"to"`
	Currency string `json:"currency"`
	Amount   int    `json:"amount"`
}

type IDResponse struct {
	ID int `json:"id"`
}

type UserPermissionsResponse struct {
	Permissions []int `json:"permissions"`
}

type TransactionAmountResponse struct {
	Amount int `json:"amount"`
}

type Transaction struct {
	SenderID             *int      `json:"sender_id,omitempty"`
	ReceiverID           int       `json:"receiver_id,omitempty"`
	InitiatorID          int       `json:"initiator_id,omitempty"`
	TransactionStatus    int       `json:"transaction_status,omitempty"`
	SenderBalanceAfter   *int64    `json:"sender_balance_after,omitempty"`
	ReceiverBalanceAfter int64     `json:"receiver_balance_after,omitempty"`
	Currency             string    `json:"currency,omitempty"`
	Amount               int64     `json:"amount,omitempty"`
	Fee                  int64     `json:"fee,omitempty"`
	CreatedAt            time.Time `json:"created_at,omitempty"`
}

type TransactionResponse struct {
	Transactions []Transaction `json:"transactions"`
}

type PrintMoneyRequest struct {
	ReceiverID int    `json:"receiver_id"`
	Currency   string `json:"currency"`
	Amount     int    `json:"amount"`
}

type ModifyPermissionRequest struct {
	PermissionID int  `json:"permission_id"`
	UserID       int  `json:"user_id"`
	Enabled      bool `json:"enabled"`
}

type ChangePasswordRequest struct {
	UserID   int    `json:"user_id"`
	Password string `json:"password"`
}

type UsernameResponse struct {
	Username string `json:"username"`
}

type LoginAttempt struct {
	Count        int
	BlockedUntil time.Time
}

type RateLimitInfo struct {
	Requests  int
	ResetTime time.Time
}
