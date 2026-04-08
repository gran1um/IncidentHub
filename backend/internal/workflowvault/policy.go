package workflowvault

import (
	"strings"

	"github.com/google/uuid"
)

const RefPrefix = "vault://"

//nolint:gochecknoglobals // Static policy map used for vault-sensitive field detection.
var sensitiveFieldIDsByNodeType = map[string]map[string]struct{}{
	"s3_object": {
		"accessKeyId":     {},
		"secretAccessKey": {},
		"sessionToken":    {},
	},
	"cassandra_query": {
		"token":    {},
		"username": {},
		"password": {},
		"headers":  {},
	},
	"redis": {
		"url":      {},
		"username": {},
		"password": {},
	},
}

func SensitiveFields(nodeType string) map[string]struct{} {
	normalized := strings.TrimSpace(strings.ToLower(nodeType))
	fields := sensitiveFieldIDsByNodeType[normalized]
	if len(fields) == 0 {
		return nil
	}
	copyFields := make(map[string]struct{}, len(fields))
	for fieldID := range fields {
		copyFields[fieldID] = struct{}{}
	}
	return copyFields
}

func IsSensitiveField(nodeType string, fieldID string) bool {
	fields := sensitiveFieldIDsByNodeType[strings.TrimSpace(strings.ToLower(nodeType))]
	_, ok := fields[strings.TrimSpace(fieldID)]
	return ok
}

func IsRef(raw string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(raw)), RefPrefix)
}

func ParseRef(raw string) (uuid.UUID, bool) {
	trimmed := strings.TrimSpace(raw)
	if !IsRef(trimmed) {
		return uuid.Nil, false
	}
	secretID, err := uuid.Parse(strings.TrimSpace(trimmed[len(RefPrefix):]))
	if err != nil {
		return uuid.Nil, false
	}
	return secretID, true
}
