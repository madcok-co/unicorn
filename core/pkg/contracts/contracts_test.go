package contracts

import (
	"context"
	"crypto/tls"
	"testing"
	"time"
)

// ============ Identity tests ============

func TestIdentity_HasRole(t *testing.T) {
	id := &Identity{
		Roles: []string{"admin", "user", "editor"},
	}

	if !id.HasRole("admin") {
		t.Error("expected HasRole('admin') to be true")
	}
	if !id.HasRole("user") {
		t.Error("expected HasRole('user') to be true")
	}
	if id.HasRole("superadmin") {
		t.Error("expected HasRole('superadmin') to be false")
	}
	if id.HasRole("") {
		t.Error("expected HasRole('') to be false")
	}
}

func TestIdentity_HasRole_EmptyRoles(t *testing.T) {
	id := &Identity{}
	if id.HasRole("admin") {
		t.Error("expected HasRole to be false when Roles is nil")
	}
}

func TestIdentity_HasScope(t *testing.T) {
	id := &Identity{
		Scopes: []string{"read:users", "write:users", "delete:users"},
	}

	if !id.HasScope("read:users") {
		t.Error("expected HasScope('read:users') to be true")
	}
	if !id.HasScope("delete:users") {
		t.Error("expected HasScope('delete:users') to be true")
	}
	if id.HasScope("admin:users") {
		t.Error("expected HasScope('admin:users') to be false")
	}
}

func TestIdentity_HasScope_EmptyScopes(t *testing.T) {
	id := &Identity{}
	if id.HasScope("read:users") {
		t.Error("expected HasScope to be false when Scopes is nil")
	}
}

func TestIdentity_HasAnyRole(t *testing.T) {
	id := &Identity{
		Roles: []string{"admin", "user"},
	}

	if !id.HasAnyRole("admin") {
		t.Error("expected HasAnyRole('admin') to be true")
	}
	if !id.HasAnyRole("user", "editor") {
		t.Error("expected HasAnyRole('user', 'editor') to be true (user matches)")
	}
	if id.HasAnyRole("superadmin", "editor") {
		t.Error("expected HasAnyRole('superadmin', 'editor') to be false")
	}
	if id.HasAnyRole() {
		t.Error("expected HasAnyRole() with no args to be false")
	}
}

func TestIdentity_HasAllScopes(t *testing.T) {
	id := &Identity{
		Scopes: []string{"read:users", "write:users", "delete:users"},
	}

	if !id.HasAllScopes("read:users", "write:users") {
		t.Error("expected HasAllScopes('read:users', 'write:users') to be true")
	}
	if id.HasAllScopes("read:users", "admin:users") {
		t.Error("expected HasAllScopes with a missing scope to be false")
	}
	if !id.HasAllScopes("read:users", "write:users", "delete:users") {
		t.Error("expected HasAllScopes with all matching scopes to be true")
	}
}

func TestIdentity_HasAllScopes_EmptyArgs(t *testing.T) {
	id := &Identity{
		Scopes: []string{"read:users"},
	}

	// With no scopes requested, none are missing so true
	if !id.HasAllScopes() {
		t.Error("expected HasAllScopes() with no args to be true (vacuous truth)")
	}
}

// ============ TLSConfig tests ============

func TestTLSConfig_ToTLSConfig_NotEnabled(t *testing.T) {
	cfg := &TLSConfig{
		Enabled: false,
	}
	result, err := cfg.ToTLSConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil *tls.Config when not enabled")
	}
}

func TestTLSConfig_ToTLSConfig_EnabledDefaults(t *testing.T) {
	cfg := &TLSConfig{
		Enabled: true,
	}
	result, err := cfg.ToTLSConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil *tls.Config")
	}
	if result.MinVersion != tls.VersionTLS12 {
		t.Errorf("expected default MinVersion %d, got %d", tls.VersionTLS12, result.MinVersion)
	}
}

func TestTLSConfig_ToTLSConfig_RespectsMinVersion(t *testing.T) {
	cfg := &TLSConfig{
		Enabled:    true,
		MinVersion: tls.VersionTLS13,
	}
	result, err := cfg.ToTLSConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MinVersion != tls.VersionTLS13 {
		t.Errorf("expected MinVersion %d, got %d", tls.VersionTLS13, result.MinVersion)
	}
}

