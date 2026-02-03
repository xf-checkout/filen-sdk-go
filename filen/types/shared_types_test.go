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
	"lastModified": "8589934592",
	"creation": 8589934592,
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
		LastModified: 8589934592,
		Created:      8589934592,
		Hash:         "",
	}

	if fileMeta != expected {
		t.Fatal("fileMeta and expected are not equal")
	}

	fileMetaJson2 := `
{
	"name": "test.txt",
	"size": 100,
	"mime": "text/plain",
	"key": "1234567890123456789012345678901234567890123456789012345678901234",
	"lastModified": 8589934592,
	"creation": 8589934592,
	"hash": ""
}
`

	err = json.Unmarshal([]byte(fileMetaJson2), &fileMeta)
	if err != nil {
		t.Fatal(err)
	}

	if fileMeta != expected {
		t.Fatal("fileMeta2 and expected are not equal")
	}

	fileMetaJson3 := `
{
	"name": "test.txt",
	"size": 100,
	"mime": "text/plain",
	"key": "1234567890123456789012345678901234567890123456789012345678901234",
	"lastModified": 8589934592.0,
	"creation": 8589934592,
	"hash": ""
}
`

	err = json.Unmarshal([]byte(fileMetaJson3), &fileMeta)
	if err != nil {
		t.Fatal(err)
	}

	if fileMeta != expected {
		t.Fatal("fileMeta3 and expected are not equal")
	}

	fileMetaJson4 := `
{
	"name": "test.txt",
	"size": 100,
	"mime": "text/plain",
	"key": "1234567890123456789012345678901234567890123456789012345678901234",
	"lastModified": "8589934592.0",
	"creation": 8589934592,
	"hash": ""
}
`

	err = json.Unmarshal([]byte(fileMetaJson4), &fileMeta)
	if err != nil {
		t.Fatal(err)
	}

	if fileMeta != expected {
		t.Fatal("fileMeta3 and expected are not equal")
	}

	fileMetaJson = `
{
	"name": "test.txt",
	"size": 100,
	"mime": "text/plain",
	"key": "1234567890123456789012345678901234567890123456789012345678901234",
	"lastModified": "8589934592a",
	"creation": 8589934592,
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
