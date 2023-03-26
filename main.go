package main

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"discropalypse/acropalypse"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/schollz/progressbar/v3"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var packageFile = flag.String("package", "", "path to your discord data dump")
var concurrency = flag.Uint("threads", 8, "number of concurrent downloads")
var logFile = flag.String("log", "discropalypse.log", "location to store download error logs")
var output = flag.String("output", "./download", "location to store the downloaded and recovered images")
var device = flag.String("device", "", "either one of [p3, p3xl, p3a, p3axl, p4, p4xl, p4a, p5, p5a, p6, p6pro, p6a, p7, p7pro] or a custom resolution e.g. 1920x1080")

func scrape(m *acropalypse.Acropalypse, link string) error {
	hasher := sha256.New()
	hasher.Write([]byte(link))

	linkHash := hex.EncodeToString(hasher.Sum(nil))
	if _, err := os.Stat(path.Join(*output, linkHash+"-recovered.png")); err == nil {
		log.Printf("skipping %s because recovered image already exists\n", link)
		return nil
	}

	res, err := http.Get(link)
	if err != nil {
		return fmt.Errorf("failed to get image: %w", err)
	}

	defer res.Body.Close()

	reader := bufio.NewReader(res.Body)
	header, err := reader.Peek(24)

	if err != nil {
		return fmt.Errorf("could not read png header: %w", err)
	}

	width, height, err := readDimensions(header)
	if err != nil {
		return fmt.Errorf("failed to read image dimensions: %w", err)
	}

	if width > m.Width || height > m.Height {
		return nil
	}

	image, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to get full image: %w", err)
	}

	recovered, err := m.Recover(context.Background(), image)
	if err != nil {
		return fmt.Errorf("failed to recover image with url %s: %w", link, err)
	}

	if recovered == nil {
		return nil
	}

	err = os.WriteFile(path.Join(*output, linkHash+".png"), image, 0644)
	if err != nil {
		return fmt.Errorf("failed to write image to disk: %w", err)
	}

	err = os.WriteFile(path.Join(*output, linkHash+"-recovered.png"), recovered, 0644)
	if err != nil {
		return fmt.Errorf("failed to write recovered image to disk: %w", err)
	}

	return nil
}

func getResolution(device string) (uint32, uint32) {
	if device == "p3" || device == "p4a" || device == "p5a" || device == "p6" || device == "p6a" || device == "p6pro" || device == "p7pro" {
		return 1440, 3120
	} else if device == "p3axl" {
		return 1080, 2160
	} else if device == "p3xl" {
		return 1440, 2960
	} else if device == "p3a" {
		return 1080, 2220
	} else if device == "p4" {
		return 1080, 2280
	} else if device == "p4xl" {
		return 1440, 3040
	} else if device == "p5" {
		return 1080, 2340
	} else if device == "p7" {
		return 1080, 2400
	}

	match, err := regexp.MatchString("^\\d+x\\d+$", device)
	if err != nil || !match {
		return 0, 0
	}
	res := strings.Split(device, "x")
	width, _ := strconv.Atoi(res[0])
	height, _ := strconv.Atoi(res[1])

	return uint32(width), uint32(height)
}

func main() {
	flag.Parse()

	usage := func() {
		_, _ = fmt.Fprintln(os.Stderr, "Usage: discropalypse")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if len(*packageFile) == 0 {
		_, _ = fmt.Fprintf(os.Stderr, "You must provide a data dump!\n\n")
		usage()
	}

	if len(*device) == 0 {
		_, _ = fmt.Fprintf(os.Stderr, "You must provide a device!\n\n")
		usage()
	}

	width, height := getResolution(*device)
	if width == 0 {
		_, _ = fmt.Fprintf(os.Stderr, "Invalid device or custom resolution!\n\n")
		usage()
	}

	fmt.Println("fetching acropalypse from jsdelivr")
	wasm, err := acropalypse.Fetch()
	if err != nil {
		panic(fmt.Errorf("failed to init acropalypse: %w", err))
	}
	fmt.Println("fetched acropalypse")

	archive, err := zip.OpenReader(*packageFile)
	if err != nil {
		panic(fmt.Errorf("failed to open archive: %w", err))
	}

	err = os.MkdirAll(*output, 0775)
	if err != nil {
		panic(fmt.Errorf("failed to create output directory: %w", err))
	}

	logStream, err := os.OpenFile(*logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(fmt.Errorf("failed to open log file: %w", err))
	}
	defer logStream.Close()
	log.SetOutput(logStream)

	fmt.Println("scraping...")
	links, errors := extractLinks(archive)
	progress := progressbar.Default(-1)

	go func() {
		for {
			err, ok := <-errors
			if !ok {
				return
			}
			log.Println("error in parsing discord data dump", err)
		}
	}()

	var wg sync.WaitGroup
	wg.Add(int(*concurrency))

	thread := func() {
		defer wg.Done()

		for {
			m, err := acropalypse.Init(context.Background(), wasm, width, height)
			if err != nil {
				panic(fmt.Errorf("failed to create acropalypse: %w", err))
			}
			link, ok := <-links
			if !ok {
				return
			}
			err = scrape(m, link)
			if err != nil {
				log.Println("failed to scrape", link, err)
			}
			_ = progress.Add(1)
			_ = m.Close(context.Background())
		}
	}

	for i := uint(0); i < *concurrency; i++ {
		go thread()
	}

	wg.Wait()

	_ = progress.Finish()
}