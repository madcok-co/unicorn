# Sidecar Pattern

Unicorn supports an in-process sidecar model for running auxiliary processes
alongside your main application. Sidecars handle cross-cutting concerns in
production service mesh environments without polluting business logic.

## Overview

| Sidecar | Package | Purpose |
|---------|---------|---------|
| **ManagementServer** | `contrib/sidecar/management` | Kubernetes probes, Prometheus metrics, pprof |
| **ConfigWatcher** | `contrib/sidecar/configwatcher` | Hot-reload config without restart |
| **ServiceRegistrar** | `contrib/sidecar/discovery` | Auto register/deregister with Consul |
| **SecretRotator** | `contrib/sidecar/secretrotator` | Rotate credentials from Vault/K8s |

## How It Works

Sidecars implement the `contracts.Sidecar` interface and hook into the application
lifecycle via `app.AddSidecar()`.

```
App.Start()
  │
  ├── HTTP adapter started
  ├── Broker adapter started
  ├── Cron adapter started
  │
  ├── Sidecar 1 started (concurrent goroutine)
  ├── Sidecar 2 started (concurrent goroutine)
  └── Sidecar N started (concurrent goroutine)

App.Shutdown()
  │
  ├── Sidecar N stopped (5s graceful window)
  ├── ...
  ├── Sidecar 1 stopped
  │
  ├── DB closed
  ├── Cache closed
  └── Broker disconnected
```

**Key properties:**

- Sidecars start **after** main adapters are up
- Sidecar failures are **non-fatal** — the app logs a warning and continues
- Sidecars stop **before** infrastructure is closed (DB, cache, broker)
- Each `Stop()` call gets a **5-second graceful window**

## The Sidecar Interface

```go
import "github.com/madcok-co/unicorn/core/pkg/contracts"

type Sidecar interface {
    Name()              string
    Start(ctx context.Context) error   // blocks until ctx cancelled
    Stop(ctx context.Context)  error   // graceful shutdown
}
```

You can implement this interface to build custom sidecars.

## Installation

```bash
# Management server (health probes + metrics + pprof)
go get github.com/madcok-co/unicorn/contrib/sidecar/management@latest

# Config hot-reload
go get github.com/madcok-co/unicorn/contrib/sidecar/configwatcher@latest

# Consul service discovery
go get github.com/madcok-co/unicorn/contrib/sidecar/discovery@latest

# Secret rotation
go get github.com/madcok-co/unicorn/contrib/sidecar/secretrotator@latest
```

---

## ManagementServer

A dedicated HTTP server on a separate port (default `9090`) for infrastructure
concerns that must not share the application port.

### Endpoints

| Endpoint | Purpose |
|----------|---------|
| `GET /health` | Aggregate health check (all registered checkers) |
| `GET /health/live` | Kubernetes liveness probe — always `200` while running |
| `GET /health/ready` | Kubernetes readiness probe — `200` when all checkers pass |
| `GET /health/startup` | Kubernetes startup probe — `200` after `SetStartupComplete()` |
| `GET /metrics` | Prometheus text format — Go runtime stats + custom metrics |
| `GET /debug/pprof/*` | Go runtime profiler |

### Basic Usage

```go
import "github.com/madcok-co/unicorn/contrib/sidecar/management"

mgmt := management.New(&management.Config{
    Port:          9090,
    EnablePprof:   true,
    EnableMetrics: true,
})

app.AddSidecar(mgmt)
```

### Health Checkers

Register checkers for your infrastructure dependencies:

```go
mgmt.AddChecker("database", func(ctx context.Context) management.HealthResult {
    if err := db.PingContext(ctx); err != nil {
        return management.HealthResult{
            Status:  management.StatusDown,
            Message: err.Error(),
        }
    }
    return management.HealthResult{Status: management.StatusUp}
})

mgmt.AddChecker("cache", func(ctx context.Context) management.HealthResult {
    if err := redis.Ping(ctx).Err(); err != nil {
        // Cache down is degraded, not down — app still accepts traffic
        return management.HealthResult{
            Status:  management.StatusDegraded,
            Message: err.Error(),
        }
    }
    return management.HealthResult{Status: management.StatusUp}
})

mgmt.AddChecker("upstream", func(ctx context.Context) management.HealthResult {
    resp, err := http.Get("http://payment-service/health/live")
    if err != nil || resp.StatusCode != 200 {
        return management.HealthResult{Status: management.StatusDown, Message: "payment-service unreachable"}
    }
    return management.HealthResult{Status: management.StatusUp}
})
```

