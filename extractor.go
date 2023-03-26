package main

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var linkRegex = regexp.MustCompile("https?://\\S+\\.png")

func extractLinks(archive *zip.ReadCloser) (chan string, chan error) {
	links := make(chan string)
	errors := make(chan error)

	go func() {
		for _, file := range archive.File {
			if !strings.HasSuffix(file.Name, "messages.csv") {
				continue
			}

			messageFile, err := file.Open()
			if err != nil {
				errors <- fmt.Errorf("failed to read %s: %w", file.Name, err)
				continue
			}

			reader := csv.NewReader(messageFile)

			// handle csv header
			_, err = reader.Read()
			if err != nil {
				errors <- fmt.Errorf("failed to read %s: %w", file.Name, err)
				continue
			}

			for {
				record, err := reader.Read()
				if err != nil {
					if err != io.EOF {
						errors <- fmt.Errorf("failed to read message in %s: %w", file.Name, err)
					}
					break
				}

				// handle attachments field by splitting on space
				if len(record[3]) > 0 {
					for _, link := range strings.Split(record[3], " ") {
						if strings.HasSuffix(link, ".png") {
							links <- link
						}
					}
				}

				// handle links in text with basic regex matching
				if len(record[2]) > 0 {
					for _, link := range linkRegex.FindAllString(record[2], -1) {
						links <- link
					}
				}
			}
		}

		close(links)
		close(errors)
	}()

	return links, errors
}
