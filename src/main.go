// main.go
package main

import (
	_ "embed"
	"os"
	"syscall"

	"github.com/sirupsen/logrus"
)

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	user32               = syscall.NewLazyDLL("user32.dll")
	procGetConsoleWindow = kernel32.NewProc("GetConsoleWindow")
	procShowWindow       = user32.NewProc("ShowWindow")
)

func hideConsole() {
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd != 0 {
		_, _, err := procShowWindow.Call(hwnd, uintptr(0))
		if err != nil {
			return
		} // SW_HIDE = 0
	}
}

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
		os.Exit(0)
	}

	// Скрываем консоль (только если не в режиме отладки)
	if !isDebugMode() {
		hideConsole()
	}

	// Проверка: уже запущено?
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

	// Проверка обновлений при первом запуске
	if firstRun {
		logger.Info("Первый запуск: проверка обновлений...")
		checkForUpdates()
	}

	logger.Infof("Запуск WebDAV Drive %s", version)
	runTrayMode()
}

// Вспомогательная функция для отладки
func isDebugMode() bool {
	for _, arg := range os.Args {
		if arg == "--debug" || arg == "--log" {
			return true
		}
	}
	return false
}
