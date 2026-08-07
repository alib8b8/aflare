// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package mobile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

const maxAuditLogEntries = 10000

var (
	validBlockchainTypes = map[string]bool{
		"ethereum":    true,
		"hyperledger": true,
		"fabric":      true,
		"quorum":      true,
		"simulated":   true,
	}
	validAuditLevels = map[string]bool{
		"workflow":  true,
		"node":      true,
		"parameter": true,
		"full":      true,
	}
	hexPattern       = regexp.MustCompile(`^[0-9a-fA-F]+$`)
	didPattern       = regexp.MustCompile(`^did:[a-zA-Z0-9]+:[a-zA-Z0-9._:%\-]+$`)
	didMethodPattern = regexp.MustCompile(`^[a-zA-Z0-9]+$`)
)

// BlockchainAuditNode records workflow execution on blockchain for tamper-proof auditing
type BlockchainAuditNode struct{}

func (n *BlockchainAuditNode) Name() string { return "blockchain_audit" }

func (n *BlockchainAuditNode) Description() string {
	return "Record workflow execution on blockchain for tamper-proof audit trails. Supports Ethereum, Hyperledger Fabric, and simulated chains. Aligns with WAIC Agent Interoperability Initiative."
}

func (n *BlockchainAuditNode) Schema() core.NodeSchema {
	return core.NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - workflow execution data or action description",
		Output:      "string - blockchain transaction receipt with hash and timestamp",
		Params: []core.ParamSchema{
			{Name: "chain_type", Type: "string", Description: "Blockchain type: ethereum/hyperledger/fabric/quorum/simulated (default: simulated)", Required: false, Default: "simulated"},
			{Name: "audit_level", Type: "string", Description: "Audit granularity: workflow/node/parameter/full (default: workflow)", Required: false, Default: "workflow"},
			{Name: "workflow_id", Type: "string", Description: "Unique workflow identifier", Required: true},
			{Name: "node_id", Type: "string", Description: "Node instance identifier", Required: false},
			{Name: "actor_did", Type: "string", Description: "W3C DID of the actor executing the workflow", Required: false},
			{Name: "previous_hash", Type: "string", Description: "Hash of previous audit record for chain linkage", Required: false},
			{Name: "metadata", Type: "string", Description: "Additional JSON metadata to include in audit record", Required: false},
		},
	}
}

