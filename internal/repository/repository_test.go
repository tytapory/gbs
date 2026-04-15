package repository

import (
	"context"
	"database/sql"
	"fmt"
	"gbs/internal/config"
	"gbs/internal/models"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var repositoryTemplateDatabaseConfig config.DatabaseConfig
var mainConnection *sql.DB
var defaultPasswordHash = "123456789012345678901234567890123456789012345678901234567890"

type user struct {
	id           uuid.UUID
	username     string
	passwordHash string
	balances     []models.Balance
	permissions  []models.Permission
}

func TestMain(m *testing.M) {
	code := 1
	defer func() {
		os.Exit(code)
	}()

	ctx := context.Background()

	templateDBName := "gbs"
	user := "gbs"
	password := "password"

	createTables := filepath.Join("..", "..", "db", "migrations", "001-create_tables.sql")
	instertDefaultData := filepath.Join("..", "..", "db", "migrations", "002-insert_default_data.sql")
	createIndexes := filepath.Join("..", "..", "db", "migrations", "003-create_indexes.sql")

	pgContainer, err := postgres.Run(ctx,
		"postgres:18",
		postgres.WithInitScripts(createTables, instertDefaultData, createIndexes),
		postgres.WithDatabase(templateDBName),
		postgres.WithUsername(user),
		postgres.WithPassword(password),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)

	defer func() {
		if err := testcontainers.TerminateContainer(pgContainer); err != nil {
			log.Printf("failed to terminate container: %s", err)
		}
	}()

	if err != nil {
		log.Printf("test postgres container error: %s\n", err.Error())

		return
	}

	host, err := pgContainer.Host(ctx)
	if err != nil {
		log.Printf("test postgres container get host error: %s\n", err.Error())

		return
	}

	mappedPort, err := pgContainer.MappedPort(ctx, "5432/tcp")
	if err != nil {
		log.Printf("test postgres container get mapped port error: %s\n", err.Error())

		return
	}

	port := mappedPort.Port()

	repositoryTemplateDatabaseConfig = config.DatabaseConfig{Host: host, Port: port, User: user, Password: password, DBName: templateDBName, SSLMode: "disable"}
	mainConnectionDSN := fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=disable", user, password, host, port)

	mainConnection, err = sql.Open("postgres", mainConnectionDSN)
	if err != nil {
		log.Printf("connect to test db err: %s", err.Error())

		return
	}

	code = m.Run()
}

func TestGetUserPermissions(t *testing.T) {
	t.Parallel()

	users := []user{
		{
			id:           uuid.New(),
			username:     "test_user",
			passwordHash: defaultPasswordHash,
			permissions:  []models.Permission{models.SendFunds, models.ReceiveFunds},
		}, {
			id:           uuid.New(),
			username:     "user_with_no_rights",
			passwordHash: defaultPasswordHash,
			permissions:  []models.Permission{},
		},
	}

	r, _, teardown := setupTestDB(t, users)
	defer teardown()

	tests := []struct {
		name      string
		userID    uuid.UUID
		want      []models.Permission
		wantError error
	}{
		{
			name:      "happy path",
			userID:    users[0].id,
			want:      users[0].permissions,
			wantError: nil,
		}, {
			name:      "user does not exists",
			userID:    uuid.Nil,
			want:      []models.Permission{},
			wantError: nil,
		}, {
			name:      "user with no permissions",
			userID:    users[1].id,
			want:      users[1].permissions,
			wantError: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q := r.NewSingleQuery()
			require.NotNil(t, q)

			perms, err := r.GetUserPermissions(q, test.userID)
			if test.wantError == nil {
				assert.NoError(t, err)
			} else {
				assert.IsType(t, test.wantError, err)
			}
			assert.Equal(t, test.want, perms)
		})
	}
}

