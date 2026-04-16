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

type dbData struct {
	users []user
	logs  []models.Transaction
}

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

	data := dbData{
		users: []user{
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
		},
	}

	r, db, teardown := setupTestDB(t, data)
	defer teardown()

	tests := []struct {
		name      string
		userID    uuid.UUID
		want      []models.Permission
		wantError error
	}{
		{
			name:      "happy path",
			userID:    data.users[0].id,
			want:      data.users[0].permissions,
			wantError: nil,
		}, {
			name:      "user does not exists",
			userID:    uuid.Nil,
			want:      []models.Permission{},
			wantError: nil,
		}, {
			name:      "user with no permissions",
			userID:    data.users[1].id,
			want:      data.users[1].permissions,
			wantError: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetDatabase(t, db, data)

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

	data := dbData{
		users: []user{
			{
				id:           uuid.New(),
				username:     "test_user",
				passwordHash: defaultPasswordHash,
			},
		},
	}

	r, db, teardown := setupTestDB(t, data)
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
			username:  data.users[0].username,
			wantID:    data.users[0].id,
			wantHash:  data.users[0].passwordHash,
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
			resetDatabase(t, db, data)

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

	data := dbData{
		users: []user{
			{
				username:     "test_user",
				passwordHash: defaultPasswordHash,
			},
		},
	}

	r, db, teardown := setupTestDB(t, data)
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
			username:     data.users[0].username,
			passwordHash: data.users[0].passwordHash,
			wantError:    &models.ConflictError{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetDatabase(t, db, data)

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

	data := dbData{
		users: []user{
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
		},
	}

	r, db, teardown := setupTestDB(t, data)
	defer teardown()

	tests := []struct {
		name      string
		id        uuid.UUID
		want      []models.Balance
		wantError error
	}{
		{
			name:      "happy path",
			id:        data.users[0].id,
			want:      data.users[0].balances,
			wantError: nil,
		}, {
			name:      "user does not exist",
			id:        uuid.Nil,
			want:      []models.Balance{},
			wantError: nil,
		}, {
			name:      "user with no balances",
			id:        data.users[1].id,
			want:      []models.Balance{},
			wantError: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetDatabase(t, db, data)

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

	data := dbData{
		users: []user{
			{
				id:           uuid.New(),
				username:     "test_user",
				passwordHash: defaultPasswordHash,
				balances: []models.Balance{
					{Currency: "gcoin", Amount: 1000},
				},
			},
		},
	}

	r, db, teardown := setupTestDB(t, data)
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
			id:        data.users[0].id,
			currency:  data.users[0].balances[0].Currency,
			want:      data.users[0].balances[0],
			wantError: nil,
		}, {
			name:      "balance does not exist",
			id:        data.users[0].id,
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
			resetDatabase(t, db, data)

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

	data := dbData{
		users: []user{
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
		},
	}

	r, db, teardown := setupTestDB(t, data)
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
			id:        data.users[0].id,
			balance:   data.users[0].balances[0],
			amount:    67,
			want:      data.users[0].balances[0].Amount + 67,
			wantError: nil,
		},
		{
			name:      "substract",
			id:        data.users[0].id,
			balance:   data.users[0].balances[1],
			amount:    -67,
			want:      data.users[0].balances[1].Amount - 67,
			wantError: nil,
		},
		{
			name:      "balance does not exist",
			id:        data.users[0].id,
			balance:   models.Balance{Currency: "not exist", Amount: 0},
			amount:    67,
			want:      67,
			wantError: nil,
		}, {
			name:      "balance go negative",
			id:        data.users[0].id,
			balance:   data.users[0].balances[2],
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
			resetDatabase(t, db, data)

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
	data := dbData{
		users: []user{
			{
				id:           uuid.New(),
				username:     "test_user",
				passwordHash: defaultPasswordHash,
				balances: []models.Balance{
					{Currency: "gcoin", Amount: 1000},
				},
			},
		},
	}

	r, db, teardown := setupTestDB(t, data)
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
			id:        data.users[0].id,
			currency:  data.users[0].balances[0].Currency,
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
			id:        data.users[0].id,
			currency:  "not_exist",
			amount:    67,
			want:      0,
			wantError: &models.NotFoundError{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetDatabase(t, db, data)

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

	data := dbData{
		users: []user{
			{
				id:           uuid.New(),
				username:     "test_user",
				passwordHash: defaultPasswordHash,
			},
		},
	}

	r, db, teardown := setupTestDB(t, data)
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
				ReceiverID:           data.users[0].id,
				InitiatorID:          data.users[0].id,
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
				InitiatorID:          data.users[0].id,
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
				ReceiverID:           data.users[0].id,
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
			resetDatabase(t, db, data)

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

	data := dbData{
		users: []user{
			{
				id:           uuid.New(),
				username:     "test_user",
				passwordHash: defaultPasswordHash,
			},
		},
	}
	r, db, teardown := setupTestDB(t, data)
	defer teardown()

	tests := []struct {
		name      string
		username  string
		want      uuid.UUID
		wantError error
	}{
		{
			name:      "happy path",
			username:  data.users[0].username,
			want:      data.users[0].id,
			wantError: nil,
		}, {
			name:      "user does not exist",
			username:  "not exist",
			want:      uuid.Nil,
			wantError: &models.NotFoundError{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetDatabase(t, db, data)

			q := r.NewSingleQuery()
			require.NotNil(t, q)

			id, err := r.GetUserID(q, test.username)
			if test.wantError == nil {
				assert.NoError(t, err)
			} else {
				assert.IsType(t, test.wantError, err)
			}
			assert.Equal(t, test.want, id)
		})
	}
}

func TestGetUsername(t *testing.T) {
	t.Parallel()

	data := dbData{
		users: []user{
			{
				id:           uuid.New(),
				username:     "test_user",
				passwordHash: defaultPasswordHash,
			},
		},
	}

	r, db, teardown := setupTestDB(t, data)
	defer teardown()

	tests := []struct {
		name      string
		id        uuid.UUID
		want      string
		wantError error
	}{
		{
			name:      "happy path",
			id:        data.users[0].id,
			want:      data.users[0].username,
			wantError: nil,
		}, {
			name:      "user does not exist",
			id:        uuid.Nil,
			want:      "",
			wantError: &models.NotFoundError{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetDatabase(t, db, data)

			q := r.NewSingleQuery()
			require.NotNil(t, q)

			username, err := r.GetUsername(q, test.id)
			if test.wantError == nil {

				assert.NoError(t, err)
			} else {
				assert.IsType(t, test.wantError, err)
			}
			assert.Equal(t, test.want, username)
		})
	}
}

func TestGetTransactionCount(t *testing.T) {
	t.Parallel()

	data := dbData{
		users: []user{
			{
				id:           uuid.New(),
				username:     "test_user",
				passwordHash: defaultPasswordHash,
			},
			{
				id:           uuid.New(),
				username:     "test_user2",
				passwordHash: defaultPasswordHash,
			},
			{
				id:           uuid.New(),
				username:     "test_user3",
				passwordHash: defaultPasswordHash,
			},
			{
				id:           uuid.New(),
				username:     "no_transactions_user",
				passwordHash: defaultPasswordHash,
			},
		},
	}

	data.logs = []models.Transaction{
		{
			SenderID:             &data.users[0].id,
			ReceiverID:           data.users[1].id,
			InitiatorID:          data.users[2].id,
			SenderBalanceAfter:   nil,
			ReceiverBalanceAfter: 0,
			Currency:             "gbscoin",
			Amount:               67,
			Fee:                  nil,
		},
		{
			SenderID:             &data.users[1].id,
			ReceiverID:           data.users[0].id,
			InitiatorID:          data.users[0].id,
			SenderBalanceAfter:   nil,
			ReceiverBalanceAfter: 0,
			Currency:             "gbscoin",
			Amount:               67,
			Fee:                  nil,
		},
	}

	r, db, teardown := setupTestDB(t, data)
	defer teardown()

	tests := []struct {
		name      string
		id        uuid.UUID
		want      int64
		wantError error
	}{
		{
			name:      "happy path 1",
			id:        data.users[0].id,
			want:      2,
			wantError: nil,
		},
		{
			name:      "happy path 2",
			id:        data.users[1].id,
			want:      2,
			wantError: nil,
		},
		{
			name:      "happy path 3",
			id:        data.users[2].id,
			want:      1,
			wantError: nil,
		},
		{
			name:      "user with no transactions",
			id:        data.users[3].id,
			want:      0,
			wantError: nil,
		}, {
			name:      "user does not exist",
			id:        uuid.Nil,
			want:      0,
			wantError: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetDatabase(t, db, data)

			q := r.NewSingleQuery()
			require.NotNil(t, q)

			amount, err := r.GetTransactionCount(q, test.id)
			if test.wantError == nil {
				assert.NoError(t, err)
			} else {
				assert.IsType(t, test.wantError, err)
			}
			assert.Equal(t, test.want, amount)
		})
	}
}

func setupTestDB(t *testing.T, data dbData) (Repository, *sql.DB, func()) {
	t.Helper()

	r, db, dbName, err := createEmptyTestRepository(t)
	require.NoError(t, err)
	require.NotEmpty(t, dbName)
	require.NotNil(t, r)

	err = insertDataInMockDB(data, db)
	require.NoError(t, err)

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

func insertDataInMockDB(data dbData, testDB *sql.DB) error {
	for _, u := range data.users {
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

	for _, log := range data.logs {
		id := uuid.New()
		_, err := testDB.Exec("INSERT INTO transaction_logs(id, sender_id, receiver_id, initiator_id, sender_balance_after, receiver_balance_after, currency, amount, fee) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)", id, log.SenderID, log.ReceiverID, log.InitiatorID, log.SenderBalanceAfter, log.ReceiverBalanceAfter, log.Currency, log.Amount, log.Fee)
		if err != nil {
			return err
		}
	}

	return nil
}

func resetDatabase(t *testing.T, db *sql.DB, data dbData) {
	t.Helper()

	_, err := db.Exec("TRUNCATE TABLE users, balances, user_permission, transaction_logs, refresh_tokens RESTART IDENTITY CASCADE")
	require.NoError(t, err)

	err = insertDataInMockDB(data, db)
	require.NoError(t, err)
}
