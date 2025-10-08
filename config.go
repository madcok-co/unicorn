// ============================================
// UNICORN Framework - Configuration
// ============================================

package unicorn

// Config represents the complete framework configuration.
type Config struct {
	App       AppConfig               `yaml:"app"`
	Server    ServerConfig            `yaml:"server"`
	Databases []DatabaseConfig        `yaml:"database"`
	Redis     []RedisConfig           `yaml:"redis"`
	Kafka     KafkaConfig             `yaml:"kafka"`
	Plugins   map[string]PluginConfig `yaml:"plugins"`
	Cron      CronConfig              `yaml:"cron"`
}

// AppConfig contains application metadata.
type AppConfig struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Environment string `yaml:"environment"`
}

// ServerConfig contains server configurations.
type ServerConfig struct {
	HTTP      HTTPConfig      `yaml:"http"`
	GRPC      GRPCConfig      `yaml:"grpc"`
	WebSocket WebSocketConfig `yaml:"websocket"`
}

// HTTPConfig contains HTTP server configuration.
type HTTPConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Host    string `yaml:"host"`
}

// GRPCConfig contains gRPC server configuration.
type GRPCConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Host    string `yaml:"host"`
}

// WebSocketConfig contains WebSocket server configuration.
type WebSocketConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Host    string `yaml:"host"`
}

// PluginConfig contains plugin configuration.
type PluginConfig struct {
	Enabled bool                   `yaml:"enabled"`
	Config  map[string]interface{} `yaml:"config"`
}

// CronConfig contains cron job configurations.
type CronConfig struct {
	Jobs []CronJobConfig `yaml:"jobs"`
}

// CronJobConfig contains a single cron job configuration.
type CronJobConfig struct {
	Name        string                 `yaml:"name"`
	Schedule    string                 `yaml:"schedule"`
	ServiceName string                 `yaml:"service_name"`
	Enabled     bool                   `yaml:"enabled"`
	Timeout     string                 `yaml:"timeout"`
	Request     map[string]interface{} `yaml:"request"`
}
