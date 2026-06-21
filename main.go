package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/ini.v1"
)

const defaultIni = `[build]
source = assets
target = public

[dependencies]

[sass]
version = 1.79.2
entrypoints = 
`

var v = "1.0.0"
var httpClient = &http.Client{Timeout: 30 * time.Second}

func loadConfig() (string, string, []string, map[string][]string, string, []string) {
	var src, dest string
	var ignores []string
	var deps map[string][]string
	var sassVersion string
	var sassEps []string

	filename := "packly.ini"
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		_ = os.WriteFile(filename, []byte(defaultIni), 0644)
		fmt.Println("Created default packly.ini configuration file.")
	}

	cfg, err := ini.Load(filename)
	if err != nil {
		log.Fatalf("Failed to load packly.ini: %v", err)
	}

	buildSec := cfg.Section("build")
	src = buildSec.Key("source").MustString("assets")
	target := buildSec.Key("target").MustString("public")
	dest = filepath.Join(target, "build")
	ignorePatterns := buildSec.Key("ignore").MustString("")
	ignores = []string{}
	if ignorePatterns != "" {
		for _, pattern := range strings.Split(ignorePatterns, ",") {
			ignores = append(ignores, strings.TrimSpace(pattern))
		}
	}

	deps = make(map[string][]string)
	for _, name := range cfg.Section("dependencies").KeyStrings() {
		urlStr := cfg.Section("dependencies").Key(name).String()
		parts := strings.Split(urlStr, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		deps[name] = parts
	}

	sassSec := cfg.Section("sass")
	sassVersion = sassSec.Key("version").String()
	if sassSec.HasKey("entrypoints") {
		eps := sassSec.Key("entrypoints").String()
		for _, ep := range strings.Split(eps, ",") {
			sassEps = append(sassEps, strings.TrimSpace(ep))
		}
	}

	return src, dest, ignores, deps, sassVersion, sassEps
}

