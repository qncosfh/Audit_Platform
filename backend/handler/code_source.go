package handler

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"platform/config"
	"platform/model"
	"platform/util"
)

type CodeSourceHandler struct{}

func NewCodeSourceHandler() *CodeSourceHandler {
	return &CodeSourceHandler{}
}

type UploadResponse struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Size     int64  `json:"size"`
	Language string `json:"language"`
	Status   string `json:"status"`
}

// 获取沙盒目录
func getSandboxDir(userID uint) string {
	return filepath.Join(config.GetUploadPath(), fmt.Sprintf("user_%d", userID))
}

// getExtractDir 获取代码源的解压目录
func getExtractDir(userID uint, codeSourceID uint) string {
	return filepath.Join(config.GetUploadPath(), fmt.Sprintf("user_%d", userID), "code_sources", fmt.Sprintf("%d", codeSourceID))
}

// UploadZip ZIP文件上传（异步处理 - 快速返回）
func (h *CodeSourceHandler) UploadZip(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, util.Error(400, "文件上传失败"))
		return
	}

	// 验证文件
	if err := util.ValidateFileSize(file.Size, config.AppConfig.MaxUploadSize); err != nil {
		c.JSON(http.StatusBadRequest, util.Error(400, err.Error()))
		return
	}
	if err := util.ValidateFileType(file.Filename, []string{".zip"}); err != nil {
		c.JSON(http.StatusBadRequest, util.Error(400, err.Error()))
		return
	}

	// 生成唯一文件名并保存
	fileUUID := uuid.New().String()
	filename := fmt.Sprintf("%s%s", fileUUID, filepath.Ext(file.Filename))
	sandboxDir := getSandboxDir(userID)
	os.MkdirAll(sandboxDir, 0755)
	filePath := filepath.Join(sandboxDir, filename)

	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "文件保存失败"))
		return
	}

	// 创建数据库记录
	codeSource := model.CodeSource{
		UserID:   userID,
		Type:     model.CodeSourceTypeZip,
		Name:     file.Filename,
		Size:     file.Size,
		FilePath: filePath,
		Status:   "processing",
	}

	if err := util.DB.Create(&codeSource).Error; err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "数据库保存失败"))
		return
	}

	// 异步处理：解压和语言检测
	go h.processZipAsync(codeSource.ID, filePath, userID)

	// 立即返回
	c.JSON(http.StatusOK, util.Success(UploadResponse{
		ID:       codeSource.ID,
		Name:     codeSource.Name,
		Type:     string(codeSource.Type),
		Size:     file.Size,
		Language: "processing",
		Status:   "processing",
	}))
}

// cleanupTempFiles 清理临时文件
func (h *CodeSourceHandler) cleanupTempFiles(filePath string, extractDir string) {
	// 清理上传的临时文件
	if err := os.Remove(filePath); err != nil {
		fmt.Printf("清理上传文件失败: %v\n", err)
	}
}

// processZipAsync 异步处理ZIP文件（优化版）
func (h *CodeSourceHandler) processZipAsync(codeSourceID uint, filePath string, userID uint) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("处理ZIP文件失败: %v\n", r)
		}
	}()

	extractDir := getExtractDir(userID, codeSourceID)
	os.MkdirAll(extractDir, 0755)

	// 优化：使用并行解压
	if err := unzipFileParallel(filePath, extractDir); err != nil {
		fmt.Printf("解压失败: %v\n", err)
		util.DB.Model(&model.CodeSource{}).Where("id = ?", codeSourceID).Update("status", "failed")
		// 清理临时文件
		h.cleanupTempFiles(filePath, extractDir)
		return
	}

	// 优化：并行计算大小和检测语言
	var totalSize int64
	var language string
	var wg sync.WaitGroup
	var mu sync.Mutex

	wg.Add(2)

	// 并行计算大小
	go func() {
		defer wg.Done()
		var size int64
		filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				size += info.Size()
			}
			return nil
		})
		mu.Lock()
		totalSize = size
		mu.Unlock()
	}()

	// 并行检测语言
	go func() {
		defer wg.Done()
		lang := detectCodeLanguageFromDir(extractDir)
		mu.Lock()
		language = lang
		mu.Unlock()
	}()

	wg.Wait()

	// 更新记录
	util.DB.Model(&model.CodeSource{}).Where("id = ?", codeSourceID).Updates(map[string]interface{}{
		"status":   "active",
		"size":     totalSize,
		"path":     extractDir,
		"language": language,
	})

	fmt.Printf("ZIP处理完成: ID=%d, Language=%s, Size=%d\n", codeSourceID, language, totalSize)
}

