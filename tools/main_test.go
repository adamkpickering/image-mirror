package main

import (
	"testing"

	"github.com/rancher/image-mirror/pkg/config"
	"github.com/rancher/image-mirror/pkg/regsync"
	"github.com/stretchr/testify/assert"
)

func TestGetRegsyncEntries(t *testing.T) {
	type TestCase struct {
		Name                     string
		SpecifiedTargetImageName string
		ExpectedEntries          []regsync.ConfigSync
	}
	for _, testCase := range []TestCase{
		{
			Name:                     "should use default image name when TargetImageName is not set",
			SpecifiedTargetImageName: "",
			ExpectedEntries: []regsync.ConfigSync{
				{
					Source: "test-org/test-image:v1.2.3",
					Target: "docker.io/test1/mirrored-test-org-test-image:v1.2.3",
					Type:   "image",
				},
				{
					Source: "test-org/test-image:v2.3.4",
					Target: "docker.io/test1/mirrored-test-org-test-image:v2.3.4",
					Type:   "image",
				},
			},
		},
		{
			Name:                     "should use TargetImageName when it is set",
			SpecifiedTargetImageName: "other-org-test-image",
			ExpectedEntries: []regsync.ConfigSync{
				{
					Source: "test-org/test-image:v1.2.3",
					Target: "docker.io/test1/other-org-test-image:v1.2.3",
					Type:   "image",
				},
				{
					Source: "test-org/test-image:v2.3.4",
					Target: "docker.io/test1/other-org-test-image:v2.3.4",
					Type:   "image",
				},
			},
		},
	} {
		t.Run(testCase.Name, func(t *testing.T) {
			inputImage, err := config.NewImage("test-org/test-image", []string{
				"v1.2.3",
				"v2.3.4",
			})
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			inputImage.SetTargetImageName(testCase.SpecifiedTargetImageName)
			inputRepository := config.Repository{
				EnvVarPrefix: "TEST1",
				BaseUrl:      "docker.io/test1",
			}
			regsyncEntries, err := convertConfigImageToRegsyncImages(inputRepository, inputImage)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			assert.Len(t, regsyncEntries, len(testCase.ExpectedEntries))
			for _, expectedEntry := range testCase.ExpectedEntries {
				assert.Contains(t, regsyncEntries, expectedEntry)
			}
		})
	}
}
