package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/getlantern/systray"
)

// Обработчик трея
func onReady() {
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
	if !connected {
		mOpen.Disable()
	}
	mSettings := systray.AddMenuItem("Настройки", "Редактировать config.yaml")
	mLogs := systray.AddMenuItem("Логи", "Посмотреть webdav-drive.log")
	mReset := systray.AddMenuItem("Сбросить пароль", "Удалить учётные данные")
	mExit := systray.AddMenuItem("Выход", "Завершить приложение")

	// Горутина обработки
	go func() {
		for {
			select {
			case <-mConnectEnable.ClickedCh:
				if connectWithLogging() {
					systray.SetIcon(iconOn)
					mOpen.Enable()
					// Переключаем: скрываем "Подключить", показываем "Отключить"
					mConnectEnable.Hide()
					mConnectDisable.Show()
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
					// Переключаем: скрываем "Отключить", показываем "Подключить"
					mConnectDisable.Hide()
					mConnectEnable.Show()
				}

			case <-mOpen.ClickedCh:
				openDriveInExplorer()

			case <-mSettings.ClickedCh:
				openConfig()

			case <-mLogs.ClickedCh:
				openLogs()

			case <-mReset.ClickedCh:
				resetWithLogging()
				systray.SetIcon(iconOff)
				mOpen.Disable()
				mConnectDisable.Hide()
				mConnectEnable.Show()

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

// Подключение диска
func connectDrive(cfg *Config) error {
	if isDriveMapped(cfg.DriveLetter) {
		return nil
	}
	cmd := exec.Command("net", "use", cfg.DriveLetter, cfg.WebDAVURL, "/persistent:yes")
	return cmd.Run()
}

// Открыть диск в Проводнике
func openDriveInExplorer() {
	drive := globalCfg.DriveLetter
	logger.Infof("Открытие %s в Проводнике", drive)
	cmd := exec.Command("explorer", drive)
	_ = cmd.Run()
}

// Открыть config.yaml
func openConfig() {
	logger.Info("Открытие config.yaml в редакторе")
	cmd := exec.Command("notepad", getConfigPath())
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
	if isDriveMapped(globalCfg.DriveLetter) {
		logger.Info("Диск уже подключён")
		return true
	}

	// Первая попытка (возможно, учётные данные уже есть)
	if err := connectDrive(globalCfg); err == nil {
		logger.Info("Диск успешно подключён")
		return true
	}

	return false

	// Если не удалось — запрашиваем учётные данные
	//logger.Warn("Подключение не удалось. Запрос учётных данных...")

	// Извлекаем хост из URL для отображения
	//u, err := url.Parse(globalCfg.WebDAVURL)
	//if err != nil {
	//	logger.Errorf("Неверный URL: %v", err)
	//	return false
	//}
	//host := u.Host

	// Запрашиваем логин/пароль через GUI
	//username, password, ok, err := promptCredentials(host)
	//if err != nil || !ok {
	//	logger.Warn("Отменено пользователем или ошибка ввода")
	//	return false
	//}

	// Сохраняем учётные данные в Windows
	//if err = saveCredentials(globalCfg.WebDAVURL, username, password); err != nil {
	//	logger.Errorf("Не удалось сохранить учётные данные: %v", err)
	//	return false
	//}

	// Повторная попытка подключения
	//logger.Info("Повторная попытка подключения...")
	//if err = connectDrive(globalCfg); err != nil {
	//	logger.Errorf("Ошибка подключения после ввода учётных данных: %v", err)
	//	// Опционально: удаляем неверные учётные данные
	//	err = deleteCredentials(globalCfg.WebDAVURL)
	//	if err != nil {
	//		logger.Warning("err = deleteCredentials(globalCfg.WebDAVURL) - error: %v", err)
	//		return false
	//	}
	//	return false
	//}

	//logger.Info("Диск успешно подключён после ввода учётных данных")
	//return true
}

func resetWithLogging() {
	logger.Info("Сброс учётных данных")
	if err := deleteCredentials(globalCfg.WebDAVURL); err != nil {
		logger.Warnf("Ошибка сброса: %v", err)
	}
}
