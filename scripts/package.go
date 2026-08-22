//go:build ignore

package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type TargetPlatform struct {
	OS        string
	Arch      string
	BinaryExt string
}

var platforms = []TargetPlatform{
	{OS: "windows", Arch: "amd64", BinaryExt: ".exe"},
	{OS: "linux", Arch: "amd64", BinaryExt: ""},
	{OS: "darwin", Arch: "amd64", BinaryExt: ""},
	{OS: "darwin", Arch: "arm64", BinaryExt: ""},
}

func main() {
	var version string
	var commit string
	var buildDate string

	flag.StringVar(&version, "version", "v0.1.0-dev", "Release version string")
	flag.StringVar(&commit, "commit", "", "Git commit SHA")
	flag.StringVar(&buildDate, "build-date", "", "Build date UTC")
	flag.Parse()

	if commit == "" {
		commit = getGitCommit()
	}
	if buildDate == "" {
		buildDate = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	}

	distDir := "dist"
	binDir := filepath.Join(distDir, "bin")

	if err := os.MkdirAll(binDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", binDir, err)
		os.Exit(1)
	}

	ldflags := fmt.Sprintf("-s -w -X github.com/charleszardd/daegsa/internal/cli.Version=%s -X github.com/charleszardd/daegsa/internal/cli.Commit=%s -X github.com/charleszardd/daegsa/internal/cli.BuildDate=%s -X github.com/charleszardd/daegsa/internal/report.DefaultDaegsaVersion=%s -X github.com/charleszardd/daegsa/internal/report.DefaultCommit=%s -X github.com/charleszardd/daegsa/internal/report.DefaultBuildDate=%s",
		version, commit, buildDate, version, commit, buildDate)

	fmt.Printf("Building release binaries for %s (commit: %s, date: %s)...\n", version, commit, buildDate)

	for _, p := range platforms {
		binName := fmt.Sprintf("daegsa-%s-%s%s", p.OS, p.Arch, p.BinaryExt)
		outPath := filepath.Join(binDir, binName)

		fmt.Printf("  -> Building %s/%s -> %s\n", p.OS, p.Arch, outPath)

		cmd := exec.Command("go", "build", "-trimpath", "-ldflags", ldflags, "-o", outPath, "./cmd/daegsa")
		cmd.Env = append(os.Environ(), "GOOS="+p.OS, "GOARCH="+p.Arch, "CGO_ENABLED=0")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Build failed for %s/%s: %v\n", p.OS, p.Arch, err)
			os.Exit(1)
		}

		// Also copy windows/amd64 directly into dist/daegsa.exe for easy invocation
		if p.OS == "windows" && p.Arch == "amd64" {
			standalonePath := filepath.Join(distDir, "daegsa.exe")
			if err := copyFile(outPath, standalonePath); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to copy standalone binary: %v\n", err)
			}
		}
		if p.OS == runtime.GOOS && p.Arch == runtime.GOARCH && p.OS != "windows" {
			standalonePath := filepath.Join(distDir, "daegsa")
			if err := copyFile(outPath, standalonePath); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to copy standalone host binary: %v\n", err)
			}
		}
	}

	// Create archives (.zip for windows, .tar.gz for unix)
	fmt.Println("Packaging release archives...")
	for _, p := range platforms {
		binName := fmt.Sprintf("daegsa-%s-%s%s", p.OS, p.Arch, p.BinaryExt)
		srcBinPath := filepath.Join(binDir, binName)
		archiveBase := fmt.Sprintf("daegsa-%s-%s-%s", version, p.OS, p.Arch)

		if p.OS == "windows" {
			zipPath := filepath.Join(distDir, archiveBase+".zip")
			if err := createZipArchive(zipPath, srcBinPath, "daegsa.exe"); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create %s: %v\n", zipPath, err)
				os.Exit(1)
			}
			fmt.Printf("  -> Created %s\n", zipPath)
		} else {
			tarPath := filepath.Join(distDir, archiveBase+".tar.gz")
			if err := createTarGzArchive(tarPath, srcBinPath, "daegsa"); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create %s: %v\n", tarPath, err)
				os.Exit(1)
			}
			fmt.Printf("  -> Created %s\n", tarPath)
		}
	}

	// Generate SHA256SUMS
	fmt.Println("Generating SHA256SUMS...")
	if err := generateChecksums(distDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate checksums: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Release packaging complete.")
}

func getGitCommit() string {
	cmd := exec.Command("git", "rev-parse", "--short=12", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func createZipArchive(zipPath, binPath, binNameInArchive string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	// Add executable
	if err := addFileToZip(w, binPath, binNameInArchive); err != nil {
		return err
	}

	// Add README and examples if they exist
	if fileExists("README.md") {
		_ = addFileToZip(w, "README.md", "README.md")
	}
	_ = addDirToZip(w, "examples", "examples")

	return nil
}

func addFileToZip(w *zip.Writer, srcPath, destPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(destPath)
	header.Method = zip.Deflate

	writer, err := w.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, srcFile)
	return err
}

func addDirToZip(w *zip.Writer, srcDir, destDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		return addFileToZip(w, path, filepath.Join(destDir, rel))
	})
}

func createTarGzArchive(tarPath, binPath, binNameInArchive string) error {
	tarFile, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	gw := gzip.NewWriter(tarFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	if err := addFileToTar(tw, binPath, binNameInArchive, 0755); err != nil {
		return err
	}

	if fileExists("README.md") {
		_ = addFileToTar(tw, "README.md", "README.md", 0644)
	}
	_ = addDirToTar(tw, "examples", "examples")

	return nil
}

func addFileToTar(tw *tar.Writer, srcPath, destPath string, mode int64) error {
	file, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name:    filepath.ToSlash(destPath),
		Size:    info.Size(),
		Mode:    mode,
		ModTime: info.ModTime(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	_, err = io.Copy(tw, file)
	return err
}

func addDirToTar(tw *tar.Writer, srcDir, destDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		return addFileToTar(tw, path, filepath.Join(destDir, rel), 0644)
	})
}

func generateChecksums(distDir string) error {
	entries, err := os.ReadDir(distDir)
	if err != nil {
		return err
	}

	var sums []string
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "SHA256SUMS" || entry.Name() == "sbom-cyclonedx.json" {
			continue
		}

		filePath := filepath.Join(distDir, entry.Name())
		hash, err := computeSHA256(filePath)
		if err != nil {
			return err
		}
		sums = append(sums, fmt.Sprintf("%s  %s", hash, entry.Name()))
	}

	sort.Strings(sums)
	sumsContent := strings.Join(sums, "\n") + "\n"

	sumsPath := filepath.Join(distDir, "SHA256SUMS")
	return os.WriteFile(sumsPath, []byte(sumsContent), 0644)
}

func computeSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