// unzipFileParallel 并行解压ZIP文件（优化版）
func unzipFileParallel(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	// 收集所有需要解压的文件
	type fileTask struct {
		file *zip.File
		path string
	}

	var tasks []fileTask
	for _, file := range r.File {
		name := file.Name
		if strings.HasPrefix(name, "__MACOSX/") || strings.HasPrefix(filepath.Base(name), "._") {
			continue
		}

		path := filepath.Join(dest, name)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}

		tasks = append(tasks, fileTask{file: file, path: path})
	}

	// 优化：并行解压文件
	numWorkers := runtime.NumCPU()
	if len(tasks) < 100 {
		numWorkers = 4
	}
	if numWorkers > 8 {
		numWorkers = 8
	}

	semaphore := make(chan struct{}, numWorkers)
	var wg sync.WaitGroup
	errors := make([]error, 0)
	var mu sync.Mutex

	for _, task := range tasks {
		wg.Add(1)
		go func(t fileTask) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if t.file.FileInfo().IsDir() {
				os.MkdirAll(t.path, 0755)
				return
			}

			os.MkdirAll(filepath.Dir(t.path), 0755)

			outFile, err := os.OpenFile(t.path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, t.file.Mode())
			if err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
				return
			}
			defer outFile.Close()

			rc, err := t.file.Open()
			if err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
				return
			}
			defer rc.Close()

			_, err = io.Copy(outFile, rc)
			if err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
			}
		}(task)
	}

	wg.Wait()

	// 如果有错误，返回第一个错误
	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

// UploadJar JAR文件上传（异步处理 - 快速返回）
func (h *CodeSourceHandler) UploadJar(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, util.Error(400, "文件上传失败"))
		return
	}

	// 验证文件
	if err := util.ValidateFileSize(file.Size, config.AppConfig.MaxUploadSize); err != nil {
		c.JSON(http.StatusBadRequest, util.Error(400, err.Error()))
		return
	}
	if err := util.ValidateFileType(file.Filename, []string{".jar"}); err != nil {
		c.JSON(http.StatusBadRequest, util.Error(400, err.Error()))
		return
	}

	// 保存文件
	fileUUID := uuid.New().String()
	filename := fmt.Sprintf("%s%s", fileUUID, filepath.Ext(file.Filename))
	sandboxDir := getSandboxDir(userID)
	os.MkdirAll(sandboxDir, 0755)
	filePath := filepath.Join(sandboxDir, filename)

	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "文件保存失败"))
		return
	}

	// 创建数据库记录
	codeSource := model.CodeSource{
		UserID:   userID,
		Type:     model.CodeSourceTypeJar,
		Name:     file.Filename,
		Size:     file.Size,
		FilePath: filePath,
		Status:   "processing",
	}

	if err := util.DB.Create(&codeSource).Error; err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "数据库保存失败"))
		return
	}

	// 异步处理：解压和反编译
	go h.processJarAsync(codeSource.ID, filePath, userID)

	// 立即返回
	c.JSON(http.StatusOK, util.Success(UploadResponse{
		ID:       codeSource.ID,
		Name:     codeSource.Name,
		Type:     string(codeSource.Type),
		Size:     file.Size,
		Language: "processing",
		Status:   "processing",
	}))
}

// processJarAsync 异步处理JAR文件
func (h *CodeSourceHandler) processJarAsync(codeSourceID uint, filePath string, userID uint) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("处理JAR文件失败: %v\n", r)
		}
	}()

	extractDir := getExtractDir(userID, codeSourceID)
	os.MkdirAll(extractDir, 0755)

	// 解压
	if err := unzipFile(filePath, extractDir); err != nil {
		fmt.Printf("解压JAR失败: %v\n", err)
		util.DB.Model(&model.CodeSource{}).Where("id = ?", codeSourceID).Update("status", "failed")
		return
	}

	// 并行反编译
	h.decompileJarAsync(extractDir)

	// 计算大小
	var totalSize int64
	filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && !strings.HasSuffix(path, ".class") {
			totalSize += info.Size()
		}
		return nil
	})

	// 更新记录
	util.DB.Model(&model.CodeSource{}).Where("id = ?", codeSourceID).Updates(map[string]interface{}{
		"status":   "active",
		"size":     totalSize,
		"path":     extractDir,
		"language": "java",
	})

	fmt.Printf("JAR处理完成: ID=%d, Size=%d\n", codeSourceID, totalSize)
}

