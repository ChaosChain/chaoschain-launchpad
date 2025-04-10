package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Transaction represents a basic transaction structure
type Transaction struct {
	Type      string  `json:"type" amino:"bytes"` // Transaction type (e.g., "register_validator")
	From      string  `json:"from" amino:"bytes"`
	To        string  `json:"to" amino:"bytes"`
	Amount    float64 `json:"amount" amino:"fixed64"`
	Fee       uint64  `json:"fee" amino:"varint"`
	Content   string  `json:"content" amino:"bytes"`
	Timestamp int64   `json:"timestamp" amino:"varint"`
	Signature string  `json:"signature" amino:"bytes"`
	PublicKey string  `json:"publicKey" amino:"bytes"`
	ChainID   string  `json:"chainID" amino:"bytes"`
	Hash      []byte  `json:"hash" amino:"bytes"` // Transaction hash
	Data      []byte  `json:"data" amino:"bytes"`
}

// GenerateKeyPair creates a new key pair for signing transactions
func GenerateKeyPair() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

// SignTransaction signs a transaction with the given private key
func (tx *Transaction) SignTransaction(privateKey *ecdsa.PrivateKey) error {
	// Create hash of transaction data
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s%s%.8f%d%s%d%s", tx.From, tx.To, tx.Amount, tx.Fee, tx.Content, tx.Timestamp, tx.ChainID)))

	// Sign the hash
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, hash[:])
	if err != nil {
		return err
	}

	// Store signature and public key
	tx.Signature = hex.EncodeToString(append(r.Bytes(), s.Bytes()...))
	tx.PublicKey = hex.EncodeToString(elliptic.MarshalCompressed(privateKey.PublicKey.Curve, privateKey.PublicKey.X, privateKey.PublicKey.Y))

	return nil
}

// VerifyTransaction verifies the transaction signature
func (tx *Transaction) VerifyTransaction(from string) bool {
	// TODO: In the final implementation, we would:
	// 1. Decode the signature and public key
	// 2. Recreate the transaction hash
	// 3. Verify the signature using the public key

	// For now, just verify the sender matches
	return tx.From == from
}

func (tx *Transaction) GetHash() []byte {
	if len(tx.Hash) == 0 {
		// Calculate hash excluding the signature fields
		data := fmt.Sprintf("%s%s%.8f%d%s%d%s",
			tx.From, tx.To, tx.Amount, tx.Fee,
			tx.Content, tx.Timestamp, tx.ChainID)
		hash := sha256.Sum256([]byte(data))
		tx.Hash = hash[:]
	}
	return tx.Hash
}

func (tx *Transaction) Marshal() ([]byte, error) {
	// Properly marshal the transaction to JSON
	jsonBytes, err := json.Marshal(tx)
	if err != nil {
		return nil, err
	}
	return jsonBytes, nil
}
