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

	return
}

func TestGetUserPermissions(t *testing.T) {
	users := []user{
		user{
			id:           uuid.New(),
			username:     "test_user",
			passwordHash: defaultPasswordHash,
			permissions:  []models.Permission{models.SendFunds, models.ReceiveFunds},
		},
	}

	r, _, teardown := setupTestDB(t, users)
	defer teardown()

	q := r.NewSingleQuery()
	require.NotNil(t, q)

	perms, err := r.GetUserPermissions(q, users[0].id)
	assert.NoError(t, err)
	assert.ElementsMatch(t, users[0].permissions, perms)

	perms, err = r.GetUserPermissions(q, uuid.Nil)
	assert.NoError(t, err)
	assert.ElementsMatch(t, perms, []models.Permission{})
}

func TestGetUserIDAndPasswordHash(t *testing.T) {
	users := []user{
		user{
			id:           uuid.New(),
			username:     "test_user",
			passwordHash: defaultPasswordHash,
		},
	}

	r, _, teardown := setupTestDB(t, users)
	defer teardown()

	q := r.NewSingleQuery()
	require.NotNil(t, q)

	id, hash, err := r.GetUserIDAndPasswordHash(q, users[0].username)
	assert.NoError(t, err)
	assert.Equal(t, users[0].id, id)
	assert.Equal(t, users[0].passwordHash, hash)

	id, hash, err = r.GetUserIDAndPasswordHash(q, "")
	assert.IsType(t, &models.NotFoundError{}, err)
	assert.Equal(t, uuid.Nil, id)
	assert.Equal(t, "", hash)
}

func TestRegisterUser(t *testing.T) {
	r, dbName, teardown := setupTestDB(t, []user{})
	defer teardown()

	users := []user{
		user{
			username:     "test_user",
			passwordHash: defaultPasswordHash,
		},
	}

	q := r.NewSingleQuery()
	require.NotNil(t, q)

	id, err := r.RegisterUser(q, users[0].username, users[0].passwordHash)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, id)

	users[0].id = id
	err = insertUsersInMockDB(users, dbName)
	assert.Error(t, err)

	id, err = r.RegisterUser(q, users[0].username, users[0].passwordHash)
	assert.IsType(t, &models.ConflictError{}, err)
	assert.Equal(t, uuid.Nil, id)
}

func TestGetBalances(t *testing.T) {
	users := []user{
		user{
			id:           uuid.New(),
			username:     "test_user",
			passwordHash: defaultPasswordHash,
			balances: []models.Balance{
				models.Balance{Currency: "gcoin", Amount: 1000},
				models.Balance{Currency: "stascoin", Amount: 2000},
			},
		},
	}

	r, _, teardown := setupTestDB(t, users)
	defer teardown()

	q := r.NewSingleQuery()
	require.NotNil(t, q)

	balances, err := r.GetBalances(q, users[0].id)
	assert.NoError(t, err)
	assert.ElementsMatch(t, users[0].balances, balances)

	balances, err = r.GetBalances(q, uuid.Nil)
	assert.NoError(t, err)
	assert.ElementsMatch(t, balances, []models.Balance{})
}

func TestGetBalanceByCurrencyAndLock(t *testing.T) {
	users := []user{
		user{
			id:           uuid.New(),
			username:     "test_user",
			passwordHash: defaultPasswordHash,
			balances: []models.Balance{
				models.Balance{Currency: "gcoin", Amount: 1000},
				models.Balance{Currency: "stascoin", Amount: 2000},
			},
		},
	}

	r, dbName, teardown := setupTestDB(t, users)
	defer teardown()

	q := r.NewTransaction()
	require.NotNil(t, q)

	balances, err := r.GetBalanceByCurrencyAndLock(q, users[0].id, users[0].balances[0].Currency)
	assert.NoError(t, err)
	assert.Equal(t, users[0].balances[0], balances)

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", repositoryTemplateDatabaseConfig.User, repositoryTemplateDatabaseConfig.Password, repositoryTemplateDatabaseConfig.Host, repositoryTemplateDatabaseConfig.Port, dbName)

	testDBConnection, err := sql.Open("postgres", dsn)
	require.NoError(t, err)

	defer testDBConnection.Close()

	_, err = testDBConnection.Exec("SELECT 1 FROM balances WHERE user_id = $1 AND currency = $2 FOR UPDATE NOWAIT", users[0].id, users[0].balances[0].Currency)
	assert.Error(t, err)

	_, err = testDBConnection.Exec("SELECT 1 FROM balances WHERE user_id = $1 AND currency = $2 FOR UPDATE NOWAIT", users[0].id, users[0].balances[1].Currency)
	assert.NoError(t, err)

	r.CommitTransaction(q)

	_, err = testDBConnection.Exec("UPDATE balances SET amount = $1 WHERE user_id = $2 AND currency = $3", users[0].balances[0].Amount, users[0].id, users[0].balances[0].Currency)
	assert.NoError(t, err)
}

