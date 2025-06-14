package search

import (
	"github.com/FilenCloudDienste/filen-sdk-go/filen/client"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
	"sort"
	"strings"
)

func nameSplitter(input string, minLength int, maxLength int) []string {
	normalized := strings.ToLower(strings.TrimSpace(input))
	if normalized == "" {
		return []string{}
	}
	runed := []rune(normalized)
	result := make(map[string]struct{})
	result[string(runed)] = struct{}{}
	maxLength = min(maxLength, len(runed))

	for i := 0; i <= len(runed); i++ {
		for j := minLength; j <= maxLength && j+i <= len(runed); j++ {
			result[string(runed[i:i+j])] = struct{}{}
		}
	}
	return processTokens(result)
}

func NameSplitter(input string) []string {
	return nameSplitter(input, 2, 16)
}

func processTokens(result map[string]struct{}) []string {
	collator := collate.New(language.English)
	// Convert map keys to slice
	tokens := make([]string, 0, len(result))
	for token := range result {
		tokens = append(tokens, token)
	}

	// Sort tokens by length, then lexicographically
	sort.SliceStable(tokens, func(i, j int) bool {
		lengthDiff := len(tokens[i]) - len(tokens[j])

		if lengthDiff != 0 {
			return lengthDiff < 0 // For ascending order by length
		}

		return collator.CompareString(tokens[i], tokens[j]) < 0
	})

	// Slice to maximum 256 elements
	if len(tokens) > 4096 {
		tokens = tokens[:4096]
	}

	return tokens
}

func generateSearchIndexHashes(input string, key crypto.HMACKey) []string {
	names := NameSplitter(strings.ToLower(input))
	hashes := make([]string, 0, len(names))

	for _, name := range names {
		hashes = append(hashes, key.Hash([]byte(name)))
	}

	return hashes
}

// GenerateSearchIndexHashes is a helper function to generate search index hashes
// for a given input string
func GenerateSearchIndexHashes(input string, key crypto.HMACKey, uuid string, typ string) []client.V3SearchAddItem {
	hashes := generateSearchIndexHashes(input, key)

	items := make([]client.V3SearchAddItem, 0, len(hashes))
	for _, hash := range hashes {
		items = append(items, client.V3SearchAddItem{
			UUID: uuid,
			Hash: hash,
			Type: typ,
		})
	}
	return items
}
