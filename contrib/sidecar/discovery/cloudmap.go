package discovery

import (
	"context"
	"fmt"
	"time"
)

// AWSCloudMapInstance describes a service instance to register with AWS Cloud Map.
type AWSCloudMapInstance struct {
	// NamespaceID is the Cloud Map namespace ID (e.g. "ns-xxxxxxxxxxxx")
	NamespaceID string

	// ServiceID is the Cloud Map service ID (e.g. "srv-xxxxxxxxxxxx")
	ServiceID string

	// InstanceID uniquely identifies this instance. Recommended: use the EC2/ECS instance ID.
	InstanceID string

	// Attributes are the instance attributes visible to DNS and API callers.
	// Standard keys for HTTP namespaces:
	//   "AWS_INSTANCE_IPV4" — private IP of the instance
	//   "AWS_INSTANCE_PORT" — port the service listens on
	//   "AWS_INIT_HEALTH_STATUS" — "HEALTHY" or "UNHEALTHY" (default HEALTHY)
	Attributes map[string]string

	// HeartbeatInterval controls how often the health status is refreshed.
	// Default: 20s. Must be shorter than the Cloud Map TTL on the service.
	HeartbeatInterval time.Duration
}

func (i *AWSCloudMapInstance) defaults() {
	if i.HeartbeatInterval == 0 {
		i.HeartbeatInterval = 20 * time.Second
	}
	if i.Attributes == nil {
		i.Attributes = make(map[string]string)
	}
}

// CloudMapRegisterFunc registers an instance with AWS Cloud Map.
// Implement using aws-sdk-go-v2:
//
//	import "github.com/aws/aws-sdk-go-v2/service/servicediscovery"
//
//	client := servicediscovery.NewFromConfig(awsCfg)
//	register := func(ctx context.Context, inst *discovery.AWSCloudMapInstance) error {
//	    _, err := client.RegisterInstance(ctx, &servicediscovery.RegisterInstanceInput{
//	        ServiceId:  aws.String(inst.ServiceID),
//	        InstanceId: aws.String(inst.InstanceID),
//	        Attributes: inst.Attributes,
//	    })
//	    return err
//	}
type CloudMapRegisterFunc func(ctx context.Context, instance *AWSCloudMapInstance) error

// CloudMapDeregisterFunc deregisters an instance from AWS Cloud Map.
//
//	deregister := func(ctx context.Context, serviceID, instanceID string) error {
//	    _, err := client.DeregisterInstance(ctx, &servicediscovery.DeregisterInstanceInput{
//	        ServiceId:  aws.String(serviceID),
//	        InstanceId: aws.String(instanceID),
//	    })
//	    return err
//	}
type CloudMapDeregisterFunc func(ctx context.Context, serviceID, instanceID string) error

// CloudMapHealthFunc updates the health status of an instance.
// Pass nil to skip active health updates (Cloud Map will use its own checks).
//
//	health := func(ctx context.Context, serviceID, instanceID string) error {
//	    _, err := client.UpdateInstanceCustomHealthStatus(ctx,
//	        &servicediscovery.UpdateInstanceCustomHealthStatusInput{
//	            ServiceId:  aws.String(serviceID),
//	            InstanceId: aws.String(instanceID),
//	            Status:     types.CustomHealthStatusHealthy,
//	        })
//	    return err
//	}
type CloudMapHealthFunc func(ctx context.Context, serviceID, instanceID string) error

// AWSCloudMapRegistrar is a sidecar that registers and deregisters a service
// instance with AWS Cloud Map. Compatible with ECS, EC2, and Lambda deployments.
type AWSCloudMapRegistrar struct {
	instance   *AWSCloudMapInstance
	register   CloudMapRegisterFunc
	deregister CloudMapDeregisterFunc
	health     CloudMapHealthFunc
}

// NewAWSCloudMap creates an AWSCloudMapRegistrar.
//
// health may be nil if you rely on Cloud Map's built-in Route 53 health checks
// or if the service is configured with no health checks.
func NewAWSCloudMap(
	instance *AWSCloudMapInstance,
	register CloudMapRegisterFunc,
	deregister CloudMapDeregisterFunc,
	health CloudMapHealthFunc,
) *AWSCloudMapRegistrar {
	instance.defaults()
	return &AWSCloudMapRegistrar{
		instance:   instance,
		register:   register,
		deregister: deregister,
		health:     health,
	}
}

// Name implements contracts.Sidecar.
func (r *AWSCloudMapRegistrar) Name() string {
	return fmt.Sprintf("aws-cloudmap-registrar(%s)", r.instance.InstanceID)
}

// Start implements contracts.Sidecar. Registers the instance then sends periodic
// health updates until ctx is cancelled.
func (r *AWSCloudMapRegistrar) Start(ctx context.Context) error {
	if err := r.register(ctx, r.instance); err != nil {
		return fmt.Errorf("cloud map register %s: %w", r.instance.InstanceID, err)
	}

	if r.health == nil {
		// No active health reporting — just wait for shutdown signal
		<-ctx.Done()
		return nil
	}

	ticker := time.NewTicker(r.instance.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			_ = r.health(ctx, r.instance.ServiceID, r.instance.InstanceID)
		}
	}
}

// Stop implements contracts.Sidecar. Deregisters the instance from Cloud Map.
func (r *AWSCloudMapRegistrar) Stop(ctx context.Context) error {
	return r.deregister(ctx, r.instance.ServiceID, r.instance.InstanceID)
}
