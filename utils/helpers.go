package utils

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const recursionMaxNums = 1000

// https://github.com/technosophos/k8s-helm/commit/431cc46cad3ae5248e32df1f6c44f2f4ce5547ba
func toYaml(v interface{}) string {
	data, err := yaml.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}

// https://github.com/helm/helm/blob/main/pkg/engine/engine.go#L129
func includeFun(t *template.Template, includedNames map[string]int) func(string, interface{}) (string, error) {
	return func(name string, data interface{}) (string, error) {
		var buf strings.Builder
		if v, ok := includedNames[name]; ok {
			if v > recursionMaxNums {
				return "", errors.Wrapf(fmt.Errorf("unable to execute template"), "rendering template has a nested reference name: %s", name)
			}
			includedNames[name]++
		} else {
			includedNames[name] = 1
		}
		err := t.ExecuteTemplate(&buf, name, data)
		includedNames[name]--
		return buf.String(), err
	}
}

// https://github.com/helm/helm/blob/main/pkg/engine/engine.go#L148
func tplFun(parent *template.Template, includedNames map[string]int) func(string, interface{}) (string, error) {
	return func(tpl string, vals interface{}) (string, error) {
		t, err := parent.Clone()
		if err != nil {
			return "", errors.Wrapf(err, "cannot clone template")
		}

		// // Re-inject the missingkey option, see text/template issue https://github.com/golang/go/issues/43022
		// // We have to go by strict from our engine configuration, as the option fields are private in Template.
		// // TODO: Remove workaround (and the strict parameter) once we build only with golang versions with a fix.
		// if strict {
		// 	t.Option("missingkey=error")
		// } else {
		// 	t.Option("missingkey=zero")
		// }

		// Re-inject 'include' so that it can close over our clone of t;
		// this lets any 'define's inside tpl be 'include'd.
		t.Funcs(template.FuncMap{
			"include": includeFun(t, includedNames),
			"tpl":     tplFun(t, includedNames),
		})

		// We need a .New template, as template text which is just blanks
		// or comments after parsing out defines just adds new named
		// template definitions without changing the main template.
		// https://pkg.go.dev/text/template#Template.Parse
		// Use the parent's name for lack of a better way to identify the tpl
		// text string. (Maybe we could use a hash appended to the name?)
		t, err = t.New(parent.Name()).Parse(tpl)
		if err != nil {
			return "", errors.Wrapf(err, "cannot parse template %q", tpl)
		}

		var buf strings.Builder
		if err := t.Execute(&buf, vals); err != nil {
			return "", errors.Wrapf(err, "error during tpl function execution for %q", tpl)
		}

		// See comment in renderWithReferences explaining the <no value> hack.
		return strings.ReplaceAll(buf.String(), "<no value>", ""), nil
	}
}

// Parses a template defined in a file.
func ParseTemplate(
	file string,
) *template.Template {
	tpl := template.New(filepath.Base(file))
	includedNames := make(map[string]int)

	tpl.Funcs(
		template.FuncMap{
			"toYaml":  toYaml,
			"include": includeFun(tpl, includedNames),
			"tpl":     tplFun(tpl, includedNames),
		}).Funcs(sprig.FuncMap())

	var err error

	path, err := filepath.Abs(file)
	if err != nil {
		Error("%s", err)
	}

	tpl, err = tpl.ParseFiles(path)

	if err != nil {
		Error(
			"Could not parse template '%s': %s",
			file, err,
		)
	}

	return tpl
}

// Loads the includable template definitions.
func InitTemplateDirs(template *template.Template, templateDirs []string) *template.Template {
	for _, dir := range templateDirs {
		Debug(
			"Including templates from '%s'", dir,
		)

		path, err := filepath.Abs(dir)
		if err != nil {
			Error("%s", err)
		}

		glob := filepath.Join(path, "*.tpl")

		template, err = template.ParseGlob(glob)
		if err != nil {
			Error(
				"Could not parse templates in '%s': %s",
				dir, err,
			)
		}
	}
	return template
}

// Executes a template with the provided data.
func ExecuteTemplate(
	tplData map[string]interface{},
	tpl *template.Template,
) []byte {
	Debug(
		"Rendering template '%s'",
		tpl.Name(),
	)

	var rendered bytes.Buffer

	if err := tpl.ExecuteTemplate(
		&rendered,
		tpl.Name(),
		&tplData,
	); err != nil {
		Error(
			"Could not execute template '%s': %s",
			tpl.Name(), err,
		)
	}

	return rendered.Bytes()
}

// Loads yml data from a byte array.
func LoadYMLFromBytes(
	content []byte,
	obj interface{},
) {
	if viper.GetBool("debug") {
		Debug(
			fmt.Sprintf("Loading yaml structure: \n\n%s\n\n", string(content)),
		)
	}

	if err := yaml.Unmarshal(content, obj); err != nil {
		Error(
			"Failed to parse yaml: %s", err,
		)
	}
}

// Loads yml data from a file.
func LoadYMLFromFile(
	filename string,
	obj interface{},
) {
	Debug(
		"Loading yaml content from '%s'", filename,
	)

	path, err := filepath.Abs(filename)
	if err != nil {
		Error("%s", err)
	}

	yml, err := os.ReadFile(path)
	if err != nil {
		Error(
			"Failed to load file '%s': %s", filename, err,
		)
	}

	LoadYMLFromBytes(yml, obj)
}

// Returns the map specified by path
// If the path does not exist it will be created
// Returns nil if the element referenced by path is not a map.
func UpdateAndGetMapElementByPath(
	structure map[string]interface{},
	keyPath []string,
) map[string]interface{} {
	if len(keyPath) == 0 {
		return structure
	}

	var val interface{}
	var ok bool

	if val, ok = structure[keyPath[0]]; !ok {
		val = make(map[string]interface{})
		structure[keyPath[0]] = val
	}

	if nestedMap, ok := val.(map[string]interface{}); ok {
		return UpdateAndGetMapElementByPath(nestedMap, keyPath[1:])
	}

	return nil
}