func TestSetBalance(t *testing.T) {
	users := []user{
		user{
			id:           uuid.New(),
			username:     "test_user",
			passwordHash: defaultPasswordHash,
			balances: []models.Balance{
				models.Balance{Currency: "gcoin", Amount: 1000},
			},
		},
	}

	r, dbName, teardown := setupTestDB(t, users)
	defer teardown()

	q := r.NewSingleQuery()
	require.NotNil(t, q)

	newAmount := 67
	err := r.SetBalance(q, users[0].id, users[0].balances[0].Currency, int64(newAmount))
	assert.NoError(t, err)

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", repositoryTemplateDatabaseConfig.User, repositoryTemplateDatabaseConfig.Password, repositoryTemplateDatabaseConfig.Host, repositoryTemplateDatabaseConfig.Port, dbName)

	testDBConnection, err := sql.Open("postgres", dsn)
	require.NoError(t, err)

	defer testDBConnection.Close()

	var newBalance int
	err = testDBConnection.QueryRow("SELECT amount FROM balances WHERE user_id = $1 AND currency = $2", users[0].id, users[0].balances[0].Currency).Scan(&newBalance)
	assert.NoError(t, err)
	assert.Equal(t, newAmount, newBalance)
}

func TestAddBalanceAndReturnNew(t *testing.T) {
	users := []user{
		user{
			id:           uuid.New(),
			username:     "test_user",
			passwordHash: defaultPasswordHash,
			balances: []models.Balance{
				models.Balance{Currency: "gcoin", Amount: 1000},
			},
		},
	}

	r, dbName, teardown := setupTestDB(t, users)
	defer teardown()

	q := r.NewSingleQuery()
	require.NotNil(t, q)

	additionalBalance := 67
	expectedNew := users[0].balances[0].Amount + int64(additionalBalance)
	new, err := r.AddBalanceAndReturnNew(q, users[0].id, users[0].balances[0].Currency, int64(additionalBalance))
	assert.NoError(t, err)
	assert.Equal(t, expectedNew, new)

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", repositoryTemplateDatabaseConfig.User, repositoryTemplateDatabaseConfig.Password, repositoryTemplateDatabaseConfig.Host, repositoryTemplateDatabaseConfig.Port, dbName)

	testDBConnection, err := sql.Open("postgres", dsn)
	require.NoError(t, err)

	defer testDBConnection.Close()

	var actualValueInDB int64
	err = testDBConnection.QueryRow("SELECT amount FROM balances WHERE user_id = $1 AND currency = $2", users[0].id, users[0].balances[0].Currency).Scan(&actualValueInDB)
	assert.NoError(t, err)
	assert.Equal(t, expectedNew, actualValueInDB)
}

func setupTestDB(t *testing.T, users []user) (Repository, string, func()) {
	t.Helper()

	r, dbName, err := createEmptyTestRepository()
	require.NoError(t, err)
	require.NotEmpty(t, dbName)
	require.NotNil(t, r)

	if len(users) > 0 {
		err = insertUsersInMockDB(users, dbName)
		require.NoError(t, err)
	}

	teardown := func() {
		err := closeDBByName(dbName, r)
		if err != nil {
			t.Errorf("could not delete test database: %s", err.Error())
		}
	}

	return r, dbName, teardown
}

func createEmptyTestRepository() (Repository, string, error) {
	testDBName := fmt.Sprintf("test_%d", time.Now().UnixNano())
	_, err := mainConnection.Exec(fmt.Sprintf("CREATE DATABASE %s TEMPLATE gbs", testDBName))

	if err != nil {
		return nil, "", err
	}

	testRepository, err := NewRepositoryImplementation(config.DatabaseConfig{Host: repositoryTemplateDatabaseConfig.Host, Port: repositoryTemplateDatabaseConfig.Port, User: repositoryTemplateDatabaseConfig.User, Password: repositoryTemplateDatabaseConfig.Password, DBName: testDBName, SSLMode: repositoryTemplateDatabaseConfig.SSLMode})
	if err != nil {
		return nil, "", err
	}

	return testRepository, testDBName, nil
}

func closeDBByName(testDBName string, testRepository Repository) error {
	err := testRepository.Close()
	if err != nil {
		return err
	}

	_, err = mainConnection.Exec(fmt.Sprintf("DROP DATABASE %s", testDBName))

	return err
}

func insertUsersInMockDB(users []user, testDBName string) error {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", repositoryTemplateDatabaseConfig.User, repositoryTemplateDatabaseConfig.Password, repositoryTemplateDatabaseConfig.Host, repositoryTemplateDatabaseConfig.Port, testDBName)

	testDBConnection, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}

	defer testDBConnection.Close()

	for _, u := range users {
		_, err = testDBConnection.Exec(
			`INSERT INTO users (id, username, password_hash) VALUES ($1, $2, $3)`,
			u.id, u.username, u.passwordHash,
		)
		if err != nil {
			return err
		}

		for _, b := range u.balances {
			_, err = testDBConnection.Exec(`INSERT INTO balances(user_id, currency, amount) VALUES ($1, $2, $3)`, u.id, b.Currency, b.Amount)
			if err != nil {
				return err
			}
		}

		for _, p := range u.permissions {
			_, err = testDBConnection.Exec(`INSERT INTO user_permission (user_id, permission_id) VALUES ($1, $2)`, u.id, p)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
