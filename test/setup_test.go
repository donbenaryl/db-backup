package test

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

// TestDatabase represents a test database connection
type TestDatabase struct {
	Host     string
	Port     int
	Username string
	Password string
	Database string
	conn     *sql.DB
}

// NewTestDatabase creates a new test database connection
func NewTestDatabase(host string, port int, username, password, database string) *TestDatabase {
	return &TestDatabase{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		Database: database,
	}
}

// Connect establishes a connection to the test database
func (td *TestDatabase) Connect() error {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		td.Host, td.Port, td.Username, td.Password, td.Database)

	var err error
	td.conn, err = sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	// Test the connection
	if err := td.conn.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	return nil
}

// Close closes the database connection
func (td *TestDatabase) Close() error {
	if td.conn != nil {
		return td.conn.Close()
	}
	return nil
}

// SetupTestData creates test tables and inserts sample data
func (td *TestDatabase) SetupTestData() error {
	if td.conn == nil {
		return fmt.Errorf("database connection not established")
	}

	// Create users table
	createUsersTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(50) UNIQUE NOT NULL,
		email VARCHAR(100) UNIQUE NOT NULL,
		first_name VARCHAR(50) NOT NULL,
		last_name VARCHAR(50) NOT NULL,
		age INTEGER,
		phone VARCHAR(20),
		address TEXT,
		is_active BOOLEAN DEFAULT true,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := td.conn.Exec(createUsersTableSQL); err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	// Insert users data
	insertUsersSQL := `
	INSERT INTO users (username, email, first_name, last_name, age, phone, address, is_active) VALUES 
	('john_doe', 'john.doe@example.com', 'John', 'Doe', 28, '+1-555-0101', '123 Main St, New York, NY 10001', true),
	('jane_smith', 'jane.smith@example.com', 'Jane', 'Smith', 32, '+1-555-0102', '456 Oak Ave, Los Angeles, CA 90210', true),
	('bob_johnson', 'bob.johnson@example.com', 'Bob', 'Johnson', 45, '+1-555-0103', '789 Pine Rd, Chicago, IL 60601', true),
	('alice_brown', 'alice.brown@example.com', 'Alice', 'Brown', 29, '+1-555-0104', '321 Elm St, Houston, TX 77001', false),
	('charlie_wilson', 'charlie.wilson@example.com', 'Charlie', 'Wilson', 38, '+1-555-0105', '654 Maple Dr, Phoenix, AZ 85001', true),
	('diana_davis', 'diana.davis@example.com', 'Diana', 'Davis', 26, '+1-555-0106', '987 Cedar Ln, Philadelphia, PA 19101', true),
	('eve_miller', 'eve.miller@example.com', 'Eve', 'Miller', 41, '+1-555-0107', '147 Birch St, San Antonio, TX 78201', false),
	('frank_garcia', 'frank.garcia@example.com', 'Frank', 'Garcia', 33, '+1-555-0108', '258 Spruce Ave, San Diego, CA 92101', true)
	ON CONFLICT (email) DO NOTHING;`

	if _, err := td.conn.Exec(insertUsersSQL); err != nil {
		return fmt.Errorf("failed to insert users data: %w", err)
	}

	// Create test_users table (for backward compatibility)
	createTestUsersTableSQL := `
	CREATE TABLE IF NOT EXISTS test_users (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		email VARCHAR(100) UNIQUE NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := td.conn.Exec(createTestUsersTableSQL); err != nil {
		return fmt.Errorf("failed to create test_users table: %w", err)
	}

	// Insert test_users data
	insertTestUsersSQL := `
	INSERT INTO test_users (name, email) VALUES 
	('John Doe', 'john@example.com'),
	('Jane Smith', 'jane@example.com'),
	('Bob Johnson', 'bob@example.com')
	ON CONFLICT (email) DO NOTHING;`

	if _, err := td.conn.Exec(insertTestUsersSQL); err != nil {
		return fmt.Errorf("failed to insert test_users data: %w", err)
	}

	// Create another test table
	createTable2SQL := `
	CREATE TABLE IF NOT EXISTS test_products (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		price DECIMAL(10,2) NOT NULL,
		description TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := td.conn.Exec(createTable2SQL); err != nil {
		return fmt.Errorf("failed to create test_products table: %w", err)
	}

	// Insert product test data
	insertProductsSQL := `
	INSERT INTO test_products (name, price, description) VALUES 
	('Laptop', 999.99, 'High-performance laptop'),
	('Mouse', 29.99, 'Wireless mouse'),
	('Keyboard', 79.99, 'Mechanical keyboard')
	ON CONFLICT DO NOTHING;`

	if _, err := td.conn.Exec(insertProductsSQL); err != nil {
		return fmt.Errorf("failed to insert product test data: %w", err)
	}

	return nil
}

// VerifyTestData verifies that test data exists in the database
func (td *TestDatabase) VerifyTestData() error {
	if td.conn == nil {
		return fmt.Errorf("database connection not established")
	}

	// Check users table
	var usersCount int
	if err := td.conn.QueryRow("SELECT COUNT(*) FROM users").Scan(&usersCount); err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	if usersCount < 8 {
		return fmt.Errorf("expected at least 8 users, got %d", usersCount)
	}

	// Check test_users table (for backward compatibility)
	var testUserCount int
	if err := td.conn.QueryRow("SELECT COUNT(*) FROM test_users").Scan(&testUserCount); err != nil {
		return fmt.Errorf("failed to count test_users: %w", err)
	}

	if testUserCount < 3 {
		return fmt.Errorf("expected at least 3 test_users, got %d", testUserCount)
	}

	// Check products table
	var productCount int
	if err := td.conn.QueryRow("SELECT COUNT(*) FROM test_products").Scan(&productCount); err != nil {
		return fmt.Errorf("failed to count products: %w", err)
	}

	if productCount < 3 {
		return fmt.Errorf("expected at least 3 products, got %d", productCount)
	}

	return nil
}

// WaitForDatabase waits for the database to be ready
func WaitForDatabase(host string, port int, username, password, database string, maxRetries int) error {
	for i := 0; i < maxRetries; i++ {
		db := NewTestDatabase(host, port, username, password, database)
		if err := db.Connect(); err == nil {
			db.Close()
			return nil
		}
		log.Printf("Waiting for database %s:%d (attempt %d/%d)...", host, port, i+1, maxRetries)
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("database %s:%d not ready after %d attempts", host, port, maxRetries)
}

// SetupTestEnvironment sets up the test environment
func SetupTestEnvironment() error {
	// Wait for databases to be ready
	if err := WaitForDatabase("localhost", 5433, "testuser", "testpass", "testdb1", 30); err != nil {
		return fmt.Errorf("testdb1 not ready: %w", err)
	}

	if err := WaitForDatabase("localhost", 5434, "testuser", "testpass", "testdb2", 30); err != nil {
		return fmt.Errorf("testdb2 not ready: %w", err)
	}

	// Setup test data for both databases
	db1 := NewTestDatabase("localhost", 5433, "testuser", "testpass", "testdb1")
	if err := db1.Connect(); err != nil {
		return fmt.Errorf("failed to connect to testdb1: %w", err)
	}
	defer db1.Close()

	if err := db1.SetupTestData(); err != nil {
		return fmt.Errorf("failed to setup test data for testdb1: %w", err)
	}

	db2 := NewTestDatabase("localhost", 5434, "testuser", "testpass", "testdb2")
	if err := db2.Connect(); err != nil {
		return fmt.Errorf("failed to connect to testdb2: %w", err)
	}
	defer db2.Close()

	if err := db2.SetupTestData(); err != nil {
		return fmt.Errorf("failed to setup test data for testdb2: %w", err)
	}

	// Create test backup directory
	if err := os.MkdirAll("/tmp/test-backups", 0755); err != nil {
		return fmt.Errorf("failed to create test backup directory: %w", err)
	}

	return nil
}

// CleanupTestEnvironment cleans up the test environment
func CleanupTestEnvironment() error {
	// Remove test backup directory
	if err := os.RemoveAll("/tmp/test-backups"); err != nil {
		log.Printf("Warning: failed to cleanup test backup directory: %v", err)
	}
	return nil
}
