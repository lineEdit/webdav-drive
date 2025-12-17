package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

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

// Загрузка конфига из JSON
func loadConfig() (*Config, error) {
	configPath := getConfigPath()

	// Если файла нет, создаем дефолтный
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := saveDefaultConfig(); err != nil {
			return nil, err
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		err := saveDefaultConfig()
		if err != nil {
			return nil, err
		}
	}

	// Чистим данные
	cfg.DriveLetter = strings.TrimSpace(cfg.DriveLetter)
	cfg.WebDAVURL = strings.TrimSpace(cfg.WebDAVURL)
	cfg.WebDAVURL = strings.TrimRight(cfg.WebDAVURL, "/")

	// Валидация
	if cfg.DriveLetter == "" {
		cfg.DriveLetter = "Z:"
	}

	return &cfg, nil
}

// Сохранение конфига в JSON
func saveConfig(cfg *Config) error {
	// Чистим данные перед сохранением
	cleanCfg := *cfg
	cleanCfg.DriveLetter = strings.TrimSpace(cleanCfg.DriveLetter)
	cleanCfg.WebDAVURL = strings.TrimSpace(cleanCfg.WebDAVURL)
	cleanCfg.WebDAVURL = strings.TrimRight(cleanCfg.WebDAVURL, "/")

	data, err := json.MarshalIndent(&cleanCfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(getConfigPath(), data, 0600)
}

// Сохранение дефолтного конфига
func saveDefaultConfig() error {
	cfg := Config{
		DriveLetter: "Z:",
		WebDAVURL:   "https://your-webdav-server.com/remote.php/dav/files/your-username",
	}
	return saveConfig(&cfg)
}