func TestGetUserIDAndPasswordHash(t *testing.T) {
	t.Parallel()

	users := []user{
		{
			id:           uuid.New(),
			username:     "test_user",
			passwordHash: defaultPasswordHash,
		},
	}

	r, _, teardown := setupTestDB(t, users)
	defer teardown()

	tests := []struct {
		name      string
		username  string
		wantID    uuid.UUID
		wantHash  string
		wantError error
	}{
		{
			name:      "happy path",
			username:  users[0].username,
			wantID:    users[0].id,
			wantHash:  users[0].passwordHash,
			wantError: nil,
		},
		{
			name:      "user does not exist",
			username:  "not_exist",
			wantID:    uuid.Nil,
			wantHash:  "",
			wantError: &models.NotFoundError{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q := r.NewSingleQuery()
			require.NotNil(t, q)

			id, hash, err := r.GetUserIDAndPasswordHash(q, test.username)
			if test.wantError == nil {
				assert.NoError(t, err)
			} else {
				assert.IsType(t, test.wantError, err)
			}
			assert.Equal(t, test.wantID, id)
			assert.Equal(t, test.wantHash, hash)
		})
	}
}

func TestRegisterUser(t *testing.T) {
	t.Parallel()

	users := []user{
		{
			username:     "test_user",
			passwordHash: defaultPasswordHash,
		},
	}

	r, db, teardown := setupTestDB(t, users)
	defer teardown()

	tests := []struct {
		name         string
		username     string
		passwordHash string
		wantError    error
	}{
		{
			name:         "happy path",
			username:     "new_user",
			passwordHash: defaultPasswordHash,
			wantError:    nil,
		},
		{
			name:         "user already exist",
			username:     users[0].username,
			passwordHash: users[0].passwordHash,
			wantError:    &models.ConflictError{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q := r.NewSingleQuery()
			require.NotNil(t, q)

			id, err := r.RegisterUser(q, test.username, test.passwordHash)
			if test.wantError == nil {
				assert.NoError(t, err)

				var actualID uuid.UUID
				err = db.QueryRow("SELECT id FROM users WHERE username = $1", test.username).Scan(&actualID)
				assert.NoError(t, err)
				assert.Equal(t, id, actualID)
			} else {
				assert.IsType(t, test.wantError, err)
			}
		})
	}
}

func TestGetBalances(t *testing.T) {
	t.Parallel()

	users := []user{
		{
			id:           uuid.New(),
			username:     "test_user",
			passwordHash: defaultPasswordHash,
			balances: []models.Balance{
				{Currency: "gcoin", Amount: 1000},
				{Currency: "stascoin", Amount: 2000},
			},
		},
		{
			id:           uuid.New(),
			username:     "poor_user",
			passwordHash: defaultPasswordHash,
		},
	}

	r, _, teardown := setupTestDB(t, users)
	defer teardown()

	tests := []struct {
		name      string
		id        uuid.UUID
		want      []models.Balance
		wantError error
	}{
		{
			name:      "happy path",
			id:        users[0].id,
			want:      users[0].balances,
			wantError: nil,
		}, {
			name:      "user does not exist",
			id:        uuid.Nil,
			want:      []models.Balance{},
			wantError: nil,
		}, {
			name:      "user with no balances",
			id:        users[1].id,
			want:      []models.Balance{},
			wantError: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q := r.NewSingleQuery()
			require.NotNil(t, q)
			balances, err := r.GetBalances(q, test.id)
			if test.wantError == nil {
				assert.NoError(t, err)
			} else {
				assert.IsType(t, test.wantError, err)
			}
			assert.Equal(t, test.want, balances)
		})
	}
}

