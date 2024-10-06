package config

type AmpqConfig struct {
	Address string
}

func NewAmpqConfig() *AmpqConfig {
	return &AmpqConfig{
		Address: env("AMQP_ADDRESS", "amqp://guest:guest@0.0.0.0:5672"),
	}
}