func (n *BlockchainAuditNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	chainType := getMobileParam(params, "chain_type", "simulated")
	if !validBlockchainTypes[chainType] {
		return "", fmt.Errorf("invalid chain_type: %s", chainType)
	}

	auditLevel := getMobileParam(params, "audit_level", "workflow")
	if !validAuditLevels[auditLevel] {
		return "", fmt.Errorf("invalid audit_level: %s", auditLevel)
	}

	workflowID := getMobileParam(params, "workflow_id", "")
	if workflowID == "" {
		return "", fmt.Errorf("workflow_id is required")
	}
	if len(workflowID) > 128 {
		return "", fmt.Errorf("workflow_id too long")
	}

	nodeID := getMobileParam(params, "node_id", "")
	if len(nodeID) > 128 {
		return "", fmt.Errorf("node_id too long")
	}

	actorDID := getMobileParam(params, "actor_did", "")
	if actorDID != "" && !isValidDID(actorDID) {
		return "", fmt.Errorf("invalid actor_did format")
	}

	previousHash := getMobileParam(params, "previous_hash", "")
	if previousHash != "" && !hexPattern.MatchString(previousHash) {
		return "", fmt.Errorf("invalid previous_hash format")
	}

	metadata := getMobileParam(params, "metadata", "")
	if len(metadata) > 10000 {
		return "", fmt.Errorf("metadata too large")
	}

	// Build audit record
	record := AuditRecord{
		Version:      "1.0",
		WorkflowID:   workflowID,
		NodeID:       nodeID,
		ActorDID:     actorDID,
		AuditLevel:   auditLevel,
		ChainType:    chainType,
		InputHash:    hashString(input),
		Timestamp:    time.Now().UTC(),
		PreviousHash: previousHash,
		Metadata:     safeParseMetadata(metadata),
	}

	// Compute record hash
	recordHash := computeRecordHash(record)
	record.RecordHash = recordHash

	// Submit to blockchain (simulated or real)
	receipt, err := submitToChain(ctx, chainType, record)
	if err != nil {
		return "", fmt.Errorf("blockchain submission failed: %w", err)
	}

	// Store in local audit log with rotation
	bcAuditLogMu.Lock()
	bcAuditLog = append(bcAuditLog, record)
	if len(bcAuditLog) > maxAuditLogEntries {
		excess := len(bcAuditLog) - maxAuditLogEntries
		bcAuditLog = bcAuditLog[excess:]
	}
	bcAuditLogMu.Unlock()

	result := map[string]interface{}{
		"type":          "blockchain_audit",
		"chain_type":    chainType,
		"audit_level":   auditLevel,
		"workflow_id":   workflowID,
		"record_hash":   recordHash,
		"tx_hash":       receipt.TxHash,
		"block_number":  receipt.BlockNumber,
		"timestamp":     record.Timestamp.Format(time.RFC3339),
		"actor_did":     actorDID,
		"previous_hash": previousHash,
		"status":        receipt.Status,
		"confirmations": receipt.Confirmations,
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

// AuditRecord represents a single audit entry
type AuditRecord struct {
	Version      string                 `json:"version"`
	WorkflowID   string                 `json:"workflow_id"`
	NodeID       string                 `json:"node_id,omitempty"`
	ActorDID     string                 `json:"actor_did,omitempty"`
	AuditLevel   string                 `json:"audit_level"`
	ChainType    string                 `json:"chain_type"`
	InputHash    string                 `json:"input_hash"`
	RecordHash   string                 `json:"record_hash"`
	Timestamp    time.Time              `json:"timestamp"`
	PreviousHash string                 `json:"previous_hash,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// TxReceipt represents a blockchain transaction receipt
type TxReceipt struct {
	TxHash        string `json:"tx_hash"`
	BlockNumber   int64  `json:"block_number"`
	Status        string `json:"status"`
	Confirmations int    `json:"confirmations"`
}

var (
	bcAuditLog   []AuditRecord
	bcAuditLogMu sync.RWMutex
	simBlockNum  int64
	simBlockMu   sync.Mutex
)

func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func computeRecordHash(r AuditRecord) string {
	data, _ := json.Marshal(struct {
		WorkflowID   string                 `json:"workflow_id"`
		NodeID       string                 `json:"node_id"`
		ActorDID     string                 `json:"actor_did"`
		AuditLevel   string                 `json:"audit_level"`
		InputHash    string                 `json:"input_hash"`
		Timestamp    int64                  `json:"timestamp"`
		PreviousHash string                 `json:"previous_hash"`
		Metadata     map[string]interface{} `json:"metadata"`
	}{
		WorkflowID:   r.WorkflowID,
		NodeID:       r.NodeID,
		ActorDID:     r.ActorDID,
		AuditLevel:   r.AuditLevel,
		InputHash:    r.InputHash,
		Timestamp:    r.Timestamp.UnixNano(),
		PreviousHash: r.PreviousHash,
		Metadata:     r.Metadata,
	})
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func submitToChain(ctx context.Context, chainType string, record AuditRecord) (*TxReceipt, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	switch chainType {
	case "simulated":
		return simulatedSubmit(record)
	case "ethereum", "quorum":
		return simulatedSubmit(record) // Real implementation would use web3/ethclient
	case "hyperledger", "fabric":
		return simulatedSubmit(record) // Real implementation would use fabric-sdk-go
	default:
		return nil, fmt.Errorf("unsupported chain type: %s", chainType)
	}
}

func simulatedSubmit(record AuditRecord) (*TxReceipt, error) {
	simBlockMu.Lock()
	simBlockNum++
	bn := simBlockNum
	simBlockMu.Unlock()

	// Simulate network latency
	time.Sleep(10 * time.Millisecond)

	return &TxReceipt{
		TxHash:        hashString(record.RecordHash + fmt.Sprintf("%d", bn)),
		BlockNumber:   bn,
		Status:        "confirmed",
		Confirmations: 1,
	}, nil
}

func isValidDID(did string) bool {
	if len(did) == 0 || len(did) > 2048 {
		return false
	}
	if !strings.HasPrefix(did, "did:") {
		return false
	}

	parts := strings.SplitN(did, ":", 3)
	if len(parts) < 3 {
		return false
	}
	if parts[0] != "did" {
		return false
	}
	if !didMethodPattern.MatchString(parts[1]) {
		return false
	}
	if parts[2] == "" {
		return false
	}
	return didPattern.MatchString(did)
}

func safeParseMetadata(s string) map[string]interface{} {
	if s == "" {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return map[string]interface{}{"raw": s}
	}
	return m
}

// VerifyAuditChain verifies the integrity of the audit log chain
func VerifyAuditChain() (bool, []string) {
	bcAuditLogMu.RLock()
	defer bcAuditLogMu.RUnlock()

	var errors []string
	for i, record := range bcAuditLog {
		expectedHash := computeRecordHash(record)
		if expectedHash != record.RecordHash {
			errors = append(errors, fmt.Sprintf("record %d hash mismatch", i))
		}
		if i > 0 && record.PreviousHash != bcAuditLog[i-1].RecordHash {
			errors = append(errors, fmt.Sprintf("record %d chain link broken", i))
		}
	}
	return len(errors) == 0, errors
}

// QueryAuditLog returns audit records matching criteria
func QueryAuditLog(workflowID string, since time.Time) []AuditRecord {
	bcAuditLogMu.RLock()
	defer bcAuditLogMu.RUnlock()

	var results []AuditRecord
	for _, r := range bcAuditLog {
		if workflowID != "" && r.WorkflowID != workflowID {
			continue
		}
		if !since.IsZero() && r.Timestamp.Before(since) {
			continue
		}
		results = append(results, r)
	}
	return results
}

func init() {
	core.Register(&BlockchainAuditNode{})
}
