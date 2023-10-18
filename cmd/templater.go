package cmd

import (
	"bytes"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bossm8/dockerfile-templater/utils"
)

var (
	config       string
	verbose      bool
	debug        bool
	printVersion bool

	version string = "dev"

	TemplaterCMD = &cobra.Command{
		Use:              "templater",
		Short:            "Process Dockerfile templates",
		Long:             "Generate Dockerfiles in multiple variants from a template",
		PersistentPreRun: preRun,
		Run:              run,
	}
)

const (
	dockerfileTplFlag     = "dockerfile.tpl"
	dockerfileTplDirFlag  = "dockerfile.tpldir"
	tplAdditionalVarsFlag = "dockerfile.var"

	variantsDefFlag = "variants.def"
	variantsCfgFlag = "variants.cfg"

	outDirFlag = "out.dir"
	outFmtFlag = "out.fmt"
)

func init() {
	TemplaterCMD.PersistentFlags().StringP(
		dockerfileTplFlag, "t", "Dockerfile.tpl",
		"Path to the Dockerfile template",
	)
	_ = viper.BindPFlag(
		dockerfileTplFlag,
		TemplaterCMD.PersistentFlags().Lookup(dockerfileTplFlag),
	)

	TemplaterCMD.PersistentFlags().StringArrayP(
		dockerfileTplDirFlag, "d", make([]string, 0),
		"Path to a directory containing includable template definitions",
	)
	_ = viper.BindPFlag(
		dockerfileTplDirFlag,
		TemplaterCMD.PersistentFlags().Lookup(dockerfileTplDirFlag),
	)

	TemplaterCMD.PersistentFlags().StringToStringP(
		tplAdditionalVarsFlag, "a", make(map[string]string, 0),
		"Key=Value pairs of additional variables / variable overrides which "+
			"should be available when rendendering the Dockerfile template",
	)
	_ = viper.BindPFlag(
		tplAdditionalVarsFlag,
		TemplaterCMD.PersistentFlags().Lookup(tplAdditionalVarsFlag),
	)

	TemplaterCMD.PersistentFlags().StringP(
		variantsDefFlag, "i", "variants.yml",
		"Path to the variants definition. "+
			"This file may be a templated yml which will be processes when variants.cfg is defined",
	)
	_ = viper.BindPFlag(
		variantsDefFlag,
		TemplaterCMD.PersistentFlags().Lookup(variantsDefFlag),
	)

	TemplaterCMD.PersistentFlags().StringP(
		variantsCfgFlag, "g", "",
		"Path to the variants configuration yml. "+
			"This flag is optional and when provided, variants.def will be treated as template",
	)
	_ = viper.BindPFlag(
		variantsCfgFlag,
		TemplaterCMD.PersistentFlags().Lookup(variantsCfgFlag),
	)

	TemplaterCMD.PersistentFlags().StringP(
		outDirFlag, "o", "dockerfiles",
		"Directory to write generated Dockerfiles to",
	)
	_ = viper.BindPFlag(
		outDirFlag,
		TemplaterCMD.PersistentFlags().Lookup(outDirFlag),
	)

	TemplaterCMD.PersistentFlags().StringP(
		outFmtFlag, "f", "Dockerfile.{{ .image.name }}.{{ .image.tag }}",
		"Name format for generated Dockerfiles. "+
			"The format accepts a valid go template string which may contain any keys present in the variants",
	)
	_ = viper.BindPFlag(
		outFmtFlag,
		TemplaterCMD.PersistentFlags().Lookup(outFmtFlag),
	)

	TemplaterCMD.Flags().StringVarP(
		&config, "config", "c", "", "Configuration file",
	)
	TemplaterCMD.Flags().BoolVarP(
		&verbose, "verbose", "v", false, "Be more verbose",
	)
	TemplaterCMD.Flags().BoolVarP(
		&debug, "debug", "y", false, "Output processed yml variant files",
	)
	TemplaterCMD.Flags().BoolVarP(
		&printVersion, "version", "V", false, "Get the templater version",
	)

	viper.SetEnvPrefix("DTPL")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
}

func run(_ *cobra.Command, _ []string) {
	templater := &templater{
		DockerfileTpl:       viper.GetString(dockerfileTplFlag),
		DockerfileTplDirs:   viper.GetStringSlice(dockerfileTplDirFlag),
		OutputDir:           viper.GetString(outDirFlag),
		AdditionalVariables: viper.GetStringMapString(tplAdditionalVarsFlag),
	}
	variants := &variants{
		VariantsTplFile: viper.GetString(variantsDefFlag),
		VariantsCfgFile: viper.GetString(variantsCfgFlag),
	}

	variants.Load()

	if verbose {
		variants.Debug()
	}

	templater.Init()
	templater.Render(variants.Variants)
}

