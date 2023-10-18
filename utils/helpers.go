package utils

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// Parses a template defined in a file.
func ParseTemplate(
	file string,
) *template.Template {
	tpl := template.New(filepath.Base(file)).Funcs(sprig.FuncMap())

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

// Executes a template with the provided data.
func ExectuteTemplate(
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
