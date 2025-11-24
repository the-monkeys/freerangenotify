package repository

import (
	"encoding/json"
	"fmt"
)

// mapToStruct converts a map[string]interface{} to a struct
func mapToStruct(data map[string]interface{}, target interface{}) error {
	// Convert map to JSON bytes
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal map to JSON: %w", err)
	}

	// Unmarshal JSON bytes to struct
	if err := json.Unmarshal(jsonBytes, target); err != nil {
		return fmt.Errorf("failed to unmarshal JSON to struct: %w", err)
	}

	return nil
}

// structToMap converts a struct to a map[string]interface{}
func structToMap(data interface{}) (map[string]interface{}, error) {
	// Convert struct to JSON bytes
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal struct to JSON: %w", err)
	}

	// Unmarshal JSON bytes to map
	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to map: %w", err)
	}

	return result, nil
}
