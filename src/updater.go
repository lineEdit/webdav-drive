// updater.go
//go:build windows

package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/getlantern/systray"
	"github.com/go-toast/toast"
)

const (
	githubRepo  = "lineEdit/webdav-drive"
	appName     = "webdav-drive"
	assetSuffix = "_windows_amd64.exe"
)

// GitHubRelease Структура ответа GitHub Releases API
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// Показать уведомление Windows
func showNotification(title, message string) {
	notif := toast.Notification{
		AppID:   "WebDAV Drive", // Должен совпадать с AppID в реестре (опционально)
		Title:   title,
		Message: message,
	}
	if err := notif.Push(); err != nil {
		logger.Debugf("Не удалось показать уведомление: %v", err)
	}
}

// checkForUpdates — основная функция проверки
func checkForUpdates() {
	logger.Info("Проверка обновлений...")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/" + githubRepo + "/releases/latest")
	if err != nil {
		logger.Debugf("Не удалось проверить обновления: %v", err)
		return
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			logger.Warning(err)
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		logger.Debugf("GitHub API вернул статус: %d", resp.StatusCode)
		return
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		logger.Warnf("Ошибка декодирования релиза: %v", err)
		return
	}

	latestTag := release.TagName
	latestVersion := strings.TrimPrefix(latestTag, "v")
	current := strings.TrimPrefix(version, "v")

	if latestVersion == current {
		logger.Info("Обновлений нет")
		return
	}

	// Ищем нужный ассет
	var downloadURL string
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, assetSuffix) {
			downloadURL = asset.URL
			break
		}
	}

	if downloadURL == "" {
		logger.Warn("Не найден бинарник для Windows")
		return
	}

	logger.Infof("Доступна новая версия: %s", latestVersion)
	showNotification("WebDAV Drive", fmt.Sprintf("Доступна новая версия: %s", latestVersion))

	// Добавляем пункт в трей
	menuItem := systray.AddMenuItem(
		fmt.Sprintf("🔄 Обновить до %s", latestVersion),
		"Скачать и установить обновление",
	)

	go func() {
		<-menuItem.ClickedCh
		systray.Quit()
		performUpdate(downloadURL)
	}()
}

// performUpdate — скачивает и проверяет файлы
func performUpdate(downloadURL string) {
	logger.Info("Начало обновления...")

	exe, err := os.Executable()
	if err != nil {
		fatalExit("Не удалось определить путь к exe: %v", err)
	}
	exeDir := filepath.Dir(exe)
	tempExe := filepath.Join(exeDir, appName+"-update.exe")
	tempSha := tempExe + ".sha256"

	// 1. Скачиваем .exe
	showNotification("WebDAV Drive", "Скачивание обновления...")
	if err := downloadFile(downloadURL, tempExe); err != nil {
		fatalExit("Ошибка скачивания exe: %v", err)
	}

	// 2. Скачиваем .sha256
	shaURL := downloadURL + ".sha256"
	showNotification("WebDAV Drive", "Проверка целостности...")
	if err := downloadFile(shaURL, tempSha); err != nil {
		err := os.Remove(tempExe)
		if err != nil {
			logger.Warning(err)
			return
		}
		fatalExit("Ошибка скачивания хеша: %v", err)
	}

	// 3. Читаем ожидаемый хеш
	shaContent, err := os.ReadFile(tempSha)
	if err != nil {
		err = os.Remove(tempExe)
		if err != nil {
			logger.Warning(err)
			return
		}
		err = os.Remove(tempSha)
		if err != nil {
			logger.Warning(err)
			return
		}
		fatalExit("Не удалось прочитать хеш: %v", err)
	}
	expectedHash := strings.TrimSpace(string(shaContent))

	// 4. Проверяем хеш
	if !verifySHA256(tempExe, expectedHash) {
		err = os.Remove(tempExe)
		if err != nil {
			logger.Warning(err)
			return
		}
		err = os.Remove(tempSha)
		if err != nil {
			logger.Warning(err)
			return
		}
		fatalExit("Контрольная сумма не совпадает!")
	}

	// 5. Запускаем обновлять с откатом
	launchUpdaterWithRollback(exe, tempExe, tempSha)
}

// verifySHA256 — проверяет хеш файла
func verifySHA256(filePath, expectedHash string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			logger.Warning(err)
		}
	}(file)

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return false
	}
	actualHash := fmt.Sprintf("%x", hasher.Sum(nil))
	return strings.EqualFold(actualHash, expectedHash)
}

// downloadFile — скачивает файл по URL
func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("http.Get: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			logger.Warning(err)
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		return fmt.Errorf("сервер вернул статус %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("os.Create: %w", err)
	}
	defer func(out *os.File) {
		err = out.Close()
		if err != nil {
			logger.Warning(err)
		}
	}(out)

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("io.Copy: %w", err)
	}
	return nil
}

// fatalExit — логирует, показывает уведомление и завершает
func fatalExit(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	logger.Error(msg)
	showNotification("Ошибка обновления", msg)
	time.Sleep(5 * time.Second)
	os.Exit(1)
}

// launchUpdaterWithRollback — запускает безопасный обновлять с откатом
func launchUpdaterWithRollback(currentExe, newExe, shaFile string) {
	backupExe := currentExe + ".backup"

	updaterScript := fmt.Sprintf(`
$ErrorActionPreference = "Stop"
try {
    Write-Host "Создание резервной копии..."
    Copy-Item "%s" "%s" -Force

    Write-Host "Замена исполняемого файла..."
    Move-Item "%s" "%s" -Force

    Write-Host "Запуск новой версии для теста..."
    $proc = Start-Process "%s" -ArgumentList "--test-startup" -PassThru -WindowStyle Hidden
    $proc.WaitForExit(5000)

    if ($proc.ExitCode -ne 0) {
        throw "Новая версия завершилась с ошибкой (код: $($proc.ExitCode))"
    }

    Write-Host "Обновление успешно!"
    [Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
    $template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
    $xml = [xml] $template.GetXml()
    $xml.GetElementsByTagName("text")[0].AppendChild($xml.CreateTextNode("WebDAV Drive")) | Out-Null
    $xml.GetElementsByTagName("text")[1].AppendChild($xml.CreateTextNode("Обновление успешно установлено!")) | Out-Null
    $toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
    [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("WebDAV Drive").Show($toast)
} catch {
    Write-Host "Ошибка: $_. Запуск отката..."
    if (Test-Path "%s") {
        Move-Item "%s" "%s" -Force
        Start-Process "%s" -WindowStyle Minimized
    }
    exit 1
} finally {
    if (Test-Path "%s") { Remove-Item "%s" }
    if (Test-Path "%s") { Remove-Item "%s" }
    Remove-Item $MyInvocation.MyCommand.Path
}
`, currentExe, backupExe, newExe, currentExe, currentExe,
		backupExe, backupExe, currentExe, currentExe,
		newExe, newExe, shaFile, shaFile)

	updaterPath := filepath.Join(filepath.Dir(currentExe), "updater.ps1")
	err := os.WriteFile(updaterPath, []byte(updaterScript), 0644)
	if err != nil {
		fatalExit("Не удалось создать обновление: %v", err)
	}

	// Запускаем PowerShell в скрытом режиме
	cmd := exec.Command("powershell.exe", "-ExecutionPolicy", "Bypass", "-File", updaterPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}

	if err := cmd.Start(); err != nil {
		fatalExit("Не удалось запустить обновление: %v", err)
	}

	logger.Info("Обновление запущено. Завершение...")
	os.Exit(0)
}
