package migration

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// Migration represents a single database migration
type Migration struct {
	Version     int64
	Description string
	Up          func(db contracts.Database) error
	Down        func(db contracts.Database) error
}

// FileMigration represents a file-based migration
type FileMigration struct {
	Version     int64
	Description string
	UpSQL       string
	DownSQL     string
}

// Config defines migration configuration
type Config struct {
	// TableName is the name of the migrations table (default: schema_migrations)
	TableName string

	// Database is the database adapter to use
	Database contracts.Database

	// Logger for migration output
	Logger contracts.Logger

	// SkipTableCreation skips creating the migrations table if true
	SkipTableCreation bool
}

// DefaultConfig returns default migration configuration
func DefaultConfig() *Config {
	return &Config{
		TableName: "schema_migrations",
	}
}

// Migrator handles database migrations
type Migrator struct {
	config     *Config
	migrations []Migration
}

// New creates a new Migrator instance
func New(config *Config) *Migrator {
	if config == nil {
		config = DefaultConfig()
	}
	if config.TableName == "" {
		config.TableName = "schema_migrations"
	}
	return &Migrator{
		config:     config,
		migrations: make([]Migration, 0),
	}
}

// Register registers a new migration
func (m *Migrator) Register(migration Migration) *Migrator {
	m.migrations = append(m.migrations, migration)
	return m
}

// RegisterMany registers multiple migrations
func (m *Migrator) RegisterMany(migrations []Migration) *Migrator {
	m.migrations = append(m.migrations, migrations...)
	return m
}

// Up runs all pending migrations
func (m *Migrator) Up(ctx context.Context) error {
	if m.config.Database == nil {
		return fmt.Errorf("database not configured")
	}

	// Create migrations table if needed
	if !m.config.SkipTableCreation {
		if err := m.createMigrationsTable(ctx); err != nil {
			return fmt.Errorf("failed to create migrations table: %w", err)
		}
	}

	// Get current version
	currentVersion, err := m.getCurrentVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	m.log("Current version: %d", currentVersion)

	// Run pending migrations
	applied := 0
	for _, migration := range m.migrations {
		if migration.Version <= currentVersion {
			continue
		}

		m.log("Running migration %d: %s", migration.Version, migration.Description)

		if err := migration.Up(m.config.Database); err != nil {
			return fmt.Errorf("migration %d failed: %w", migration.Version, err)
		}

		if err := m.recordMigration(ctx, migration.Version, migration.Description); err != nil {
			return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
		}

		applied++
		m.log("Migration %d completed", migration.Version)
	}

	if applied == 0 {
		m.log("No pending migrations")
	} else {
		m.log("Applied %d migration(s)", applied)
	}

	return nil
}

// Down rolls back the last migration
func (m *Migrator) Down(ctx context.Context) error {
	if m.config.Database == nil {
		return fmt.Errorf("database not configured")
	}

	// Get current version
	currentVersion, err := m.getCurrentVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	if currentVersion == 0 {
		m.log("No migrations to rollback")
		return nil
	}

	// Find migration to rollback
	var targetMigration *Migration
	for i := range m.migrations {
		if m.migrations[i].Version == currentVersion {
			targetMigration = &m.migrations[i]
			break
		}
	}

	if targetMigration == nil {
		return fmt.Errorf("migration %d not found", currentVersion)
	}

	m.log("Rolling back migration %d: %s", targetMigration.Version, targetMigration.Description)

	if err := targetMigration.Down(m.config.Database); err != nil {
		return fmt.Errorf("rollback %d failed: %w", targetMigration.Version, err)
	}

	if err := m.removeMigration(ctx, targetMigration.Version); err != nil {
		return fmt.Errorf("failed to remove migration %d: %w", targetMigration.Version, err)
	}

	m.log("Rollback %d completed", targetMigration.Version)

	return nil
}

// DownTo rolls back to a specific version
func (m *Migrator) DownTo(ctx context.Context, targetVersion int64) error {
	currentVersion, err := m.getCurrentVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	if targetVersion >= currentVersion {
		m.log("Already at or below target version")
		return nil
	}

	for currentVersion > targetVersion {
		if err := m.Down(ctx); err != nil {
			return err
		}
		currentVersion, err = m.getCurrentVersion(ctx)
		if err != nil {
			return fmt.Errorf("failed to get current version: %w", err)
		}
	}

	return nil
}

// Status shows the current migration status
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	if m.config.Database == nil {
		return nil, fmt.Errorf("database not configured")
	}

	currentVersion, err := m.getCurrentVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current version: %w", err)
	}

	statuses := make([]MigrationStatus, 0, len(m.migrations))
	for _, migration := range m.migrations {
		status := MigrationStatus{
			Version:     migration.Version,
			Description: migration.Description,
			Applied:     migration.Version <= currentVersion,
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

// MigrationStatus represents the status of a migration
type MigrationStatus struct {
	Version     int64
	Description string
	Applied     bool
	AppliedAt   *time.Time
}

// createMigrationsTable creates the migrations tracking table
func (m *Migrator) createMigrationsTable(ctx context.Context) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version BIGINT PRIMARY KEY,
			description VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`, m.config.TableName)

	_, err := m.config.Database.Exec(ctx, query)
	return err
}

// getCurrentVersion gets the latest applied migration version
func (m *Migrator) getCurrentVersion(ctx context.Context) (int64, error) {
	query := fmt.Sprintf(`
		SELECT COALESCE(MAX(version), 0) as version FROM %s
	`, m.config.TableName)

	var result struct {
		Version int64 `db:"version"`
	}

	err := m.config.Database.FindOne(ctx, &result, query)
	if err != nil && err.Error() != "sql: no rows in result set" {
		return 0, err
	}

	return result.Version, nil
}

// recordMigration records a completed migration
func (m *Migrator) recordMigration(ctx context.Context, version int64, description string) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (version, description, applied_at)
		VALUES ($1, $2, $3)
	`, m.config.TableName)

	_, err := m.config.Database.Exec(ctx, query, version, description, time.Now())
	return err
}

// removeMigration removes a migration record (for rollback)
func (m *Migrator) removeMigration(ctx context.Context, version int64) error {
	query := fmt.Sprintf(`
		DELETE FROM %s WHERE version = $1
	`, m.config.TableName)

	_, err := m.config.Database.Exec(ctx, query, version)
	return err
}

// log logs a message if logger is configured
func (m *Migrator) log(format string, args ...interface{}) {
	if m.config.Logger != nil {
		m.config.Logger.Info(fmt.Sprintf(format, args...))
	}
}

// FromEmbedFS loads migrations from an embedded filesystem
func FromEmbedFS(fs embed.FS, path string) ([]FileMigration, error) {
	// This is a placeholder for loading from embed.FS
	// Implementation would parse SQL files and extract version, description, up/down SQL
	return nil, fmt.Errorf("not implemented yet")
}

// FileMigrationsToMigrations converts FileMigrations to Migrations
func FileMigrationsToMigrations(fileMigrations []FileMigration) []Migration {
	migrations := make([]Migration, len(fileMigrations))
	for i, fm := range fileMigrations {
		migrations[i] = Migration{
			Version:     fm.Version,
			Description: fm.Description,
			Up: func(db contracts.Database) error {
				_, err := db.Exec(context.Background(), fm.UpSQL)
				return err
			},
			Down: func(db contracts.Database) error {
				_, err := db.Exec(context.Background(), fm.DownSQL)
				return err
			},
		}
	}
	return migrations
}