func TestTLSConfig_ToTLSConfig_RespectsMaxVersion(t *testing.T) {
	cfg := &TLSConfig{
		Enabled:    true,
		MaxVersion: tls.VersionTLS13,
	}
	result, err := cfg.ToTLSConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MaxVersion != tls.VersionTLS13 {
		t.Errorf("expected MaxVersion %d, got %d", tls.VersionTLS13, result.MaxVersion)
	}
}

func TestTLSConfig_ToTLSConfig_RespectsInsecureSkipVerify(t *testing.T) {
	cfg := &TLSConfig{
		Enabled:            true,
		InsecureSkipVerify: true,
	}
	result, err := cfg.ToTLSConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify to be true")
	}
}

func TestTLSConfig_ToTLSConfig_RespectsClientAuth(t *testing.T) {
	cfg := &TLSConfig{
		Enabled:    true,
		ClientAuth: tls.RequireAndVerifyClientCert,
	}
	result, err := cfg.ToTLSConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Errorf("expected ClientAuth %d, got %d", tls.RequireAndVerifyClientCert, result.ClientAuth)
	}
}

func TestTLSConfig_ToTLSConfig_RespectsCipherSuites(t *testing.T) {
	cfg := &TLSConfig{
		Enabled:      true,
		CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256},
	}
	result, err := cfg.ToTLSConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.CipherSuites) != 2 {
		t.Errorf("expected 2 cipher suites, got %d", len(result.CipherSuites))
	}
	if result.CipherSuites[0] != tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384 {
		t.Error("first cipher suite does not match")
	}
}

func TestTLSConfig_ToTLSConfig_EmptyCipherSuites(t *testing.T) {
	cfg := &TLSConfig{
		Enabled: true,
	}
	result, err := cfg.ToTLSConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.CipherSuites) != 0 {
		t.Error("expected no cipher suites set when not specified")
	}
}

// ============ CORSConfig tests ============

func TestDefaultCORSConfig(t *testing.T) {
	cfg := DefaultCORSConfig()
	if cfg == nil {
		t.Fatal("expected non-nil CORSConfig")
	}

	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "*" {
		t.Errorf("expected AllowedOrigins ['*'], got %v", cfg.AllowedOrigins)
	}

	expectedMethods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	if len(cfg.AllowedMethods) != len(expectedMethods) {
		t.Errorf("expected %d allowed methods, got %d", len(expectedMethods), len(cfg.AllowedMethods))
	}
	for i, m := range expectedMethods {
		if i >= len(cfg.AllowedMethods) || cfg.AllowedMethods[i] != m {
			t.Errorf("expected AllowedMethods[%d] = %q, got %v", i, m, cfg.AllowedMethods)
		}
	}

	expectedHeaders := []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"}
	if len(cfg.AllowedHeaders) != len(expectedHeaders) {
		t.Errorf("expected %d allowed headers, got %d", len(expectedHeaders), len(cfg.AllowedHeaders))
	}

	if len(cfg.ExposedHeaders) != 1 || cfg.ExposedHeaders[0] != "X-Request-ID" {
		t.Errorf("expected ExposedHeaders ['X-Request-ID'], got %v", cfg.ExposedHeaders)
	}

	if cfg.AllowCredentials != false {
		t.Error("expected AllowCredentials to be false by default")
	}

	if cfg.MaxAge != 86400 {
		t.Errorf("expected MaxAge 86400, got %d", cfg.MaxAge)
	}
}

// ============ Context Identity tests ============

func TestGetIdentityFromContext_NotSet(t *testing.T) {
	ctx := context.Background()
	id, ok := GetIdentityFromContext(ctx)
	if ok {
		t.Error("expected ok to be false when identity not set")
	}
	if id != nil {
		t.Error("expected nil identity when not set")
	}
}

func TestSetAndGetIdentityFromContext(t *testing.T) {
	ctx := context.Background()
	identity := &Identity{
		ID:    "user-123",
		Name:  "Test User",
		Roles: []string{"admin"},
	}

	ctx = SetIdentityInContext(ctx, identity)
	retrieved, ok := GetIdentityFromContext(ctx)

	if !ok {
		t.Fatal("expected ok to be true after setting identity")
	}
	if retrieved == nil {
		t.Fatal("expected non-nil identity")
	}
	if retrieved.ID != "user-123" {
		t.Errorf("expected ID 'user-123', got %q", retrieved.ID)
	}
	if retrieved.Name != "Test User" {
		t.Errorf("expected Name 'Test User', got %q", retrieved.Name)
	}
	if len(retrieved.Roles) != 1 || retrieved.Roles[0] != "admin" {
		t.Errorf("expected Roles ['admin'], got %v", retrieved.Roles)
	}
}

