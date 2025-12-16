package main

import (
	"net/url"
	"os"
	"time"

	"github.com/getlantern/systray"
)

// Трей-режим
func runTrayMode() {
	connected := isDriveMapped(globalCfg.DriveLetter)
	if connected {
		logger.Info("Диск уже подключён при запуске")
	}

	// Фоновая проверка обновлений
	go func() {
		time.Sleep(24 * time.Hour)
		checkForUpdates()
	}()

	systray.Run(onReady, onExit)
}

// CLI-режим (для первоначальной настройки)
func runCLIMode() {
	if _, err := os.Stat(getConfigPath()); os.IsNotExist(err) {
		logger.Println("🆕 Первый запуск: создаю config.json...")
		if err = saveDefaultConfig(); err != nil {
			logger.Printf("❌ Ошибка создания конфига: %v\n", err)
			return
		}
		logger.Println("✅ config.json создан. Отредактируйте его и запустите снова.")
		return
	}

	cfg, err := loadConfig()
	if err != nil {
		logger.Printf("❌ Ошибка загрузки конфига: %v\n", err)
		return
	}

	if err = connectDrive(cfg); err == nil {
		logger.Println("✅ Диск подключён!")
		return
	}

	logger.Println("❌ Подключение не удалось. Введите логин/пароль.")
	//username := readInput("📧 Логин: ")
	//password := readInput("🔑 Пароль: ")
	u, err := url.Parse(cfg.WebDAVURL)
	var host string
	if err != nil {
		logger.Fatal(err)
	} else {
		host = u.Host
	}
	username, password, ok, err := promptCredentials(host)
	if err != nil || !ok {
		logger.Println("❌ Отменено пользователем или ошибка ввода")
		return
	}

	logger.Println("💾 Сохраняю в Windows Credential Manager...")
	if err = saveCredentials(cfg.WebDAVURL, username, password); err != nil {
		logger.Printf("❌ Ошибка сохранения: %v\n", err)
		return
	}

	logger.Println("🔁 Повторное подключение...")
	if err = connectDrive(cfg); err != nil {
		logger.Printf("❌ Ошибка подключения: %v\n", err)
		return
	}
	logger.Println("✅ Готово!")
}
