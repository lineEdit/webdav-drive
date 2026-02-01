// tray.go
package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

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
	mOpen := systray.AddMenuItem("Проводник", "Открыть в Проводнике")
	mSettings := systray.AddMenuItem("Настройки", "Редактировать config.json")
	mLogs := systray.AddMenuItem("Логи", "Посмотреть webdav-drive.log")
	mCheckUpdate := systray.AddMenuItem("Проверить обновления", "Проверить наличие новой версии")
	mExit := systray.AddMenuItem("Выход", "Завершить приложение")

	// Начальное состояние кнопок
	if connected {
		mConnectEnable.Disable()
		mConnectDisable.Enable()
		mOpen.Enable()
	} else {
		mConnectEnable.Enable()
		mConnectDisable.Disable()
		mOpen.Disable()
		connectWithLogging()
	}

	// Горутина обработки событий
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

// setDriveLabel устанавливает понятное имя диска через реестр
func setDriveLabel(driveLetter, label string) error {
	drive := strings.TrimSuffix(driveLetter, ":")
	if drive == "" {
		return fmt.Errorf("некорректная буква диска: %s", driveLetter)
	}
	keyPath := fmt.Sprintf(`HKEY_CURRENT_USER\Network\%s`, drive)
	cmd := exec.Command("reg", "add", keyPath, "/v", "DescriptiveName", "/t", "REG_SZ", "/d", label, "/f")
	return cmd.Run()
}

// Подключение диска для GUI-приложения (без консоли)
func connectDrive(cfg *Config) error {
	drive := cfg.DriveLetter
	if !strings.HasSuffix(drive, ":") {
		drive += ":"
	}

	if isDriveMapped(drive) {
		logger.Infof("Диск %s уже подключен", drive)
		return nil
	}

	// Формируем метку
	var label string
	if u, err := url.Parse(cfg.WebDAVURL); err == nil {
		label = fmt.Sprintf("WebDAV Drive — %s", u.Host)
	} else {
		label = "WebDAV Drive"
	}

	logger.Infof("Подключение диска %s к URL: %s", drive, cfg.WebDAVURL)

	// способ: передаём аргументы напрямую
	cmd := exec.Command("cmd", "/C", "start", "WebDAV Connect", "/min", "net", "use", drive, cfg.WebDAVURL, "/persistent:yes")

	err := cmd.Run()
	if err != nil {
		logger.Errorf("Ошибка подключения диска: %v", err)
		return err
	}

	// Ждём немного, чтобы соединение установилось
	time.Sleep(2 * time.Second)

	// Проверяем, что диск действительно подключился
	if !isDriveMapped(drive) {
		logger.Error("Диск не подключился (возможно, отменено пользователем)")
		return fmt.Errorf("подключение не удалось")
	}

	logger.Infof("Диск %s успешно подключен", drive)

	if err := setDriveLabel(drive, label); err != nil {
		logger.Warnf("Не удалось установить метку диска '%s': %v", label, err)
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

// Открыть лог
func openLogs() {
	logger.Info("Открытие webdav-drive.log в редакторе")
	cmd := exec.Command("notepad", getLogPath())
	_ = cmd.Run()
}

func onExit() {
	logger.Info("Приложение завершено")
	os.Exit(0)
}

// Основная логика подключения — делегирует аутентификацию Windows
func connectWithLogging() bool {
	logger.Info("Попытка подключения диска...")

	rawURL := globalCfg.WebDAVURL
	cleanURL := strings.TrimSpace(rawURL)
	if cleanURL == "" {
		logger.Error("WebDAV URL не задан")
		return false
	}
	if !strings.HasSuffix(cleanURL, "/") {
		cleanURL += "/"
	}

	cfg := &Config{
		DriveLetter: globalCfg.DriveLetter,
		WebDAVURL:   cleanURL,
	}

	logger.Infof("Нормализованный URL: %s", cleanURL)

	if err := connectDrive(cfg); err != nil {
		logger.Errorf("Не удалось подключить диск: %v", err)
		return false
	}

	logger.Info("Диск успешно подключён")
	return true
}

// Проверка на уже запущенный экземпляр
func isAlreadyRunning() bool {
	mutexName, _ := windows.UTF16PtrFromString("WebDAVDrive_Mutex_" + os.Getenv("USERNAME"))
	_, err := windows.CreateMutex(nil, false, mutexName)
	return err != nil
}
