// Package strongerrors defines a set of error interfaces that packages should use for communicating classes of errors.
// Errors that cross the package boundary should implement one (and only one) of these interfaces.
//
// Packages should not reference these interfaces directly, only implement them.
// To check if a particular error implements one of these interfaces, there are helper
// functions provided (e.g. `Is<SomeError>`) which can be used rather than asserting the interfaces directly.
// If you must assert on these interfaces, be sure to check the causal chain (`err.Cause()`).
//
// A set of helper functions are provided to take any error and turn it into a specific error class.
// This frees you from defining the same error classes all over your code. However, you can still
// implement the error classes ony our own if you desire.
package strongerrors
