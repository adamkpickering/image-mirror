package config

import (
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

func (ia *ImageAccumulator) Images() []*Image {
	images := make([]*Image, 0, len(ia.mapping))
	for _, image := range ia.mapping {
		images = append(images, image)
	}
	return images
}
