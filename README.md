<br/>
<p align="center">
  <h3 align="center">Filen SDK Go</h3>

  <p align="center">
    SDK to interact with Filen for Go.
    <br/>
    <br/>
  </p>
</p>

![Contributors](https://img.shields.io/github/contributors/FilenCloudDienste/filen-sdk-go?color=dark-green) ![Forks](https://img.shields.io/github/forks/FilenCloudDienste/filen-sdk-go?style=social) ![Stargazers](https://img.shields.io/github/stars/FilenCloudDienste/filen-sdk-go?style=social) ![Issues](https://img.shields.io/github/issues/FilenCloudDienste/filen-sdk-go) ![License](https://img.shields.io/github/license/FilenCloudDienste/filen-sdk-go)

> **Note:** This SDK is incomplete and primarily intended for our fork of rclone.

### Installation

```sh
go get github.com/FilenCloudDienste/filen-sdk-go
```

### Usage

1. Initialize the SDK

```go
import (
    "context"
    "github.com/FilenCloudDienste/filen-sdk-go/filen"
)

// Create a new client with email and password
client, err := filen.New(context.Background(), "your@email.com", "your-password", "your-2fa-code") // 2FA code is "XXXXXX" if not enabled
if err != nil {
    panic(err)
}

// Or with API key
client, err := filen.NewWithAPIKey(context.Background(), "your@email.com", "your-password", "your-api-key")
if err != nil {
    panic(err)
}
```

2. Interact with the cloud

```go
ctx := context.Background()

// Create a directory
dir, err := client.FindDirectoryOrCreate(ctx, "Documents/Projects")
if err != nil {
    panic(err)
}

// List directory contents
files, directories, err := client.ReadDirectory(ctx, dir)
if err != nil {
    panic(err)
}

// Upload a file
import (
    "os"
    "github.com/FilenCloudDienste/filen-sdk-go/filen/types"
)

// Open the local file
localFile, err := os.Open("/path/to/local/file.txt")
if err != nil {
    panic(err)
}
defer localFile.Close()

// Create metadata for the file
incompleteFile, err := types.NewIncompleteFileFromOSFile(client.AuthVersion, localFile, dir)
if err != nil {
    panic(err)
}

// Upload the file
uploadedFile, err := client.UploadFromReader(ctx, incompleteFile, localFile)
if err != nil {
    panic(err)
}

// Download a file
file, err := client.FindFile(ctx, "Documents/Projects/report.pdf")
if err != nil {
    panic(err)
}

err = client.DownloadToPath(ctx, file, "/local/path/to/report.pdf")
if err != nil {
    panic(err)
}

// Stream download a file
reader := client.GetDownloadReader(ctx, file)
defer reader.Close()
// Use reader as a standard io.Reader
```

3. File sharing

```go
// Share with another Filen user
err = client.ShareItemToUser(ctx, file, "recipient@email.com")
if err != nil {
    panic(err)
}

```

## License

Distributed under the AGPL-3.0 License. See [LICENSE](https://github.com/FilenCloudDienste/filen-sdk-go/blob/main/LICENSE.md) for more information.