package main

import (
	"bytes"
	"net/url"
	"os"
	"os/exec"

	"golang.org/x/sys/windows"
)

// Сохранение учётных данных
func saveCredentials(target, username, password string) error {
	logger.Infof("Сохранение учетных данных для: %s, пользователь: %s", target, username)

	cmd := exec.Command("cmdkey", "/generic:"+target, "/user:"+username, "/pass:"+password)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		logger.Errorf("Ошибка сохранения учетных данных:")
		logger.Errorf("  Команда: cmdkey /generic:%s /user:%s /pass:****", target, username)
		logger.Errorf("  Stdout: %s", stdout.String())
		logger.Errorf("  Stderr: %s", stderr.String())
		logger.Errorf("  Ошибка: %v", err)
		return err
	}

	logger.Infof("Учетные данные успешно сохранены в Windows Credential Manager")
	return nil
}

// Удаление учётных данных
func deleteCredentials(target string) error {
	// Удаляем по полному URL
	err := exec.Command("cmdkey", "/delete:"+target).Run()
	if err != nil {
		logger.Warning("Failed to delete credentials")
		return err
	}

	// Извлекаем домен
	u, err := url.Parse(target)
	if err == nil {
		domain := u.Host
		err = exec.Command("cmdkey", "/delete:"+domain).Run()
		if err != nil {
			logger.Warning("Failed to delete credentials")
			return err
		}
		err = exec.Command("cmdkey", "/delete:https://"+domain).Run()
		if err != nil {
			logger.Warning("Failed to delete credentials")
			return err
		}
	}
	return nil // игнорируем ошибки
}

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
