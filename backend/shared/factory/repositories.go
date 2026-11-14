// Package factory is the composition root for the shared repositories: the
// only package outside shared/adapters that names a concrete adapter.
// Services ask it for ports and never import an adapter themselves.
package factory

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/adapters/postgres"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DriverPostgres is the default (and today the only) database driver.
const DriverPostgres = "postgres"

// Config selects and locates the database behind the repository ports.
type Config struct {
	// Driver picks the adapter. Empty means DriverPostgres.
	Driver string
	URL    string
}

// ConfigFromEnv reads DATABASE_DRIVER and DATABASE_URL.
func ConfigFromEnv() Config {
	return Config{Driver: os.Getenv("DATABASE_DRIVER"), URL: os.Getenv("DATABASE_URL")}
}

func (c Config) driver() string {
	d := strings.ToLower(strings.TrimSpace(c.Driver))
	if d == "" {
		return DriverPostgres
	}
	return d
}

// Database is an open database: every repository port, plus what is needed
// to close it.
type Database struct {
	*Repositories

	// Pool is set only for DriverPostgres. It exists for the few components
	// that are Postgres by nature (the agent's SQL tools, the Postgres document
	// store); everything else uses the repository ports.
	Pool *pgxpool.Pool

	close func()
}

// Close releases the connection.
func (d *Database) Close() {
	if d.close != nil {
		d.close()
	}
}

// ConnectDB connects to the database cfg names and builds its repositories.
func ConnectDB(ctx context.Context, cfg Config) (*Database, error) {
	switch cfg.driver() {
	case DriverPostgres:
		pool, close, err := acquirePool(ctx, cfg.URL)
		if err != nil {
			return nil, err
		}
		return &Database{
			Repositories: newPostgresRepositories(pool),
			Pool:         pool,
			close:        close,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported database driver %q", cfg.Driver)
	}
}

// OpenSQLPool opens a bare connection pool, for components that issue SQL
// themselves (the agent's read-only query tools). It fails for drivers that
// are not SQL databases.
func OpenSQLPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	switch cfg.driver() {
	case DriverPostgres:
		return postgres.ConnectWithURL(ctx, cfg.URL)
	default:
		return nil, fmt.Errorf("database driver %q has no SQL pool", cfg.Driver)
	}
}

// Repositories holds every repository port plus the transaction runner.
type Repositories struct {
	Account              repository.AccountRepository
	Agent                repository.AgentRepository
	AgentMemory          repository.AgentMemoryRepository
	APIKey               repository.APIKeyRepository
	Auth                 repository.AuthRepository
	Budget               repository.BudgetRepository
	BulkInserter         repository.BulkInserter
	Category             repository.CategoryRepository
	CategoryGoal         repository.CategoryGoalRepository
	CategoryGroup        repository.CategoryGroupRepository
	CipherPrediction     repository.CipherPredictionRepository
	DevicePushToken      repository.DevicePushTokenRepository
	Embedding            repository.EmbeddingRepository
	GoogleProvider       repository.GoogleProviderRepository
	InboundMessage       repository.InboundMessageRepository
	LoanMetadata         repository.LoanMetadataRepository
	MailConnection       repository.MailConnectionRepository
	MonthlyBudget        repository.MonthlyBudgetRepository
	Payee                repository.PayeesRepository
	PayeeRule            repository.PayeeRuleRepository
	PipelineRun          repository.PipelineRunRepository
	PipelineStep         repository.PipelineStepRepository
	CounterpartyAlias    repository.CounterpartyAliasRepository
	TransactionSource    repository.TransactionSourceRepository
	Prediction           repository.PredictionRepository
	PredictionReview     repository.PredictionReviewRepository
	RecurringTransaction repository.RecurringTransactionRepository
	Report               repository.ReportRepository
	StatementImport      repository.StatementImportRepository
	StatementPassword    repository.StatementPasswordRepository
	Tag                  repository.TagRepository
	Transaction          repository.TransactionRepository
	TransactionDocument  repository.TransactionDocumentRepository
	TransactionEmbedding repository.TransactionEmbeddingRepository
	DBTxn                repository.DBTxn
	User                 repository.UserRepository
	UserLLMProvider      repository.UserLLMProviderRepository
	UserLLMModel         repository.UserLLMModelRepository
}

// newPostgresRepositories builds every repository on one pool.
func newPostgresRepositories(pool *pgxpool.Pool) *Repositories {
	return &Repositories{
		Account:              postgres.NewAccountRepository(pool),
		Agent:                postgres.NewAgentRepository(pool),
		AgentMemory:          postgres.NewAgentMemoryRepository(pool),
		APIKey:               postgres.NewAPIKeyRepository(pool),
		Auth:                 postgres.NewAuthRepository(pool),
		Budget:               postgres.NewBudgetRepository(pool),
		BulkInserter:         postgres.NewBulkInserter(pool),
		Category:             postgres.NewCategoryRepository(pool),
		CategoryGoal:         postgres.NewCategoryGoalRepository(pool),
		CategoryGroup:        postgres.NewCategoryGroupRepository(pool),
		CipherPrediction:     postgres.NewCipherPredictionRepository(pool),
		DevicePushToken:      postgres.NewDevicePushTokenRepository(pool),
		Embedding:            postgres.NewEmbeddingRepository(pool),
		GoogleProvider:       postgres.NewGoogleProviderRepository(pool),
		InboundMessage:       postgres.NewInboundMessageRepository(pool),
		LoanMetadata:         postgres.NewLoanMetadataRepository(pool),
		MailConnection:       postgres.NewMailConnectionRepository(pool),
		MonthlyBudget:        postgres.NewMonthlyBudgetRepository(pool),
		Payee:                postgres.NewPayeesRepository(pool),
		PayeeRule:            postgres.NewPayeeRuleRepository(pool),
		PipelineRun:          postgres.NewPipelineRunRepository(pool),
		PipelineStep:         postgres.NewPipelineStepRepository(pool),
		CounterpartyAlias:    postgres.NewCounterpartyAliasRepository(pool),
		TransactionSource:    postgres.NewTransactionSourceRepository(pool),
		Prediction:           postgres.NewPredictionRepository(pool),
		PredictionReview:     postgres.NewPredictionReviewRepository(pool),
		RecurringTransaction: postgres.NewRecurringTransactionRepository(pool),
		Report:               postgres.NewReportRepository(pool),
		StatementImport:      postgres.NewStatementImportRepository(pool),
		StatementPassword:    postgres.NewStatementPasswordRepository(pool),
		Tag:                  postgres.NewTagRepository(pool),
		Transaction:          postgres.NewTransactionRepository(pool),
		TransactionDocument:  postgres.NewTransactionDocumentRepository(pool),
		TransactionEmbedding: postgres.NewTransactionEmbeddingRepository(pool),
		DBTxn:                postgres.NewDBTxn(pool),
		User:                 postgres.NewUserRepository(pool),
		UserLLMProvider:      postgres.NewUserLLMProviderRepository(pool),
		UserLLMModel:         postgres.NewUserLLMModelRepository(pool),
	}
}