// decompileJarAsync 并行反编译JAR
func (h *CodeSourceHandler) decompileJarAsync(outputDir string) {
	cfrPath := filepath.Join(config.GetUploadPath(), "cfr.jar")
	if _, err := os.Stat(cfrPath); err != nil {
		fmt.Printf("cfr.jar不存在: %v\n", err)
		return
	}

	// 收集需要反编译的文件
	var classFiles []string
	filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".class") {
			return nil
		}
		javaPath := strings.TrimSuffix(path, ".class") + ".java"
		if _, err := os.Stat(javaPath); err != nil {
			classFiles = append(classFiles, path)
		}
		return nil
	})

	// 限制数量
	if len(classFiles) > 500 {
		classFiles = classFiles[:500]
	}

	// 并行反编译 - 修复竞态条件和错误处理
	semaphore := make(chan struct{}, 5)
	var wg sync.WaitGroup
	var mu sync.Mutex // 保护错误日志
	errorCount := 0

	for _, classPath := range classFiles {
		wg.Add(1)
		go func(cp string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			javaPath := strings.TrimSuffix(cp, ".class") + ".java"
			cmd := exec.Command("java", "-jar", cfrPath, cp, "--outputpath", filepath.Dir(javaPath))

			// 修复：捕获并记录命令执行错误
			output, err := cmd.CombinedOutput()
			if err != nil {
				mu.Lock()
				errorCount++
				if errorCount <= 5 { // 只记录前5个错误，避免日志过多
					fmt.Printf("反编译失败 [%s]: %v, output: %s\n", cp, err, string(output))
				}
				mu.Unlock()
			}
		}(classPath)
	}

	wg.Wait()

	if errorCount > 5 {
		fmt.Printf("反编译完成，但有 %d 个文件处理失败\n", errorCount)
	}

	// 删除.class文件
	filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".class") {
			os.Remove(path)
		}
		return nil
	})
}

func (h *CodeSourceHandler) AddGitRepo(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	var req struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("JSON解析失败: %v, 请求体: %+v\n", err, req)
		c.JSON(http.StatusBadRequest, util.Error(400, "请求格式错误: "+err.Error()))
		return
	}

	// 验证URL不为空
	if req.URL == "" {
		c.JSON(http.StatusBadRequest, util.Error(400, "URL不能为空"))
		return
	}

	// 验证URL格式，防止命令注入
	gitURL := req.URL
	fmt.Printf("收到Git仓库URL: %s\n", gitURL)

	if !isValidGitURL(gitURL) {
		fmt.Printf("URL验证失败: %s\n", gitURL)
		c.JSON(http.StatusBadRequest, util.Error(400, "无效的Git仓库URL格式，请检查URL是否正确"))
		return
	}

	sandboxDir := getSandboxDir(userID)
	os.MkdirAll(sandboxDir, 0755)

	repoUUID := uuid.New().String()
	repoDir := filepath.Join(sandboxDir, repoUUID)

	fmt.Printf("开始克隆Git仓库到: %s\n", repoDir)

	// 使用参数化方式执行git clone，防止命令注入
	var cloneErr error
	var stderr []byte
	cmd := exec.Command("git", "clone", "--depth", "1", gitURL, repoDir)
	cmdOutput, err := cmd.CombinedOutput()

	fmt.Printf("git clone执行结果: err=%v, output=%s\n", err, string(cmdOutput))

	if err != nil {
		stderr = cmdOutput
		// 尝试添加.git后缀
		if !strings.HasSuffix(gitURL, ".git") {
			cmd = exec.Command("git", "clone", "--depth", "1", gitURL+".git", repoDir)
			cmdOutput, cloneErr = cmd.CombinedOutput()
			stderr = cmdOutput
		} else {
			cloneErr = err
		}
		if cloneErr != nil {
			// 清理已创建的目录
			os.RemoveAll(repoDir)

			// 分析错误原因，返回更详细的错误信息
			errMsg := string(stderr)
			errorDetail := parseGitCloneError(errMsg)

			c.JSON(http.StatusBadRequest, util.Error(400, errorDetail))
			return
		}
	}

	// 获取项目信息
	repoName := filepath.Base(req.URL)
	if strings.HasSuffix(repoName, ".git") {
		repoName = strings.TrimSuffix(repoName, ".git")
	}

	entries, _ := os.ReadDir(repoDir)
	if len(entries) == 1 && entries[0].IsDir() {
		repoName = entries[0].Name()
		repoDir = filepath.Join(repoDir, repoName)
	}

	var totalSize int64
	filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	language := detectCodeLanguageFromDir(repoDir)

	codeSource := model.CodeSource{
		UserID:   userID,
		Type:     model.CodeSourceTypeGit,
		Name:     repoName,
		URL:      req.URL,
		Path:     repoDir,
		Size:     totalSize,
		Language: language,
		Status:   "active",
	}

	if err := util.DB.Create(&codeSource).Error; err != nil {
		os.RemoveAll(filepath.Dir(repoDir))
		c.JSON(http.StatusInternalServerError, util.Error(500, "数据库保存失败"))
		return
	}

	c.JSON(http.StatusOK, util.Success(UploadResponse{
		ID:       codeSource.ID,
		Name:     codeSource.Name,
		Type:     string(codeSource.Type),
		Size:     codeSource.Size,
		Language: language,
		Status:   "active",
	}))
}

