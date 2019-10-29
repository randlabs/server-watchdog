# Rundown-Protection

Golang implementation of a rundown protection for accessing a shared object.

Some times we need to access a shared resource than can be created and deleted by a routine we don't control. Or the opposite, we know when we create and delete the resource but we don't control when it is accessed.

Unlike `sync.WaitGroup`, this library does not panic if you try to access an unavailable resource. Just check the return `Acquire` method to ensure if you can safely use the resource.

## Usage

```go
import rp "github.com/randlabs/rundown-protection"
```

```go
//create a new rundown protection object.
r := rp.Create()

if r.Acquire() {
    //access granted
    go func() {
        //use the shared resource
        r.Release()
    }()
}

//wait until all references to the shared resource are released
r.Wait()

//trying to acquire the shared resource will fail
if r.Acquire() {
    //this code is never executed
}
```


Run demo: `go run ./_examples/demo.go`

## License

[Apache](/LICENSE)
