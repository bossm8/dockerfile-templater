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
	"text/template"

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

// The container for the variants yml
type templateData struct {
	Variants []*variant `yaml:"variants"`
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
	levelInfo           = "INFO"
	levelWarn           = "WARN"
	levelDebug          = "DEBUG"
)

// Log level mappings to the real log function
var logs = map[logLevel]func(string, ...any){
	levelError: log.Fatalf,
	levelInfo:  log.Printf,
	levelWarn:  log.Printf,
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

// Renders the gotemplate from the file with the passed data into memory
func renderTpl(
	filename *string,
	data interface{},
) []byte {
	logf(
		levelDebug,
		"Rendering template '%s'",
		*filename,
	)

	tpl, err := template.ParseFiles(*filename)
	if err != nil {
		logf(
			levelError,
			"Could not parse template '%s': %s",
			*filename, err,
		)
	}

	var rendered bytes.Buffer
	err = tpl.Execute(
		&rendered,
		data,
	)
	if err != nil {
		logf(
			levelError,
			"Could not execute template '%s': %s",
			*filename, err,
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
			"Failed to parse yaml: %s",
			err,
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
		"Loading yaml content from '%s'",
		*filename,
	)

	yml, err := os.ReadFile(*filename)
	if err != nil {
		logf(
			levelError,
			"Failed to load file '%s': %s",
			*filename, err,
		)
	}

	loadYMLFromBytes(yml, obj)
}

// Makes sure the output directory exists
func ensureOutDirExists(
	dirname *string,
) {
	logf(
		levelDebug,
		"Checking that output directory '%s' exists",
		*dirname,
	)

	_, err := os.Stat(*dirname)

	if os.IsNotExist(err) {
		logf(
			levelInfo,
			"Creating non existing output directory '%s'",
			*dirname,
		)
		if err = os.Mkdir(*dirname, os.ModePerm); err != nil {
			logf(
				levelError,
				"Failed creating output directory '%s': %s\n",
				*dirname, err,
			)
		}
	} else if err != nil {
		logf(
			levelError,
			"Failed checking output directory: %s",
			err,
		)
	}
}

// Writes a dockerfile with the passed content to filename
func writeDockerFile(
	filename *string,
	content []byte,
) {
	logf(
		levelInfo,
		"Writing to '%s'",
		*filename,
	)

	if err := os.WriteFile(
		*filename,
		content,
		os.ModePerm,
	); err != nil {
		logf(
			levelError,
			"Could not write Dockerfile to '%s': %s",
			*filename, err,
		)
	}
}

// Loads the variants from a template yml file and processes it with the data
// from the variants configuration file
func loadVariantsAsTemplate(
	variantsCfgFile *string,
	variantsTplFile *string,
	variants *templateData,
) {
	logf(
		levelDebug,
		"Loading variant config from '%s'",
		*variantsCfgFile,
	)
	logf(
		levelDebug,
		"Variants ('%s') will be treated as template",
		*variantsTplFile,
	)

	vc := variantsTemplateData{}
	loadYMLFromFile(
		variantsCfgFile,
		&vc,
	)

	res := renderTpl(
		variantsTplFile,
		&vc,
	)
	loadYMLFromBytes(
		res,
		&variants,
	)
}

// Loads the variants from a non-template yml file
func loadVariantsAsPlain(
	variantsFile *string,
	variants *templateData,
) {
	logf(
		levelDebug,
		"Loading variants from '%s'",
		*variantsFile,
	)

	loadYMLFromFile(
		variantsFile,
		&variants,
	)
}

// Loads the variants from the (optional) configuration and the template
func loadVariants(
	variantsCfgFile *string,
	variantsTplFile *string,
) *templateData {
	variants := templateData{}

	if *variantsCfgFile != "" {
		loadVariantsAsTemplate(
			variantsCfgFile,
			variantsTplFile,
			&variants,
		)
	} else {
		loadVariantsAsPlain(
			variantsTplFile,
			&variants,
		)
	}

	if len(variants.Variants) == 0 {
		logf(levelError, "No variants configured")
	}

	if verbose {
		debugVariants(&variants)
	}

	return &variants
}

// Debug prints the processed variants as yml
func debugVariants(
	variants *templateData,
) {
	logf(
		levelDebug,
		"Building Dockerfiles for variants:",
	)

	yml := "\n"

	for _, variant := range variants.Variants {
		res, err := yaml.Marshal(variant)

		if err != nil {
			logf(
				levelWarn,
				"Could not marshal variant %+v for debugging",
				variant,
			)
			continue
		}

		yml += string(res) + "\n"
	}

	fmt.Printf(yml)
}

// Renders the dockerfile template for each variant to a Dockerfile into the
// output directory
func renderDockerfiles(
	variants *templateData,
	outputDir *string,
	dockerfileTpl *string,
	dockerfileSep *string,
) {
	for _, variant := range variants.Variants {

		dockerfile := fmt.Sprintf(
			"Dockerfile%s%s%s%s",
			*dockerfileSep,
			*variant.Image.Name,
			*dockerfileSep,
			*variant.Image.Tag,
		)
		dockerfile = path.Join(
			*outputDir,
			dockerfile,
		)

		// Re-add the image struct with lowercase values since otherwise they
		// are not accessible or when added with the Image struct itself
		// only under .Name and .Tag
		variant.Data["image"] = map[string]interface{}{
			"name": *variant.Image.Name,
			"tag":  *variant.Image.Tag,
		}

		res := renderTpl(
			dockerfileTpl,
			&variant.Data,
		)

		writeDockerFile(
			&dockerfile,
			res,
		)
	}
}

func main() {
	variantsTplFile := flag.String(
		"variants",
		"variants.yml",
		"The main variants definition file, "+
			"this file will be treated as a template when variants.cfg is defined",
	)
	variantsCfgFile := flag.String(
		"variants.cfg",
		"",
		"Optional variants configuration. "+
			"If provided, the variants yml will be treated as template "+
			"and this configuration will be applied on it",
	)
	outputDir := flag.String(
		"out.dir",
		"dockerfiles",
		"Where to output the rendered Dockerfiles",
	)
	dockerfileTpl := flag.String(
		"dockerfile.tpl",
		"Dockerfile.tpl",
		"The template dockerile to use",
	)
	dockerfileSep := flag.String(
		"dockerfile.sep",
		"_-_",
		"The separator used in the outputted Dockerfiles",
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

	variants := loadVariants(
		variantsCfgFile,
		variantsTplFile,
	)

	ensureOutDirExists(outputDir)

	renderDockerfiles(
		variants,
		outputDir,
		dockerfileTpl,
		dockerfileSep,
	)
}
