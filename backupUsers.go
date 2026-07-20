package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/go-github/github"
	"github.com/fsnotify/fsnotify"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
)

const (
	DataDir     = "data"
	SyncInterval = 10 * time.Second
)

var (
	backupDirty atomic.Bool
	watcher     *fsnotify.Watcher
	syncMutex   sync.Mutex
	githubClient *github.Client
	defaultBranch string
	repoOwner string
	repoName string
)

type BackupManager struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func NewBackupManager(ctx context.Context) *BackupManager {
	childCtx, cancel := context.WithCancel(ctx)
	return &BackupManager{
		ctx:    childCtx,
		cancel: cancel,
	}
}

func (bm *BackupManager) Start() error {
	if err := bm.initGitHubClient(); err != nil {
		return fmt.Errorf("init github client: %w", err)
	}

	if err := bm.detectDefaultBranch(); err != nil {
		return fmt.Errorf("detect default branch: %w", err)
	}

	if err := bm.ensureDataDir(); err != nil {
		return fmt.Errorf("ensure data dir: %w", err)
	}

	if err := bm.downloadFromGitHub(); err != nil {
	}

	if err := bm.setupFileWatcher(); err != nil {
		return fmt.Errorf("setup file watcher: %w", err)
	}

	go bm.syncLoop()

	return nil
}

func (bm *BackupManager) Stop() {
	bm.cancel()
	if watcher != nil {
		watcher.Close()
	}
}

func (bm *BackupManager) initGitHubClient() error {
	if err := godotenv.Load(); err != nil {
		return fmt.Errorf("load .env file: %w", err)
	}

	gitHubToken := os.Getenv("GITHUB_TOKEN")
	if gitHubToken == "" {
		return fmt.Errorf("GITHUB_TOKEN not configured in .env file")
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: gitHubToken},
	)
	tc := oauth2.NewClient(bm.ctx, ts)
	githubClient = github.NewClient(tc)
	return nil
}

func (bm *BackupManager) detectDefaultBranch() error {
	ctx, cancel := context.WithTimeout(bm.ctx, 30*time.Second)
	defer cancel()

	repoOwner = os.Getenv("REPO_OWNER")
	repoName = os.Getenv("REPO_NAME")

	if repoOwner == "" || repoName == "" {
		return fmt.Errorf("REPO_OWNER or REPO_NAME not configured in .env file")
	}

	repo, _, err := githubClient.Repositories.Get(ctx, repoOwner, repoName)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	if repo.DefaultBranch == nil || *repo.DefaultBranch == "" {
		return fmt.Errorf("repository default branch is empty")
	}

	defaultBranch = *repo.DefaultBranch
	return nil
}

func (bm *BackupManager) ensureDataDir() error {
	if err := os.MkdirAll(DataDir, 0755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	return nil
}

func (bm *BackupManager) downloadFromGitHub() error {
	ctx, cancel := context.WithTimeout(bm.ctx, 60*time.Second)
	defer cancel()

	_, dirContents, resp, err := githubClient.Repositories.GetContents(ctx, repoOwner, repoName, DataDir, &github.RepositoryContentGetOptions{
		Ref: defaultBranch,
	})
	if resp != nil {
		resp.Body.Close()
	}
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("get contents: %w", err)
	}

	if dirContents == nil {
		return nil
	}

	for _, item := range dirContents {
		if item == nil {
			continue
		}

		localPath := filepath.Join(DataDir, *item.Name)
		
		if *item.Type == "file" {
			if err := bm.downloadFile(ctx, item, localPath); err != nil {
				continue
			}
		} else if *item.Type == "dir" {
			if err := bm.downloadDirectory(ctx, *item.Path, localPath); err != nil {
				continue
			}
		}
	}

	return nil
}

func (bm *BackupManager) downloadFile(ctx context.Context, file *github.RepositoryContent, localPath string) error {
	if file.Content == nil {
		content, _, resp, err := githubClient.Repositories.GetContents(ctx, repoOwner, repoName, *file.Path, &github.RepositoryContentGetOptions{
			Ref: defaultBranch,
		})
		if resp != nil {
			resp.Body.Close()
		}
		if err != nil {
			return fmt.Errorf("get file content: %w", err)
		}
		file = content
	}

	content, err := file.GetContent()
	if err != nil {
		return fmt.Errorf("decode content: %w", err)
	}

	if err := os.WriteFile(localPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

func (bm *BackupManager) downloadDirectory(ctx context.Context, remotePath, localPath string) error {
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	_, dirContents, resp, err := githubClient.Repositories.GetContents(ctx, repoOwner, repoName, remotePath, &github.RepositoryContentGetOptions{
		Ref: defaultBranch,
	})
	if resp != nil {
		resp.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("get directory contents: %w", err)
	}

	for _, item := range dirContents {
		if item == nil {
			continue
		}

		itemLocalPath := filepath.Join(localPath, *item.Name)
		
		if *item.Type == "file" {
			if err := bm.downloadFile(ctx, item, itemLocalPath); err != nil {
				continue
			}
		} else if *item.Type == "dir" {
			if err := bm.downloadDirectory(ctx, *item.Path, itemLocalPath); err != nil {
				continue
			}
		}
	}

	return nil
}

func (bm *BackupManager) setupFileWatcher() error {
	var err error
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}

	if err := bm.addWatchRecursively(DataDir); err != nil {
		return fmt.Errorf("add watch recursively: %w", err)
	}

	go bm.watchEvents()

	return nil
}

func (bm *BackupManager) addWatchRecursively(path string) error {
	err := filepath.Walk(path, func(walkPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			watcher.Add(walkPath)
		}
		return nil
	})
	return err
}

