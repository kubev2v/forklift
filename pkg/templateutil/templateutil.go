package templateutil

import (
	"bytes"
	"text/template"

	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
)

// AddStringFuncs adds safe string manipulation functions to the provided FuncMap
func AddStringFuncs(funcMap template.FuncMap) template.FuncMap {
	if funcMap == nil {
		funcMap = make(template.FuncMap)
	}

	// Add string functions
	funcMap["lower"] = ToLower
	funcMap["upper"] = ToUpper
	funcMap["contains"] = Contains
	funcMap["replace"] = Replace
	funcMap["trim"] = Trim
	funcMap["trimAll"] = TrimAll
	funcMap["trimSuffix"] = TrimSuffix
	funcMap["trimPrefix"] = TrimPrefix
	funcMap["title"] = Title
	funcMap["untitle"] = ToLower
	funcMap["repeat"] = Repeat
	funcMap["substr"] = Substr
	funcMap["nospace"] = Nospace
	funcMap["trunc"] = Trunc
	funcMap["initials"] = Initials
	funcMap["hasPrefix"] = HasPrefix
	funcMap["hasSuffix"] = HasSuffix

	return funcMap
}

// AddMathFuncs adds safe math functions to the provided FuncMap
func AddMathFuncs(funcMap template.FuncMap) template.FuncMap {
	if funcMap == nil {
		funcMap = make(template.FuncMap)
	}

	// Add math functions
	funcMap["add"] = Add
	funcMap["add1"] = Add1
	funcMap["sub"] = Sub
	funcMap["div"] = Div
	funcMap["mod"] = Mod
	funcMap["mul"] = Mul
	funcMap["max"] = Max
	funcMap["min"] = Min
	funcMap["floor"] = Floor
	funcMap["ceil"] = Ceil
	funcMap["round"] = Round

	return funcMap
}

// AddTemplateFuncs adds all template functions to the provided FuncMap
func AddTemplateFuncs(funcMap template.FuncMap) template.FuncMap {
	funcMap = AddStringFuncs(funcMap)
	funcMap = AddMathFuncs(funcMap)
	return funcMap
}

// ExecuteTemplate parses and executes a Go template with the provided data
// Returns the rendered result as a string or an error
func ExecuteTemplate(templateText string, templateData interface{}) (string, error) {
	var buf bytes.Buffer

	// Create a new template
	tmpl := template.New("template")

	// Add all template functions to the template before parsing
	tmpl = tmpl.Funcs(AddTemplateFuncs(nil))

	// Parse template syntax
	var err error
	tmpl, err = tmpl.Parse(templateText)
	if err != nil {
		return "", liberr.Wrap(err, "Invalid template syntax")
	}

	// Execute template
	err = tmpl.Execute(&buf, templateData)
	if err != nil {
		return "", liberr.Wrap(err, "Template execution failed")
	}

	return buf.String(), nil
}