func TestSetIdentityInContext_Overwrite(t *testing.T) {
	ctx := context.Background()
	first := &Identity{ID: "first", Name: "First"}
	second := &Identity{ID: "second", Name: "Second"}

	ctx = SetIdentityInContext(ctx, first)
	ctx = SetIdentityInContext(ctx, second)

	retrieved, ok := GetIdentityFromContext(ctx)
	if !ok {
		t.Fatal("expected identity to be set")
	}
	if retrieved.ID != "second" {
		t.Errorf("expected overwritten ID 'second', got %q", retrieved.ID)
	}
}

// ============ DatabaseConfig tests ============

func TestDatabaseConfig_Creation(t *testing.T) {
	cfg := DatabaseConfig{
		Driver:   "postgres",
		Host:     "localhost",
		Port:     5432,
		Username: "admin",
		Password: "secret",
		Database: "mydb",
		SSLMode:  "disable",
		Options: map[string]string{
			"application_name": "myapp",
		},
	}

	if cfg.Driver != "postgres" {
		t.Errorf("expected Driver 'postgres', got %q", cfg.Driver)
	}
	if cfg.Host != "localhost" {
		t.Errorf("expected Host 'localhost', got %q", cfg.Host)
	}
	if cfg.Port != 5432 {
		t.Errorf("expected Port 5432, got %d", cfg.Port)
	}
	if cfg.Username != "admin" {
		t.Errorf("expected Username 'admin', got %q", cfg.Username)
	}
	if cfg.Password != "secret" {
		t.Errorf("expected Password 'secret', got %q", cfg.Password)
	}
	if cfg.Database != "mydb" {
		t.Errorf("expected Database 'mydb', got %q", cfg.Database)
	}
	if cfg.SSLMode != "disable" {
		t.Errorf("expected SSLMode 'disable', got %q", cfg.SSLMode)
	}
	if cfg.Options["application_name"] != "myapp" {
		t.Errorf("expected Options['application_name'] = 'myapp', got %q", cfg.Options["application_name"])
	}
}

func TestDatabaseConfig_Defaults(t *testing.T) {
	cfg := DatabaseConfig{}
	if cfg.Driver != "" {
		t.Error("expected empty Driver by default")
	}
	if cfg.Port != 0 {
		t.Error("expected Port 0 by default")
	}
}

// ============ PutOptions tests ============

func TestPutOptions_Creation(t *testing.T) {
	opts := PutOptions{
		ContentType:  "application/json",
		CacheControl: "max-age=3600",
		ACL:          "public-read",
		Metadata: map[string]string{
			"uploaded-by": "test-user",
		},
	}

	if opts.ContentType != "application/json" {
		t.Errorf("expected ContentType 'application/json', got %q", opts.ContentType)
	}
	if opts.CacheControl != "max-age=3600" {
		t.Errorf("expected CacheControl 'max-age=3600', got %q", opts.CacheControl)
	}
	if opts.ACL != "public-read" {
		t.Errorf("expected ACL 'public-read', got %q", opts.ACL)
	}
	if opts.Metadata["uploaded-by"] != "test-user" {
		t.Errorf("expected Metadata['uploaded-by'] = 'test-user', got %q", opts.Metadata["uploaded-by"])
	}
}

// ============ FileInfo tests ============

func TestFileInfo_Creation(t *testing.T) {
	now := time.Now()
	info := FileInfo{
		Path:         "/uploads/report.pdf",
		Size:         2048,
		ContentType:  "application/pdf",
		LastModified: now,
		ETag:         "abc123",
		Metadata: map[string]string{
			"source": "s3",
		},
	}

	if info.Path != "/uploads/report.pdf" {
		t.Errorf("expected Path '/uploads/report.pdf', got %q", info.Path)
	}
	if info.Size != 2048 {
		t.Errorf("expected Size 2048, got %d", info.Size)
	}
	if info.ContentType != "application/pdf" {
		t.Errorf("expected ContentType 'application/pdf', got %q", info.ContentType)
	}
	if !info.LastModified.Equal(now) {
		t.Errorf("expected LastModified %v, got %v", now, info.LastModified)
	}
	if info.ETag != "abc123" {
		t.Errorf("expected ETag 'abc123', got %q", info.ETag)
	}
	if info.Metadata["source"] != "s3" {
		t.Errorf("expected Metadata['source'] = 's3', got %q", info.Metadata["source"])
	}
}