func (bm *BackupManager) watchEvents() {
	for {
		select {
		case <-bm.ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			bm.handleFileEvent(event)
		case _, ok := <-watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

func (bm *BackupManager) handleFileEvent(event fsnotify.Event) {
	if event.Op&fsnotify.Create == fsnotify.Create {
		info, err := os.Stat(event.Name)
		if err == nil && info.IsDir() {
			bm.addWatchRecursively(event.Name)
		}
	}

	if event.Op&fsnotify.Create == fsnotify.Create ||
		event.Op&fsnotify.Write == fsnotify.Write ||
		event.Op&fsnotify.Remove == fsnotify.Remove ||
		event.Op&fsnotify.Rename == fsnotify.Rename {
		backupDirty.Store(true)

		if filepath.Base(event.Name) == "UsersMaiL.json" && (event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create) {
			_ = reloadJSONStore()
		}
	}
}

func (bm *BackupManager) syncLoop() {
	ticker := time.NewTicker(SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-bm.ctx.Done():
			return
		case <-ticker.C:
			if backupDirty.Load() {
				if err := bm.syncToGitHub(); err != nil {
				} else {
					backupDirty.Store(false)
				}
			}
		}
	}
}

func (bm *BackupManager) syncToGitHub() error {
	syncMutex.Lock()
	defer syncMutex.Unlock()

	ctx, cancel := context.WithTimeout(bm.ctx, 120*time.Second)
	defer cancel()

	localFiles, err := bm.collectLocalFiles(DataDir)
	if err != nil {
		return fmt.Errorf("collect local files: %w", err)
	}

	remoteFiles, err := bm.collectRemoteFiles(ctx, DataDir)
	if err != nil && !isNotFound(err) {
		return fmt.Errorf("collect remote files: %w", err)
	}

	if err := bm.uploadNewFiles(ctx, localFiles, remoteFiles); err != nil {
		return fmt.Errorf("upload new files: %w", err)
	}

	if err := bm.updateModifiedFiles(ctx, localFiles, remoteFiles); err != nil {
		return fmt.Errorf("update modified files: %w", err)
	}

	if err := bm.deleteRemovedFiles(ctx, localFiles, remoteFiles); err != nil {
		return fmt.Errorf("delete removed files: %w", err)
	}

	return nil
}

func (bm *BackupManager) collectLocalFiles(root string) (map[string]string, error) {
	files := make(map[string]string)
	
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files[relPath] = path
		}
		return nil
	})
	
	return files, err
}

func (bm *BackupManager) collectRemoteFiles(ctx context.Context, root string) (map[string]*github.RepositoryContent, error) {
	files := make(map[string]*github.RepositoryContent)
	
	_, dirContents, resp, err := githubClient.Repositories.GetContents(ctx, repoOwner, repoName, root, &github.RepositoryContentGetOptions{
		Ref: defaultBranch,
	})
	if resp != nil {
		resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}

	if dirContents == nil {
		return files, nil
	}

	for _, item := range dirContents {
		if item == nil {
			continue
		}
		
		relPath := *item.Name
		files[relPath] = item

		if *item.Type == "dir" {
			subFiles, err := bm.collectRemoteFilesRecursive(ctx, *item.Path, relPath)
			if err != nil {
				continue
			}
			for k, v := range subFiles {
				files[k] = v
			}
		}
	}

	return files, nil
}

func (bm *BackupManager) collectRemoteFilesRecursive(ctx context.Context, remotePath, relPrefix string) (map[string]*github.RepositoryContent, error) {
	files := make(map[string]*github.RepositoryContent)
	
	_, dirContents, resp, err := githubClient.Repositories.GetContents(ctx, repoOwner, repoName, remotePath, &github.RepositoryContentGetOptions{
		Ref: defaultBranch,
	})
	if resp != nil {
		resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}

	if dirContents == nil {
		return files, nil
	}

	for _, item := range dirContents {
		if item == nil {
			continue
		}
		
		relPath := filepath.Join(relPrefix, *item.Name)
		files[relPath] = item

		if *item.Type == "dir" {
			subFiles, err := bm.collectRemoteFilesRecursive(ctx, *item.Path, relPath)
			if err != nil {
				return nil, err
			}
			for k, v := range subFiles {
				files[k] = v
			}
		}
	}

	return files, nil
}

