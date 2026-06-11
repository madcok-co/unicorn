package discovery

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ============ Defaults ============

func TestAWSCloudMapInstance_Defaults(t *testing.T) {
	inst := &AWSCloudMapInstance{
		NamespaceID: "ns-abc",
		ServiceID:   "srv-abc",
		InstanceID:  "i-123",
	}
	inst.defaults()

	if inst.HeartbeatInterval != 20*time.Second {
		t.Fatalf("expected 20s heartbeat, got %v", inst.HeartbeatInterval)
	}
	if inst.Attributes == nil {
		t.Fatal("Attributes should be initialised to non-nil map")
	}
}

func TestAWSCloudMapInstance_ExplicitValues_NotOverwritten(t *testing.T) {
	inst := &AWSCloudMapInstance{
		HeartbeatInterval: 5 * time.Second,
		Attributes:        map[string]string{"AWS_INSTANCE_IPV4": "10.0.0.1"},
	}
	inst.defaults()

	if inst.HeartbeatInterval != 5*time.Second {
		t.Fatalf("explicit heartbeat should not be overwritten, got %v", inst.HeartbeatInterval)
	}
	if inst.Attributes["AWS_INSTANCE_IPV4"] != "10.0.0.1" {
		t.Fatal("existing attributes should not be overwritten")
	}
}

// ============ Constructor ============

func TestNewAWSCloudMap_SetsFields(t *testing.T) {
	inst := &AWSCloudMapInstance{
		NamespaceID: "ns-abc",
		ServiceID:   "srv-abc",
		InstanceID:  "i-123",
	}
	var registered bool
	reg := NewAWSCloudMap(
		inst,
		func(_ context.Context, _ *AWSCloudMapInstance) error { registered = true; return nil },
		func(_ context.Context, _, _ string) error { return nil },
		nil,
	)

	if reg.instance != inst {
		t.Fatal("instance not stored")
	}
	if reg.register == nil || reg.deregister == nil {
		t.Fatal("function fields should not be nil")
	}
	if reg.health != nil {
		t.Fatal("health should be nil when passed nil")
	}
	_ = registered
}

// ============ Name ============

func TestAWSCloudMapRegistrar_Name(t *testing.T) {
	inst := &AWSCloudMapInstance{InstanceID: "i-0abc123"}
	r := NewAWSCloudMap(inst,
		func(_ context.Context, _ *AWSCloudMapInstance) error { return nil },
		func(_ context.Context, _, _ string) error { return nil },
		nil,
	)

	name := r.Name()
	if !strings.Contains(name, "i-0abc123") {
		t.Fatalf("Name() should contain instance ID, got %q", name)
	}
	if !strings.Contains(name, "cloudmap") {
		t.Fatalf("Name() should contain 'cloudmap', got %q", name)
	}
}

// ============ Start — register + heartbeat ============

