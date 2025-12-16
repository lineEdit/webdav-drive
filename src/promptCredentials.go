package main

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

// Windows API константы и структуры
const (
	CREDUI_FLAGS_ALWAYS_SHOW_UI       = 0x00080000
	CREDUI_FLAGS_GENERIC_CREDENTIALS  = 0x00040000
	CREDUI_FLAGS_DO_NOT_PERSIST       = 0x00000002
	CREDUI_FLAGS_EXCLUDE_CERTIFICATES = 0x00000008
)

var (
	credUIDLL                  = syscall.NewLazyDLL("credui.dll")
	credUIPromptForCredentials = credUIDLL.NewProc("CredUIPromptForCredentialsW")
	creduiConfirmCredentials   = credUIDLL.NewProc("CredUIConfirmCredentialsW")
)

type CREDUI_INFO struct {
	cbSize         uint32
	hwndParent     uintptr
	pszMessageText *uint16
	pszCaptionText *uint16
	hbmBanner      uintptr
}

func promptCredentials(host string) (username, password string, ok bool, err error) {
	// Создаем буферы для результата
	resultChan := make(chan struct {
		username, password string
		ok                 bool
		err                error
	}, 1)

	// Запускаем в отдельной горутине, но синхронизируем с главным потоком
	go func() {
		const MAX_UNAME = 256
		const MAX_PWD = 256

		var unameBuf [MAX_UNAME]uint16
		var pwdBuf [MAX_PWD]uint16

		caption := "WebDAV Drive"
		message := "Введите учётные данные для доступа к WebDAV-серверу"

		var creduiInfo CREDUI_INFO
		creduiInfo.cbSize = uint32(unsafe.Sizeof(creduiInfo))

		// Преобразуем строки в UTF16
		captionPtr, _ := syscall.UTF16PtrFromString(caption)
		messagePtr, _ := syscall.UTF16PtrFromString(message)
		hostPtr, _ := syscall.UTF16PtrFromString(host)

		creduiInfo.pszCaptionText = captionPtr
		creduiInfo.pszMessageText = messagePtr

		flags := CREDUI_FLAGS_ALWAYS_SHOW_UI |
			CREDUI_FLAGS_GENERIC_CREDENTIALS |
			CREDUI_FLAGS_DO_NOT_PERSIST |
			CREDUI_FLAGS_EXCLUDE_CERTIFICATES

		var save uint32

		// Вызываем Windows API
		ret, _, _ := credUIPromptForCredentials.Call(
			uintptr(unsafe.Pointer(&creduiInfo)),
			uintptr(unsafe.Pointer(hostPtr)),
			0,
			0,
			uintptr(unsafe.Pointer(&unameBuf[0])),
			uintptr(MAX_UNAME),
			uintptr(unsafe.Pointer(&pwdBuf[0])),
			uintptr(MAX_PWD),
			uintptr(unsafe.Pointer(&save)),
			uintptr(flags),
		)

		if ret == 0 { // ERROR_SUCCESS
			username = syscall.UTF16ToString(unameBuf[:])
			password = syscall.UTF16ToString(pwdBuf[:])
			ok = username != "" && password != ""

			// Подтверждаем учетные данные
			if save != 0 && ok {
				creduiConfirmCredentials.Call(
					uintptr(unsafe.Pointer(hostPtr)),
					uintptr(1), // TRUE = подтвердить
				)
			}
		} else {
			ok = false
		}

		resultChan <- struct {
			username, password string
			ok                 bool
			err                error
		}{username, password, ok, nil}
	}()

	// Ждем результат с таймаутом
	select {
	case result := <-resultChan:
		return result.username, result.password, result.ok, result.err
	case <-time.After(30 * time.Second):
		return "", "", false, fmt.Errorf("таймаут ожидания ввода")
	}
}
