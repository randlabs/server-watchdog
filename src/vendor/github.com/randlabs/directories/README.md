# Directories

Helper cross-platform Golang library to retrieve commonly used directories.

The library tries to get directory names using platform specific API instead of relying, for e.g., in environment variables that might not be present.

## Usage

```go
import "github.com/randlabs/directories"
```

```go
dir, err := GetHomeDirectory()
if err == nil {
	fmt.Printf("Home directory: %v\n", dir)
} 
```

## License

[Apache](/LICENSE)
