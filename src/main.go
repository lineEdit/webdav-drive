package main

import (
	"bufio"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/getlantern/systray"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
	"gopkg.in/natefinch/lumberjack.v2"
	"gopkg.in/yaml.v3"
)

// Версия устанавливается через -X main.version= при сборке
var version = "1.0.0-dev"

//go:embed assets/icon-on.ico
var iconOn []byte

//go:embed assets/icon-off.ico
var iconOff []byte

// Проверка: запущено ли уже приложение?
func isAlreadyRunning() bool {
	mutexName, _ := windows.UTF16PtrFromString("WebDAVDrive_Mutex_" + os.Getenv("USERNAME"))
	mutex, err := windows.CreateMutex(nil, false, mutexName)
	if err != nil {
		return true
	}
	// Не закрываем мьютекс — он живёт, пока живёт процесс
	// Но утечек нет: Windows закроет его при завершении процесса
	_ = mutex
	return false
}

type Config struct {
	DriveLetter string `yaml:"drive_letter"`
	WebDAVURL   string `yaml:"webdav_url"` // ← Исправлено: было WebDAVUD
}

var logger *logrus.Logger
var globalCfg *Config

// Получаем папку %LOCALAPPDATA%\WebDAV Drive
func getAppDataDir() string {
	appData := os.Getenv("LOCALAPPDATA")
	if appData == "" {
		appData = os.Getenv("APPDATA")
	}
	if appData == "" {
		if executable, err := os.Executable(); err == nil {
			appData = filepath.Dir(executable)
		} else {
			appData = "."
		}
	}
	appDir := filepath.Join(appData, "WebDAV Drive")
	if err := os.MkdirAll(appDir, 0700); err != nil {
		// Если не можем создать — работаем в текущей директории
		return "."
	}
	return appDir
}

func getConfigPath() string {
	return filepath.Join(getAppDataDir(), "config.yaml")
}

func getLogPath() string {
	return filepath.Join(getAppDataDir(), "webdav-drive.log")
}

