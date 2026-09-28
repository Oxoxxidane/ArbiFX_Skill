// Standard-library-only builder. go run scripts/release.go -root SKILL -out OUTPUT.
package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(dir, goBin string, env []string, args ...string) {
	c := exec.Command(goBin, args...)
	c.Dir = dir
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Env = env
	must(c.Run())
}
func replaceEnv(base []string, values map[string]string) []string {
	out := []string{}
	for _, v := range base {
		key := strings.SplitN(v, "=", 2)[0]
		if _, ok := values[key]; !ok {
			out = append(out, v)
		}
	}
	for k, v := range values {
		out = append(out, k+"="+v)
	}
	return out
}
func sum(path string) string {
	f, e := os.Open(path)
	must(e)
	defer f.Close()
	h := sha256.New()
	_, e = io.Copy(h, f)
	must(e)
	return hex.EncodeToString(h.Sum(nil))
}
func pack(root, path, platform string) {
	f, e := os.Create(path)
	must(e)
	z := zip.NewWriter(f)
	e = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(rel, "bin/") && platform != "all" && !strings.HasPrefix(rel, "bin/"+platform+"/") {
			return nil
		}
		header := &zip.FileHeader{Name: "arbifx-http/" + rel, Method: zip.Deflate}
		header.SetModTime(time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC))
		header.SetMode(0644)
		if strings.HasSuffix(rel, ".sh") || (strings.HasPrefix(rel, "bin/") && !strings.HasSuffix(rel, ".exe")) {
			header.SetMode(0755)
		}
		entry, err := z.CreateHeader(header)
		if err != nil {
			return err
		}
		input, err := os.Open(p)
		if err != nil {
			return err
		}
		defer input.Close()
		_, err = io.Copy(entry, input)
		return err
	})
	must(e)
	must(z.Close())
	must(f.Close())
}
func main() {
	root := flag.String("root", ".", "skill root")
	output := flag.String("out", "release", "distribution output")
	packageOnly := flag.Bool("package-only", false, "repackage existing binaries after documentation-only changes")
	flag.Parse()
	absRoot, e := filepath.Abs(*root)
	must(e)
	absOut, e := filepath.Abs(*output)
	must(e)
	rel, e := filepath.Rel(absRoot, absOut)
	if e != nil && filepath.VolumeName(absRoot) == filepath.VolumeName(absOut) {
		must(e)
	}
	if e == nil && (rel == "." || (!strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "..")) {
		must(fmt.Errorf("output must be outside skill folder"))
	}
	must(os.MkdirAll(absOut, 0755))
	source := filepath.Join(absRoot, "cli")
	protocol, e := os.ReadFile(filepath.Join(source, "protocol.go"))
	must(e)
	versionMatch := regexp.MustCompile(`(?m)^const version = "([0-9]+\.[0-9]+\.[0-9]+)"$`).FindSubmatch(protocol)
	if versionMatch == nil {
		must(fmt.Errorf("cannot read CLI version from protocol.go"))
	}
	releaseVersion := string(versionMatch[1])
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		goBin += ".exe"
	}
	base := replaceEnv(os.Environ(), map[string]string{"CGO_ENABLED": "0", "GOOS": runtime.GOOS, "GOARCH": runtime.GOARCH, "GOTOOLCHAIN": "local", "ARBIFX_TEST_EXE": ""})
	if !*packageOnly {
		run(source, goBin, base, "vet", "./...")
		run(source, goBin, base, "test", "-count=1", "-v", "./...")
	}
	platforms := []string{"windows-amd64", "windows-arm64", "darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64"}
	hashes := []string{}
	for _, platform := range platforms {
		split := strings.Split(platform, "-")
		name := "arbifx"
		if split[0] == "windows" {
			name += ".exe"
		}
		path := filepath.Join(absRoot, "bin", platform, name)
		must(os.MkdirAll(filepath.Dir(path), 0755))
		env := replaceEnv(base, map[string]string{"GOOS": split[0], "GOARCH": split[1]})
		if !*packageOnly {
			run(source, goBin, env, "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -buildid=", "-o", path, ".")
		}
		hashes = append(hashes, sum(path)+"  bin/"+platform+"/"+name)
		fmt.Println("Binary", platform)
		if !*packageOnly && platform == runtime.GOOS+"-"+runtime.GOARCH {
			run(source, goBin, replaceEnv(base, map[string]string{"ARBIFX_TEST_EXE": path}), "test", "-count=1", "-v", "./...")
		}
	}
	sort.Strings(hashes)
	must(os.WriteFile(filepath.Join(absOut, "BINARY-SHA256SUMS.txt"), []byte(strings.Join(hashes, "\n")+"\n"), 0644))
	archiveHashes := []string{}
	for _, platform := range append(platforms, "all") {
		name := "arbifx-http-" + releaseVersion + "-" + platform + ".zip"
		path := filepath.Join(absOut, name)
		pack(absRoot, path, platform)
		archiveHashes = append(archiveHashes, sum(path)+"  "+name)
		fmt.Println("Packaged", name)
	}
	must(os.WriteFile(filepath.Join(absOut, "SHA256SUMS.txt"), []byte(strings.Join(archiveHashes, "\n")+"\n"), 0644))
}