func preRun(_ *cobra.Command, _ []string) {
	if printVersion {
		log.Println(version)
		os.Exit(0)
	}

	if verbose {
		utils.SetVerbose()
	}

	if config != "" {
		utils.Debug(
			"Loading flags from configuration file '%s'",
			config,
		)

		abs, err := filepath.Abs(config)
		if err != nil {
			utils.Error(
				"Could not find config file '%s': %s",
				config, err,
			)
		}

		viper.SetConfigType("yaml")
		viper.SetConfigFile(abs)

		if err := viper.ReadInConfig(); err != nil {
			utils.Error(
				"Failed to read config file '%s': %s",
				config, err,
			)
		}
	}
}

// The actual variant of Dockerfile which will be passed to the template.
type variant struct {
	Name  *string `yaml:"name"`
	Image *struct {
		Name *string `yaml:"name"`
		Tag  *string `yaml:"tag"`
	} `yaml:"image"`
	Data map[string]interface{} `yaml:",inline"`
}

// Verifies if the required attributes for each variant are defined and
// fails if not.
func (v *variant) Verify() {
	// logs missing attributes as error.
	var logMissingAttribute = func(attribute string) {
		utils.Error(
			"Variant missing required attribute '%s'\n"+
				"Required Structure:\n\n"+
				"variants:\n\n"+
				"   - name: <VARIANT_NAME>\n"+
				"     image:\n"+
				"       name: <IMAGE_NAME>\n"+
				"       tag: <IMAGE_TAG>\n\n",
			attribute,
		)
	}

	if v.Name == nil {
		logMissingAttribute("name")
	}
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

// Get the output filename of this variant.
func (v *variant) OutputFile() string {
	fmt := viper.GetString(outFmtFlag)

	tpl, err := template.New("OutputFile").Parse(fmt)
	if err != nil {
		utils.Error(
			"Failed to parse output file format '%s': %s",
			fmt, err,
		)
	}

	var filename = bytes.Buffer{}
	if err := tpl.Execute(&filename, v.Data); err != nil {
		utils.Error(
			"Failed to generate output file name: %s",
			err,
		)
	}

	return filename.String()
}

// Returns the variant as yml.
func (v *variant) String(dataOnly bool) string {
	var res []byte
	var err error

	if !dataOnly {
		res, err = yaml.Marshal(v)
	} else {
		res, err = yaml.Marshal(v.Data)
	}

	if err == nil {
		return string(res)
	}

	utils.Warn(
		"Could not marshal variant %+v for debugging", v,
	)

	return ""
}

// Adds the variables to the data passed to the template.
func (v *variant) UpdateData(variables map[string]string) {
	for key, val := range variables {

		// Check if a variant name prefix is specified in the key
		// if yes, add it only to the variant with the matching name
		// if no, add it to all
		variantKey := strings.Split(key, ":")
		if len(variantKey) == 2 {
			if variantKey[0] != *v.Name {
				utils.Debug(
					"Skip adding value '%s' to variant '%s' as names do not match",
					key, *v.Name,
				)
				continue
			}
		}

		elem := v.Data
		keyPath := variantKey[len(variantKey)-1]
		keyPathList := strings.Split(keyPath, ".")

		// If there are multiple keys in the path we need to traverse the
		// struct and find the one containing the last key.
		// The algorithm below returns structs only and we want the struct
		// containing the last key, which is why we omit it in the keyPath argument
		if len(keyPathList) > 1 {
			elem = utils.UpdateAndGetMapElementByPath(
				elem,
				keyPathList[:len(keyPathList)-1],
			)
		}

		if elem == nil {
			utils.Warn(
				"Please check the path of the additional variable '%s'."+
					"The key path '%s' is invalid for variant '%s'",
				key, keyPath, *v.Name,
			)
		}

		lastKey := keyPathList[len(keyPathList)-1]

		if curr, ok := elem[lastKey]; ok {
			utils.Warn(
				"Overriding variant value '%s' of '%s' with '%s'",
				curr, keyPath, val,
			)
		}

		utils.Debug("Adding variable '%s' with value '%s'", keyPath, val)
		elem[lastKey] = val
	}
}

// Adds the image object to the Data struct which will be passed to the template.
func (v *variant) SetDataImage() {
	v.Data["image"] = map[string]interface{}{
		"name": *v.Image.Name,
		"tag":  *v.Image.Tag,
	}
	v.Data["name"] = *v.Name
}

// The container for the variants yml.
type variants struct {
	Variants []*variant `yaml:"variants"`

	VariantsCfgFile string
	VariantsTplFile string
}

// Verifies if the variants configuration is valid.
func (t *variants) Verify() {
	if len(t.Variants) == 0 {
		utils.Error("No variants configured")
	}

	for _, v := range t.Variants {
		v.Verify()
	}
}

// Outputs the processed variants as yml.
func (t *variants) Debug() {
	if !debug {
		return
	}
	utils.Debug(
		"Building Dockerfiles for variants:",
	)

	yml := "\n"

	for _, variant := range t.Variants {
		yml += variant.String(false) + "\n"
	}

	log.Print(yml)
}

// Loads the variants configuration from a templated variants.yml.
func (t *variants) loadFromTemplate() {
	utils.Debug(
		"Loading variant config from '%s'", t.VariantsCfgFile,
	)
	utils.Debug(
		"Variants ('%s') will be treated as template", t.VariantsTplFile,
	)

	var vc map[string]interface{}

	utils.LoadYMLFromFile(t.VariantsCfgFile, &vc)

	tpl := utils.ParseTemplate(t.VariantsTplFile)
	res := utils.ExecuteTemplate(vc, tpl)

	utils.LoadYMLFromBytes(res, t)
}

// Loads the variants configuration from a plain variants.yml.
func (t *variants) loadFromPlain() {
	utils.Debug(
		"Loading variants from '%s'", t.VariantsTplFile,
	)

	utils.LoadYMLFromFile(t.VariantsTplFile, t)
}

// Loads the template data from the yml file(s).
func (t *variants) Load() {
	if t.VariantsCfgFile == "" {
		t.loadFromPlain()
	} else {
		t.loadFromTemplate()
	}

	t.Verify()
}

// templater holds the main logic to render the Dockerfiles to the output directory.
type templater struct {
	DockerfileTpl     string
	DockerfileTplDirs []string
	OutputDir         string

	AdditionalVariables map[string]string

	template *template.Template
}

// Renders the Dockerfiles to the output directory.
func (t *templater) Render(variants []*variant) {
	for _, variant := range variants {

		variant.SetDataImage()
		variant.UpdateData(t.AdditionalVariables)

		if len(t.AdditionalVariables) > 0 && debug {
			utils.Debug("Adjusted variant: \n\n")
			log.Printf("%s\n", variant.String(true))
		}

		dockerfile := path.Join(
			t.OutputDir,
			variant.OutputFile(),
		)

		dockerfile, err := filepath.Abs(dockerfile)
		if err != nil {
			utils.Error("%s", err)
		}

		rendered := utils.ExecuteTemplate(
			variant.Data,
			t.template,
		)

		utils.Info(
			"Writing to '%s'", dockerfile,
		)

		if err := os.WriteFile(dockerfile, rendered, os.ModePerm); err != nil {
			utils.Error(
				"Could not write Dockerfile to '%s': %s", dockerfile, err,
			)
		}
	}
}

// Loads the includable template definitions.
func (t *templater) initTemplateDirs() {
	for _, dir := range t.DockerfileTplDirs {
		utils.Debug(
			"Including templates from '%s'", dir,
		)

		path, err := filepath.Abs(dir)
		if err != nil {
			utils.Error("%s", err)
		}

		glob := filepath.Join(path, "*.tpl")

		t.template, err = t.template.ParseGlob(glob)
		if err != nil {
			utils.Error(
				"Could not parse templates in '%s': %s",
				dir, err,
			)
		}
	}
}

// Initializes the main Dockerfile template.
func (t *templater) initTemplate() {
	t.template = utils.ParseTemplate(t.DockerfileTpl)
	t.initTemplateDirs()
}

// Creates the output directory.
func (t *templater) createOutDir() {
	utils.Info(
		"Creating non existing output directory '%s'", t.OutputDir,
	)

	if err := os.Mkdir(t.OutputDir, os.ModePerm); err != nil {
		utils.Error(
			"Failed creating output directory '%s': %s\n", t.OutputDir, err,
		)
	}
}

// Makes sure the output directory exists.
func (t *templater) ensureOutDir() {
	utils.Debug(
		"Checking that output directory '%s' exists", t.OutputDir,
	)

	_, err := os.Stat(t.OutputDir)

	if err != nil && os.IsNotExist(err) {
		t.createOutDir()
	} else if err != nil {
		utils.Error(
			"Failed checking output directory: %s", err,
		)
	}
}

// Initializes the templater by preparing the template and the output.
func (t *templater) Init() {
	t.initTemplate()
	t.ensureOutDir()
}
