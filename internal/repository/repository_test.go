package repository

import (
	"context"
	"database/sql"
	"fmt"
	"gbs/internal/config"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var repositoryTemplateDatabaseConfig config.DatabaseConfig
var mainConnection *sql.DB

var commonPasswordHash = "12345678901234567890123456789012"
var sendReceiveUsername = "sendReceiveUser"

func TestMain(m *testing.M) {
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
				WithOccurrence(2).WithStartupTimeout(5*time.Second)),
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

	repositoryTemplateDatabaseConfig = config.DatabaseConfig{Host: host, Port: port, User: user, Password: password, DBName: templateDBName}
	mainConnectionDSN := fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=disable", user, password, host, port)

	mainConnection, err = sql.Open("postgres", mainConnectionDSN)
	if err != nil {
		log.Printf("connect to test db err: %s", err.Error())

		return
	}

	err = insertDefaultDataToTemplate()
	if err != nil {
		log.Printf("failed to insert default data to test db: %s", err.Error())

		return
	}

	code := m.Run()

	if err := testcontainers.TerminateContainer(pgContainer); err != nil {
		log.Printf("failed to terminate container: %s", err)
	}

	os.Exit(code)
}

func insertDefaultDataToTemplate() error {
	newUserID, err := uuid.NewV7()
	if err != nil {
		return err
	}

	_, err = mainConnection.Exec(`INSERT INTO users(id, username, password_hash) VALUES ($1, $2, $3)`, newUserID, sendReceiveUsername, commonPasswordHash)
	if err != nil {
		return err
	}

}

func createEmptyTestRepository() (Repository, func() error, error) {
	testDBName := fmt.Sprintf("test_%d", time.Now().UnixNano())
	_, err := mainConnection.Exec(fmt.Sprintf("CREATE DATABASE %s TEMPLATE gbs", testDBName))

	if err != nil {
		return nil, nil, err
	}

	testRepository, err := NewRepositoryImplementation(config.DatabaseConfig{Host: repositoryTemplateDatabaseConfig.Host, Port: repositoryTemplateDatabaseConfig.Port, User: repositoryTemplateDatabaseConfig.User, Password: repositoryTemplateDatabaseConfig.Password, DBName: testDBName, SSLMode: repositoryTemplateDatabaseConfig.SSLMode})
	if err != nil {
		return nil, nil, err
	}

	return testRepository, func() error {
		err := testRepository.Close()
		if err != nil {
			return err
		}

		_, err = mainConnection.Exec(fmt.Sprintf("DROP DATABASE %s", testDBName))

		return err
	}, nil
}

func TestGetUserPermissions(t *testing.T) {
}
