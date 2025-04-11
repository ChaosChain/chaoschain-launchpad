package da

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/Layr-Labs/eigenda/encoding/utils/codec"
)

// StoreData stores data in EigenDA and publishes dataID to NATS
func (s *DataAvailabilityService) StoreData(data map[string]interface{}) (string, error) {
	if data == nil {
		return "", fmt.Errorf("data is required")
	}

	// Convert data to JSON bytes
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal data: %w", err)
	}

	// Encode data to be compatible with bn254 field element constraints
	encodedData := codec.ConvertByPaddingEmptyByte(jsonData)

	// Add retry logic for dispersing the blob
	var dataID string
	err = retry(3, 2*time.Second, func() error {
		// Context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), EIGENDA_REQUEST_TIMEOUT)
		defer cancel()

		// Custom quorums (none for now, means we're dispersing to the default quorums)
		quorums := []uint8{}

		// Disperse the blob
		_, requestID, err := s.client.DisperseBlob(ctx, encodedData, quorums)
		if err != nil {
			return fmt.Errorf("error dispersing blob: %w", err)
		}

		// Convert requestID to string for use as dataID
		dataID = string(requestID)
		return nil
	})

	if err != nil {
		return "", err
	}

	// Wait for blob to be confirmed or finalized
	status, err := s.waitForBlobStatus(dataID)
	if err != nil {
		return dataID, fmt.Errorf("blob dispersed but status tracking failed: %w", err)
	}

	// Publish event using the messenger
	message := fmt.Sprintf(`{"dataID":"%s","status":"%s","timestamp":%d}`,
		dataID, status, time.Now().Unix())
	if err := s.messenger.PublishGlobal(SUBJECT_DATA_STORED, message); err != nil {
		return dataID, fmt.Errorf("data stored but failed to publish event: %w", err)
	}

	return dataID, nil
}

// RetrieveData retrieves data from EigenDA using dataID
func (s *DataAvailabilityService) RetrieveData(dataID string) (map[string]interface{}, error) {
	if dataID == "" {
		return nil, fmt.Errorf("dataID is required")
	}

	// Create a context with timeout for retrieval
	ctx, cancel := context.WithTimeout(context.Background(), EIGENDA_REQUEST_TIMEOUT)
	defer cancel()

	// Retrieve the blob from the disperser
	blobData, err := s.retrieveBlobFromDisperser(ctx, dataID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve blob: %w", err)
	}

	// Remove null bytes padding from the data
	decodedData := s.removeNullBytesPadding(blobData)

	// Log the retrieved data for debugging
	log.Printf("Retrieved data (length: %d): %s", len(decodedData), string(decodedData))

	// Check if data is empty
	if len(decodedData) == 0 {
		return nil, fmt.Errorf("retrieved data is empty after removing null bytes")
	}

	// Unmarshal the JSON data
	var result map[string]interface{}
	if err := json.Unmarshal(decodedData, &result); err != nil {
		// Try to decode using codec if standard unmarshal fails
		decodedBytes := codec.RemoveEmptyByteFromPaddedBytes(blobData)
		if len(decodedBytes) > 0 {
			if err := json.Unmarshal(decodedBytes, &result); err != nil {
				return nil, fmt.Errorf("failed to unmarshal retrieved data: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to unmarshal retrieved data: %w", err)
		}
	}

	// Publish event that data was retrieved
	retrieveMsg := fmt.Sprintf(`{"dataID":"%s","timestamp":%d}`, dataID, time.Now().Unix())
	if err := s.messenger.PublishGlobal(SUBJECT_DATA_RETRIEVED, retrieveMsg); err != nil {
		return nil, fmt.Errorf("failed to publish data retrieved event: %w", err)
	}

	return result, nil
}

// GetBlobStatus retrieves the current status of a blob from EigenDA
func (s *DataAvailabilityService) GetBlobStatus(dataID string) (interface{}, error) {
	if dataID == "" {
		return nil, fmt.Errorf("dataID is required")
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), EIGENDA_REQUEST_TIMEOUT)
	defer cancel()

	// Get the blob status using the client
	statusReply, err := s.client.GetBlobStatus(ctx, []byte(dataID))
	if err != nil {
		return nil, fmt.Errorf("error getting blob status: %w", err)
	}

	return statusReply, nil
}

// retrieveBlobFromDisperser retrieves a blob from EigenDA using the disperser client
func (s *DataAvailabilityService) retrieveBlobFromDisperser(ctx context.Context, dataID string) ([]byte, error) {
	// First, get the blob status to get the batch information needed for retrieval
	statusReply, err := s.client.GetBlobStatus(ctx, []byte(dataID))
	if err != nil {
		return nil, fmt.Errorf("failed to get blob status for retrieval: %w", err)
	}

	// Check if we have the necessary information for retrieval
	if statusReply.Info == nil || statusReply.Info.BlobVerificationProof == nil {
		return nil, fmt.Errorf("blob status doesn't contain verification proof needed for retrieval")
	}

	// Extract the required parameters from the status reply
	batchHeaderHash := statusReply.Info.BlobVerificationProof.BatchMetadata.BatchHeaderHash
	blobIndex := statusReply.Info.BlobVerificationProof.BlobIndex

	// Log the retrieval parameters for debugging
	log.Printf("Retrieving blob with batch header hash: %x, blob index: %d",
		batchHeaderHash, blobIndex)

	// Use the client's RetrieveBlob method with the correct parameters
	data, err := s.client.RetrieveBlob(ctx, batchHeaderHash, uint32(blobIndex))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve blob: %w", err)
	}

	return data, nil
}

// waitForBlobStatus polls the blob status until it's finalized or failed
func (s *DataAvailabilityService) waitForBlobStatus(requestID string) (string, error) {
	// Create a context for the overall status checking
	statusOverallCtx, statusOverallCancel := context.WithTimeout(context.Background(), EIGENDA_MAX_WAIT_TIME)
	defer statusOverallCancel()

	ticker := time.NewTicker(EIGENDA_POLL_INTERVAL)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Create a new context for each status request
			statusCtx, statusCancel := context.WithTimeout(statusOverallCtx, EIGENDA_REQUEST_TIMEOUT)

			// Get the blob status
			statusReply, err := s.client.GetBlobStatus(statusCtx, []byte(requestID))
			statusCancel()

			if err != nil {
				return "ERROR", fmt.Errorf("error getting blob status: %w", err)
			}

			// Check if the status is done
			status := statusReply.Status.String()
			if status == "FINALIZED" {
				// fmt.Printf("Blob Status is finalized: %s\n", statusReply)
				return "FINALIZED", nil
			} else if status == "CONFIRMED" {
				// fmt.Printf("Blob Status is confirmed: %s\n", statusReply)
				return "CONFIRMED", nil
			} else if status == "FAILED" {
				fmt.Printf("Blob Status is failed: %s\n", statusReply)
				return "FAILED", fmt.Errorf("blob dispersal failed with status: %v", status)
			}

			// Continue polling for other statuses
			fmt.Printf("Current Blob Status: %s\n", status)

		case <-statusOverallCtx.Done():
			return "TIMEOUT", fmt.Errorf("timed out waiting for blob to finalize")
		}
	}
}
