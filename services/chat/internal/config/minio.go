package config

import "time"

type MinIOConfig struct {
	Endpoint       string
	AccessKey      string
	SecretKey      string
	Bucket         string
	UseTLS         bool
	DownloadURLTTL time.Duration
}
