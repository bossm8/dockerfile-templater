// Dockerfile Templating Utility
// Generates Dockerfiles from a template with different variants read from
// yaml files

// Author: <bossm8@hotmail.com>

package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"gopkg.in/yaml.v3"
)

// Must have configuration for each variant
type image struct {
	Name *string `yaml:"name"`
	Tag  *string `yaml:"tag"`
}

// The actual variant of Dockerfile which will be passed to the template
type variant struct {
	Image *image                 `yaml:"image"`
	Data  map[string]interface{} `yaml:",inline"`
}

// Verifies if the required attributes for each variant are defined and
// fails if not
func (v *variant) Verify() {
	if v.Image == nil {
		logMissingAttribute("image")
	}
	if v.Image.Name == nil {
		logMissingAttribute("image.name")
	}
	if v.Image.Tag == nil {
		logMissingAttribute("image.tag")
	}
}

// Get the output filename of this variant
func (v *variant) OutputFile(fmt *string) string {
	tpl, err := template.New("OutputFile").Parse(*fmt)
	if err != nil {
		logf(
			levelError,
			"Failed to parse output file format '%s': %s",
			*fmt, err,
		)
	}

	var filename = bytes.Buffer{}
	if err := tpl.Execute(&filename, v.Data); err != nil {
		logf(
			levelError,
			"Failed to generate output file name: %s",
			err,
		)
	}

	return filename.String()
}

// Returns the variant as yml
func (v *variant) ToString() string {
	res, err := yaml.Marshal(v)
	if err == nil {
		return string(res)
	}
	logf(
		levelWarn,
		"Could not marshal variant %+v for debugging", v,
	)
	return ""
}

// The container for the variants yml
type templateData struct {
	Variants []*variant `yaml:"variants"`

	VariantsCfgFile *string
	VariantsTplFile *string
}

// Verifies if the variants configuration is valid
func (t *templateData) Verify() {

	if len(t.Variants) == 0 {
		logf(levelError, "No variants configured")
	}

	for _, v := range t.Variants {
		v.Verify()
	}

}

// Outputs the processed variants as yml
func (t *templateData) Debug() {
	logf(
		levelDebug,
		"Building Dockerfiles for variants:",
	)

	yml := "\n"

	for _, variant := range t.Variants {
		res := variant.ToString()
		yml += string(res) + "\n"
	}

	fmt.Print(yml)
}

// Loads the variants configuration from a templated variants.yml
func (t *templateData) loadFromTemplate() {
	logf(
		levelDebug,
		"Loading variant config from '%s'", *t.VariantsCfgFile,
	)
	logf(
		levelDebug,
		"Variants ('%s') will be treated as template", *t.VariantsTplFile,
	)

	vc := variantsTemplateData{}
	loadYMLFromFile(t.VariantsCfgFile, &vc)

	tpl := parseTemplate(t.VariantsTplFile)
	res := exectuteTemplate(&vc, tpl)

	loadYMLFromBytes(res, t)
}

// Loads the variants configuration from a plain variants.yml
func (t *templateData) loadFromPlain() {
	logf(
		levelDebug,
		"Loading variants from '%s'", *t.VariantsTplFile,
	)

	loadYMLFromFile(t.VariantsTplFile, t)
}

// Loads the template data from the yml file(s)
func (t *templateData) Load() {

	if t.VariantsCfgFile != nil && *t.VariantsCfgFile != "" {
		t.loadFromTemplate()
	} else {
		t.loadFromPlain()
	}

	t.Verify()
}

// templater holds the main logic to render the Dockerfiles to the output directory
type templater struct {
	DockerfileTpl    *string
	DockerfileTplDir *string
	OutputDir        *string
	OutputFmt        *string

	template *template.Template
}

// Renders the Dockerfiles to the output directory
func (t *templater) Render(variants []*variant) {

	for _, variant := range variants {
		// Re-add the image struct with lowercase values since otherwise they
		// are not accessible or when added with the Image struct itself
		// only under .Name and .Tag
		variant.Data["image"] = map[string]interface{}{
			"name": *variant.Image.Name,
			"tag":  *variant.Image.Tag,
		}

		dockerfile := path.Join(
			*t.OutputDir,
			variant.OutputFile(t.OutputFmt),
		)

		rendered := exectuteTemplate(
			variant.Data,
			t.template,
		)

		logf(
			levelInfo,
			"Writing to '%s'", dockerfile,
		)

		if err := os.WriteFile(dockerfile, rendered, os.ModePerm); err != nil {
			logf(
				levelError,
				"Could not write Dockerfile to '%s': %s", dockerfile, err,
			)
		}
	}
}

// Loads the includable template definitions
func (t *templater) initTemplateDir() {
	if t.DockerfileTplDir == nil || *t.DockerfileTplDir == "" {
		return
	}

	glob := filepath.Join(*t.DockerfileTplDir, "*.tpl")

	var err error
	t.template, err = t.template.ParseGlob(glob)
	if err != nil {
		logf(
			levelError,
			"Could not parse templates in '%s': %s",
			*t.DockerfileTplDir, err,
		)
	}
}

// Initializes the main Dockerfile template
func (t *templater) initTemplate() {
	t.template = parseTemplate(t.DockerfileTpl)
	t.initTemplateDir()
}

// Creates the output directory
func (t *templater) createOutDir() {
	logf(
		levelInfo,
		"Creating non existing output directory '%s'", *t.OutputDir,
	)
	if err := os.Mkdir(*t.OutputDir, os.ModePerm); err != nil {
		logf(
			levelError,
			"Failed creating output directory '%s': %s\n", *t.OutputDir, err,
		)
	}
}