func copyFile(src, dst string) error {

	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func md5sum(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

func downloadFile(urlStr string) (string, error) {
	resp, err := httpClient.Get(urlStr)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "packly-*")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	_, err = io.Copy(tmp, resp.Body)
	return tmp.Name(), err
}

func unzip(source, destination string) error {
	r, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer r.Close()

	var root string
	if len(r.File) > 0 && r.File[0].FileInfo().IsDir() {
		root = r.File[0].Name
	}

	for _, f := range r.File {
		p := f.Name
		if root != "" && strings.HasPrefix(p, root) {
			p = strings.TrimPrefix(p, root)
		}
		if p == "" || isIgnored(p) {
			continue
		}
		outPath := filepath.Join(destination, p)

		if f.FileInfo().IsDir() {
			os.MkdirAll(outPath, 0755)
			continue
		}

		os.MkdirAll(filepath.Dir(outPath), 0755)
		out, err := os.Create(outPath)
		if err != nil {
			return err
		}
		in, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}
		_, err = io.Copy(out, in)
		out.Close()
		in.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func untar(source, destination string) error {
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		outPath := filepath.Join(destination, header.Name)
		info := header.FileInfo()

		if info.IsDir() {
			os.MkdirAll(outPath, 0755)
			continue
		}

		os.MkdirAll(filepath.Dir(outPath), 0755)
		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		_, err = io.Copy(out, tarReader)
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func isIgnored(p string) bool {
	p = strings.ToLower(filepath.ToSlash(p))
	ignores := []string{"/docs/", "/site/", "/tests/", "/test/", "/demo/", "/samples/", ".github", ".travis.yml"}
	for _, ig := range ignores {
		if strings.Contains("/"+p+"/", ig) {
			return true
		}
	}
	return false
}

func installDependencies(source string, deps map[string][]string, force bool) {
	vendor := filepath.Join(source, "vendor")
	_ = os.MkdirAll(vendor, 0755)

	gitIgnore := filepath.Join(vendor, ".gitignore")
	if _, err := os.Stat(gitIgnore); os.IsNotExist(err) {
		_ = os.WriteFile(gitIgnore, []byte("*"), 0644)
	}

	fmt.Println("Installing dependencies...")

	if force {
		fmt.Println("Force mode enabled. Reinstalling all dependencies...")
	}

	// Sort dependency names for deterministic and organized output
	var sortedNames []string
	for name := range deps {
		sortedNames = append(sortedNames, name)
	}
	sort.Strings(sortedNames)

	for i, name := range sortedNames {
		urlList := deps[name]
		target := filepath.Join(vendor, name)

		if force {
			if _, err := os.Stat(target); err == nil {
				_ = os.RemoveAll(target)
			}
		}

		fmt.Printf("\n📦 [%d/%d] %s\n", i+1, len(sortedNames), name)

		for _, urlStr := range urlList {
			marker := filepath.Join(target, fmt.Sprintf(".%s_installed", md5sum(urlStr)))

			if _, err := os.Stat(marker); err == nil {
				fmt.Printf("   ✓ %s (already installed)\n", urlStr)
				continue
			}

			fmt.Printf("   → Downloading %s...\n", urlStr)
			tmp, err := downloadFile(urlStr)
			if err != nil {
				log.Fatalf("Failed to download '%s': %v", urlStr, err)
			}

			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					os.Remove(tmp)
					log.Fatalf("Failed to create directory '%s': %v", target, err)
				}
			}

			if strings.HasSuffix(strings.ToLower(urlStr), ".zip") {
				if err := unzip(tmp, target); err != nil {
					os.Remove(tmp)
					log.Fatalf("Failed to unzip '%s': %v", urlStr, err)
				}
			} else {
				filename := path.Base(urlStr)
				if idx := strings.Index(filename, "?"); idx != -1 {
					filename = filename[:idx]
				}

				if err := copyFile(tmp, filepath.Join(target, filename)); err != nil {
					os.Remove(tmp)
					log.Fatalf("Failed to copy file for '%s': %v", urlStr, err)
				}
			}
			_ = os.WriteFile(marker, []byte(urlStr), 0644)
			_ = os.Remove(tmp)
			fmt.Printf("   ✓ Successfully installed.\n")
		}
	}
	fmt.Println("\nAll dependencies processed.")
}

func compileSass(src string, dest string, sassVersion string, entrypoints []string) {
	if len(entrypoints) == 0 {
		return
	}

	binaryName := "sass"
	if runtime.GOOS == "windows" {
		binaryName = "sass.bat"
	}
	binaryPath := filepath.Join(Getcwd(), ".sass", "dart-sass", binaryName)

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		fmt.Println("Sass binary not found. Downloading...")

		arch := "linux-x64"
		if runtime.GOOS == "windows" {
			arch = "windows-x64"
		}
		if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
			arch = "macos-arm64"
		}

		binaryURL := fmt.Sprintf("https://github.com/sass/dart-sass/releases/download/%s/dart-sass-%s-%s.tar.gz", sassVersion, sassVersion, arch)

		tmp, err := downloadFile(binaryURL)
		if err != nil {
			log.Fatalf("Failed to download Sass: %v", err)
		}
		defer os.Remove(tmp)

		target := filepath.Join(Getcwd(), ".sass")
		_ = os.MkdirAll(target, 0755)

		gitIgnore := filepath.Join(target, ".gitignore")
		if _, err := os.Stat(gitIgnore); os.IsNotExist(err) {
			_ = os.WriteFile(gitIgnore, []byte("*"), 0644)
		}

		if strings.HasSuffix(strings.ToLower(binaryURL), ".zip") {
			if err := unzip(tmp, target); err != nil {
				log.Fatalf("Failed to unzip Sass: %v", err)
			}
		} else if strings.HasSuffix(strings.ToLower(binaryURL), ".tar.gz") {
			if err := untar(tmp, target); err != nil {
				log.Fatalf("Failed to untar Sass: %v", err)
			}
		} else {
			log.Fatalf("Unsupported archive format for Sass: %s", binaryURL)
		}

		if runtime.GOOS != "windows" {
			_ = os.Chmod(binaryPath, 0755)
		}
		fmt.Println("Sass binary installed successfully.")
	}

	for _, ep := range entrypoints {
		inputPath := filepath.Join(src, ep)
		if _, err := os.Stat(inputPath); os.IsNotExist(err) {
			fmt.Printf("Warning: Sass entrypoint not found: %s\n", inputPath)
			continue
		}

		outputPath := filepath.Join(dest, strings.TrimSuffix(ep, filepath.Ext(ep))+".css")
		_ = os.MkdirAll(filepath.Dir(outputPath), 0755)
		fmt.Printf("Compiling Sass: %s -> %s\n", inputPath, outputPath)

		cmd := exec.Command(binaryPath, "--style=compressed", "--no-source-map", inputPath, outputPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Fatalf("Sass compilation failed for %s: %v\nOutput:\n%s", inputPath, err, string(output))
		}
	}
}