// ============ BrokerMessage tests ============

func TestNewBrokerMessage(t *testing.T) {
	body := []byte(`{"hello": "world"}`)
	msg := NewBrokerMessage("orders", body)

	if msg == nil {
		t.Fatal("expected non-nil BrokerMessage")
	}
	if msg.Topic != "orders" {
		t.Errorf("expected Topic 'orders', got %q", msg.Topic)
	}
	if string(msg.Body) != `{"hello": "world"}` {
		t.Errorf("expected Body %q, got %q", `{"hello": "world"}`, string(msg.Body))
	}
	if msg.Headers == nil {
		t.Error("expected Headers map to be initialized")
	}
	if len(msg.Headers) != 0 {
		t.Errorf("expected empty Headers, got %v", msg.Headers)
	}
	if msg.Timestamp.IsZero() {
		t.Error("expected non-zero Timestamp")
	}
}

func TestNewBrokerMessageWithKey(t *testing.T) {
	body := []byte("payload")
	key := []byte("partition-key-42")
	msg := NewBrokerMessageWithKey("events", key, body)

	if msg == nil {
		t.Fatal("expected non-nil BrokerMessage")
	}
	if msg.Topic != "events" {
		t.Errorf("expected Topic 'events', got %q", msg.Topic)
	}
	if string(msg.Key) != "partition-key-42" {
		t.Errorf("expected Key 'partition-key-42', got %q", string(msg.Key))
	}
	if string(msg.Body) != "payload" {
		t.Errorf("expected Body 'payload', got %q", string(msg.Body))
	}
}

func TestBrokerMessage_SetHeader(t *testing.T) {
	msg := NewBrokerMessage("test", []byte("body"))

	result := msg.SetHeader("X-Custom-Header", "value1")
	if result != msg {
		t.Error("expected SetHeader to return same message for chaining")
	}
	if msg.Headers["X-Custom-Header"] != "value1" {
		t.Errorf("expected header 'X-Custom-Header' = 'value1', got %q", msg.Headers["X-Custom-Header"])
	}

	// Chaining
	msg.SetHeader("X-Second", "value2").SetHeader("X-Third", "value3")
	if len(msg.Headers) != 3 {
		t.Errorf("expected 3 headers, got %d", len(msg.Headers))
	}
}

func TestBrokerMessage_SetHeader_NilHeaders(t *testing.T) {
	msg := &BrokerMessage{
		Topic: "test",
		Body:  []byte("body"),
	}
	msg.SetHeader("X-Key", "X-Value")
	if msg.Headers == nil {
		t.Fatal("expected Headers to be initialized by SetHeader")
	}
	if msg.Headers["X-Key"] != "X-Value" {
		t.Errorf("expected header value, got %q", msg.Headers["X-Key"])
	}
}

func TestBrokerMessage_GetHeader(t *testing.T) {
	msg := NewBrokerMessage("test", []byte("body"))
	msg.SetHeader("Content-Type", "application/json")

	if v := msg.GetHeader("Content-Type"); v != "application/json" {
		t.Errorf("expected 'application/json', got %q", v)
	}
	if v := msg.GetHeader("Non-Existent"); v != "" {
		t.Errorf("expected empty string for missing header, got %q", v)
	}
}

func TestBrokerMessage_GetHeader_NilHeaders(t *testing.T) {
	msg := &BrokerMessage{
		Topic: "test",
		Body:  []byte("body"),
	}
	if v := msg.GetHeader("any"); v != "" {
		t.Errorf("expected empty string when Headers is nil, got %q", v)
	}
}

func TestBrokerMessage_WithRetry(t *testing.T) {
	msg := NewBrokerMessage("test", []byte("body"))
	result := msg.WithRetry(5)

	if result != msg {
		t.Error("expected WithRetry to return same message for chaining")
	}
	if msg.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", msg.MaxRetries)
	}
}