// Логирование с ротацией
func initLogger(enableConsole bool) {
	logger = logrus.New()

	// Ротация логов: макс. размер 5 МБ, до 3 архивов, не сжимать
	logFile := &lumberjack.Logger{
		Filename:   getLogPath(),
		MaxSize:    5,     // мегабайт
		MaxBackups: 3,     // сколько архивных файлов хранить
		MaxAge:     30,    // дней хранения (0 = бесконечно)
		Compress:   false, // можно true для .gz
	}

	if enableConsole {
		// Вывод и в консоль, и в файл
		logger.SetOutput(io.MultiWriter(logFile, os.Stdout))
	} else {
		// Только в файл
		logger.SetOutput(logFile)
	}

	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

// Загрузка конфига
func loadConfig() (*Config, error) {
	data, err := os.ReadFile(getConfigPath())
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	// Удаляем пробелы и слэши в конце
	cfg.WebDAVURL = strings.TrimSpace(cfg.WebDAVURL)
	cfg.WebDAVURL = strings.TrimRight(cfg.WebDAVURL, "/")
	return &cfg, nil
}

// Сохранение дефолтного конфига
func saveDefaultConfig() error {
	cfg := Config{
		DriveLetter: "\"Z:\"",
		WebDAVURL:   "\"https://your-webdav-server.com/remote.php/dav/files/your-username\"",
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(getConfigPath(), data, 0600)
}

// Чтение с консоли
func readInput(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// Сохранение учётных данных
func saveCredentials(target, username, password string) error {
	cmd := exec.Command("cmdkey", "/generic:"+target, "/user:"+username, "/pass:"+password)
	return cmd.Run()
}

// Удаление учётных данных
func deleteCredentials(target string) error {
	cmd := exec.Command("cmdkey", "/delete:"+target)
	err := cmd.Run()
	// Игнорируем "учётная запись не найдена" (exit status 1)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 1 {
				logger.Debugf("Учётные данные для %s не найдены — пропуск", target)
				return nil
			}
		}
	}
	return err
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

	mConnect := systray.AddMenuItem("Подключить диск", "Подключить WebDAV как сетевой диск")
	mOpen := systray.AddMenuItem("Открыть диск", "Открыть в Проводнике")
	mSettings := systray.AddMenuItem("Настройки", "Редактировать config.yaml")
	mReset := systray.AddMenuItem("Сбросить пароль", "Удалить учётные данные")
	mExit := systray.AddMenuItem("Выход", "Завершить приложение")

	if !connected {
		mOpen.Disable()
	}

	go func() {
		for {
			select {
			case <-mConnect.ClickedCh:
				if connectWithLogging() {
					systray.SetIcon(iconOn)
					mOpen.Enable()
				}
			case <-mOpen.ClickedCh:
				openDriveInExplorer()
			case <-mSettings.ClickedCh:
				openConfig()
			case <-mReset.ClickedCh:
				resetWithLogging()
				systray.SetIcon(iconOff)
				mOpen.Disable()
			case <-mExit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
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
	err := connectDrive(globalCfg)
	if err != nil {
		logger.Errorf("Ошибка подключения: %v", err)
		return false
	}
	logger.Info("Диск успешно подключён")
	return true
}

func resetWithLogging() {
	logger.Info("Сброс учётных данных")
	if err := deleteCredentials(globalCfg.WebDAVURL); err != nil {
		logger.Warnf("Ошибка сброса: %v", err)
	}
}

// CLI-режим (для первоначальной настройки)
func runCLIMode() {
	if _, err := os.Stat(getConfigPath()); os.IsNotExist(err) {
		fmt.Println("🆕 Первый запуск: создаю config.yaml...")
		if err := saveDefaultConfig(); err != nil {
			fmt.Printf("❌ Ошибка создания конфига: %v\n", err)
			return
		}
		fmt.Println("✅ config.yaml создан. Отредактируйте его и запустите снова.")
		return
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("❌ Ошибка загрузки конфига: %v\n", err)
		return
	}

	if err := connectDrive(cfg); err == nil {
		fmt.Println("✅ Диск подключён!")
		return
	}

	fmt.Println("❌ Подключение не удалось. Введите логин/пароль.")
	username := readInput("📧 Логин: ")
	password := readInput("🔑 Пароль: ")

	fmt.Println("💾 Сохраняю в Windows Credential Manager...")
	if err := saveCredentials(cfg.WebDAVURL, username, password); err != nil {
		fmt.Printf("❌ Ошибка сохранения: %v\n", err)
		return
	}

	fmt.Println("🔁 Повторное подключение...")
	if err := connectDrive(cfg); err != nil {
		fmt.Printf("❌ Ошибка подключения: %v\n", err)
		return
	}
	fmt.Println("✅ Готово!")
}

// Трей-режим
func runTrayMode() {
	// Заглушка для автообновления (реализована в updater.go)
	checkForUpdates := func() {
		// Реализация будет подключена из updater.go
		// Если updater.go не подключен — ничего не делаем
	}

	connected := isDriveMapped(globalCfg.DriveLetter)
	if connected {
		logger.Info("Диск уже подключён при запуске")
	}

	// Фоновая проверка обновлений
	go func() {
		time.Sleep(5 * time.Second)
		checkForUpdates()
	}()

	systray.Run(onReady, onExit)
}

func main() {
	// Поддержка --test-startup (для отката)
	if len(os.Args) > 1 && os.Args[1] == "--test-startup" {
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}

	// Проверка: уже запущено?
	if isAlreadyRunning() {
		os.Exit(1)
	}

	var (
		enableLog bool
		firstRun  bool
		doReset   bool
	)

	for _, arg := range os.Args[1:] {
		switch arg {
		case "--log", "-l":
			enableLog = true
		case "--first-run":
			firstRun = true
		case "--reset", "-r":
			doReset = true
		}
	}

	initLogger(enableLog)

	// Сброс учётных данных
	if doReset {
		cfg, err := loadConfig()
		if err != nil {
			logger.Fatal("config.yaml не найден")
		}
		if err := deleteCredentials(cfg.WebDAVURL); err != nil {
			logger.Warnf("Ошибка сброса: %v", err)
		}
		logger.Info("Учётные данные сброшены.")
		return
	}

	// Первый запуск без конфига → CLI-режим
	if _, err := os.Stat(getConfigPath()); os.IsNotExist(err) {
		logger.Info("config.yaml не найден — запуск в CLI-режиме")
		runCLIMode()
		return
	}

	var err error
	globalCfg, err = loadConfig()
	if err != nil {
		logger.Fatalf("Ошибка загрузки конфига: %v", err)
	}

	// Проверка обновлений при первом запуске
	if firstRun {
		logger.Info("Первый запуск: проверка обновлений...")
		checkForUpdates()
	}

	logger.Infof("Запуск WebDAV Drive %s", version)
	runTrayMode()
}
