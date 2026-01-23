package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Config struct {
	DriveLetter string `json:"drive_letter"`
	WebDAVURL   string `json:"webdav_url"`
}

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
		return "."
	}
	return appDir
}

func getConfigPath() string {
	return filepath.Join(getAppDataDir(), "config.json")
}

func saveDefaultConfig() error {
	cfg := Config{
		DriveLetter: "N:",
		WebDAVURL:   "https://cloud.example.com/remote.php/dav/",
	}
	data, err := json.MarshalIndent(&cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(getConfigPath(), data, 0600)
}

func loadConfig() (*Config, error) {
	data, err := os.ReadFile(getConfigPath())
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func openConfig() {
	logger.Info("Открытие config.json в редакторе")
	configPath := getConfigPath()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := saveDefaultConfig(); err != nil {
			log.Fatal(err)
		}
		logger.Infof("Создан файл конфигурации: %s", configPath)
	}

	cmd := exec.Command("notepad", configPath)
	if err := cmd.Start(); err != nil {
		cmd = exec.Command("cmd", "/C", "start", "", configPath)
		_ = cmd.Start()
	}
	logger.Infof("Открыт файл: %s", configPath)
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
