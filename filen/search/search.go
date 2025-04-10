package search

import (
	"github.com/FilenCloudDienste/filen-sdk-go/filen/client"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var wordSplitterRegex = regexp.MustCompile(`[\s\-_.;:,]+`)
var cleanPrefixRegex = regexp.MustCompile(`[^a-z0-9]`)
var numberRegex = regexp.MustCompile(`\d{3,}`)

func nameSplitter(input string, minLength int, maxLength int) []string {
	normalized := strings.ToLower(strings.TrimSpace(input))
	if normalized == "" {
		return []string{}
	}
	result := make(map[string]struct{})
	result[normalized] = struct{}{}
	maxLength = min(maxLength, len(normalized))

	for i := 0; i <= len(normalized); i++ {
		for j := minLength; j <= maxLength && j+i <= len(normalized); j++ {
			result[normalized[i:i+j]] = struct{}{}
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

// Helper function to convert map keys to slice
func mapToSlice(m map[string]struct{}) []string {
	result := make([]string, 0, len(m))
	for key := range m {
		result = append(result, key)
	}
	return result
}

// Helper function to remove diacritics (equivalent to normalize("NFD") and removing combining marks)
func removeDiacritics(s string) string {
	result := make([]rune, 0, len(s))
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) {
			// Skip combining marks
			continue
		}
		result = append(result, r)
	}
	return string(result)
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
