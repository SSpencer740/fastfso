package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	arProject  = "fastfso"
	arLocation = "us-east4"
	arRepo     = "fastfso"
	arRegistry = arLocation + "-docker.pkg.dev/" + arProject + "/" + arRepo
)

type arImage struct {
	Package    string     `json:"package"`
	Version    string     `json:"version"`
	CreateTime string     `json:"createTime"`
	UpdateTime string     `json:"updateTime"`
	Tags       []string   `json:"tags"`
	Metadata   arMetadata `json:"metadata"`
}

type arMetadata struct {
	ImageSizeBytes string `json:"imageSizeBytes"`
}

func cmdRegistry() {
	step("Fetching Artifact Registry stats...")
	info(fmt.Sprintf("registry: %s", arRegistry))
	fmt.Println()

	images, err := listARImages()
	if err != nil {
		fail(fmt.Sprintf("gcloud failed: %v", err))
		os.Exit(1)
	}

	if len(images) == 0 {
		warn("no images found in registry")
		return
	}

	// Compute stats.
	var totalBytes int64
	packages := map[string]int{}
	for _, img := range images {
		var size int64
		fmt.Sscanf(img.Metadata.ImageSizeBytes, "%d", &size)
		totalBytes += size
		packages[img.Package]++
	}

	// Summary.
	fmt.Printf("  %-18s %s\n",
		colorize(ansiBold, "Images:"),
		colorize(ansiCyan, fmt.Sprintf("%d", len(images))))
	fmt.Printf("  %-18s %s\n",
		colorize(ansiBold, "Total size:"),
		colorize(ansiCyan, humanBytes(totalBytes)))
	fmt.Printf("  %-18s %s\n",
		colorize(ansiBold, "Packages:"),
		colorize(ansiCyan, fmt.Sprintf("%d", len(packages))))
	fmt.Println()

	// Per-package breakdown.
	type pkgStat struct {
		name  string
		count int
		bytes int64
	}
	var stats []pkgStat
	for pkg := range packages {
		var b int64
		var c int
		for _, img := range images {
			if img.Package == pkg {
				c++
				var size int64
				fmt.Sscanf(img.Metadata.ImageSizeBytes, "%d", &size)
				b += size
			}
		}
		stats = append(stats, pkgStat{name: pkg, count: c, bytes: b})
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].bytes > stats[j].bytes })

	fmt.Println(colorize(ansiBold, "  Per-package breakdown:"))
	for _, s := range stats {
		shortName := s.name
		if idx := strings.LastIndex(s.name, "/"); idx >= 0 {
			shortName = s.name[idx+1:]
		}
		fmt.Printf("    %-20s %4d images   %s\n",
			colorize(ansiCyan, shortName),
			s.count,
			humanBytes(s.bytes))
	}
	fmt.Println()

	// Most recent images.
	sort.Slice(images, func(i, j int) bool { return images[i].CreateTime > images[j].CreateTime })
	n := 10
	if len(images) < n {
		n = len(images)
	}
	fmt.Println(colorize(ansiBold, fmt.Sprintf("  %d most recent images:", n)))
	for _, img := range images[:n] {
		tag := strings.Join(img.Tags, ", ")
		if tag == "" {
			tag = img.Version
			if len(tag) > 12 {
				tag = tag[:12]
			}
		}
		shortPkg := img.Package
		if idx := strings.LastIndex(shortPkg, "/"); idx >= 0 {
			shortPkg = shortPkg[idx+1:]
		}
		var size int64
		fmt.Sscanf(img.Metadata.ImageSizeBytes, "%d", &size)
		ts := img.CreateTime
		if len(ts) > 19 {
			ts = ts[:19]
		}
		fmt.Printf("    %s  %-14s  %-20s  %s\n",
			colorize(ansiDim, ts),
			colorize(ansiCyan, shortPkg),
			tag,
			humanBytes(size))
	}

	fmt.Println()
	success("done")
}

func listARImages() ([]arImage, error) {
	var buf bytes.Buffer
	var errBuf bytes.Buffer
	cmd := exec.Command("gcloud", "artifacts", "docker", "images", "list",
		arRegistry,
		"--include-tags",
		"--format=json",
		"--project="+arProject,
	)
	cmd.Stdout = &buf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%v: %s", err, errBuf.String())
	}

	var images []arImage
	if err := json.Unmarshal(buf.Bytes(), &images); err != nil {
		return nil, fmt.Errorf("parsing gcloud output: %w", err)
	}
	return images, nil
}

