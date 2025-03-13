package models

type ErrorResponse struct {
	Message string `json:"message"`
}

type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
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
	SenderID   int    `json:"sender_id"`
	ReceiverID int    `json:"receiver_id"`
	Initiator  int    `json:"initiator"`
	Currency   string `json:"currency"`
	Amount     int    `json:"amount"`
	Fee        int    `json:"fee"`
	CreatedAt  int64  `json:"created_at"`
}

type PrintMoneyRequest struct {
	ReceiverID int    `json:"receiver_id"`
	Currency   string `json:"currency"`
	Amount     int    `json:"amount"`
}
