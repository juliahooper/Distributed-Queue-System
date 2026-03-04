// Package azure provides adapters that integrate the distributed queue system
// with Azure cloud services: Azure Service Bus for message brokering and
// Azure Blob Storage for persistent message storage.
package azure

import (
	"fmt"
	"os"
)

// Config holds connection details for Azure services.
type Config struct {
	// Service Bus
	ServiceBusConnectionString string
	ServiceBusQueueName        string

	// Blob Storage
	StorageConnectionString string
	StorageContainerName    string
}

// LoadConfigFromEnv reads Azure configuration from environment variables.
// Returns an error if any required variable is missing.
func LoadConfigFromEnv() (*Config, error) {
	cfg := &Config{
		ServiceBusConnectionString: os.Getenv("AZURE_SERVICEBUS_CONNECTION_STRING"),
		ServiceBusQueueName:        os.Getenv("AZURE_SERVICEBUS_QUEUE_NAME"),
		StorageConnectionString:    os.Getenv("AZURE_STORAGE_CONNECTION_STRING"),
		StorageContainerName:       os.Getenv("AZURE_STORAGE_CONTAINER_NAME"),
	}

	if cfg.ServiceBusQueueName == "" {
		cfg.ServiceBusQueueName = "broker-queue" // default
	}
	if cfg.StorageContainerName == "" {
		cfg.StorageContainerName = "broker-logs" // default
	}

	return cfg, nil
}

// Validate checks that the required connection strings are present.
func (c *Config) Validate() error {
	if c.ServiceBusConnectionString == "" {
		return fmt.Errorf("AZURE_SERVICEBUS_CONNECTION_STRING is required")
	}
	if c.StorageConnectionString == "" {
		return fmt.Errorf("AZURE_STORAGE_CONNECTION_STRING is required")
	}
	return nil
}

// HasServiceBus returns true if Service Bus connection string is configured.
func (c *Config) HasServiceBus() bool {
	return c.ServiceBusConnectionString != ""
}

// HasBlobStorage returns true if Blob Storage connection string is configured.
func (c *Config) HasBlobStorage() bool {
	return c.StorageConnectionString != ""
}
