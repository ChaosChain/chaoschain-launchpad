package da

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Layr-Labs/eigenda/api/clients"
	"github.com/Layr-Labs/eigenda/core/auth"
	"github.com/NethermindEth/chaoschain-launchpad/communication"
	"github.com/nats-io/nats.go"
)

// SetupGlobalDAService initializes the global DataAvailabilityService instance
func SetupGlobalDAService(natsURL string) error {
	globalDAServiceOnce.Do(func() {
		var service *DataAvailabilityService
		service, globalDAServiceErr = NewDataAvailabilityService(natsURL)
		if globalDAServiceErr != nil {
			log.Printf("Failed to initialize global DA service: %v", globalDAServiceErr)
			return
		}

		// Set up subscriptions for data events
		globalDAServiceErr = service.SetupSubscriptions(
			func(dataID string) { log.Printf("Data stored with ID: %s", dataID) },
			func(dataID string) { log.Printf("Data retrieved with ID: %s", dataID) },
		)
		if globalDAServiceErr != nil {
			log.Printf("Failed to set up DA service subscriptions: %v", globalDAServiceErr)
			return
		}

		GlobalDAService = service
		log.Println("Global EigenDA service initialized successfully")

		// Initialize the master index
		if err := InitializeMasterIndex(); err != nil {
			log.Printf("Failed to initialize master index: %v", err)
			globalDAServiceErr = err
			return
		}
		log.Println("Master index initialized successfully")
	})

	return globalDAServiceErr
}

// GetGlobalDAService returns the global DataAvailabilityService instance
func GetGlobalDAService() *DataAvailabilityService {
	if GlobalDAService == nil {
		log.Println("Warning: Global DA service not initialized")
	}
	return GlobalDAService
}

// CloseGlobalDAService closes the global DataAvailabilityService instance
func CloseGlobalDAService() {
	if GlobalDAService != nil {
		GlobalDAService = nil
		log.Println("Global EigenDA service closed")
	}
}

// NewDataAvailabilityService creates a new DA service
func NewDataAvailabilityService(natsURL string) (*DataAvailabilityService, error) {
	messenger, err := communication.NewMessenger(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create messenger: %w", err)
	}

	// Get authentication key from environment
	eigendaAuthKey, ok := os.LookupEnv("EIGENDA_AUTH_PK")
	if !ok {
		return nil, fmt.Errorf("EIGENDA_AUTH_PK environment variable not set")
	}

	// Validate key length and remove optional '0x' prefix
	eigendaAuthKey = strings.TrimSpace(eigendaAuthKey)
	eigendaAuthKey = strings.TrimPrefix(eigendaAuthKey, "0x")
	eigendaAuthKey = strings.ReplaceAll(eigendaAuthKey, ".", "")
	if len(eigendaAuthKey) < 64 {
		eigendaAuthKey = strings.Repeat("0", 64-len(eigendaAuthKey)) + eigendaAuthKey
	} else if len(eigendaAuthKey) > 64 {
		return nil, fmt.Errorf("invalid EIGENDA_AUTH_PK length: got %d, expected 64 hex characters", len(eigendaAuthKey))
	}

	// Validate that the key is a valid hex string
	if _, err := hex.DecodeString(eigendaAuthKey); err != nil {
		return nil, fmt.Errorf("invalid EIGENDA_AUTH_PK: hex decoding failed: %w", err)
	}

	// Set up authentication with private key using decoded bytes
	signer := auth.NewLocalBlobRequestSigner("0x" + eigendaAuthKey)

	// Configuration for the disperser client
	config := &clients.Config{
		Hostname:          EIGENDA_HOST,
		Port:              EIGENDA_PORT,
		Timeout:           EIGENDA_REQUEST_TIMEOUT,
		UseSecureGrpcFlag: true, // should be true for production
	}

	// Create the disperser client
	client, err := clients.NewDisperserClient(config, signer)
	if err != nil {
		return nil, fmt.Errorf("failed to create disperser client: %w", err)
	}

	service := &DataAvailabilityService{
		messenger: messenger,
		client:    client,
	}

	return service, nil
}

// SetupSubscriptions sets up NATS subscriptions for DA events
func (s *DataAvailabilityService) SetupSubscriptions(dataStoredHandler, dataRetrievedHandler func(dataID string)) error {
	// Subscribe to data stored events
	if dataStoredHandler != nil {
		err := s.messenger.SubscribeGlobal(SUBJECT_DATA_STORED, func(msg *nats.Msg) {
			var data map[string]interface{}
			if err := json.Unmarshal(msg.Data, &data); err != nil {
				fmt.Printf("Error parsing data stored event: %v\n", err)
				return
			}

			if dataID, ok := data["dataID"].(string); ok {
				dataStoredHandler(dataID)
			}
		})
		if err != nil {
			return fmt.Errorf("failed to subscribe to data stored events: %w", err)
		}
	}

	// Subscribe to data retrieved events
	if dataRetrievedHandler != nil {
		err := s.messenger.SubscribeGlobal(SUBJECT_DATA_RETRIEVED, func(msg *nats.Msg) {
			var data map[string]interface{}
			if err := json.Unmarshal(msg.Data, &data); err != nil {
				fmt.Printf("Error parsing data retrieved event: %v\n", err)
				return
			}

			if dataID, ok := data["dataID"].(string); ok {
				dataRetrievedHandler(dataID)
			}
		})
		if err != nil {
			return fmt.Errorf("failed to subscribe to data retrieved events: %w", err)
		}
	}

	return nil
}
