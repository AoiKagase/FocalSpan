package config

type Config struct {
	Issuer       string
	ClockSkewSec int
}

func Default() Config {
	return Config{Issuer: "authsample", ClockSkewSec: 30}
}