func TestGetBalanceByCurrencyAndLock(t *testing.T) {
	t.Parallel()

	users := []user{
		{
			id:           uuid.New(),
			username:     "test_user",
			passwordHash: defaultPasswordHash,
			balances: []models.Balance{
				{Currency: "gcoin", Amount: 1000},
			},
		},
	}

	r, db, teardown := setupTestDB(t, users)
	defer teardown()

	tests := []struct {
		name      string
		id        uuid.UUID
		currency  string
		want      models.Balance
		wantError error
	}{
		{
			name:      "happy path",
			id:        users[0].id,
			currency:  users[0].balances[0].Currency,
			want:      users[0].balances[0],
			wantError: nil,
		}, {
			name:      "balance does not exist",
			id:        users[0].id,
			currency:  "not_exist",
			want:      models.Balance{},
			wantError: &models.NotFoundError{},
		}, {
			name:      "user does not exist",
			id:        uuid.Nil,
			currency:  "not_exist",
			want:      models.Balance{},
			wantError: &models.NotFoundError{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q := r.NewTransaction()
			require.NotNil(t, q)

			balance, err := r.GetBalanceByCurrencyAndLock(q, test.id, test.currency)
			if test.wantError == nil {
				assert.NoError(t, err)
			} else {
				assert.IsType(t, test.wantError, err)
			}
			assert.Equal(t, test.want, balance)

			_, err = db.Exec("SELECT 1 FROM balances WHERE user_id = $1 AND currency = $2 FOR UPDATE NOWAIT", test.id, test.currency)
			if test.wantError == nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			err = r.CommitTransaction((q))
			require.NoError(t, err)

			_, err = db.Exec("SELECT 1 FROM balances WHERE user_id = $1 AND currency = $2 FOR UPDATE NOWAIT", test.id, test.currency)
			assert.NoError(t, err)
		})
	}
}

func TestAddBalanceAndReturnNew(t *testing.T) {
	t.Parallel()

	users := []user{
		{
			id:           uuid.New(),
			username:     "test_user",
			passwordHash: defaultPasswordHash,
			balances: []models.Balance{
				{Currency: "gcoin", Amount: 1000},
				{Currency: "stascoin", Amount: 67},
				{Currency: "megacoin", Amount: 10},
			},
		},
	}

	r, db, teardown := setupTestDB(t, users)
	defer teardown()

	tests := []struct {
		name      string
		id        uuid.UUID
		balance   models.Balance
		amount    int64
		want      int64
		wantError error
	}{
		{
			name:      "happy path",
			id:        users[0].id,
			balance:   users[0].balances[0],
			amount:    67,
			want:      users[0].balances[0].Amount + 67,
			wantError: nil,
		},
		{
			name:      "substract",
			id:        users[0].id,
			balance:   users[0].balances[1],
			amount:    -67,
			want:      users[0].balances[1].Amount - 67,
			wantError: nil,
		},
		{
			name:      "balance does not exist",
			id:        users[0].id,
			balance:   models.Balance{Currency: "not exist", Amount: 0},
			amount:    67,
			want:      67,
			wantError: nil,
		}, {
			name:      "balance go negative",
			id:        users[0].id,
			balance:   users[0].balances[2],
			amount:    -67,
			want:      0,
			wantError: &models.BadRequestError{},
		}, {
			name:      "user does not exist",
			id:        uuid.Nil,
			balance:   models.Balance{Currency: "megacoin", Amount: 0},
			amount:    67,
			want:      0,
			wantError: &models.NotFoundError{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q := r.NewSingleQuery()
			require.NotNil(t, q)

			new, err := r.AddBalanceAndReturnNew(q, test.id, test.balance.Currency, test.amount)
			if test.wantError == nil {
				assert.NoError(t, err)
			} else {
				assert.IsType(t, test.wantError, err)
			}
			assert.Equal(t, test.want, new)

			if test.wantError == nil {
				var actualValueInDB int64

				err = db.QueryRow("SELECT amount FROM balances WHERE user_id = $1 AND currency = $2", test.id, test.balance.Currency).Scan(&actualValueInDB)
				assert.NoError(t, err)
				assert.Equal(t, test.want, actualValueInDB)
			}
		})
	}
}