**Status semantics:**

| Status | `/health/ready` | Meaning |
|--------|----------------|---------|
| `up` | `200 OK` | Component is healthy |
| `degraded` | `200 OK` | Reduced capacity, still accepts traffic |
| `down` | `503` | Component is unavailable |

### Custom Metrics

Expose application-level metrics in Prometheus format:

```go
var requestCount int64

mgmt.AddMetricProvider(func() []management.MetricPoint {
    return []management.MetricPoint{
        {
            Name:  "app_requests_total",
            Help:  "Total number of requests processed.",
            Type:  "counter",
            Value: float64(atomic.LoadInt64(&requestCount)),
        },
        {
            Name:  "app_active_orders",
            Help:  "Number of orders currently being processed.",
            Type:  "gauge",
            Labels: map[string]string{"region": "us-east-1"},
            Value: float64(orderQueue.Len()),
        },
    }
})
```

### Readiness and Startup Control

```go
mgmt := management.New(nil)

// Block traffic during warm-up (e.g. loading ML model, warming cache)
mgmt.SetReady(false)

app.OnStart(func() error {
    warmUpCache()
    mgmt.SetReady(true)          // start accepting traffic
    mgmt.SetStartupComplete()    // mark startup probe as done
    return nil
})

// Drain traffic before maintenance
app.OnStop(func() error {
    mgmt.SetReady(false)
    time.Sleep(10 * time.Second) // allow LB to drain existing connections
    return nil
})
```

### Kubernetes Configuration

```yaml
# deployment.yaml
containers:
  - name: my-service
    ports:
      - name: http
        containerPort: 8080
      - name: management
        containerPort: 9090
    livenessProbe:
      httpGet:
        path: /health/live
        port: management
      initialDelaySeconds: 5
      periodSeconds: 10
    readinessProbe:
      httpGet:
        path: /health/ready
        port: management
      initialDelaySeconds: 10
      periodSeconds: 5
      failureThreshold: 3
    startupProbe:
      httpGet:
        path: /health/startup
        port: management
      failureThreshold: 30
      periodSeconds: 10
```

```yaml
# servicemonitor.yaml (Prometheus Operator)
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
spec:
  endpoints:
    - port: management
      path: /metrics
      interval: 15s
```

---

## ConfigWatcher

Monitors config files and triggers a reload callback when changes are detected.
Uses fsnotify (event-based) with automatic polling fallback.

### Basic Usage

```go
import "github.com/madcok-co/unicorn/contrib/sidecar/configwatcher"

watcher := configwatcher.New(&configwatcher.Config{
    Paths: []string{"/etc/myapp/config.yaml"},
    OnReload: func(path string, content []byte) error {
        return cfg.Reload(content)
    },
})

app.AddSidecar(watcher)
```

### Multiple Files

```go
watcher := configwatcher.New(&configwatcher.Config{
    Paths: []string{
        "/etc/myapp/config.yaml",
        "/etc/myapp/feature-flags.json",
    },
    OnReload: func(path string, content []byte) error {
        switch filepath.Ext(path) {
        case ".yaml":
            return appConfig.ReloadYAML(content)
        case ".json":
            return featureFlags.ReloadJSON(content)
        }
        return nil
    },
    Debounce:     500 * time.Millisecond, // wait for burst writes to settle
    PollInterval: 60 * time.Second,       // fallback polling interval
})
```

### Integration with Viper

