// Package selfupdate installs official release binaries on Unix hosts.
package selfupdate

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const repository = "bryanbarton525/prism"
const maxArchive = 128 << 20
const maxBinary = 256 << 20

var releaseTag = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

type Result struct {
	Current   string `json:"current"`
	Release   string `json:"release"`
	Path      string `json:"path"`
	Available bool   `json:"available"`
	Updated   bool   `json:"updated"`
}

type Updater struct {
	Client       *http.Client
	apiURL       string
	goos, goarch string
}

func New() *Updater {
	return &Updater{Client: &http.Client{Timeout: 2 * time.Minute}, apiURL: "https://api.github.com/repos/" + repository, goos: runtime.GOOS, goarch: runtime.GOARCH}
}

type release struct {
	Tag        string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []asset `json:"assets"`
}
type asset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
}

func (u *Updater) get(ctx context.Context, endpoint string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "prism-selfupdate")
	resp, err := u.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("release request returned HTTP %d", resp.StatusCode)
	}
	return resp, nil
}

// Run resolves symlinks and stages the replacement in the destination directory.
// Only explicit version requests allow a downgrade or development-build replacement.
func (u *Updater) Run(ctx context.Context, executable, current, version string, check bool) (Result, error) {
	out := Result{Current: current}
	if version != "" && !releaseTag.MatchString(version) {
		return out, fmt.Errorf("version must be a stable release tag such as v0.1.27")
	}
	path, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return out, err
	}
	out.Path = path
	endpoint := u.apiURL + "/releases/latest"
	if version != "" {
		endpoint = u.apiURL + "/releases/tags/" + url.PathEscape(version)
	}
	resp, err := u.get(ctx, endpoint)
	if err != nil {
		return out, err
	}
	var rel release
	err = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&rel)
	resp.Body.Close()
	if err != nil {
		return out, fmt.Errorf("decode release: %w", err)
	}
	if rel.Draft || rel.Prerelease || !releaseTag.MatchString(rel.Tag) || (version != "" && rel.Tag != version) {
		return out, fmt.Errorf("GitHub did not return the requested stable release")
	}
	out.Release = rel.Tag
	out.Available = rel.Tag != current
	if version == "" && releaseTag.MatchString(current) {
		out.Available = newer(rel.Tag, current)
	}
	if !out.Available {
		return out, nil
	}
	if check {
		return out, nil
	}
	if version == "" && !releaseTag.MatchString(current) {
		return out, fmt.Errorf("development build: use --version %s to explicitly replace it", rel.Tag)
	}
	if u.goos != "linux" && u.goos != "darwin" {
		return out, fmt.Errorf("self-update supports Linux and macOS; use your installer on %s", u.goos)
	}
	name := fmt.Sprintf("prism_%s_%s_%s.tar.gz", rel.Tag, u.goos, u.goarch)
	var selected *asset
	for i := range rel.Assets {
		if rel.Assets[i].Name == name {
			selected = &rel.Assets[i]
			break
		}
	}
	if selected == nil {
		return out, fmt.Errorf("release has no asset %s", name)
	}
	want, err := hex.DecodeString(strings.TrimPrefix(selected.Digest, "sha256:"))
	if !strings.HasPrefix(selected.Digest, "sha256:") || err != nil || len(want) != sha256.Size {
		return out, fmt.Errorf("release asset has no valid SHA-256 digest")
	}
	assetURL, err := url.Parse(selected.URL)
	if err != nil || assetURL.Scheme != "https" || assetURL.Host != "github.com" || assetURL.User != nil || assetURL.Path != "/"+repository+"/releases/download/"+rel.Tag+"/"+name {
		return out, fmt.Errorf("unexpected release download URL")
	}
	resp, err = u.get(ctx, selected.URL)
	if err != nil {
		return out, err
	}
	archive, err := os.CreateTemp("", "prism-release-*.tar.gz")
	if err != nil {
		resp.Body.Close()
		return out, err
	}
	defer os.Remove(archive.Name())
	defer archive.Close()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(archive, hash), io.LimitReader(resp.Body, maxArchive+1))
	resp.Body.Close()
	if err != nil {
		return out, err
	}
	if n > maxArchive {
		return out, fmt.Errorf("release archive exceeds size limit")
	}
	if hex.EncodeToString(hash.Sum(nil)) != hex.EncodeToString(want) {
		return out, fmt.Errorf("release checksum mismatch; installed executable unchanged")
	}
	if _, err = archive.Seek(0, io.SeekStart); err != nil {
		return out, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return out, err
	}
	if !info.Mode().IsRegular() {
		return out, fmt.Errorf("executable is not a regular file")
	}
	staged, err := os.CreateTemp(filepath.Dir(path), ".prism-update-*")
	if err != nil {
		return out, fmt.Errorf("stage update (use your package manager if it owns this installation): %w", err)
	}
	defer os.Remove(staged.Name())
	defer staged.Close()
	if err = extractBinary(archive, staged); err != nil {
		return out, err
	}
	if err = staged.Chmod(info.Mode().Perm()); err != nil {
		return out, err
	}
	if err = staged.Sync(); err != nil {
		return out, err
	}
	if err = staged.Close(); err != nil {
		return out, err
	}
	if err = os.Rename(staged.Name(), path); err != nil {
		return out, fmt.Errorf("replace executable: %w", err)
	}
	out.Updated = true
	return out, nil
}

func newer(a, b string) bool {
	x, y := releaseTag.FindStringSubmatch(a), releaseTag.FindStringSubmatch(b)
	for i := 1; i <= 3; i++ {
		// Canonical decimal strings compare without integer overflow.
		if len(x[i]) != len(y[i]) {
			return len(x[i]) > len(y[i])
		}
		if x[i] != y[i] {
			return x[i] > y[i]
		}
	}
	return false
}

func extractBinary(source io.Reader, dest io.Writer) error {
	gz, err := gzip.NewReader(source)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	found := false
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if h.Name != "prism" || h.Typeflag != tar.TypeReg || found {
			return fmt.Errorf("unexpected archive entry %q", h.Name)
		}
		if h.Size <= 0 || h.Size > maxBinary {
			return fmt.Errorf("invalid binary size %s", strconv.FormatInt(h.Size, 10))
		}
		if _, err = io.Copy(dest, tr); err != nil {
			return err
		}
		found = true
	}
	if !found {
		return fmt.Errorf("archive contains no Prism binary")
	}
	return nil
}
