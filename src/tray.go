package main

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/getlantern/systray"
	"golang.org/x/sys/windows"
)

// Обработчик трея
func onReady() {
	if globalCfg == nil {
		logger.Fatal("Конфигурация не загружена!")
	}

	connected := isDriveMapped(globalCfg.DriveLetter)

	if connected {
		systray.SetIcon(iconOn)
	} else {
		systray.SetIcon(iconOff)
	}

	systray.SetTitle("WebDAV Drive")
	systray.SetTooltip(fmt.Sprintf("WebDAV Drive %s — управление подключением", version))

	mConnectEnable := systray.AddMenuItem("Подключить диск", "Подключить WebDAV как сетевой диск")
	mConnectDisable := systray.AddMenuItem("Отключить диск", "Отключить WebDAV-диск")

	// Скрываем ненужный
	if connected {
		mConnectEnable.Hide()
	} else {
		mConnectDisable.Hide()
	}

	mOpen := systray.AddMenuItem("Проводник", "Открыть в Проводнике")
	// Скрываем ненужный
	// В начале onReady()
	if connected {
		mConnectEnable.Disable()
		mConnectDisable.Enable()
	} else {
		mConnectEnable.Enable()
		mConnectDisable.Disable()
	}
	mSettings := systray.AddMenuItem("Настройки", "Редактировать config.json")
	mLogs := systray.AddMenuItem("Логи", "Посмотреть webdav-drive.log")
	mCheckUpdate := systray.AddMenuItem("Проверить обновления", "Проверить наличие новой версии")
	mExit := systray.AddMenuItem("Выход", "Завершить приложение")

	// Горутина обработки
	go func() {
		for {
			select {
			case <-mConnectEnable.ClickedCh:
				if connectWithLogging() {
					systray.SetIcon(iconOn)
					mOpen.Enable()
					mConnectEnable.Disable()
					mConnectDisable.Enable()
				}

			case <-mConnectDisable.ClickedCh:
				cmd := exec.Command("net", "use", globalCfg.DriveLetter, "/delete", "/y")
				err := cmd.Run()
				if err != nil {
					logger.Errorf("Ошибка отключения диска: %v", err)
				} else {
					logger.Info("Диск отключён")
					systray.SetIcon(iconOff)
					mOpen.Disable()
					mConnectDisable.Disable()
					mConnectEnable.Enable()
				}

			case <-mOpen.ClickedCh:
				openDriveInExplorer()

			case <-mSettings.ClickedCh:
				openConfig()

			case <-mLogs.ClickedCh:
				openLogs()

			case <-mCheckUpdate.ClickedCh:
				checkForUpdates()

			case <-mExit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

// Проверка: подключён ли диск
func isDriveMapped(drive string) bool {
	cmd := exec.Command("net", "use")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), drive)
}

// setDriveLabel устанавливает понятное имя диска в Проводнике через реестр Windows
func setDriveLabel(driveLetter, label string) error {
	// Убираем двоеточие, если есть: "N:" → "N"
	drive := strings.TrimSuffix(driveLetter, ":")
	if drive == "" {
		return fmt.Errorf("некорректная буква диска: %s", driveLetter)
	}
	keyPath := fmt.Sprintf(`HKEY_CURRENT_USER\Network\%s`, drive)
	cmd := exec.Command("reg", "add", keyPath, "/v", "DescriptiveName", "/t", "REG_SZ", "/d", label, "/f")
	return cmd.Run()
}

// Подключение диска
// connectDrive подключает WebDAV-диск и устанавливает понятное имя
func connectDrive(cfg *Config) error {
	drive := cfg.DriveLetter
	if !strings.HasSuffix(drive, ":") {
		drive += ":"
	}

	if isDriveMapped(drive) {
		logger.Infof("Диск %s уже подключен", drive)
		return nil
	}

	// Формируем понятное имя на основе URL
	var label string
	if u, err := url.Parse(cfg.WebDAVURL); err == nil {
		label = fmt.Sprintf("WebDAV Drive — %s", u.Host)
	} else {
		label = "WebDAV Drive"
		logger.Warnf("Не удалось распарсить URL для метки: %v", err)
	}

	logger.Infof("Подключение диска %s к URL: %s", drive, cfg.WebDAVURL)

	cmd := exec.Command("net", "use", drive, cfg.WebDAVURL, "/persistent:yes")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		logger.Errorf("Ошибка подключения диска:")
		logger.Errorf("  Stderr: %s", strings.TrimSpace(stderr.String()))
		logger.Errorf("  Ошибка: %v", err)
		return err
	}

	logger.Infof("Диск %s успешно подключен", drive)

	// Устанавливаем понятное имя
	if err := setDriveLabel(drive, label); err != nil {
		logger.Warnf("Не удалось установить метку диска '%s': %v", label, err)
	} else {
		logger.Infof("Метка диска установлена: %s", label)
	}

	return nil
}

// Открыть диск в Проводнике
func openDriveInExplorer() {
	drive := globalCfg.DriveLetter
	logger.Infof("Открытие %s в Проводнике", drive)
	cmd := exec.Command("explorer", drive)
	_ = cmd.Run()
}

// Открыть webdav-drive.log
func openLogs() {
	logger.Info("Открытие webdav-drive.log в редакторе")
	cmd := exec.Command("notepad", getLogPath())
	_ = cmd.Run()
}

func onExit() {
	logger.Info("Приложение завершено")
	os.Exit(0)
}

func connectWithLogging() bool {
	logger.Info("Попытка подключения диска...")

	// Нормализуем URL
	webdavURL := strings.TrimSpace(globalCfg.WebDAVURL)
	if !strings.HasSuffix(webdavURL, "/") {
		webdavURL += "/"
	}

	// Подключаем напрямую — Windows сам запросит логин при необходимости
	cfg := &Config{
		DriveLetter: globalCfg.DriveLetter,
		WebDAVURL:   webdavURL,
	}

	if err := connectDrive(cfg); err != nil {
		logger.Errorf("Не удалось подключить диск: %v", err)
		return false
	}

	logger.Info("Диск успешно подключён")
	return true
}

// isAlreadyRunning — проверка на уже запущенный экземпляр
func isAlreadyRunning() bool {
	mutexName, _ := windows.UTF16PtrFromString("WebDAVDrive_Mutex_" + os.Getenv("USERNAME"))
	_, err := windows.CreateMutex(nil, false, mutexName)
	return err != nil // true, если мьютекс уже существует
}