```go
import (
    "github.com/spf13/viper"
    "github.com/madcok-co/unicorn/contrib/sidecar/configwatcher"
)

v := viper.New()
v.SetConfigFile("/etc/myapp/config.yaml")
v.ReadInConfig()

watcher := configwatcher.New(&configwatcher.Config{
    Paths: []string{"/etc/myapp/config.yaml"},
    OnReload: func(path string, content []byte) error {
        return v.ReadInConfig()
    },
    ErrHandler: func(path string, err error) {
        log.Printf("config reload failed %s: %v", path, err)
    },
})
```

### Kubernetes ConfigMap

When a ConfigMap is mounted as a volume, Kubernetes performs an atomic file
swap. The watcher handles this via parent directory watching:

```yaml
# deployment.yaml
volumes:
  - name: config
    configMap:
      name: myapp-config
volumeMounts:
  - name: config
    mountPath: /etc/myapp
    readOnly: true
```

```go
watcher := configwatcher.New(&configwatcher.Config{
    Paths:    []string{"/etc/myapp/config.yaml"},
    OnReload: func(path string, content []byte) error { ... },
})
```

The watcher automatically detects Kubernetes' rename-on-write pattern.

---

## ServiceRegistrar

Automatically registers and deregisters the service with Consul when the
application starts and stops. No external Consul SDK required.

### Basic Usage

```go
import "github.com/madcok-co/unicorn/contrib/sidecar/discovery"

registrar := discovery.NewConsul(
    &discovery.ServiceDefinition{
        Name:    "payment-service",
        Address: "10.0.1.5",
        Port:    8080,
    },
    nil, // use defaults: Consul at 127.0.0.1:8500
)

app.AddSidecar(registrar)
```

### Full Configuration

```go
registrar := discovery.NewConsul(
    &discovery.ServiceDefinition{
        ID:      "payment-service-10.0.1.5-8080", // unique per instance
        Name:    "payment-service",
        Address: "10.0.1.5",
        Port:    8080,
        Tags:    []string{"v2", "production", "us-east-1"},
        Meta: map[string]string{
            "version":    "2.4.1",
            "team":       "payments",
            "deployment": "blue",
        },
        HealthCheckURL:      "http://10.0.1.5:9090/health/live", // use management port
        HealthCheckInterval: 10 * time.Second,
        HealthCheckTimeout:  5 * time.Second,
        DeregisterAfter:     time.Minute,
    },
    &discovery.Config{
        ConsulAddr:        "http://consul.internal:8500",
        Token:             os.Getenv("CONSUL_TOKEN"),
        HeartbeatInterval: 5 * time.Second,
    },
)
```

### Pairing with ManagementServer

ServiceRegistrar and ManagementServer work best together — point Consul's
health check at the management server's liveness endpoint:

```go
mgmt := management.New(&management.Config{Port: 9090})

registrar := discovery.NewConsul(
    &discovery.ServiceDefinition{
        Name:           "my-service",
        Address:        podIP,
        Port:           8080,
        HealthCheckURL: fmt.Sprintf("http://%s:9090/health/live", podIP),
    },
    nil,
)

app.AddSidecar(mgmt).AddSidecar(registrar)
```

---

## SecretRotator

Periodically fetches secrets from an external source and fires a rotation
callback when a value changes — enabling zero-downtime credential rotation.

### Basic Usage

```go
import "github.com/madcok-co/unicorn/contrib/sidecar/secretrotator"

rotator := secretrotator.New().
    Watch(&secretrotator.WatchEntry{
        Name:     "db-password",
        Interval: 5 * time.Minute,
        Fetch:    secretrotator.FetchFromVault(&secretrotator.VaultConfig{
            SecretPath: "secret/data/myapp/database",
            Token:      os.Getenv("VAULT_TOKEN"),
            Field:      "password",
        }),
        OnRotate: func(ctx context.Context, name, oldVal, newVal string) error {
            return db.UpdatePassword(ctx, newVal)
        },
    })

app.AddSidecar(rotator)
```

### Built-in Fetchers

**Vault KV v2 (HashiCorp)**