func (h *CodeSourceHandler) List(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("pageSize", "10")

	var codeSources []model.CodeSource
	var total int64

	util.DB.Model(&model.CodeSource{}).Where("user_id = ?", userID).Count(&total)

	offset := (util.StringToInt(page) - 1) * util.StringToInt(pageSize)
	util.DB.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(util.StringToInt(pageSize)).
		Offset(offset).
		Find(&codeSources)

	c.JSON(http.StatusOK, util.Success(gin.H{
		"items":    codeSources,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	}))
}

func (h *CodeSourceHandler) Delete(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	id := c.Param("id")
	var codeSource model.CodeSource
	if err := util.DB.Where("id = ? AND user_id = ?", id, userID).First(&codeSource).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "代码源不存在"))
		return
	}

	if codeSource.FilePath != "" {
		util.DeleteFile(codeSource.FilePath)
	}

	util.DB.Delete(&codeSource)
	c.JSON(http.StatusOK, util.Success(nil))
}

func (h *CodeSourceHandler) Get(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	id := c.Param("id")
	var codeSource model.CodeSource
	if err := util.DB.Where("id = ? AND user_id = ?", id, userID).First(&codeSource).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "代码源不存在"))
		return
	}

	var fileTree []map[string]interface{}
	var err error

	if codeSource.Path != "" {
		if _, err := os.Stat(codeSource.Path); err == nil {
			fileTree, err = h.getDirectoryFileTree(codeSource.Path)
		} else {
			switch codeSource.Type {
			case model.CodeSourceTypeZip, model.CodeSourceTypeJar:
				fileTree, err = h.getArchiveFileTree(codeSource.FilePath)
			default:
				fileTree = []map[string]interface{}{}
			}
		}
	} else {
		switch codeSource.Type {
		case model.CodeSourceTypeZip, model.CodeSourceTypeJar:
			fileTree, err = h.getArchiveFileTree(codeSource.FilePath)
		default:
			fileTree = []map[string]interface{}{}
		}
	}

	if err != nil {
		fileTree = []map[string]interface{}{}
	}

	c.JSON(http.StatusOK, util.Success(gin.H{
		"id":        codeSource.ID,
		"name":      codeSource.Name,
		"type":      codeSource.Type,
		"size":      codeSource.Size,
		"status":    codeSource.Status,
		"language":  codeSource.Language,
		"filePath":  codeSource.FilePath,
		"path":      codeSource.Path,
		"url":       codeSource.URL,
		"fileTree":  fileTree,
		"createdAt": codeSource.CreatedAt,
		"updatedAt": codeSource.UpdatedAt,
	}))
}