func TestAWSCloudMapRegistrar_Start_RegisterCalled(t *testing.T) {
	var registered bool
	inst := &AWSCloudMapInstance{
		NamespaceID: "ns-abc",
		ServiceID:   "srv-abc",
		InstanceID:  "i-123",
	}
	r := NewAWSCloudMap(inst,
		func(_ context.Context, got *AWSCloudMapInstance) error {
			registered = true
			if got.InstanceID != "i-123" {
				t.Errorf("unexpected InstanceID: %s", got.InstanceID)
			}
			return nil
		},
		func(_ context.Context, _, _ string) error { return nil },
		nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	r.Start(ctx)

	if !registered {
		t.Fatal("register func should have been called")
	}
}

func TestAWSCloudMapRegistrar_Start_HeartbeatCalled(t *testing.T) {
	var heartbeats atomic.Int32
	inst := &AWSCloudMapInstance{
		ServiceID:         "srv-abc",
		InstanceID:        "i-123",
		HeartbeatInterval: 20 * time.Millisecond,
	}
	r := NewAWSCloudMap(inst,
		func(_ context.Context, _ *AWSCloudMapInstance) error { return nil },
		func(_ context.Context, _, _ string) error { return nil },
		func(_ context.Context, serviceID, instanceID string) error {
			heartbeats.Add(1)
			if serviceID != "srv-abc" {
				t.Errorf("unexpected serviceID: %s", serviceID)
			}
			if instanceID != "i-123" {
				t.Errorf("unexpected instanceID: %s", instanceID)
			}
			return nil
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	r.Start(ctx)

	if heartbeats.Load() == 0 {
		t.Fatal("health func should have been called at least once")
	}
}

func TestAWSCloudMapRegistrar_Start_NilHealth_JustWaits(t *testing.T) {
	inst := &AWSCloudMapInstance{ServiceID: "srv", InstanceID: "i-1"}
	r := NewAWSCloudMap(inst,
		func(_ context.Context, _ *AWSCloudMapInstance) error { return nil },
		func(_ context.Context, _, _ string) error { return nil },
		nil, // nil health func
	)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Should return when ctx is cancelled, not panic or loop forever
	err := r.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAWSCloudMapRegistrar_Start_RegisterError(t *testing.T) {
	inst := &AWSCloudMapInstance{ServiceID: "srv", InstanceID: "i-1"}
	r := NewAWSCloudMap(inst,
		func(_ context.Context, _ *AWSCloudMapInstance) error {
			return errors.New("access denied")
		},
		func(_ context.Context, _, _ string) error { return nil },
		nil,
	)

	ctx := context.Background()
	err := r.Start(ctx)
	if err == nil {
		t.Fatal("expected error when register fails")
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Fatalf("expected 'access denied' in error, got: %v", err)
	}
}

func TestAWSCloudMapRegistrar_Start_HeartbeatError_Continues(t *testing.T) {
	// Heartbeat errors should be swallowed (non-fatal) — loop continues.
	var calls atomic.Int32
	inst := &AWSCloudMapInstance{
		ServiceID:         "srv",
		InstanceID:        "i-1",
		HeartbeatInterval: 20 * time.Millisecond,
	}
	r := NewAWSCloudMap(inst,
		func(_ context.Context, _ *AWSCloudMapInstance) error { return nil },
		func(_ context.Context, _, _ string) error { return nil },
		func(_ context.Context, _, _ string) error {
			calls.Add(1)
			return errors.New("temporary error")
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	r.Start(ctx) // should not return early due to heartbeat error

	if calls.Load() < 2 {
		t.Fatalf("expected multiple heartbeat calls despite errors, got %d", calls.Load())
	}
}

// ============ Stop — deregister ============

func TestAWSCloudMapRegistrar_Stop_DeregisterCalled(t *testing.T) {
	var gotServiceID, gotInstanceID string
	inst := &AWSCloudMapInstance{
		ServiceID:  "srv-abc",
		InstanceID: "i-123",
	}
	r := NewAWSCloudMap(inst,
		func(_ context.Context, _ *AWSCloudMapInstance) error { return nil },
		func(_ context.Context, serviceID, instanceID string) error {
			gotServiceID = serviceID
			gotInstanceID = instanceID
			return nil
		},
		nil,
	)

	if err := r.Stop(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotServiceID != "srv-abc" {
		t.Fatalf("expected serviceID 'srv-abc', got %q", gotServiceID)
	}
	if gotInstanceID != "i-123" {
		t.Fatalf("expected instanceID 'i-123', got %q", gotInstanceID)
	}
}

func TestAWSCloudMapRegistrar_Stop_DeregisterError(t *testing.T) {
	inst := &AWSCloudMapInstance{ServiceID: "srv", InstanceID: "i-1"}
	r := NewAWSCloudMap(inst,
		func(_ context.Context, _ *AWSCloudMapInstance) error { return nil },
		func(_ context.Context, _, _ string) error { return errors.New("deregister failed") },
		nil,
	)

	err := r.Stop(context.Background())
	if err == nil {
		t.Fatal("expected error from deregister")
	}
}