```go
secretrotator.FetchFromVault(&secretrotator.VaultConfig{
    Addr:       "https://vault.internal:8200",
    Token:      os.Getenv("VAULT_TOKEN"), // or use Kubernetes service account auth
    SecretPath: "secret/data/myapp/db",
    Field:      "password",
})
```

**Kubernetes Mounted Secret**

```go
// K8s mounts secrets as files under /var/run/secrets/
secretrotator.FetchFromFile("/var/run/secrets/db-password")
```

```yaml
# deployment.yaml
volumes:
  - name: db-secret
    secret:
      secretName: myapp-db-credentials
volumeMounts:
  - name: db-secret
    mountPath: /var/run/secrets
    readOnly: true
```

**Environment Variable**

```go
secretrotator.FetchFromEnv("DB_PASSWORD")
```

**Custom Fetcher**

```go
secretrotator.FetchFunc(func(ctx context.Context) (string, error) {
    resp, err := awsSecretsManager.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
        SecretId: aws.String("myapp/db/password"),
    })
    if err != nil {
        return "", err
    }
    return aws.ToString(resp.SecretString), nil
})
```

### Multiple Secrets

```go
rotator := secretrotator.New().
    Watch(&secretrotator.WatchEntry{
        Name:     "db-password",
        Interval: 5 * time.Minute,
        Fetch:    secretrotator.FetchFromVault(&secretrotator.VaultConfig{
            SecretPath: "secret/data/myapp/db",
            Token:      vaultToken,
        }),
        OnRotate: func(ctx context.Context, name, _, newVal string) error {
            return reconnectDatabase(ctx, newVal)
        },
        ForceOnStart: true, // apply the secret immediately on startup
    }).
    Watch(&secretrotator.WatchEntry{
        Name:     "jwt-signing-key",
        Interval: 24 * time.Hour,
        Fetch:    secretrotator.FetchFromFile("/var/run/secrets/jwt-key"),
        OnRotate: func(ctx context.Context, name, _, newVal string) error {
            return jwtValidator.UpdateKey([]byte(newVal))
        },
    }).
    Watch(&secretrotator.WatchEntry{
        Name:     "api-key",
        Interval: time.Hour,
        Fetch:    secretrotator.FetchFromEnv("THIRD_PARTY_API_KEY"),
        OnRotate: func(ctx context.Context, name, _, newVal string) error {
            thirdPartyClient.SetAPIKey(newVal)
            return nil
        },
    })
```

### Reading Current Values

```go
rotator := secretrotator.New().Watch(...)
app.AddSidecar(rotator)

// In handler — read the currently active secret value
func MyHandler(ctx *context.Context) error {
    key, ok := rotator.CurrentValue("api-key")
    if !ok {
        return errors.New("api-key not yet loaded")
    }
    return callExternalAPI(key)
}
```

---

## Custom Sidecar

Implement `contracts.Sidecar` to build your own:

```go
import (
    "context"
    "github.com/madcok-co/unicorn/core/pkg/contracts"
)

type CacheWarmer struct {
    db    contracts.Database
    cache contracts.Cache
}

func (c *CacheWarmer) Name() string { return "cache-warmer" }

func (c *CacheWarmer) Start(ctx context.Context) error {
    // Initial warm-up
    if err := c.warm(ctx); err != nil {
        return err
    }

    // Periodic refresh every 10 minutes
    ticker := time.NewTicker(10 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return nil
        case <-ticker.C:
            _ = c.warm(ctx) // non-fatal
        }
    }
}

func (c *CacheWarmer) Stop(_ context.Context) error { return nil }

func (c *CacheWarmer) warm(ctx context.Context) error {
    var products []Product
    if err := c.db.FindAll(ctx, &products, "active = true"); err != nil {
        return err
    }
    for _, p := range products {
        c.cache.Set(ctx, "product:"+p.ID, p, time.Hour)
    }
    return nil
}
```

```go
app.AddSidecar(&CacheWarmer{db: app.Adapters().DB, cache: app.Adapters().Cache})
```

---

