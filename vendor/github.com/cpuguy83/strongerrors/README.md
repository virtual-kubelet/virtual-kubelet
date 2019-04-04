# strongerrors
A simple Go package for defining common error classes.

Commonly in go applications you either end up defining lots of custom, seminal errors or even custom error types and checking directly against those types. A problem with this pattern is it becomes difficult to wrap errors with extra context without masking the underlying error and potentially causing an error check to miss. The [errors package](https://github.com/pkg/errors) helps to solve some of this where you can use the `errors.Wrap` pattern to wrap errors and then `errors.Cause()` to traverse the causal chain. Unfortunately there still exists a problem of requiring deep knowledge and reliance on underlying packages that you may not even control in order to check errors against.

This package defines a common set of error interfaces that can be used in your packages to so that consumers of your package do not need to know about the real error types, or compare even seminal errors. This enables users to worry about the kind of failure rather than its underlying type. These interfaces can be bubbled all the way up the stack to produce proper error codes (or wrapped and converted to another error code) in your RPC of choice (e.g. http or GRPC).

This is mostly taken from github.com/moby/moby/api/errdefs which I helped create to solve some of the above issues in that codebase.

### Example Usage


```go
type errNotFound string

func(e errNotFound) Error() string {
	return "not found: " + e
}

func(errNotFound) NotFound() {}

type Foo struct{}

func Get(id string) (*Foo, error) {
	return nil, errNotFound(id)
}

func(w http.ResponseWriter, req *http.Request) {
	id := getID(req)
	f, err := Get(id)
	if err != nil {
		if IsNotFound(err) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error{(w, err.Error(), http.StatusInternalServerError)}
		return
	}
	// yay there's a foo object
}
```

Alternatively, you can use one of the helper functions instead of defining your own error types:

```go
	func Get(id string) (*Foo, error) {
		return nil, NotFound(errors.New("not found"))
	}
```

There are also helpers for converting errors to status codes (or GRPC status errors), but you can also implement your own mapping.

```go
	if err != nil {
		code, ok := status.HTTPCode(err)
		if !ok {
			// error type didn't match a defined type
		}
		http.Error(w, err.Error(), code)
		return
	}
```

```go
	err := status.FromGRPC(err)
	switch {
	case IsNotFound(err):
		// do stuff
	}
```

```go
	if err != nil {
		return ToGRPC(err)
	}
```

These errors can be used in conjuction with `errors.Wrap` from the erros package. To properly check the error type you should use the `Is<Kind>` helpers which checks the full causal chain.

### Contributions

Contributions are always welcome. Keep in mind that the scope of this package is quite small, and the error classes should be kept to a minimum. The thing to keep in mind when thinking about new error classes is "How will the caller be able to react to this error".
The existing errors *should* cover most error cases.

If you think we're missing a case open an issue so we can discuss it.