func (h *CodeSourceHandler) getArchiveFileTree(filePath string) ([]map[string]interface{}, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return []map[string]interface{}{}, nil
	}

	previewDir := filepath.Join(config.GetUploadPath(), "code-source-preview", uuid.New().String())
	defer os.RemoveAll(previewDir)

	os.MkdirAll(previewDir, 0755)
	if err := unzipFile(filePath, previewDir); err != nil {
		return nil, err
	}

	return h.buildFileTree(previewDir, "", 20), nil
}

func unzipFile(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, file := range r.File {
		name := file.Name
		if strings.HasPrefix(name, "__MACOSX/") || strings.HasPrefix(filepath.Base(name), "._") {
			continue
		}

		path := filepath.Join(dest, name)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}

		if file.FileInfo().IsDir() {
			os.MkdirAll(path, 0755)
			continue
		}

		os.MkdirAll(filepath.Dir(path), 0755)

		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			fmt.Printf("创建文件失败: %s, error: %v\n", path, err)
			continue
		}
		// 确保outFile被正确关闭
		defer outFile.Close()

		rc, err := file.Open()
		if err != nil {
			fmt.Printf("打开压缩文件内文件失败: %s, error: %v\n", name, err)
			continue
		}
		// 确保rc被正确关闭 - 修复文件描述符泄漏
		defer rc.Close()

		_, err = io.Copy(outFile, rc)
		if err != nil {
			fmt.Printf("复制文件内容失败: %s, error: %v\n", name, err)
		}
	}

	return nil
}

func (h *CodeSourceHandler) getDirectoryFileTree(dirPath string) ([]map[string]interface{}, error) {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return []map[string]interface{}{}, nil
	}
	return h.buildFileTree(dirPath, "", 20), nil
}

func (h *CodeSourceHandler) buildFileTree(rootPath, relativePath string, maxDepth int) []map[string]interface{} {
	if maxDepth <= 0 {
		return []map[string]interface{}{}
	}

	currentPath := rootPath
	if relativePath != "" {
		currentPath = filepath.Join(rootPath, relativePath)
	}

	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return []map[string]interface{}{}
	}

	var result []map[string]interface{}
	for _, entry := range entries {
		name := entry.Name()
		if name == "." || name == ".." {
			continue
		}

		fullPath := filepath.Join(currentPath, name)
		relPath := name
		if relativePath != "" {
			relPath = filepath.Join(relativePath, name)
		}

		if entry.IsDir() {
			result = append(result, map[string]interface{}{
				"name":     name,
				"type":     "directory",
				"path":     relPath,
				"children": h.buildFileTree(rootPath, relPath, maxDepth-1),
			})
		} else {
			info, _ := os.Stat(fullPath)
			result = append(result, map[string]interface{}{
				"name": name,
				"type": "file",
				"path": relPath,
				"size": info.Size(),
				"ext":  strings.ToLower(filepath.Ext(name)),
			})
		}
	}

	return result
}

// validatePath 安全验证文件路径，防止路径遍历攻击
func validatePath(baseDir, requestedPath string) (string, bool) {
	// 清理请求的路径
	cleanPath := filepath.Clean(requestedPath)

	// 检查是否包含路径遍历字符
	if strings.Contains(cleanPath, "..") {
		return "", false
	}

	// 检查是否以绝对路径开头
	if strings.HasPrefix(cleanPath, "/") || filepath.IsAbs(cleanPath) {
		return "", false
	}

	// 构建完整路径
	fullPath := filepath.Join(baseDir, cleanPath)

	// 使用filepath.EvalSymlinks解析符号链接
	realPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		// 如果是符号链接解析失败，检查原路径是否存在
		if _, err := os.Stat(fullPath); err != nil {
			return "", false
		}
		realPath = fullPath
	}

	// 验证最终路径是否在允许的目录内
	realBaseDir, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		realBaseDir = baseDir
	}

	// 确保路径以基础目录开头
	if !strings.HasPrefix(realPath, realBaseDir+string(os.PathSeparator)) && realPath != realBaseDir {
		return "", false
	}

	return realPath, true
}

