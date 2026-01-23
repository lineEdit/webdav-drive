package main

import (
	_ "embed"
	"os"

	"github.com/sirupsen/logrus"
)

var version = "0.0.0-dev"

//go:embed assets/icon-on.ico
var iconOn []byte

//go:embed assets/icon-off.ico
var iconOff []byte

var logger *logrus.Logger
var globalCfg *Config

func main() {
	// Обработка --test-startup ДО любой инициализации
	if len(os.Args) > 1 && os.Args[1] == "--test-startup" {
		// Только быстрый выход — без создания мьютекса, логов и т.д.
		os.Exit(0)
	}

	// Проверка на уже запущенный экземпляр
	if isAlreadyRunning() {
		os.Exit(1)
	}

	var (
		enableLog bool
		firstRun  bool
	)

	for _, arg := range os.Args[1:] {
		switch arg {
		case "--log", "-l":
			enableLog = true
		case "--first-run":
			firstRun = true
		}
	}

	initLogger(enableLog)

	// Первый запуск без конфига → CLI-режим
	if _, err := os.Stat(getConfigPath()); os.IsNotExist(err) {
		logger.Info("config.json не найден — запуск в CLI-режиме")
		runCLIMode()
		return
	}

	if firstRun {
		logger.Info("Первый запуск: проверка обновлений...")
		checkForUpdates()
	}

	logger.Infof("Запуск WebDAV Drive %s", version)
	runTrayMode()
}