## Production Setup: Full Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
    "github.com/madcok-co/unicorn/core/pkg/app"
    "github.com/madcok-co/unicorn/core/pkg/context"
    "github.com/madcok-co/unicorn/contrib/sidecar/configwatcher"
    "github.com/madcok-co/unicorn/contrib/sidecar/discovery"
    "github.com/madcok-co/unicorn/contrib/sidecar/management"
    "github.com/madcok-co/unicorn/contrib/sidecar/secretrotator"
)

func main() {
    podIP := os.Getenv("POD_IP")

    // Management server: health probes + metrics + pprof
    mgmt := management.New(&management.Config{
        Port:          9090,
        EnablePprof:   true,
        EnableMetrics: true,
    })
    mgmt.AddChecker("database", dbHealthChecker)
    mgmt.AddChecker("cache", cacheHealthChecker)

    // Config hot-reload from K8s ConfigMap
    cfgWatcher := configwatcher.New(&configwatcher.Config{
        Paths:    []string{"/etc/myapp/config.yaml"},
        OnReload: reloadAppConfig,
    })

    // Consul service registration
    registrar := discovery.NewConsul(
        &discovery.ServiceDefinition{
            Name:           "payment-service",
            Address:        podIP,
            Port:           8080,
            Tags:           []string{"v2", os.Getenv("DEPLOY_ENV")},
            HealthCheckURL: fmt.Sprintf("http://%s:9090/health/live", podIP),
        },
        &discovery.Config{
            ConsulAddr: os.Getenv("CONSUL_ADDR"),
            Token:      os.Getenv("CONSUL_TOKEN"),
        },
    )

    // Secret rotation from Vault
    rotator := secretrotator.New().
        Watch(&secretrotator.WatchEntry{
            Name:     "db-password",
            Interval: 5 * time.Minute,
            Fetch: secretrotator.FetchFromVault(&secretrotator.VaultConfig{
                Addr:       os.Getenv("VAULT_ADDR"),
                Token:      os.Getenv("VAULT_TOKEN"),
                SecretPath: "secret/data/payment-service/db",
                Field:      "password",
            }),
            OnRotate: func(ctx context.Context, _, _, newVal string) error {
                return reconnectDB(ctx, newVal)
            },
            ForceOnStart: true,
        })

    application := app.New(&app.Config{
        Name:       "payment-service",
        Version:    "2.4.1",
        EnableHTTP: true,
        HTTP:       &httpAdapter.Config{Host: "0.0.0.0", Port: 8080},
    }).
        AddSidecar(mgmt).
        AddSidecar(cfgWatcher).
        AddSidecar(registrar).
        AddSidecar(rotator)

    application.RegisterHandler(ProcessPayment).
        HTTP("POST", "/payments").
        Done()

    application.OnStart(func() error {
        mgmt.SetStartupComplete()
        return nil
    })

    if err := application.Start(); err != nil {
        os.Exit(1)
    }
}

func ProcessPayment(ctx *context.Context, req PaymentRequest) (*PaymentResponse, error) {
    // Business logic only
    return &PaymentResponse{ID: "pay_123", Status: "processed"}, nil
}
```

---

## Best Practices

### Port Allocation

Use a consistent convention across all services:

| Port | Purpose |
|------|---------|
| `8080` | Main application (HTTP) |
| `9090` | Management (health + metrics + pprof) |
| `50051` | gRPC |

### Health Check Design

```go
// DO: check the dependency, not just the adapter
mgmt.AddChecker("database", func(ctx context.Context) management.HealthResult {
    if err := db.PingContext(ctx); err != nil {
        return management.HealthResult{Status: management.StatusDown, Message: err.Error()}
    }
    return management.HealthResult{Status: management.StatusUp}
})

// DON'T: return up without actually checking
mgmt.AddChecker("database", func(ctx context.Context) management.HealthResult {
    return management.HealthResult{Status: management.StatusUp} // always lies
})
```

### Rotation Callbacks

```go
// DO: make rotation callbacks idempotent
OnRotate: func(ctx context.Context, name, old, new string) error {
    if old == new {
        return nil // no-op, safe to call multiple times
    }
    return reconnectDB(ctx, new)
},

