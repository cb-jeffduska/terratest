package helm

import (
	"encoding/json"
	"errors"
	"github.com/ghodss/yaml"
	gwErrors "github.com/gruntwork-io/gruntwork-cli/errors"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/files"
)

const UNKNOWN_HELM_VERSION_ERR = "-1"
const HELM_V3 = "3"
const HELM_V2 = "2"


// Get the Helm version
func getHelmVersion(t *testing.T) (string, error) {

	output, err := RunHelmCommandAndGetOutputE(t, nil, "version")
	if err != nil {
		return UNKNOWN_HELM_VERSION_ERR, gwErrors.WithStackTrace(err)
	}
	if strings.Contains(output,"v3.") {
		return HELM_V3,nil
	}
	if strings.Contains(output, "v2.") {
		return HELM_V2,nil
	}

	return UNKNOWN_HELM_VERSION_ERR,errors.New("An unknown Helm version error has occured")
}

// RenderTemplate runs `helm template` to render the template given the provided options and returns stdout/stderr from
// the template command. If you pass in templateFiles, this will only render those templates. This function will fail
// the test if there is an error rendering the template.
func RenderTemplate(t *testing.T, options *Options, chartDir string, releaseName string, templateFiles []string) string {
	out, err := RenderTemplateE(t, options, chartDir, releaseName, templateFiles)
	require.NoError(t, err)
	return out
}

// RenderTemplateE runs `helm template` to render the template given the provided options and returns stdout/stderr from
// the template command. If you pass in templateFiles, this will only render those templates.
// Updated to use the getHelmVersion to handle the changes in the arguments between Helm 2 and Helm 3
func RenderTemplateE(t *testing.T, options *Options, chartDir string, releaseName string, templateFiles []string) (string, error) {
	// First, verify the charts dir exists
	absChartDir, err := filepath.Abs(chartDir)
	if err != nil {
		return "", gwErrors.WithStackTrace(err)
	}
	if !files.FileExists(chartDir) {
		return "", gwErrors.WithStackTrace(ChartNotFoundError{chartDir})
	}

	helmVersion, err := getHelmVersion(t)
	if err != nil {
		return "", gwErrors.WithStackTrace(err)
	}

	// Now construct the args
	// We first construct the template args
	args := []string{}
	if helmVersion == HELM_V2 {
	args, s, err2, done := getHelm2Args(args, releaseName, options, err, t, templateFiles, absChartDir, chartDir)
	if done {
		return s, err2
	}
	}

	// Finally, call out to helm template command
	return RunHelmCommandAndGetOutputE(t, options, "template", args...)
}

func getHelm2Args(args []string, releaseName string, options *Options, err error, t *testing.T, templateFiles []string, absChartDir string, chartDir string) ([]string, string, error, bool) {
	args = append(args, "--name", releaseName)
	if options.KubectlOptions != nil && options.KubectlOptions.Namespace != "" {
		args = append(args, "--namespace", options.KubectlOptions.Namespace)
	}
	args, err = getValuesArgsE(t, options, args...)
	if err != nil {
		return nil, "", err, true
	}
	for _, templateFile := range templateFiles {
		// validate this is a valid template file
		absTemplateFile := filepath.Join(absChartDir, templateFile)
		if !files.FileExists(absTemplateFile) {
			return nil, "", gwErrors.WithStackTrace(TemplateFileNotFoundError{Path: templateFile, ChartDir: absChartDir}), true
		}

		// Note: we only get the abs template file path to check it actually exists, but the `helm template` command
		// expects the relative path from the chart.
		args = append(args,  templateFile)
	}
	// ... and add the chart at the end as the command expects
	args = append(args, chartDir)
	return args, "", nil, false
}

// UnmarshalK8SYaml is the same as UnmarshalK8SYamlE, but will fail the test if there is an error.
func UnmarshalK8SYaml(t *testing.T, yamlData string, destinationObj interface{}) {
	require.NoError(t, UnmarshalK8SYamlE(t, yamlData, destinationObj))
}

// UnmarshalK8SYamlE can be used to take template outputs and unmarshal them into the corresponding client-go struct. For
// example, suppose you render the template into a Deployment object. You can unmarshal the yaml as follows:
//
// var deployment appsv1.Deployment
// UnmarshalK8SYamlE(t, renderedOutput, &deployment)
//
// At the end of this, the deployment variable will be populated.
func UnmarshalK8SYamlE(t *testing.T, yamlData string, destinationObj interface{}) error {
	// NOTE: the client-go library can only decode json, so we will first convert the yaml to json before unmarshaling
	jsonData, err := yaml.YAMLToJSON([]byte(yamlData))
	if err != nil {
		return gwErrors.WithStackTrace(err)
	}
	err = json.Unmarshal(jsonData, destinationObj)
	if err != nil {
		return gwErrors.WithStackTrace(err)
	}
	return nil
}
