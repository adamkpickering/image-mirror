package config

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestImageAccumulator(t *testing.T) {
	t.Run("AddImage", func(t *testing.T) {
		t.Run("should not combine identical Images with different DoNotMirror fields", func(t *testing.T) {
			accumulator := NewImageAccumulator()

			image1, err := NewImage("test-org/test-image", []string{"test1"})
			assert.NoError(t, err)
			accumulator.AddImages(image1)

			image2, err := NewImage("test-org/test-image", []string{"test2"})
			assert.NoError(t, err)
			image2.DoNotMirror = true
			accumulator.AddImages(image2)

			imageList := accumulator.Images()

			if !assert.Len(t, imageList, 2) {
				return
			}
			assert.False(t, imageList[0].DoNotMirror)
			assert.True(t, imageList[1].DoNotMirror)
		})
	})
}
