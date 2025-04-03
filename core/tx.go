package core

import (
	"encoding/json"
	"fmt"
)

// EncodeTx converts a transaction to bytes
func EncodeTx(tx Transaction) ([]byte, error) {
	return json.Marshal(tx)
}

// DecodeTx converts bytes to a transaction
func DecodeTx(data []byte) (Transaction, error) {
	var tx Transaction
	err := json.Unmarshal(data, &tx)
	return tx, err
}

// ValidateTx performs basic validation of a transaction
func ValidateTx(tx Transaction) error {
	if tx.From == "" || tx.To == "" {
		return fmt.Errorf("invalid addresses")
	}
	if tx.Amount < 0 {
		return fmt.Errorf("negative amount")
	}
	if tx.ChainID == "" {
		return fmt.Errorf("missing chain ID")
	}
	return nil
}