// DO: handle reconnection errors gracefully — old connection may still work
OnRotate: func(ctx context.Context, name, old, new string) error {
    if err := reconnectDB(ctx, new); err != nil {
        log.Errorf("reconnect failed, keeping old connection: %v", err)
        return err // rotator logs but keeps retrying on next interval
    }
    return nil
},
```

### Service Mesh Integration

When running behind Istio or Linkerd, the sidecar proxy handles mTLS and
traffic management. Configure the management server to bypass the mesh:

```yaml
# Annotate the management port to exclude from Istio proxy
annotations:
  traffic.sidecar.istio.io/excludeInboundPorts: "9090"
```

This ensures health checks from Kubernetes reach the management server
directly without going through the mesh proxy.

---

## Deploying on AWS (without Kubernetes)

All sidecars work on AWS EC2, ECS (Fargate and EC2 launch type), and Lambda
with no changes to the core API. The only differences are:

| Concern | Kubernetes | AWS (no K8s) |
|---------|-----------|--------------|
| Health probes | `livenessProbe` / `readinessProbe` YAML | ALB target group health check |
| Service discovery | Consul / CoreDNS | AWS Cloud Map |
| Secrets | Vault / K8s Secrets | AWS Secrets Manager |
| Config | ConfigMap volume mount | Parameter Store / AppConfig / EFS |
| Metrics | Prometheus Operator scrape | CloudWatch agent / ADOT collector |

### ManagementServer — ALB Health Check

No code changes. Configure the ALB target group to hit the management port:

```
ALB Target Group Health Check:
  Protocol:            HTTP
  Path:                /health/live
  Port:                9090 (override)
  Healthy threshold:   2
  Unhealthy threshold: 3
  Timeout:             5s
  Interval:            30s
  Success codes:       200
```

For ECS task definition, expose the management port alongside the app port:

```json
{
  "portMappings": [
    { "containerPort": 8080, "hostPort": 8080, "protocol": "tcp" },
    { "containerPort": 9090, "hostPort": 9090, "protocol": "tcp" }
  ]
}
```

The `/metrics` endpoint works as-is with the **CloudWatch agent** or
**AWS Distro for OpenTelemetry (ADOT)** configured to scrape Prometheus:

```yaml
# cloudwatch-agent config (prometheus scrape)
prometheus:
  prometheus_config_path: /etc/prometheusconfig.yaml
# prometheusconfig.yaml
scrape_configs:
  - job_name: my-service
    static_configs:
      - targets: ["localhost:9090"]
```

### SecretRotator — AWS Secrets Manager

The `AWSSecretGetter` functional interface lets you inject the AWS SDK client
without the framework importing any AWS packages:

```go
import (
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
    "github.com/madcok-co/unicorn/contrib/sidecar/secretrotator"
)

// Load AWS config — picks up IAM role automatically on ECS/EC2
awsCfg, _ := config.LoadDefaultConfig(context.Background())
client := secretsmanager.NewFromConfig(awsCfg)

// Wrap the SDK call into AWSSecretGetter
getter := secretrotator.AWSSecretGetter(func(ctx context.Context, secretID string) (string, error) {
    out, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
        SecretId: aws.String(secretID),
    })
    if err != nil {
        return "", err
    }
    if out.SecretString != nil {
        return *out.SecretString, nil
    }
    return string(out.SecretBinary), nil
})
```

**Plain string secret:**

```go
rotator := secretrotator.New().
    Watch(&secretrotator.WatchEntry{
        Name:     "api-key",
        Interval: time.Hour,
        Fetch:    secretrotator.FetchFromAWSSecretsManager("prod/myapp/api-key", getter),
        OnRotate: func(ctx context.Context, _, _, newVal string) error {
            thirdPartyClient.SetAPIKey(newVal)
            return nil
        },
    })
