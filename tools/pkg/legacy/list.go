package legacy

import (
	"fmt"
	"os"
	"strings"
)

type ImagesListEntry struct {
	Source      string
	Destination string
	Tag         string
}

func ParseImagesList(fileName string) ([]ImagesListEntry, error) {
	contents, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read: %s", err)
	}

	lines := strings.Split(string(contents), "\n")
	imagesList := make([]ImagesListEntry, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		elements := strings.Split(line, " ")
		if len(elements) != 3 {
			return nil, fmt.Errorf("line %q does not have 3 elements", line)
		}
		imagesListEntry := ImagesListEntry{
			Source:      elements[0],
			Destination: elements[1],
			Tag:         elements[2],
		}
		imagesList = append(imagesList, imagesListEntry)
	}

	return imagesList, nil
}
