package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"gopkg.in/yaml.v3"
)

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
		DriveLetter: "Z:",
		WebDAVURL:   "https://your-webdav-server.com/remote.php/dav/files/your-username",
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(getConfigPath(), data, 0600)
}