```

**JSON secret (RDS credentials, the most common pattern):**

AWS Secrets Manager auto-rotates RDS passwords as JSON:

```json
{
  "username": "admin",
  "password": "new-rotated-password",
  "engine":   "mysql",
  "host":     "mydb.cluster-xxxx.us-east-1.rds.amazonaws.com",
  "port":     "3306",
  "dbname":   "myapp"
}
```

```go
rotator := secretrotator.New().
    Watch(&secretrotator.WatchEntry{
        Name:     "rds-password",
        Interval: 5 * time.Minute,
        Fetch:    secretrotator.FetchFromAWSSecretsManagerJSON(
            "prod/myapp/rds",  // secret name
            "password",        // field to extract
            getter,
        ),
        OnRotate: func(ctx context.Context, _, _, newVal string) error {
            return reconnectRDS(ctx, newVal)
        },
        ForceOnStart: true,
    })
```

**IAM permissions required:**

```json
{
  "Effect": "Allow",
  "Action": [
    "secretsmanager:GetSecretValue",
    "secretsmanager:DescribeSecret"
  ],
  "Resource": "arn:aws:secretsmanager:us-east-1:123456789:secret:prod/myapp/*"
}
```

Attach this policy to the **ECS task role** or **EC2 instance profile** —
no hardcoded credentials needed.

### ServiceRegistrar — AWS Cloud Map

AWS Cloud Map is the native service discovery for ECS/EC2. Use it instead
of (or alongside) Consul:

```go
import (
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/servicediscovery"
    sdtypes "github.com/aws/aws-sdk-go-v2/service/servicediscovery/types"
    "github.com/madcok-co/unicorn/contrib/sidecar/discovery"
)

awsCfg, _ := config.LoadDefaultConfig(context.Background())
sdClient := servicediscovery.NewFromConfig(awsCfg)

registrar := discovery.NewAWSCloudMap(
    &discovery.AWSCloudMapInstance{
        NamespaceID: "ns-xxxxxxxxxxxx",
        ServiceID:   "srv-xxxxxxxxxxxx",
        InstanceID:  os.Getenv("ECS_CONTAINER_METADATA_URI"), // unique per task
        Attributes: map[string]string{
            "AWS_INSTANCE_IPV4": os.Getenv("POD_IP"),
            "AWS_INSTANCE_PORT": "8080",
        },
        HeartbeatInterval: 20 * time.Second,
    },
    // register
    func(ctx context.Context, inst *discovery.AWSCloudMapInstance) error {
        _, err := sdClient.RegisterInstance(ctx, &servicediscovery.RegisterInstanceInput{
            ServiceId:  aws.String(inst.ServiceID),
            InstanceId: aws.String(inst.InstanceID),
            Attributes: inst.Attributes,
        })
        return err
    },
    // deregister
    func(ctx context.Context, serviceID, instanceID string) error {
        _, err := sdClient.DeregisterInstance(ctx, &servicediscovery.DeregisterInstanceInput{
            ServiceId:  aws.String(serviceID),
            InstanceId: aws.String(instanceID),
        })
        return err
    },
    // health update (custom health check)
    func(ctx context.Context, serviceID, instanceID string) error {
        _, err := sdClient.UpdateInstanceCustomHealthStatus(ctx,
            &servicediscovery.UpdateInstanceCustomHealthStatusInput{
                ServiceId:  aws.String(serviceID),
                InstanceId: aws.String(instanceID),
                Status:     sdtypes.CustomHealthStatusHealthy,
            })
        return err
    },
)
```

Pass `nil` as the health function if you rely on Route 53 health checks
configured on the Cloud Map service instead.

### ConfigWatcher — AWS AppConfig / Parameter Store

For config stored in AWS Parameter Store or AppConfig, implement a custom
fetcher using the same `OnReload` callback pattern:

```go
import (
    "github.com/aws/aws-sdk-go-v2/service/ssm"
    "github.com/madcok-co/unicorn/contrib/sidecar/configwatcher"
)

ssmClient := ssm.NewFromConfig(awsCfg)

