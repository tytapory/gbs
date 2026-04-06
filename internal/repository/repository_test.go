package repository

import (
	"context"
	"fmt"
	"gbs/internal/config"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var repositoryConfig config.DatabaseConfig

func TestMain(m *testing.M) {
	ctx := context.Background()

	dbName := "gbs"
	dbUser := "gbs"
	dbPassword := "password"

	pgContainer, err := postgres.Run(ctx,
		"postgres:18",
		postgres.WithInitScripts(filepath.Join("..", "testdata", "init-db.sql")),
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(5*time.Second)),
	)

	if err != nil {
		fmt.Printf("test postgres container error: %s\n", err.Error())
		os.Exit(1)
	}

	host, err := pgContainer.Host(ctx)
	if err != nil {
		fmt.Printf("test postgres container get host error: %s\n", err.Error())
		os.Exit(1)
	}

	mappedPort, err := pgContainer.MappedPort(ctx, "5432/tcp")
	if err != nil {
		fmt.Printf("test postgres container get mapped port error: %s\n", err.Error())
		os.Exit(1)
	}

	port := mappedPort.Port()

	repositoryConfig = config.DatabaseConfig{Host: host, Port: port, Password: dbPassword, DBName: dbName}

	NewRepositoryImplementation(repositoryConfig)
}