func (bm *BackupManager) uploadNewFiles(ctx context.Context, localFiles map[string]string, remoteFiles map[string]*github.RepositoryContent) error {
	for relPath, localPath := range localFiles {
		if _, exists := remoteFiles[relPath]; !exists {
			if err := bm.uploadFileWithRetry(ctx, localPath, relPath); err != nil {
				continue
			}
		}
	}
	return nil
}

func (bm *BackupManager) updateModifiedFiles(ctx context.Context, localFiles map[string]string, remoteFiles map[string]*github.RepositoryContent) error {
	for relPath, localPath := range localFiles {
		remoteFile, exists := remoteFiles[relPath]
		if !exists {
			continue
		}

		localContent, err := os.ReadFile(localPath)
		if err != nil {
			continue
		}

		localSHA := computeSHA256(localContent)
		if remoteFile.SHA != nil && localSHA != *remoteFile.SHA {
			if err := bm.updateFileWithRetry(ctx, localPath, relPath, *remoteFile.SHA); err != nil {
				continue
			}
		}
	}
	return nil
}

func (bm *BackupManager) deleteRemovedFiles(ctx context.Context, localFiles map[string]string, remoteFiles map[string]*github.RepositoryContent) error {
	for relPath, remoteFile := range remoteFiles {
		if _, exists := localFiles[relPath]; !exists {
			if remoteFile.SHA == nil {
				continue
			}
			if err := bm.deleteFileWithRetry(ctx, relPath, *remoteFile.SHA); err != nil {
				continue
			}
		}
	}
	return nil
}

func (bm *BackupManager) uploadFileWithRetry(ctx context.Context, localPath, remotePath string) error {
	return bm.withRetry(ctx, func() error {
		return bm.uploadFile(ctx, localPath, remotePath)
	})
}

func (bm *BackupManager) updateFileWithRetry(ctx context.Context, localPath, remotePath, sha string) error {
	return bm.withRetry(ctx, func() error {
		return bm.updateFile(ctx, localPath, remotePath, sha)
	})
}

func (bm *BackupManager) deleteFileWithRetry(ctx context.Context, remotePath, sha string) error {
	return bm.withRetry(ctx, func() error {
		return bm.deleteFile(ctx, remotePath, sha)
	})
}

func (bm *BackupManager) withRetry(ctx context.Context, operation func() error) error {
	maxRetries := 3
	baseDelay := 1 * time.Second

	for i := 0; i < maxRetries; i++ {
		err := operation()
		if err == nil {
			return nil
		}

		if isRateLimit(err) || isTemporary(err) {
			delay := baseDelay * time.Duration(1<<uint(i))
			
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				continue
			}
		}

		return err
	}

	return fmt.Errorf("max retries exceeded")
}

func (bm *BackupManager) uploadFile(ctx context.Context, localPath, remotePath string) error {
	content, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	remotePath = filepath.Join(DataDir, remotePath)
	message := fmt.Sprintf("Upload %s", remotePath)
	
	_, _, err = githubClient.Repositories.CreateFile(ctx, repoOwner, repoName, remotePath, &github.RepositoryContentFileOptions{
		Message:   &message,
		Content:   content,
		Branch:    &defaultBranch,
	})
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}

	return nil
}

func (bm *BackupManager) updateFile(ctx context.Context, localPath, remotePath, sha string) error {
	content, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	remotePath = filepath.Join(DataDir, remotePath)
	message := fmt.Sprintf("Update %s", remotePath)
	
	_, _, err = githubClient.Repositories.UpdateFile(ctx, repoOwner, repoName, remotePath, &github.RepositoryContentFileOptions{
		Message:   &message,
		Content:   content,
		SHA:       &sha,
		Branch:    &defaultBranch,
	})
	if err != nil {
		return fmt.Errorf("update file: %w", err)
	}

	return nil
}

func (bm *BackupManager) deleteFile(ctx context.Context, remotePath, sha string) error {
	remotePath = filepath.Join(DataDir, remotePath)
	message := fmt.Sprintf("Delete %s", remotePath)
	
	_, _, err := githubClient.Repositories.DeleteFile(ctx, repoOwner, repoName, remotePath, &github.RepositoryContentFileOptions{
		Message:   &message,
		SHA:       &sha,
		Branch:    &defaultBranch,
	})
	if err != nil {
		return fmt.Errorf("delete file: %w", err)
	}

	return nil
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	
	if githubErr, ok := err.(*github.ErrorResponse); ok {
		return githubErr.Response.StatusCode == 404
	}
	
	return false
}

func isRateLimit(err error) bool {
	if err == nil {
		return false
	}
	
	if githubErr, ok := err.(*github.ErrorResponse); ok {
		return githubErr.Response.StatusCode == 403
	}
	
	return false
}

func isTemporary(err error) bool {
	if err == nil {
		return false
	}
	
	if githubErr, ok := err.(*github.ErrorResponse); ok {
		return githubErr.Response.StatusCode >= 500 || githubErr.Response.StatusCode == 429
	}
	
	return false
}

func computeSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}