// Makes sure the output directory exists
func (t *templater) ensureOutDir() {
	logf(
		levelDebug,
		"Checking that output directory '%s' exists", *t.OutputDir,
	)

	_, err := os.Stat(*t.OutputDir)

	if os.IsNotExist(err) {
		t.createOutDir()
	} else if err != nil {
		logf(
			levelError,
			"Failed checking output directory: %s", err,
		)
	}
}

// Initializes the templater by preparing the template and the output
func (t *templater) Init() {
	t.initTemplate()
	t.ensureOutDir()
}

// Type for the contents of the variants configuration file
// This type of content does not have any restrictions
type variantsTemplateData map[string]interface{}

// If the application should log in verbose mode
var verbose bool

// If the application should debug yaml templating steps
var debugYMLTpl bool

// Defines our log level
type logLevel string

// Supported log levels
const (
	levelError logLevel = "ERROR"
	levelInfo  logLevel = "INFO"
	levelWarn  logLevel = "WARN"
	levelDebug logLevel = "DEBUG"
)

// Log level mappings to the real log function
var logs = map[logLevel]func(string, ...any){
	levelError: log.Fatalf,
	levelWarn:  log.Printf,
	levelInfo:  log.Printf,
	levelDebug: log.Printf,
}

// Log a formatted string with the configured level
func logf(
	level logLevel,
	message string,
	v ...any,
) {
	if level == levelDebug && !verbose {
		return
	}
	logs[level](
		fmt.Sprintf("[%s]: %s", level, message),
		v...,
	)
}

// logs missing attributes as error
func logMissingAttribute(
	attribute string,
) {
	logf(
		levelError,
		"Variant missing required attribute '%s'\n"+
			"Required Structure:\n\n"+
			"variants:\n\n"+
			"   - image:\n"+
			"       name: <IMAGE_NAME>\n"+
			"       tag: <IMAGE_TAG>\n\n",
		attribute,
	)
}

// Parses a template defined in a file
func parseTemplate(
	tplFile *string,
) *template.Template {

	tpl := template.New(filepath.Base(*tplFile)).Funcs(sprig.FuncMap())
	var err error
	tpl, err = tpl.ParseFiles(*tplFile)

	if err != nil {
		logf(
			levelError,
			"Could not parse template '%s': %s",
			*tplFile, err,
		)
	}
	return tpl
}

// Executes a template with the provided data
func exectuteTemplate(
	tplData interface{},
	tpl *template.Template,
) []byte {
	logf(
		levelDebug,
		"Rendering template '%s'",
		tpl.Name(),
	)

	var rendered bytes.Buffer
	err := tpl.ExecuteTemplate(
		&rendered,
		tpl.Name(),
		&tplData,
	)
	if err != nil {
		logf(
			levelError,
			"Could not execute template '%s': %s",
			tpl.Name(), err,
		)
	}

	return rendered.Bytes()
}

// Loads yml data from a byte array
func loadYMLFromBytes(
	content []byte,
	obj interface{},
) {
	if debugYMLTpl {
		logf(
			levelDebug,
			fmt.Sprintf("Loading yaml structure: \n\n%s\n\n", string(content)),
		)
	}
	if err := yaml.Unmarshal(content, obj); err != nil {
		logf(
			levelError,
			"Failed to parse yaml: %s", err,
		)
	}
}

// Loads yml data from a file
func loadYMLFromFile(
	filename *string,
	obj interface{},
) {
	logf(
		levelDebug,
		"Loading yaml content from '%s'", *filename,
	)

	yml, err := os.ReadFile(*filename)
	if err != nil {
		logf(
			levelError,
			"Failed to load file '%s': %s", *filename, err,
		)
	}

	loadYMLFromBytes(yml, obj)
}

func main() {

	templater := &templater{}
	variants := &templateData{}

	variants.VariantsTplFile = flag.String(
		"variants",
		"variants.yml",
		"The main variants definition file, "+
			"this file will be treated as a template when variants.cfg is defined",
	)
	variants.VariantsCfgFile = flag.String(
		"variants.cfg",
		"",
		"Optional variants configuration. "+
			"If provided, the variants yml will be treated as template "+
			"and this configuration will be applied on it",
	)
	templater.OutputDir = flag.String(
		"out.dir",
		"dockerfiles",
		"Where to output the rendered Dockerfiles",
	)
	templater.OutputFmt = flag.String(
		"out.fmt",
		"Dockerfile-_-{{ .image.name }}-_-{{ .image.tag }}",
		"The naming format for the outputed Dockerfiles",
	)
	templater.DockerfileTpl = flag.String(
		"dockerfile.tpl",
		"Dockerfile.tpl",
		"The template dockerile to use",
	)
	templater.DockerfileTplDir = flag.String(
		"dockerfile.tpldir",
		"",
		"A directory containing templates to process (files must end in .tpl)",
	)

	flag.BoolVar(
		&verbose,
		"verbose",
		false,
		"Be more verbose",
	)
	flag.BoolVar(
		&debugYMLTpl,
		"yml.debug",
		false,
		"Debug yml processing, may help finding issues with your variant templates."+
			"Only takes effect when used together with -verbose",
	)

	flag.Parse()

	variants.Load()

	if verbose {
		variants.Debug()
	}

	templater.Init()
	templater.Render(variants.Variants)
}
