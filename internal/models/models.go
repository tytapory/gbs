package models

import (
	"time"

	"github.com/google/uuid"
)

type Permission int

const (
	// while it can be declared using itoa it is representing sql permissions so it is more accurate to just use ids from db
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
	Token              string    `json:"token"`
	TokenExpiry        string    `json:"token_expiry"`
	RefreshToken       uuid.UUID `json:"refresh_token"`
	RefreshTokenExpiry string    `json:"refresh_token_expiry"`
}

type RefreshRequest struct {
	RefreshToken uuid.UUID `json:"refresh_token"`
}

type RefreshResponse struct {
	Token string `json:"token"`
}

type BalanceResponse struct {
	Balances []Balance `json:"balances"`
}

type Balance struct {
	Currency string `json:"currency"`
	Amount   int64  `json:"amount"`
}

type TransactionRequest struct {
	From     uuid.UUID `json:"from"`
	To       uuid.UUID `json:"to"`
	Currency string    `json:"currency"`
	Amount   int64     `json:"amount"`
}

type IDResponse struct {
	ID uuid.UUID `json:"id"`
}

type UserPermissionsResponse struct {
	Permissions []Permission `json:"permissions"`
}

type TransactionAmountResponse struct {
	Amount int64 `json:"amount"`
}

type Transaction struct {
	SenderID             *uuid.UUID `json:"sender_id,omitempty"`
	ReceiverID           uuid.UUID  `json:"receiver_id,omitempty"`
	InitiatorID          uuid.UUID  `json:"initiator_id,omitempty"`
	SenderBalanceAfter   *int64     `json:"sender_balance_after,omitempty"`
	ReceiverBalanceAfter int64      `json:"receiver_balance_after,omitempty"`
	Currency             string     `json:"currency,omitempty"`
	Amount               int64      `json:"amount,omitempty"`
	Fee                  *int64     `json:"fee,omitempty"`
	CreatedAt            time.Time  `json:"created_at,omitempty"`
}

type TransactionResponse struct {
	Transactions []Transaction `json:"transactions"`
}

type PrintMoneyRequest struct {
	ReceiverID uuid.UUID `json:"receiver_id"`
	Currency   string    `json:"currency"`
	Amount     int64     `json:"amount"`
}

type ModifyPermissionRequest struct {
	PermissionID Permission `json:"permission_id"`
	UserID       uuid.UUID  `json:"user_id"`
	Enabled      bool       `json:"enabled"`
}

type ChangePasswordRequest struct {
	UserID   uuid.UUID `json:"user_id"`
	Password string    `json:"password"`
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
