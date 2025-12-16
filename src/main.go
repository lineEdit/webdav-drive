package main

import (
	_ "embed"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

// Версия устанавливается через -X main.version= при сборке
var version = "1.0.0-dev"

//go:embed assets/icon-on.ico
var iconOn []byte

//go:embed assets/icon-off.ico
var iconOff []byte

type Config struct {
	DriveLetter string `yaml:"drive_letter"`
	WebDAVURL   string `yaml:"webdav_url"`
}

var logger *logrus.Logger
var globalCfg *Config

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
		if err = deleteCredentials(cfg.WebDAVURL); err != nil {
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
