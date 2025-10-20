package tests

import "gbs/internal/models"

var (
	GoodUserID       = 1
	GoodUsername     = "good_username"
	GoodPassword     = "password"
	GoodPasswordHash = "$2a$10$Q90.kkijFZtOf3bXSj2XOeDe/oIOrJghDNNjf2rOw.lsuGihW6fYO%"

	WithoutPermissionsID       = 2
	WithoutPermissionsUsername = "without_permissions_username"
	WithoutPermissionsPassword = "password"
	WithoutPermissionsHash     = "$2a$10$Q90.kkijFZtOf3bXSj2XOeDe/oIOrJghDNNjf2rOw.lsuGihW6fYO%"

	DuplicateUserID   = "duplicate_user_id"
	DuplicateUsername = "duplicate_username"
	DuplicatePassword = "password"
	DuplicateHash     = "$2a$10$Q90.kkijFZtOf3bXSj2XOeDe/oIOrJghDNNjf2rOw.lsuGihW6fYO%"

	NonExistingUser   = "non_existing_user"
	NonExistingUserID = 3

	NoInitiatorID = 0

	CurrencyOne = "currency_one"
	AmountOne   = "123"
	CurrencyTwo = "currency_two"
	AmountTwo   = "456"

	EmptyBalances []models.Balance
	GoodBalances  = []models.Balance{
		{Currency: CurrencyOne, Amount: AmountOne},
		{Currency: CurrencyTwo, Amount: AmountTwo},
	}

	NoMoneyUsername = "no_money_username"
	NoMoneyUserID   = 4
)
