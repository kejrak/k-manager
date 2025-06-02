package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Kubernetes KubernetesConfig `yaml:"kubernetes"`
	Monitoring MonitoringConfig `yaml:"monitoring"`
}

type ServerConfig struct {
	Port int        `yaml:"port"`
	Host string     `yaml:"host"`
	CORS CORSConfig `yaml:"cors"`
}

type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
	AllowedMethods []string `yaml:"allowed_methods"`
}

type KubernetesConfig struct {
	UseInCluster    bool   `yaml:"use_in_cluster"`
	KubeconfigPath  string `yaml:"kubeconfig_path"`
	DefaultContext  string `yaml:"default_context"`
	RefreshInterval int    `yaml:"refresh_interval"`
}

type MonitoringConfig struct {
	HighRestartThreshold int          `yaml:"high_restart_threshold"`
	ErrorWeights         ErrorWeights `yaml:"error_weights"`
}

type ErrorWeights struct {
	CrashLoop         float64 `yaml:"crash_loop"`
	ImagePull         float64 `yaml:"image_pull"`
	HighRestarts      float64 `yaml:"high_restarts"`
	OtherErrors       float64 `yaml:"other_errors"`
	RestartMultiplier float64 `yaml:"restart_multiplier"`
}

// LoadConfig loads the configuration from the specified file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	config := &Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	// Set defaults if not specified
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}
	if config.Server.Host == "" {
		config.Server.Host = "0.0.0.0"
	}
	if len(config.Server.CORS.AllowedOrigins) == 0 {
		config.Server.CORS.AllowedOrigins = []string{"http://localhost:3000"}
	}
	if len(config.Server.CORS.AllowedMethods) == 0 {
		config.Server.CORS.AllowedMethods = []string{"GET", "POST", "OPTIONS"}
	}
	if config.Kubernetes.RefreshInterval == 0 {
		config.Kubernetes.RefreshInterval = 5
	}
	if config.Monitoring.HighRestartThreshold == 0 {
		config.Monitoring.HighRestartThreshold = 5
	}
	if config.Monitoring.ErrorWeights == (ErrorWeights{}) {
		config.Monitoring.ErrorWeights = ErrorWeights{
			CrashLoop:         3.0,
			ImagePull:         2.0,
			HighRestarts:      2.0,
			OtherErrors:       1.0,
			RestartMultiplier: 0.1,
		}
	}

	return config, nil
}

// GetConfigPath returns the configuration file path based on environment or default
func GetConfigPath() string {
	if path := os.Getenv("POD_ERROR_MONITOR_CONFIG"); path != "" {
		return path
	}
	return "config.yaml"
}
