package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/Masterminds/sprig/v3"
	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const recursionMaxNums = 1000

// https://github.com/helm/helm/blob/518c69281f42d9b3a5cf99bd959a08e048093e20/pkg/engine/funcs.go#L82
func toYAML(v interface{}) string {
	data, err := yaml.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}
	return strings.TrimSuffix(string(data), "\n")
}

// https://github.com/helm/helm/blob/518c69281f42d9b3a5cf99bd959a08e048093e20/pkg/engine/funcs.go#L97
func fromYAML(str string) map[string]interface{} {
	m := map[string]interface{}{}

	if err := yaml.Unmarshal([]byte(str), &m); err != nil {
		m["Error"] = err.Error()
	}
	return m
}

// https://github.com/helm/helm/blob/518c69281f42d9b3a5cf99bd959a08e048093e20/pkg/engine/funcs.go#L112
func fromYAMLArray(str string) []interface{} {
	a := []interface{}{}

	if err := yaml.Unmarshal([]byte(str), &a); err != nil {
		a = []interface{}{err.Error()}
	}
	return a
}

// https://github.com/helm/helm/blob/518c69281f42d9b3a5cf99bd959a08e048093e20/pkg/engine/funcs.go#L125
func toTOML(v interface{}) string {
	b := bytes.NewBuffer(nil)
	e := toml.NewEncoder(b)
	err := e.Encode(v)
	if err != nil {
		return err.Error()
	}
	return b.String()
}

// https://github.com/helm/helm/blob/518c69281f42d9b3a5cf99bd959a08e048093e20/pkg/engine/funcs.go#L139
func toJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}
	return string(data)
}

// https://github.com/helm/helm/blob/518c69281f42d9b3a5cf99bd959a08e048093e20/pkg/engine/funcs.go#L154
func fromJSON(str string) map[string]interface{} {
	m := make(map[string]interface{})

	if err := json.Unmarshal([]byte(str), &m); err != nil {
		m["Error"] = err.Error()
	}
	return m
}

// https://github.com/helm/helm/blob/518c69281f42d9b3a5cf99bd959a08e048093e20/pkg/engine/funcs.go#L169
func fromJSONArray(str string) []interface{} {
	a := []interface{}{}

	if err := json.Unmarshal([]byte(str), &a); err != nil {
		a = []interface{}{err.Error()}
	}
	return a
}

func mergeOverwriteAppendSlice(dst map[string]interface{}, srcs ...map[string]interface{}) interface{} {
	for _, src := range srcs {
		if err := mergo.MergeWithOverwrite(&dst, src, mergo.WithAppendSlice); err != nil {
			// Swallow errors inside of a template.
			return ""
		}
	}
	return dst
}

func mustMergeOverwriteAppendSlice(dst map[string]interface{}, srcs ...map[string]interface{}) (interface{}, error) {
	for _, src := range srcs {
		if err := mergo.MergeWithOverwrite(&dst, src, mergo.WithAppendSlice); err != nil {
			return nil, err
		}
	}
	return dst, nil
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
	templateDirs []string,
) *template.Template {
	tpl := template.New(filepath.Base(file))
	includedNames := make(map[string]int)

	tpl.Funcs(
		template.FuncMap{
			"toToml":                       toTOML,
			"toYaml":                       toYAML,
			"fromYaml":                     fromYAML,
			"fromYamlArray":                fromYAMLArray,
			"toJson":                       toJSON,
			"fromJson":                     fromJSON,
			"fromJsonArray":                fromJSONArray,
			"include":                      includeFun(tpl, includedNames),
			"tpl":                          tplFun(tpl, includedNames),
			"mergeOverwriteAppendSlice":    mergeOverwriteAppendSlice,
			"mustMergeOverwriteAppendSlce": mustMergeOverwriteAppendSlice,
		}).Funcs(sprig.FuncMap())

	var err error

	path, err := filepath.Abs(file)
	if err != nil {
		Error("%s", err)
	}

	tpl = InitTemplateDirs(tpl, templateDirs)
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
