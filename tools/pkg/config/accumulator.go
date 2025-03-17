package config

import (
	"slices"
)

type sourceTargetPair struct {
	SourceImage     string
	TargetImageName string
}

type ImageAccumulator struct {
	mapping map[sourceTargetPair]*Image
}

func NewImageAccumulator() *ImageAccumulator {
	return &ImageAccumulator{
		mapping: map[sourceTargetPair]*Image{},
	}
}

func (ia *ImageAccumulator) AddImage(newImage *Image) {
	pair := sourceTargetPair{
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

func (ia *ImageAccumulator) Images() []*Image {
	images := make([]*Image, 0, len(ia.mapping))
	for _, image := range ia.mapping {
		images = append(images, image)
	}
	return images
}