// protectedTags are tags that indicate an image is actively deployed.
var protectedTags = map[string]bool{
	"latest":  true,
	"staging": true,
	"prod":    true,
}

func cmdRegistryPrune() {
	step("Fetching Artifact Registry images...")
	info(fmt.Sprintf("registry: %s", arRegistry))
	fmt.Println()

	images, err := listARImages()
	if err != nil {
		fail(fmt.Sprintf("gcloud failed: %v", err))
		os.Exit(1)
	}

	cutoff := time.Now().AddDate(0, 0, -30)

	var toPrune []arImage
	var skippedTagged int
	for _, img := range images {
		created, err := time.Parse(time.RFC3339, img.CreateTime)
		if err != nil {
			warn(fmt.Sprintf("skipping image with unparseable time %q: %v", img.CreateTime, err))
			continue
		}
		if created.After(cutoff) {
			continue
		}

		// Never delete images with protected tags.
		protected := false
		for _, tag := range img.Tags {
			if protectedTags[tag] {
				protected = true
				break
			}
		}
		if protected {
			skippedTagged++
			continue
		}

		toPrune = append(toPrune, img)
	}

	if len(toPrune) == 0 {
		success("no images older than 30 days to prune")
		if skippedTagged > 0 {
			info(fmt.Sprintf("(%d protected images skipped)", skippedTagged))
		}
		return
	}

	// Show what will be deleted.
	var totalBytes int64
	for _, img := range toPrune {
		var size int64
		fmt.Sscanf(img.Metadata.ImageSizeBytes, "%d", &size)
		totalBytes += size
	}

	fmt.Printf("  Found %s to prune (%s)\n",
		colorize(ansiBold, fmt.Sprintf("%d images", len(toPrune))),
		colorize(ansiCyan, humanBytes(totalBytes)))
	if skippedTagged > 0 {
		info(fmt.Sprintf("(%d protected images skipped)", skippedTagged))
	}
	fmt.Println()

	// Confirmation prompt.
	fmt.Printf("  %s ", colorize(ansiYellow+ansiBold, "Delete these images? [y/N]"))
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer != "y" && answer != "yes" {
		warn("aborted")
		return
	}
	fmt.Println()

	// Delete with a fixed pool of 8 workers. Ctrl+C lets in-flight
	// deletes finish but cancels remaining work.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	const maxWorkers = 8
	work := make(chan arImage, len(toPrune))
	for _, img := range toPrune {
		work <- img
	}
	close(work)

	type result struct {
		img arImage
		err error
	}
	results := make(chan result, len(toPrune))

	var wg sync.WaitGroup
	for range min(maxWorkers, len(toPrune)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for img := range work {
				if ctx.Err() != nil {
					return
				}
				ref := img.Package + "@" + img.Version
				results <- result{img: img, err: deleteARImage(ref)}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	var deleted, failed int
	for r := range results {
		if r.err != nil {
			failed++
			ref := r.img.Package + "@" + r.img.Version
			fail(fmt.Sprintf("delete %s: %v", ref, r.err))
		} else {
			deleted++
			shortPkg := r.img.Package
			if idx := strings.LastIndex(shortPkg, "/"); idx >= 0 {
				shortPkg = shortPkg[idx+1:]
			}
			ver := r.img.Version
			if len(ver) > 12 {
				ver = ver[:12]
			}
			success(fmt.Sprintf("deleted %s@%s", shortPkg, ver))
		}
	}

	remaining := len(toPrune) - deleted - failed
	fmt.Println()
	if remaining > 0 {
		warn(fmt.Sprintf("interrupted: deleted %d images, %d skipped", deleted, remaining))
	} else if failed > 0 {
		warn(fmt.Sprintf("deleted %d images, %d failed", deleted, failed))
	} else {
		success(fmt.Sprintf("deleted %d images (%s freed)", deleted, humanBytes(totalBytes)))
	}
}

func deleteARImage(ref string) error {
	var errBuf bytes.Buffer
	cmd := exec.Command("gcloud", "artifacts", "docker", "images", "delete",
		ref,
		"--delete-tags",
		"--quiet",
		"--project="+arProject,
	)
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(errBuf.String()))
	}
	return nil
}

func humanBytes(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
