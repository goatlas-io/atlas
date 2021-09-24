package config

type ControllerConfig struct {
	ADSAddress   string
	ADSPort      int64
	EnvoyAddress string
}

func NewControllerConfig() *ControllerConfig {
	return &ControllerConfig{}
}

type EnvoyADSConfig struct {
	AtlasEnvoyAddress string
}

func NewEnvoyADSConfig() *EnvoyADSConfig {
	return &EnvoyADSConfig{}
}
