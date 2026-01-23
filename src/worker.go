// worker.go
package main

import (
	"os"
	"strings"
	"time"

	"github.com/getlantern/systray"
)

// Трей-режим — основной режим работы
func runTrayMode() {
	// Загружаем конфиг перед запуском трея
	cfg, err := loadConfig()
	if err != nil {
		logger.Fatalf("Не удалось загрузить config.json: %v", err)
	}
	globalCfg = cfg

	// Фоновая проверка обновлений
	go func() {
		time.Sleep(24 * time.Hour)
		checkForUpdates()
	}()

	systray.Run(onReady, onExit)
}

// CLI-режим — только для первоначальной настройки
func runCLIMode() {
	configPath := getConfigPath()

	// Если конфига нет — создаём шаблон
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		logger.Info("🆕 Первый запуск: создаю config.json...")
		if err := saveDefaultConfig(); err != nil {
			logger.Errorf("❌ Ошибка создания конфига: %v", err)
			return
		}
		logger.Info("✅ config.json создан. Отредактируйте его вручную и запустите приложение снова.")
		return
	}

	// Загружаем конфиг
	cfg, err := loadConfig()
	if err != nil {
		logger.Errorf("❌ Ошибка загрузки конфига: %v", err)
		return
	}

	// Нормализуем URL (гарантируем завершающий слэш)
	webdavURL := cfg.WebDAVURL
	if !strings.HasSuffix(webdavURL, "/") {
		webdavURL += "/"
	}
	cfg.WebDAVURL = webdavURL

	// Пробуем подключиться напрямую
	// Windows сам запросит учётные данные, если их нет
	logger.Info("Подключение к WebDAV...")
	if err := connectDrive(cfg); err != nil {
		logger.Errorf("❌ Подключение не удалось: %v", err)
		logger.Info("💡 Убедитесь, что:")
		logger.Info("   - URL заканчивается на /")
		logger.Info("   - Сервер доступен")
		logger.Info("   - Учётные данные сохранены в Windows Credential Manager")
		return
	}

	logger.Info("✅ Диск успешно подключён!")
}
