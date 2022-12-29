/*
Copyright © 2021 NAME HERE mrolim@senhasegura.com

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package dsm

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	sdk "github.com/evangoncalves/sdk"
)

var kv map[string]string

var Verbose bool
var ToolName string
var Environment string
var System string
var ApplicationName string

var RunbCmd = &cobra.Command{
	Use:   "runb",
	Short: "Running Belt plugin to insert/get/replace environment variables in most CI/CD pipelines.",
	Long:  `Running Belt plugin to insert/get/replace environment variables in most CI/CD pipelines.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isDisabled() {
			return errors.Errorf("SENHASEGURA_DISABLE_RUNB is set to true. Plugin is disabled.")
		}

		ctx := context.WithValue(context.Background(), sdk.ContextServerVariables, map[string]string{
			"baseURL": "https://stoplight.io/mocks/senhasegura/senhasegura/93588514",
		})

		configuration := sdk.NewConfiguration()
		configuration.AddDefaultHeader("Prefer", "example=Complete")

		client := sdk.NewAPIClient(configuration)

		data, _, err := client.DefaultApi.ReadData(ctx).Execute()
		if err != nil {
			return err
		}

		err = injectEnvironmentVariables(data.Secrets)
		if err != nil {
			return err
		}

		return nil
	},
}

func init() {
	RunbCmd.Flags().BoolVarP(&Verbose, "verbose", "v", false, "Verbose mode")
	RunbCmd.Flags().StringVarP(&ApplicationName, "application", "a", "", "Application name (required)")
	RunbCmd.Flags().StringVarP(&System, "system", "s", "", "Application system (required)")
	RunbCmd.Flags().StringVarP(&Environment, "environment", "e", "", "Application environment (required)")
	RunbCmd.Flags().StringVarP(&ToolName, "tool", "t", "linux", "Tool name [github, azure-devops, bamboo, bitbucket, circleci, teamcity, linux]")
	RunbCmd.MarkFlagRequired("application")
	RunbCmd.MarkFlagRequired("system")
	RunbCmd.MarkFlagRequired("environment")
}

func isDisabled() bool {
	return viper.GetBool("SENHASEGURA_DISABLE_RUNB")
}

func injectEnvironmentVariables(secrets []sdk.Secret) error {
	switch ToolName {
	case "github":
		return injectGithub(secrets)
	case "azure-devops":
		return injectAzureDevops(secrets)
	case "bamboo":
		return injectBamboo(secrets)
	case "bitbucket":
		return injectBitbucket(secrets)
	case "circleci":
		return injectCircleci(secrets)
	case "teamcity":
		return injectTeamcity(secrets)
	case "linux":
		return injectLinux(secrets)

	default:
		return errors.Errorf("Tool '%s' is invalid, it must be one of the following values: github, azure-devops, bamboo, bitbucket, circleci, teamcity or linux", ToolName)
	}
}

func injectGithub(secrets []sdk.Secret) error {
	return inject(secrets, "echo '%s=%s' >> $GITHUB_ENV\n")
}

func injectAzureDevops(secrets []sdk.Secret) error {
	return inject(secrets, "echo '##vso[task.setvariable variable=%s;issecret=true;]%s'\n")
}

func injectBamboo(secrets []sdk.Secret) error {
	return inject(secrets, "(%s)=(.[%s])\n")
}

func injectBitbucket(secrets []sdk.Secret) error {
	return inject(secrets, "export (%s)=\"(.[%s])\"\n")
}

func injectCircleci(secrets []sdk.Secret) error {
	return inject(secrets, "echo '\"'\"'export (%s)=\"(.[%s])\"'\"'\"' >> $BASH_ENV\n")
}

func injectTeamcity(secrets []sdk.Secret) error {
	return inject(secrets, "echo '\"'\"'##teamcity[setParameter name=\"(%s)\" value=\"(.[%s])\"]'\"'\"'\"\n")
}

func injectLinux(secrets []sdk.Secret) error {
	return inject(secrets, "declare -x %s='%s'\n")
}

func inject(secrets []sdk.Secret, format string) error {
	v("Injecting secrets!\n")

	kv = convertJSONToKV(secrets)

	if len(kv) == 0 {
		v("No secrets to be injected!\n")
		return nil
	}

	secretsFile := viper.GetString("SENHASEGURA_SECRETS_FILE")
	if secretsFile == "" {
		secretsFile = ".runb.vars"
	}

	file, err := os.OpenFile(secretsFile, os.O_CREATE|os.O_RDWR, 0660)
	if err != nil {
		return err
	}

	for key, value := range kv {
		v("Injecting secret into %s: %s.....", secretsFile, key)

		_, err = file.WriteString(fmt.Sprintf(format, key, value))
		if err != nil {
			return err
		}

		v("Success!\n")
	}

	file.Close()

	v("Secrets injected!\n")

	return nil
}

func convertJSONToKV(secrets []sdk.Secret) map[string]string {
	kv := make(map[string]string)

	for _, secret := range secrets {
		for _, cloud := range secret.GetCloudCredentials() {

			cloudMap, _ := cloud.ToMap()

			for k, v := range cloudMap {
				kv[k] = v.(string)
			}
		}
		for _, pam := range secret.GetPamCredentials() {

			pamMap, _ := pam.ToMap()

			for k, v := range pamMap {
				kv[k] = v.(string)
			}
		}
		for _, ephemeral := range secret.GetEphemeralCredentials() {

			ephemeralMap, _ := ephemeral.ToMap()

			for k, v := range ephemeralMap {
				kv[k] = v.(string)
			}
		}

		keyValuesMap, _ := secret.GetKeyValues().ToMap()

		for k, v := range keyValuesMap {
			kv[k] = v.(string)
		}
	}

	return kv
}

// func deleteCICDVariables() error {
// 	v("Deleting %s variables...\n", ToolName)

// 	if len(kv) == 0 {
// 		v("No variables to be deleted!\n")
// 		return nil
// 	}

// 	switch ToolName {
// 	case "gitlab":
// 		err := deleteGitLabVars()
// 		if err != nil {
// 			return err
// 		}

// 	case "github":
// 		v("Is not possible to delete %s variables!\n", ToolName)

// 	case "azure-devops":
// 		v("Is not possible to delete %s variables!\n", ToolName)

// 	case "bamboo":
// 		v("Is not possible to delete %s variables!\n", ToolName)

// 	case "bitbucket":
// 		v("Is not possible to delete %s variables!\n", ToolName)

// 	case "circleci":
// 		v("Is not possible to delete %s variables!\n", ToolName)

// 	case "teamcity":
// 		v("Is not possible to delete %s variables!\n", ToolName)

// 	case "linux":
// 		v("Is not possible to delete %s variables!\n", ToolName)

// 	default:
// 		return errors.Errorf("Tool '%s' is invalid, it must be one of the following values: github, azure-devops, bamboo, bitbucket, circleci, teamcity or linux", ToolName)
// 	}

// 	v("Finish\n")

// 	return nil
// }

// func deleteGitLabVars() error {
// 	if !IsSet("GITLAB_ACCESS_TOKEN", "CI_API_V4_URL", "CI_PROJECT_ID") {
// 		v("Deletion failed\n")
// 		v("To delete github variables, you need to define the configs GITLAB_ACCESS_TOKEN, CI_API_V4_URL and CI_PROJECT_ID\n")
// 		return nil
// 	}

// 	if len(kv) == 0 {
// 		v("Deletion failed\n")
// 		v("Has no credentials to exclude variables on '%s' tool ...\n", ToolName)
// 		return nil
// 	}

// 	headers := map[string]string{"PRIVATE-TOKEN": viper.GetString("GITLAB_ACCESS_TOKEN")}

// 	for key := range kv {
// 		v("Deleting %s variable\n", key)

// 		resource := fmt.Sprintf(
// 			"%s/projects/%s/variables/%s",
// 			viper.GetString("CI_API_V4_URL"),
// 			viper.GetString("CI_PROJECT_ID"),
// 			key,
// 		)

// 		_, err := isoSdk.DoRequest(
// 			viper.GetString("GITLAB_ACCESS_TOKEN"),
// 			resource,
// 			url.Values{},
// 			headers,
// 			http.MethodDelete,
// 		)

// 		if err != nil {
// 			v("Failed trying to delete '%s' variable\n", err.Error())
// 			continue
// 		}

// 		v("Deleted\n")
// 	}
// 	return nil
// }

func loadEnvVars() string {
	envVars := strings.Join(os.Environ(), "\n")
	envVars = base64.StdEncoding.EncodeToString([]byte(envVars))
	envVars = replaceSpecials(envVars)
	return envVars
}

func loadMapVars() string {
	if !IsSet("SENHASEGURA_MAPPING_FILE") {
		v("Mapping file not found, proceeding...\n")
	} else {
		v("Using mapping file: %s\n", viper.GetString("SENHASEGURA_MAPPING_FILE"))
	}

	content, err := os.ReadFile(viper.GetString("SENHASEGURA_MAPPING_FILE"))
	if err != nil {
		return ""
	}

	mapVars := string(content)
	mapVars = base64.StdEncoding.EncodeToString([]byte(mapVars))
	mapVars = replaceSpecials(mapVars)
	return mapVars
}
