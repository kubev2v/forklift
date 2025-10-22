package templateutil

import (
	"bytes"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// AddStringFuncs adds safe string manipulation functions to the provided FuncMap
func AddStringFuncs(funcMap template.FuncMap) template.FuncMap {
	if funcMap == nil {
		funcMap = make(template.FuncMap)
	}

	sprigFuncs := sprig.FuncMap()

	// Add string functions from sprig
	funcMap["lower"] = sprigFuncs["lower"]
	funcMap["upper"] = sprigFuncs["upper"]
	funcMap["contains"] = sprigFuncs["contains"]
	funcMap["replace"] = sprigFuncs["replace"]
	funcMap["trim"] = sprigFuncs["trim"]
	funcMap["trimAll"] = sprigFuncs["trimAll"]
	funcMap["trimSuffix"] = sprigFuncs["trimSuffix"]
	funcMap["trimPrefix"] = sprigFuncs["trimPrefix"]
	funcMap["title"] = sprigFuncs["title"]
	funcMap["untitle"] = sprigFuncs["untitle"]
	funcMap["repeat"] = sprigFuncs["repeat"]
	funcMap["substr"] = sprigFuncs["substr"]
	funcMap["nospace"] = sprigFuncs["nospace"]
	funcMap["trunc"] = sprigFuncs["trunc"]
	funcMap["initials"] = sprigFuncs["initials"]
	funcMap["hasPrefix"] = sprigFuncs["hasPrefix"]
	funcMap["hasSuffix"] = sprigFuncs["hasSuffix"]
	funcMap["mustRegexReplaceAll"] = sprigFuncs["mustRegexReplaceAll"]

	return funcMap
}

// AddMathFuncs adds safe math functions to the provided FuncMap
func AddMathFuncs(funcMap template.FuncMap) template.FuncMap {
	if funcMap == nil {
		funcMap = make(template.FuncMap)
	}

	sprigFuncs := sprig.FuncMap()

	// Add math functions from sprig
	funcMap["add"] = sprigFuncs["add"]
	funcMap["add1"] = sprigFuncs["add1"]
	funcMap["sub"] = sprigFuncs["sub"]
	funcMap["div"] = sprigFuncs["div"]
	funcMap["mod"] = sprigFuncs["mod"]
	funcMap["mul"] = sprigFuncs["mul"]
	funcMap["max"] = sprigFuncs["max"]
	funcMap["min"] = sprigFuncs["min"]
	funcMap["floor"] = sprigFuncs["floor"]
	funcMap["ceil"] = sprigFuncs["ceil"]
	funcMap["round"] = sprigFuncs["round"]

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
