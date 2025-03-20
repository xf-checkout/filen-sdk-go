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

func NameSplitter(input string) []string {

	normalized := strings.ToLower(strings.TrimSpace(input))
	if normalized == "" {
		return []string{}
	}
	length := len(normalized)

	result := make(map[string]struct{})
	result[normalized] = struct{}{}

	// Add non-accented version for better search
	normalizedPlain := removeDiacritics(normalized)

	if normalizedPlain != normalized {
		result[normalizedPlain] = struct{}{}
	}

	if length < 3 {
		return mapToSlice(result)
	}

	// Precompute frequently used values
	cleanPrefix := cleanPrefixRegex.ReplaceAllString(normalized, "")
	cleanLen := len(cleanPrefix)

	// Prefix handling
	if cleanLen >= 3 {
		result[cleanPrefix[0:3]] = struct{}{}

		if cleanLen >= 5 {
			result[cleanPrefix[0:5]] = struct{}{}
		}

		if cleanLen >= 7 {
			result[cleanPrefix[0:7]] = struct{}{}
		}

		if cleanLen >= 9 {
			result[cleanPrefix[0:9]] = struct{}{}
		}
	}

	// Number sequence extraction
	numberMatches := numberRegex.FindAllString(cleanPrefix, -1)

	for _, match := range numberMatches {
		result[match] = struct{}{}
	}

	// Sliding window
	var windowSizes []int
	if length > 15 {
		windowSizes = []int{4, 5}
	} else {
		windowSizes = []int{4}
	}

	for _, windowSize := range windowSizes {
		stride := windowSize / 2

		if length >= windowSize {
			limit := length - windowSize

			for i := 0; i <= limit; i += stride {
				result[normalized[i:i+windowSize]] = struct{}{}
			}
		}
	}

	// Word processing
	words := wordSplitterRegex.Split(normalized, -1)
	var importantWords []string

	for _, word := range words {
		if word != "" && len(word) >= 2 {
			importantWords = append(importantWords, word)
			result[word] = struct{}{}

			if len(word) > 8 {
				result[word[0:4]] = struct{}{}
				result[word[0:6]] = struct{}{}
			}
		}
	}

	// Word combinations
	importantCount := len(importantWords)

	if importantCount > 1 && importantCount <= 5 {
		for i := 0; i < importantCount-1; i++ {
			one := importantWords[i]
			two := importantWords[i+1]

			if one != "" && two != "" {
				result[one+two] = struct{}{}
			}
		}
		if importantCount >= 3 {
			one := importantWords[0]
			two := importantWords[importantCount-1]

			if one != "" && two != "" {
				result[one+two] = struct{}{}
			}
		}
	}

	// Suffix handling
	result[normalized[length-3:]] = struct{}{}

	if length >= 5 {
		result[normalized[length-5:]] = struct{}{}
	}

	if length >= 7 {
		result[normalized[length-7:]] = struct{}{}
	}

	// Extension handling
	dotIndex := strings.LastIndex(normalized, ".")

	if dotIndex > 0 && dotIndex < length-1 {
		base := normalized[0:dotIndex]
		ext := normalized[dotIndex+1:]

		result["."+ext] = struct{}{}
		result[ext] = struct{}{}

		if dotIndex < 32 {
			result[base] = struct{}{}
		}
	}

	//
	return processTokens(result)
}

var collator = collate.New(language.English)

func processTokens(result map[string]struct{}) []string {
	// Convert map keys to slice
	tokens := make([]string, 0, len(result))
	for token := range result {
		if len(token) >= 2 {
			tokens = append(tokens, token)
		}
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
	if len(tokens) > 256 {
		tokens = tokens[:256]
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