func build(src string, dest string, ignores []string, sassVersion string, sassEps []string) {
	fmt.Printf("Building assets from '%s' to '%s'...\n", src, dest)
	_ = os.RemoveAll(dest)

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".scss" || ext == ".sass" {
			return nil
		}

		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return nil
		}

		if strings.HasPrefix(filepath.Base(path), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		for _, ignore := range ignores {
			normRel := filepath.ToSlash(rel)
			normIgnore := strings.TrimSuffix(filepath.ToSlash(ignore), "/")
			if normRel == normIgnore || strings.HasPrefix(normRel, normIgnore+"/") {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		outPath := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(outPath, info.Mode())
		}
		return copyFile(path, outPath)
	})
	if err != nil {
		log.Fatalf("Build failed: %v", err)
	}

	compileSass(src, dest, sassVersion, sassEps)
	fmt.Println("Build completed successfully.")
}

// N'oublie pas d'importer "time" et "sync" au début de ton fichier.

func watch(src, dest string, ignores []string, sassURL string, sassEps []string) {
	watcher, _ := fsnotify.NewWatcher()
	defer watcher.Close()

	// 1. Fonction courte pour surveiller les dossiers (et ignorer vendor/ignores)
	watchFolders := func(root string) {
		filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {

			if d.IsDir() && d.Name() == "vendor" {
				return filepath.SkipDir
			}
			if err != nil || !d.IsDir() {
				return nil
			}
			for _, ig := range ignores {
				if strings.Contains(p, ig) {
					return nil
				}
			}
			watcher.Add(p)
			return nil
		})
	}

	watchFolders(src) // Scanner les dossiers existants
	fmt.Println("Watching for changes in", src)

	var timer *time.Timer
	var mu sync.Mutex

	for event := range watcher.Events {

		if strings.HasSuffix(event.Name, "~") || strings.HasPrefix(filepath.Base(event.Name), ".") {
			continue
		}

		if stat, err := os.Stat(event.Name); err == nil && stat.IsDir() {
			watchFolders(event.Name)
		}

		if timer != nil {
			timer.Stop()
		}

		timer = time.AfterFunc(300*time.Millisecond, func() {
			mu.Lock()
			defer mu.Unlock()
			build(src, dest, ignores, sassURL, sassEps)
		})
	}
}

func printUsage() {
	fmt.Println("Usage: packly <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  install              Install all dependencies listed in the configuration file.")
	fmt.Println("  build [--watch]      Build assets based on the configuration. Use --watch to monitor changes.")
}

func Getcwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return dir
}

func main() {
	fmt.Println("📦 Packly - Asset Bundler & Dependency Manager for Web")
	fmt.Println("---------------------------------------------------------")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	command := os.Args[1]
	src, dest, ignores, deps, sassVersion, sassEps := loadConfig()

	osName := runtime.GOOS
	archName := runtime.GOARCH

	fmt.Printf("OS: %s\n", osName)
	fmt.Printf("Arch: %s\n", archName)

	switch command {
	case "install":
		installCmd := flag.NewFlagSet("install", flag.ExitOnError)
		forceFlag := installCmd.Bool("force", false, "force reinstall of dependencies")
		if len(os.Args) > 2 {
			installCmd.Parse(os.Args[2:])
		}
		installDependencies(src, deps, *forceFlag)
	case "build":
		buildCmd := flag.NewFlagSet("build", flag.ExitOnError)
		watchFlag := buildCmd.Bool("watch", false, "watch files for changes")
		if len(os.Args) > 2 {
			buildCmd.Parse(os.Args[2:])
		}

		build(src, dest, ignores, sassVersion, sassEps)
		if *watchFlag {
			watch(src, dest, ignores, sassVersion, sassEps)
		}
	default:
		fmt.Printf("Error: Unknown command '%s'\n\n", command)
		printUsage()
		os.Exit(1)
	}
}