func TestSetBalance(t *testing.T) {
	t.Parallel()

	users := []user{
		{
			id:           uuid.New(),
			username:     "test_user",
			passwordHash: defaultPasswordHash,
			balances: []models.Balance{
				{Currency: "gcoin", Amount: 1000},
			},
		},
	}

	r, db, teardown := setupTestDB(t, users)
	defer teardown()

	tests := []struct {
		name      string
		id        uuid.UUID
		currency  string
		amount    int64
		want      int64
		wantError error
	}{
		{
			name:      "happy path",
			id:        users[0].id,
			currency:  users[0].balances[0].Currency,
			amount:    67,
			want:      67,
			wantError: nil,
		}, {
			name:      "user does not exist",
			id:        uuid.Nil,
			currency:  "not_exist",
			amount:    67,
			want:      0,
			wantError: &models.NotFoundError{},
		}, {
			name:      "currency does not exist",
			id:        users[0].id,
			currency:  "not_exist",
			amount:    67,
			want:      0,
			wantError: &models.NotFoundError{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q := r.NewSingleQuery()
			require.NotNil(t, q)

			err := r.SetBalance(q, test.id, test.currency, test.amount)
			if test.wantError == nil {
				assert.NoError(t, err)

				var newBalance int64
				err = db.QueryRow("SELECT amount FROM balances WHERE user_id = $1 AND currency = $2", test.id, test.currency).Scan(&newBalance)
				assert.NoError(t, err)
				assert.Equal(t, test.amount, newBalance)
			} else {
				assert.IsType(t, test.wantError, err)
			}
		})
	}
}

func TestLogTransaction(t *testing.T) {
	t.Parallel()

	users := []user{
		{
			id:           uuid.New(),
			username:     "test_user",
			passwordHash: defaultPasswordHash,
		},
	}

	r, db, teardown := setupTestDB(t, users)
	defer teardown()

	tests := []struct {
		name      string
		log       models.Transaction
		wantError error
	}{
		{
			name: "happy path",
			log: models.Transaction{
				SenderID:             nil,
				ReceiverID:           users[0].id,
				InitiatorID:          users[0].id,
				SenderBalanceAfter:   nil,
				ReceiverBalanceAfter: 123,
				Currency:             "stascoin",
				Amount:               11,
				Fee:                  nil,
			},
			wantError: nil,
		}, {
			name: "receiver does not exist",

			log: models.Transaction{
				SenderID:             nil,
				ReceiverID:           uuid.Nil,
				InitiatorID:          users[0].id,
				SenderBalanceAfter:   nil,
				ReceiverBalanceAfter: 123,
				Currency:             "stascoin",
				Amount:               11,
				Fee:                  nil,
			},
			wantError: &models.NotFoundError{},
		},
		{
			name: "initiator does not exist",
			log: models.Transaction{
				SenderID:             nil,
				ReceiverID:           users[0].id,
				InitiatorID:          uuid.Nil,
				SenderBalanceAfter:   nil,
				ReceiverBalanceAfter: 123,
				Currency:             "stascoin",
				Amount:               11,
				Fee:                  nil,
			},
			wantError: &models.NotFoundError{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			q := r.NewSingleQuery()
			require.NotNil(t, q)

			err := r.LogTransaction(q, test.log)
			if test.wantError == nil {
				assert.NoError(t, err)

				log := test.log

				var actualLog models.Transaction
				err = db.QueryRow("SELECT sender_id, receiver_id, initiator_id, sender_balance_after, receiver_balance_after, currency, amount, fee FROM transaction_logs LIMIT 1").Scan(&actualLog.SenderID, &actualLog.ReceiverID, &actualLog.InitiatorID, &actualLog.SenderBalanceAfter, &actualLog.ReceiverBalanceAfter, &actualLog.Currency, &actualLog.Amount, &actualLog.Fee)
				assert.NoError(t, err)
				actualLog.CreatedAt = log.CreatedAt // repository controls this there is no way to affect this
				assert.Equal(t, log, actualLog)
			} else {
				assert.IsType(t, test.wantError, err)
			}
		})
	}
}

