package main

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
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
	mSettings := systray.AddMenuItem("Настройки", "Редактировать config.json")
	mLogs := systray.AddMenuItem("Логи", "Посмотреть webdav-drive.log")
	mReset := systray.AddMenuItem("Сбросить пароль", "Удалить учётные данные")
	//mTest := systray.AddMenuItem("Тест подключения", "Тестирование подключения")
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

			//case <-mTest.ClickedCh:
			//	testConnection()

			case <-mExit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func testConnection() {
	fmt.Println("=== ТЕСТИРОВАНИЕ ПОДКЛЮЧЕНИЯ ===")

	// 1. Проверяем конфиг
	fmt.Println("Конфигурация:")
	fmt.Printf("  Буква диска: %s\n", globalCfg.DriveLetter)
	fmt.Printf("  URL: %s\n", globalCfg.WebDAVURL)

	// 2. Проверяем парсинг URL
	u, err := url.Parse(globalCfg.WebDAVURL)
	if err != nil {
		fmt.Printf("Ошибка парсинга URL: %v\n", err)
		return
	}
	fmt.Println("\nПарсинг URL:")
	fmt.Printf("  Схема: %s\n", u.Scheme)
	fmt.Printf("  Хост: %s\n", u.Host)
	fmt.Printf("  Путь: %s\n", u.Path)

	// 3. Существующие сетевые диски
	fmt.Println("\nСуществующие сетевые диски:")
	cmd := exec.Command("net", "use")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Ошибка выполнения 'net use': %v\n", err)
	} else {
		// Принудительно интерпретируем как UTF-8 (если возможно)
		fmt.Printf("%s\n", string(output))
	}

	// 4. Учетные данные в Windows Credential Manager
	fmt.Println("Проверка учетных данных в Windows Credential Manager:")
	// Запускаем через cmd /C с переключением кодовой страницы на UTF-8
	cmd = exec.Command("cmd", "/C", "chcp 65001 >nul && cmdkey /list")
	output, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Ошибка выполнения 'cmdkey /list': %v\n", err)
	} else {
		fmt.Printf("%s\n", string(output))
	}

	fmt.Println("=== КОНЕЦ ТЕСТИРОВАНИЯ ===")
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
		logger.Infof("Диск %s уже подключен", cfg.DriveLetter)
		return nil
	}

	logger.Infof("Подключение диска %s к URL: %s", cfg.DriveLetter, cfg.WebDAVURL)

	cmd := exec.Command("net", "use", cfg.DriveLetter, cfg.WebDAVURL, "/persistent:yes")

	// Захватываем stdout и stderr для отладки
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		logger.Errorf("Ошибка подключения диска:")
		logger.Errorf("  Команда: net use %s %s /persistent:yes", cfg.DriveLetter, cfg.WebDAVURL)
		logger.Errorf("  Stdout: %s", stdout.String())
		logger.Errorf("  Stderr: %s", stderr.String())
		logger.Errorf("  Ошибка: %v", err)
		return err
	}

	logger.Infof("Диск %s успешно подключен", cfg.DriveLetter)
	return nil
}

// Открыть диск в Проводнике
func openDriveInExplorer() {
	drive := globalCfg.DriveLetter
	logger.Infof("Открытие %s в Проводнике", drive)
	cmd := exec.Command("explorer", drive)
	_ = cmd.Run()
}

