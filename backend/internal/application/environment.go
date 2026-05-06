package application

// TODO: refactor to come from environment
const version string = "1.0.0"

type Config struct {
	Port    int
	Env     string
	Version string
}

func NewConfig() *Config {
	return &Config{
		Port:    5000,
		Env:     "development",
		Version: version,
	}
}
