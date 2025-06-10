# Template Utilities

Math and string template methods

## Usage

```go
import (
    "text/template"
    "github.com/kubev2v/forklift/pkg/templateutil"
)

// Add template functions to a FuncMap
funcMap := template.FuncMap{}
funcMap = templateutil.AddStringFuncs(funcMap)
funcMap = templateutil.AddMathFuncs(funcMap)
```

```go
// Or use all functions at once
funcMap = templateutil.AddTemplateFuncs(funcMap)
```

```go
// Or use the ExecuteTemplate helper
result, err := templateutil.ExecuteTemplate("Hello, {{ upper .Name }}!", data)
```

## Available Template Functions

### String Functions

The following string functions are available for use in your templates:

| Function | Description | Example |
|----------|-------------|---------|
| `lower` | Converts string to lowercase | `{{ lower "TEXT" }}` → `text` |
| `upper` | Converts string to uppercase | `{{ upper "text" }}` → `TEXT` |
| `contains` | Checks if string contains substring | `{{ contains "hello" "lo" }}` → `true` |
| `replace` | Replaces occurrences in a string | `{{"I Am Henry VIII" \| replace " " "-"}}` → `I-Am-Henry-VIII` |
| `trim` | Removes whitespace from both ends | `{{ trim "  text  " }}` → `text` |
| `trimAll` | Removes specified characters from both ends | `{{ trimAll "$" "$5.00$" }}` → `5.00` |
| `trimSuffix` | Removes suffix if present | `{{ trimSuffix ".go" "file.go" }}` → `file` |
| `trimPrefix` | Removes prefix if present | `{{ trimPrefix "go." "go.file" }}` → `file` |
| `title` | Converts to title case | `{{ title "hello world" }}` → `Hello World` |
| `untitle` | Converts to lowercase | `{{ untitle "Hello World" }}` → `hello world` |
| `repeat` | Repeats string n times | `{{ repeat 3 "abc" }}` → `abcabcabc` |
| `substr` | Extracts substring from start to end | `{{ substr 1 4 "abcdef" }}` → `bcd` |
| `nospace` | Removes all whitespace | `{{ nospace "a b  c" }}` → `abc` |
| `trunc` | Truncates string to specified length | `{{ trunc 3 "abcdef" }}` → `abc` |
| `initials` | Extracts first letter of each word | `{{ initials "John Doe" }}` → `JD` |
| `hasPrefix` | Checks if string starts with prefix | `{{ hasPrefix "go" "golang" }}` → `true` |
| `hasSuffix` | Checks if string ends with suffix | `{{ hasSuffix "ing" "coding" }}` → `true` |
| `mustRegexReplaceAll` | Replaces matches using regex with submatch expansion | `{{ mustRegexReplaceAll "a(x*)b" "-ab-axxb-" "${1}W" }}` → `-W-xxW-` |

### Math Functions

The following math functions are available for use in your templates:

| Function | Description | Example |
|----------|-------------|---------|
| `add` | Sum numbers | `{{ add 1 2 3 }}` → `6` |
| `add1` | Increment by 1 | `{{ add1 5 }}` → `6` |
| `sub` | Subtract second number from first | `{{ sub 5 3 }}` → `2` |
| `div` | Integer division | `{{ div 10 3 }}` → `3` |
| `mod` | Modulo operation | `{{ mod 10 3 }}` → `1` |
| `mul` | Multiply numbers | `{{ mul 2 3 4 }}` → `24` |
| `max` | Return largest integer | `{{ max 1 5 3 }}` → `5` |
| `min` | Return smallest integer | `{{ min 1 5 3 }}` → `1` |
| `floor` | Round down to nearest integer | `{{ floor 3.75 }}` → `3.0` |
| `ceil` | Round up to nearest integer | `{{ ceil 3.25 }}` → `4.0` |
| `round` | Round to specified decimal places | `{{ round 3.75159 2 }}` → `3.75` |