func (h *CodeSourceHandler) GetFile(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	id := c.Param("id")
	filePath := c.Query("path")

	if filePath == "" {
		c.JSON(http.StatusBadRequest, util.Error(400, "文件路径不能为空"))
		return
	}

	var codeSource model.CodeSource
	if err := util.DB.Where("id = ? AND user_id = ?", id, userID).First(&codeSource).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "代码源不存在"))
		return
	}

	// 获取基础目录
	searchPath := codeSource.Path
	if searchPath == "" {
		searchPath = filepath.Dir(codeSource.FilePath)
	}

	// 使用安全的路径验证函数
	actualPath, isValid := validatePath(searchPath, filePath)
	if !isValid {
		c.JSON(http.StatusBadRequest, util.Error(400, "无效的文件路径"))
		return
	}

	// 如果文件不存在于目录中，尝试从归档文件中获取
	if _, err := os.Stat(actualPath); err != nil {
		if codeSource.Type == model.CodeSourceTypeZip || codeSource.Type == model.CodeSourceTypeJar {
			actualPath = h.getFileFromArchive(userID, codeSource.FilePath, filePath)
		}
	}

	if actualPath == "" {
		c.JSON(http.StatusNotFound, util.Error(404, "文件不存在"))
		return
	}

	content, err := os.ReadFile(actualPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "读取文件失败"))
		return
	}

	// 解码 Unicode 转义序列
	contentStr := string(content)
	decodedContent := decodeUnicodeString(contentStr)

	ext := filepath.Ext(filePath)
	c.JSON(http.StatusOK, util.Success(gin.H{
		"name":     filepath.Base(filePath),
		"path":     filePath,
		"ext":      ext,
		"size":     len([]rune(decodedContent)),
		"content":  decodedContent,
		"language": getLanguageByExt(ext),
	}))
}

// decodeUnicodeString 解码 Unicode 转义序列
func decodeUnicodeString(s string) string {
	re := regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)
	result := re.ReplaceAllStringFunc(s, func(match string) string {
		hex := match[2:]
		code, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return match
		}
		return string(rune(code))
	})

	re2 := regexp.MustCompile(`\\U([0-9a-fA-F]{8})`)
	result = re2.ReplaceAllStringFunc(result, func(match string) string {
		hex := match[3:]
		code, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return match
		}
		return string(rune(code))
	})

	if !utf8.ValidString(result) {
		return s
	}

	return result
}

func (h *CodeSourceHandler) getFileFromArchive(userID uint, archivePath, relativePath string) string {
	previewDir := filepath.Join("./sandbox", "code-source-preview", uuid.New().String())
	defer os.RemoveAll(previewDir)

	os.MkdirAll(previewDir, 0755)
	if err := unzipFile(archivePath, previewDir); err != nil {
		return ""
	}

	targetPath := filepath.Join(previewDir, relativePath)
	if _, err := os.Stat(targetPath); err == nil {
		return targetPath
	}
	return ""
}

// isValidGitURL 验证Git URL是否安全（更宽松的验证）
func isValidGitURL(url string) bool {
	fmt.Printf("验证Git URL: %s\n", url)

	// 检查URL是否包含危险字符（防止命令注入）
	dangerousChars := []string{"`", "$", "(", ")", "&", "|", ";", "\n", "\r"}
	for _, char := range dangerousChars {
		if strings.Contains(url, char) {
			fmt.Printf("URL包含危险字符: %s\n", char)
			return false
		}
	}

	// 检查URL是否为空
	if url == "" {
		fmt.Printf("URL为空\n")
		return false
	}

	// 简化验证：只要是有效的URL格式或者包含知名Git平台域名即可
	// 允许的协议
	allowedProtocols := []string{"http://", "https://", "git://", "ssh://", "git@", "git+http://", "git+https://", "git+ssh://", "git+git://"}

	for _, proto := range allowedProtocols {
		if strings.HasPrefix(url, proto) {
			fmt.Printf("URL匹配协议: %s\n", proto)
			return true
		}
	}

	// 允许不带协议的Git URL（如：github.com/user/repo）
	// 但必须确保包含知名Git平台域名
	if strings.Contains(url, "github.com/") ||
		strings.Contains(url, "gitlab.com/") ||
		strings.Contains(url, "bitbucket.org/") ||
		strings.Contains(url, "gitee.com/") ||
		strings.Contains(url, "gitcode.com/") {
		fmt.Printf("URL包含Git托管平台域名\n")
		return true
	}

	// 允许本地Git仓库路径（仅限开发环境）
	if os.Getenv("ENVIRONMENT") != "production" {
		// 允许本地路径
		if strings.HasPrefix(url, "/") || strings.HasPrefix(url, "./") || strings.HasPrefix(url, "../") ||
			filepath.IsAbs(url) || strings.Contains(url, ".git") {
			fmt.Printf("URL是本地Git仓库路径\n")
			return true
		}
	}

	fmt.Printf("URL验证失败\n")
	return false
}

