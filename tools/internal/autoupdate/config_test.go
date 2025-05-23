package autoupdate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigEntry(t *testing.T) {
	t.Run("Validate", func(t *testing.T) {
		type testCase struct {
			Message       string
			ConfigEntry   ConfigEntry
			ExpectedError string
		}
		testCases := []testCase{
			{
				Message: "should return nil for a valid ConfigEntry",
				ConfigEntry: ConfigEntry{
					Name: "test-entry",
					GithubLatestRelease: &GithubLatestRelease{
						Owner:      "test-owner",
						Repository: "test-repo",
						Images:     []string{"rancher/rancher"},
					},
				},
				ExpectedError: "",
			},
			{
				Message: "should return error when Name is not present",
				ConfigEntry: ConfigEntry{
					Name: "",
					GithubLatestRelease: &GithubLatestRelease{
						Owner:      "test-owner",
						Repository: "test-repo",
						Images:     []string{"rancher/rancher"},
					},
				},
				ExpectedError: "must specify Name",
			},
			{
				Message: "should return error when no autoupdate strategy is present",
				ConfigEntry: ConfigEntry{
					Name: "test-entry",
				},
				ExpectedError: "must specify an autoupdate strategy",
			},
		}
		for _, testCase := range testCases {
			t.Run(testCase.Message, func(t *testing.T) {
				err := testCase.ConfigEntry.Validate()
				if testCase.ExpectedError == "" {
					assert.Nil(t, err)
				} else {
					assert.EqualError(t, err, testCase.ExpectedError)
				}
			})
		}
	})
}
