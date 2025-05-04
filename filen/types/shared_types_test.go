package types

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFileMetaMarshalling(t *testing.T) {
	fileMetaJson := `
{
	"name": "test.txt",
	"size": 100,
	"mime": "text/plain",
	"key": "1234567890123456789012345678901234567890123456789012345678901234",
	"lastModified": "1234567890",
	"creation": 1234567890,
	"hash": ""
}
`

	var fileMeta FileMetadata
	err := json.Unmarshal([]byte(fileMetaJson), &fileMeta)
	if err != nil {
		t.Fatal(err)
	}

	expected := FileMetadata{
		Name:         "test.txt",
		Size:         100,
		MimeType:     "text/plain",
		Key:          "1234567890123456789012345678901234567890123456789012345678901234",
		LastModified: 1234567890,
		Created:      1234567890,
		Hash:         "",
	}

	if fileMeta != expected {
		t.Fatal("fileMeta and expected are not equal")
	}

	fileMetaJson = `
{
	"name": "test.txt",
	"size": 100,
	"mime": "text/plain",
	"key": "1234567890123456789012345678901234567890123456789012345678901234",
	"lastModified": "1234567890a",
	"creation": 1234567890,
	"hash": ""
}
`

	fileMeta = FileMetadata{}
	err = json.Unmarshal([]byte(fileMetaJson), &fileMeta)
	if err == nil {
		t.Fatal(err)
	}
	if !strings.Contains(err.Error(), "couldn't unmarshal IntFromMaybeString") {
		t.Fatal("expected error to contain 'couldn't unmarshal IntFromMaybeString'")
	}
}
