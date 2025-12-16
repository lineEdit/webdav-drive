package main

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procAllocConsole = kernel32.NewProc("AllocConsole")
	procFreeConsole  = kernel32.NewProc("FreeConsole")
)

// Сохранение учётных данных
func saveCredentials(target, username, password string) error {
	cmd := exec.Command("cmdkey", "/generic:"+target, "/user:"+username, "/pass:"+password)
	return cmd.Run()
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

func showConsole() {
	procAllocConsole.Call()
}

func hideConsole() {
	procFreeConsole.Call()
}

// Чтение с консоли
func readInput(prompt string) string {
	// Показываем консоль
	showConsole()
	defer hideConsole() // скрываем после ввода

	// Перенаправляем stdout/stderr в консоль
	syscall.Stdout = 1
	syscall.Stderr = 2
	syscall.Stdin = 0

	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}
