/*
The error package provides practical wrap/unwrap features.
The Wrap() function will capture caller provided and the captured stack.
The stack is captured only on the first call to Wrap().
Subsequent calls to Wrap() will augment the description and key/value
pair context only.  Wrap() may safely be called with `nil`.
The Unwrap() function will provide the original wrapped error.
The New() function will create a new, wrapped error.

Example:

//
// Simple Wrap/Unwrap.
a := errors.New("No route to host").
b := Wrap(a)
c = Unwrap(b)  // a == c

//
// Wrap with context.
url := "http://host/..."
d := e1.Wrap(
    a, "Web request failed."
    "url", url)

d.Error()   // "Web request failed. caused by: 'No route to host'"
d.Context() // []string{"url", "http://host/..."}
*/
package error
