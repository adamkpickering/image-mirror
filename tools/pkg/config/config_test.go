package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImage(t *testing.T) {
	t.Run("TargetImageName", func(t *testing.T) {
		t.Run("should return default when targetImageName is not set", func(t *testing.T) {
			image := Image{
				SpecifiedTargetImageName: "",
				defaultTargetImageName:   "default",
			}
			assert.Equal(t, "default", image.TargetImageName())
		})

		t.Run("should return targetImageName when targetImageName is set", func(t *testing.T) {
			image := Image{
				SpecifiedTargetImageName: "non-default",
				defaultTargetImageName:   "default",
			}
			assert.Equal(t, "non-default", image.TargetImageName())
		})
	})

	t.Run("SetTargetImageName", func(t *testing.T) {
		t.Run(`should set targetImageName to "" when passed value matches default`, func(t *testing.T) {
			image := Image{
				SpecifiedTargetImageName: "non-default",
				defaultTargetImageName:   "default",
			}
			image.SetTargetImageName("default")
			assert.Equal(t, "", image.SpecifiedTargetImageName)
		})

		t.Run(`should set targetImageName to "" when passed value is ""`, func(t *testing.T) {
			image := Image{
				SpecifiedTargetImageName: "non-default",
				defaultTargetImageName:   "default",
			}
			image.SetTargetImageName("")
			assert.Equal(t, "", image.SpecifiedTargetImageName)
		})

		t.Run(`should set targetImageName to value when passed value does not match default`, func(t *testing.T) {
			image := Image{
				SpecifiedTargetImageName: "non-default",
				defaultTargetImageName:   "default",
			}
			image.SetTargetImageName("another-not-default")
			assert.Equal(t, "another-not-default", image.TargetImageName())
		})
	})
}
