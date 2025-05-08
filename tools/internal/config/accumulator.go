package config

import (
	"fmt"
	"slices"
)

type imageIndex struct {
	DoNotMirror     bool
	SourceImage     string
	TargetImageName string
}

type ImageAccumulator struct {
	mapping map[imageIndex]*Image
}

func NewImageAccumulator() *ImageAccumulator {
	return &ImageAccumulator{
		mapping: map[imageIndex]*Image{},
	}
}

func (ia *ImageAccumulator) AddImages(newImages ...*Image) {
	for _, newImage := range newImages {
		pair := imageIndex{
			DoNotMirror:     newImage.DoNotMirror,
			SourceImage:     newImage.SourceImage,
			TargetImageName: newImage.TargetImageName(),
		}
		existingImage, ok := ia.mapping[pair]
		if !ok {
			ia.mapping[pair] = newImage
		} else {
			for _, newTag := range newImage.Tags {
				if !slices.Contains(existingImage.Tags, newTag) {
					existingImage.Tags = append(existingImage.Tags, newTag)
				}
			}
			ia.mapping[pair] = existingImage
		}
	}
}

// Contains tells the caller whether the ImageAccumulator contains
// image. image is assumed to have a Tags element of length 1.
func (ia *ImageAccumulator) Contains(image *Image) bool {
	if len(image.Tags) > 1 {
		fmt.Printf("Warning: passed image %q contains multiple tags: %s\n", image.SourceImage, image.Tags)
	}
	pair := imageIndex{
		DoNotMirror:     image.DoNotMirror,
		SourceImage:     image.SourceImage,
		TargetImageName: image.TargetImageName(),
	}
	foundImage, ok := ia.mapping[pair]
	if !ok {
		return false
	}
	return slices.Contains(foundImage.Tags, image.Tags[0])
}

func (ia *ImageAccumulator) Images() []*Image {
	images := make([]*Image, 0, len(ia.mapping))
	for _, image := range ia.mapping {
		images = append(images, image)
	}
	return images
}