// Открыть config.json
//func openConfig() {
//	logger.Info("Открытие config.json в редакторе")
//	cmd := exec.Command("notepad", getConfigPath())
//	_ = cmd.Run()
//}

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

	// Проверяем, есть ли сохраненные учетные данные
	if hasCreds, username := checkExistingCredentials(globalCfg.WebDAVURL); hasCreds {
		logger.Infof("Найдены сохраненные учетные данные для пользователя: %s", username)
	}

	// Первая попытка (возможно, учётные данные уже есть)
	if err := connectDrive(globalCfg); err == nil {
		logger.Info("Диск успешно подключён")
		return true
	}

	// Если не удалось — запрашиваем учётные данные
	logger.Warn("Подключение не удалось. Запрос учётных данных...")

	u, err := url.Parse(globalCfg.WebDAVURL)
	if err != nil {
		logger.Errorf("Неверный URL: %v", err)
		return false
	}
	host := u.Host

	// Запрашиваем логин/пароль через GUI
	username, password, ok, err := promptCredentials(host)
	if err != nil || !ok {
		logger.Warn("Отменено пользователем или ошибка ввода")
		return false
	}

	// Пробуем подключиться с явными учетными данными
	logger.Info("Попытка подключения с явными учетными данными...")
	if err := connectDriveWithCredentials(globalCfg, username, password); err == nil {
		logger.Info("Диск успешно подключён с явными учетными данными")

		// Сохраняем учетные данные для будущего использования
		saveCredentials(globalCfg.WebDAVURL, username, password)
		return true
	}

	// Если не получилось с явными учетными данными, пробуем сохранить и подключиться стандартно
	logger.Info("Попытка сохранить учетные данные и подключиться...")
	if err = saveCredentials(globalCfg.WebDAVURL, username, password); err != nil {
		logger.Errorf("Не удалось сохранить учётные данные: %v", err)
		return false
	}

	// Повторная попытка подключения
	logger.Info("Повторная попытка подключения...")
	if err = connectDrive(globalCfg); err != nil {
		logger.Errorf("Ошибка подключения после ввода учётных данных: %v", err)

		// Пробуем альтернативные варианты
		altHosts := []string{
			host,
			"https://" + host,
			"http://" + host,
			strings.TrimPrefix(globalCfg.WebDAVURL, "https://"),
			strings.TrimPrefix(globalCfg.WebDAVURL, "http://"),
		}

		for _, altHost := range altHosts {
			if saveCredentials(altHost, username, password) == nil {
				logger.Infof("Учетные данные сохранены для: %s", altHost)
				if err = connectDrive(globalCfg); err == nil {
					logger.Info("Диск успешно подключён после сохранения для альтернативного хоста")
					return true
				}
			}
		}

		// Удаляем неверные учётные данные
		for _, altHost := range altHosts {
			deleteCredentials(altHost)
		}

		return false
	}

	logger.Info("Диск успешно подключён после ввода учётных данных")
	return true
}

func connectDriveWithCredentials(cfg *Config, username, password string) error {
	driveLetter := strings.TrimSpace(cfg.DriveLetter)
	if !strings.HasSuffix(driveLetter, ":") {
		driveLetter = driveLetter + ":"
	}
	driveLetter = strings.ToUpper(driveLetter)

	logger.Infof("Подключение диска %s с явными учетными данными", driveLetter)

	// Пробуем подключиться с явным указанием пароля через net use
	cmd := exec.Command("net", "use", driveLetter, cfg.WebDAVURL, password, "/user:"+username, "/persistent:yes")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		logger.Errorf("Ошибка подключения с явными учетными данными:")
		logger.Errorf("  Stdout: %s", stdout.String())
		logger.Errorf("  Stderr: %s", stderr.String())
		logger.Errorf("  Ошибка: %v", err)

		// Пробуем другой формат
		cmd2 := exec.Command("cmd", "/C",
			fmt.Sprintf("net use %s %s /user:%s /persistent:yes", driveLetter, cfg.WebDAVURL, username))

		// Пишем пароль в stdin
		stdin, _ := cmd2.StdinPipe()
		cmd2.Stdout = &stdout
		cmd2.Stderr = &stderr

		if err := cmd2.Start(); err == nil {
			io.WriteString(stdin, password+"\n")
			stdin.Close()
			err = cmd2.Wait()
		}

		if err != nil {
			return err
		}
	}

	logger.Infof("Диск %s успешно подключен с явными учетными данными", driveLetter)
	return nil
}

func resetWithLogging() {
	logger.Info("Сброс учётных данных")
	if err := deleteCredentials(globalCfg.WebDAVURL); err != nil {
		logger.Warnf("Ошибка сброса: %v", err)
	}
}

func checkExistingCredentials(target string) (bool, string) {
	// Проверяем, есть ли сохраненные учетные данные
	cmd := exec.Command("cmdkey", "/list")
	output, err := cmd.Output()
	if err != nil {
		return false, ""
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, target) {
			// Парсим имя пользователя
			if strings.Contains(line, "Пользователь:") || strings.Contains(line, "User:") {
				parts := strings.Split(line, ":")
				if len(parts) > 1 {
					username := strings.TrimSpace(parts[1])
					return true, username
				}
			}
		}
	}

	return false, ""
}