func TestGetUserID(t *testing.T) {
	t.Parallel()

	users := []user{
		{
			id:           uuid.New(),
			username:     "test_user",
			passwordHash: defaultPasswordHash,
		},
	}

	r, _, teardown := setupTestDB(t, users)
	defer teardown()

	q := r.NewSingleQuery()
	require.NotNil(t, q)

	user := users[0]

	id, err := r.GetUserID(q, user.username)
	assert.NoError(t, err)
	assert.Equal(t, id, user.id)
}

func TestGetUsername(t *testing.T) {
	t.Parallel()

	users := []user{
		{
			id:           uuid.New(),
			username:     "test_user",
			passwordHash: defaultPasswordHash,
		},
	}

	r, _, teardown := setupTestDB(t, users)
	defer teardown()

	q := r.NewSingleQuery()
	require.NotNil(t, q)

	user := users[0]

	id, err := r.GetUsername(q, user.id)
	assert.NoError(t, err)
	assert.Equal(t, user.username, id)
}

func setupTestDB(t *testing.T, users []user) (Repository, *sql.DB, func()) {
	t.Helper()

	r, db, dbName, err := createEmptyTestRepository(t)
	require.NoError(t, err)
	require.NotEmpty(t, dbName)
	require.NotNil(t, r)

	if len(users) > 0 {
		err = insertUsersInMockDB(users, db)
		require.NoError(t, err)
	}

	teardown := func() {
		err := closeDBByName(db, dbName, r)
		if err != nil {
			t.Errorf("could not delete test database: %s", err.Error())
		}
	}

	return r, db, teardown
}

func createEmptyTestRepository(t *testing.T) (Repository, *sql.DB, string, error) {
	cleanName := strings.ToLower(strings.ReplaceAll(t.Name(), "/", "_"))
	testDBName := fmt.Sprintf("test_%s", cleanName)
	_, err := mainConnection.Exec(fmt.Sprintf("CREATE DATABASE %s TEMPLATE gbs", testDBName))

	if err != nil {
		return nil, nil, "", err
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", repositoryTemplateDatabaseConfig.User, repositoryTemplateDatabaseConfig.Password, repositoryTemplateDatabaseConfig.Host, repositoryTemplateDatabaseConfig.Port, testDBName)

	testDBConnection, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, nil, "", err
	}

	testRepository, err := NewRepositoryImplementation(config.DatabaseConfig{Host: repositoryTemplateDatabaseConfig.Host, Port: repositoryTemplateDatabaseConfig.Port, User: repositoryTemplateDatabaseConfig.User, Password: repositoryTemplateDatabaseConfig.Password, DBName: testDBName, SSLMode: repositoryTemplateDatabaseConfig.SSLMode})
	if err != nil {
		return nil, nil, "", err
	}

	return testRepository, testDBConnection, testDBName, nil
}

func closeDBByName(testDB *sql.DB, testDBName string, testRepository Repository) error {
	err := testDB.Close()
	if err != nil {
		return err
	}

	err = testRepository.Close()
	if err != nil {
		return err
	}

	_, err = mainConnection.Exec(fmt.Sprintf("DROP DATABASE %s", testDBName))

	return err
}

func insertUsersInMockDB(users []user, testDB *sql.DB) error {
	for _, u := range users {
		_, err := testDB.Exec(
			`INSERT INTO users (id, username, password_hash) VALUES ($1, $2, $3)`,
			u.id, u.username, u.passwordHash,
		)
		if err != nil {
			return err
		}

		for _, b := range u.balances {
			_, err = testDB.Exec(`INSERT INTO balances(user_id, currency, amount) VALUES ($1, $2, $3)`, u.id, b.Currency, b.Amount)
			if err != nil {
				return err
			}
		}

		for _, p := range u.permissions {
			_, err = testDB.Exec(`INSERT INTO user_permission (user_id, permission_id) VALUES ($1, $2)`, u.id, p)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