// Poll Parameter Store every 30s for config changes
watcher := configwatcher.New(&configwatcher.Config{
    Paths:        []string{"/myapp/config"},   // used as identifier in OnReload
    PollInterval: 30 * time.Second,
    OnReload: func(path string, _ []byte) error {
        // Fetch fresh config from Parameter Store
        out, err := ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
            Name:           aws.String("/myapp/config"),
            WithDecryption: aws.Bool(true),
        })
        if err != nil {
            return err
        }
        return cfg.ReloadJSON([]byte(aws.ToString(out.Parameter.Value)))
    },
})
```

For file-based config on ECS with EFS mount or baked-in config files, the
standard `ConfigWatcher` works with no changes.

### Full AWS Production Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    awsconfig "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
    httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
    "github.com/madcok-co/unicorn/core/pkg/app"
    "github.com/madcok-co/unicorn/core/pkg/context"
    "github.com/madcok-co/unicorn/contrib/sidecar/management"
    "github.com/madcok-co/unicorn/contrib/sidecar/secretrotator"
)

func main() {
    // AWS config — automatically uses ECS task role / EC2 instance profile
    awsCfg, err := awsconfig.LoadDefaultConfig(context.Background())
    if err != nil {
        fmt.Fprintf(os.Stderr, "aws config: %v\n", err)
        os.Exit(1)
    }
    smClient := secretsmanager.NewFromConfig(awsCfg)

    // AWSSecretGetter wraps the SDK call
    getter := secretrotator.AWSSecretGetter(func(ctx context.Context, secretID string) (string, error) {
        out, err := smClient.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
            SecretId: aws.String(secretID),
        })
        if err != nil {
            return "", err
        }
        if out.SecretString != nil {
            return *out.SecretString, nil
        }
        return string(out.SecretBinary), nil
    })

    // Management server — ALB health check points to :9090/health/live
    mgmt := management.New(&management.Config{
        Port:          9090,
        EnableMetrics: true,  // scraped by ADOT / CloudWatch agent
        EnablePprof:   false, // disable in production if not needed
    })
    mgmt.AddChecker("database", dbHealthChecker)

    // Secret rotation from AWS Secrets Manager
    rotator := secretrotator.New().
        Watch(&secretrotator.WatchEntry{
            Name:     "rds-password",
            Interval: 5 * time.Minute,
            Fetch: secretrotator.FetchFromAWSSecretsManagerJSON(
                "prod/payment-service/rds",
                "password",
                getter,
            ),
            OnRotate: func(ctx context.Context, _, _, newVal string) error {
                return reconnectRDS(ctx, newVal)
            },
            ForceOnStart: true,
        }).
        Watch(&secretrotator.WatchEntry{
            Name:     "stripe-key",
            Interval: 24 * time.Hour,
            Fetch:    secretrotator.FetchFromAWSSecretsManager("prod/payment-service/stripe", getter),
            OnRotate: func(ctx context.Context, _, _, newVal string) error {
                stripeClient.SetKey(newVal)
                return nil
            },
        })

    application := app.New(&app.Config{
        Name:       "payment-service",
        Version:    "2.4.1",
        EnableHTTP: true,
        HTTP:       &httpAdapter.Config{Host: "0.0.0.0", Port: 8080},
    }).
        AddSidecar(mgmt).
        AddSidecar(rotator)

    application.RegisterHandler(ProcessPayment).
        HTTP("POST", "/payments").
        Done()

    application.OnStart(func() error {
        mgmt.SetStartupComplete()
        return nil
    })

    if err := application.Start(); err != nil {
        os.Exit(1)
    }
}
```

### IAM Permissions Summary

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "SecretsManager",
      "Effect": "Allow",
      "Action": ["secretsmanager:GetSecretValue", "secretsmanager:DescribeSecret"],
      "Resource": "arn:aws:secretsmanager:*:*:secret:prod/myapp/*"
    },
    {
      "Sid": "CloudMap",
      "Effect": "Allow",
      "Action": [
        "servicediscovery:RegisterInstance",
        "servicediscovery:DeregisterInstance",
        "servicediscovery:UpdateInstanceCustomHealthStatus"
      ],
      "Resource": "*"
    }
  ]
}
```
