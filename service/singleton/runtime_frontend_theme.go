package singleton

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nezhahq/nezha/model"
)

const (
	envExtraUserThemeRepo    = "NZ_EXTRA_USER_THEME_REPOSITORY"
	envExtraUserThemeVersion = "NZ_EXTRA_USER_THEME_VERSION"
	envExtraUserThemePath    = "NZ_EXTRA_USER_THEME_PATH"
	envExtraUserThemeName    = "NZ_EXTRA_USER_THEME_NAME"
	envExtraUserThemeDefault = "NZ_EXTRA_USER_THEME_DEFAULT"
)

var frontendPathPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
var runtimeDefaultUserTemplatePath string

func initRuntimeUserTemplateFromEnv() error {
	repository := strings.TrimSpace(os.Getenv(envExtraUserThemeRepo))
	version := strings.TrimSpace(os.Getenv(envExtraUserThemeVersion))
	if repository == "" && version == "" {
		return nil
	}
	if repository == "" || version == "" {
		return fmt.Errorf("%s and %s must be provided together", envExtraUserThemeRepo, envExtraUserThemeVersion)
	}

	repoURL, author, repoName, err := normalizeGitHubRepository(repository)
	if err != nil {
		return err
	}

	path := strings.TrimSpace(os.Getenv(envExtraUserThemePath))
	if path == "" {
		path = fmt.Sprintf("%s-dist", repoName)
	}
	if !frontendPathPattern.MatchString(path) {
		return fmt.Errorf("%s must match %s", envExtraUserThemePath, frontendPathPattern.String())
	}

	name := strings.TrimSpace(os.Getenv(envExtraUserThemeName))
	if name == "" {
		name = repoName
	}

	setAsDefault, err := parseBoolEnv(envExtraUserThemeDefault, false)
	if err != nil {
		return err
	}

	if err := downloadAndExtractTheme(repoURL, version, path); err != nil {
		return err
	}

	upsertFrontendTemplate(model.FrontendTemplate{
		Path:       path,
		Name:       name,
		Repository: repoURL,
		Author:     author,
		Version:    version,
		IsAdmin:    false,
		IsOfficial: false,
	})

	if setAsDefault {
		runtimeDefaultUserTemplatePath = path
		log.Printf("NEZHA>> Runtime user template %q set as default via %s=true", path, envExtraUserThemeDefault)
	}

	log.Printf("NEZHA>> Runtime user template loaded: repo=%s version=%s path=%s", repoURL, version, path)
	return nil
}

func GetRuntimeDefaultUserTemplate() string {
	return runtimeDefaultUserTemplatePath
}

func parseBoolEnv(key string, defaultValue bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue, nil
	}
	val, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}
	return val, nil
}

func normalizeGitHubRepository(repository string) (repoURL string, author string, repoName string, err error) {
	repo := strings.TrimSpace(strings.TrimSuffix(repository, "/"))
	if repo == "" {
		return "", "", "", errors.New("repository cannot be empty")
	}

	if strings.Contains(repo, "://") {
		u, err := url.Parse(repo)
		if err != nil {
			return "", "", "", fmt.Errorf("invalid repository URL: %w", err)
		}
		if !strings.EqualFold(u.Host, "github.com") {
			return "", "", "", fmt.Errorf("repository host must be github.com: %s", u.Host)
		}

		path := strings.Trim(strings.TrimSuffix(u.Path, ".git"), "/")
		parts := strings.Split(path, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", "", fmt.Errorf("repository path must be owner/repo: %s", repository)
		}
		return fmt.Sprintf("https://github.com/%s/%s", parts[0], parts[1]), parts[0], parts[1], nil
	}

	parts := strings.Split(strings.TrimSuffix(repo, ".git"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", fmt.Errorf("repository must be owner/repo or https://github.com/owner/repo: %s", repository)
	}
	return fmt.Sprintf("https://github.com/%s/%s", parts[0], parts[1]), parts[0], parts[1], nil
}

func downloadAndExtractTheme(repoURL, version, targetPath string) error {
	downloadURL := fmt.Sprintf("%s/releases/download/%s/dist.zip", strings.TrimSuffix(repoURL, "/"), version)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download runtime theme: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download runtime theme failed with status: %s", resp.Status)
	}

	tmpZip, err := os.CreateTemp("", "nezha-theme-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmpZip.Name())

	if _, err = io.Copy(tmpZip, resp.Body); err != nil {
		tmpZip.Close()
		return err
	}
	if err = tmpZip.Close(); err != nil {
		return err
	}

	extractDir, err := os.MkdirTemp("", "nezha-theme-extract-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(extractDir)

	if err = unzipFile(tmpZip.Name(), extractDir); err != nil {
		return err
	}

	sourceDir, err := findFrontendRootDir(extractDir)
	if err != nil {
		return err
	}

	if err = os.RemoveAll(targetPath); err != nil {
		return err
	}
	if err = os.MkdirAll(targetPath, 0750); err != nil {
		return err
	}
	return copyDir(sourceDir, targetPath)
}

func unzipFile(zipPath, dest string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	dest = filepath.Clean(dest)
	for _, f := range reader.File {
		cleanName := filepath.Clean(f.Name)
		if cleanName == "." {
			continue
		}
		targetPath := filepath.Join(dest, cleanName)
		rel, err := filepath.Rel(dest, targetPath)
		if err != nil {
			return err
		}
		if strings.HasPrefix(rel, "..") {
			return fmt.Errorf("invalid zip entry path: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err = os.MkdirAll(targetPath, f.Mode()); err != nil {
				return err
			}
			continue
		}

		if err = os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
			return err
		}

		src, err := f.Open()
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			src.Close()
			return err
		}
		_, copyErr := io.Copy(dst, src)
		closeErr := errors.Join(src.Close(), dst.Close())
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func findFrontendRootDir(baseDir string) (string, error) {
	distDir := filepath.Join(baseDir, "dist")
	if fileExists(filepath.Join(distDir, "index.html")) {
		return distDir, nil
	}

	var indexDirs []string
	err := filepath.WalkDir(baseDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), "index.html") {
			indexDirs = append(indexDirs, filepath.Dir(p))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(indexDirs) == 0 {
		return "", errors.New("no frontend root directory found in dist.zip")
	}

	indexDirs = dedupePaths(indexDirs)
	slices.Sort(indexDirs)
	for _, dir := range indexDirs {
		if strings.EqualFold(filepath.Base(dir), "dist") {
			return dir, nil
		}
	}
	if len(indexDirs) > 1 {
		log.Printf("NEZHA>> Multiple frontend roots found in runtime theme, using %s", indexDirs[0])
	}
	return indexDirs[0], nil
}

func dedupePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0750)
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		info, err := srcFile.Stat()
		if err != nil {
			srcFile.Close()
			return err
		}

		dstFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			srcFile.Close()
			return err
		}

		_, copyErr := io.Copy(dstFile, srcFile)
		closeErr := errors.Join(srcFile.Close(), dstFile.Close())
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func upsertFrontendTemplate(tpl model.FrontendTemplate) {
	for i := range FrontendTemplates {
		if FrontendTemplates[i].Path == tpl.Path {
			FrontendTemplates[i] = tpl
			return
		}
	}
	FrontendTemplates = append(FrontendTemplates, tpl)
}