func getLanguageByExt(ext string) string {
	ext = strings.ToLower(ext)
	langMap := map[string]string{
		".go": "go", ".java": "java", ".py": "python", ".js": "javascript",
		".ts": "typescript", ".cs": "csharp", ".rb": "ruby", ".php": "php",
		".swift": "swift", ".kt": "kotlin", ".scala": "scala", ".rs": "rust",
		".c": "c", ".cpp": "cpp", ".h": "c",
		".vue": "xml", ".html": "xml", ".css": "css", ".scss": "scss",
		".json": "json", ".xml": "xml", ".yaml": "yaml", ".yml": "yaml",
		".sql": "sql", ".sh": "bash", ".md": "markdown",
	}
	if lang, ok := langMap[ext]; ok {
		return lang
	}
	return "plaintext"
}

func detectCodeLanguageFromDir(dirPath string) string {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return ""
	}

	extCounts := make(map[string]int)
	skipExts := map[string]bool{
		".zip": true, ".jar": true, ".png": true, ".jpg": true, ".gif": true,
		".pdf": true, ".doc": true, ".txt": true, ".log": true, ".md": true,
		".class": true, ".pyc": true, ".o": true, ".so": true, ".dll": true,
		".DS_Store": true, ".mod": true, ".sum": true, ".lock": true,
	}
	skipDirs := map[string]bool{
		"node_modules": true, ".git": true, "target": true, "build": true,
		"dist": true, "vendor": true, "venv": true, "__pycache__": true,
		".idea": true, ".vscode": true, ".github": true,
	}

	filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info.IsDir() && skipDirs[filepath.Base(path)] {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !skipExts[ext] {
			extCounts[ext]++
		}
		return nil
	})

	priorityExts := []string{".java", ".py", ".js", ".ts", ".cs", ".go", ".rb", ".php", ".swift", ".kt", ".scala", ".rs", ".c", ".cpp", ".vue", ".sql"}
	for _, ext := range priorityExts {
		if extCounts[ext] > 0 {
			return getLanguageByExt(ext)
		}
	}
	return ""
}

// parseGitCloneError 解析 Git 克隆错误信息，返回用户友好的错误描述
func parseGitCloneError(errMsg string) string {
	if strings.Contains(errMsg, "Could not resolve host") {
		return "无法连接到Git服务器，请检查网络连接或URL是否正确"
	}
	if strings.Contains(errMsg, "Connection refused") {
		return "连接被拒绝，请检查Git服务器是否可访问"
	}
	if strings.Contains(errMsg, "Authentication failed") || strings.Contains(errMsg, "403") {
		return "认证失败，请检查仓库权限或提供认证信息"
	}
	if strings.Contains(errMsg, "Repository not found") || strings.Contains(errMsg, "404") {
		return "仓库不存在，请检查URL是否正确"
	}
	if strings.Contains(errMsg, "not found") {
		return "仓库未找到，请确认URL是否正确"
	}
	if strings.Contains(errMsg, "timed out") || strings.Contains(errMsg, "Timeout") {
		return "连接超时，请检查网络连接或稍后重试"
	}
	if strings.Contains(errMsg, "SSL") || strings.Contains(errMsg, "certificate") {
		return "SSL证书验证失败，请检查Git服务器配置"
	}
	if strings.Contains(errMsg, "does not exist") {
		return "Git命令未找到，请确保服务器已安装Git"
	}
	if len(errMsg) > 0 {
		for _, line := range strings.Split(errMsg, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "Cloning into") {
				return "克隆失败: " + line
			}
		}
	}
	return "无法克隆仓库"
}
